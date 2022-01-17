/*
Copyright 2021 Juicedata Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mount

import (
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	k8sMount "k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type PodMount struct {
	k8sMount.SafeFormatAndMount
	K8sClient *k8sclient.K8sClient
}

func NewPodMount(client *k8sclient.K8sClient, mounter k8sMount.SafeFormatAndMount) MntInterface {
	return &PodMount{mounter, client}
}

func (p *PodMount) JMount(jfsSetting *jfsConfig.JfsSetting, volumeId, mountPath string, target string, options []string) error {
	return p.waitUntilMount(jfsSetting, volumeId, target, mountPath, p.getCommand(jfsSetting, mountPath, options))
}

func (p *PodMount) getCommand(jfsSetting *jfsConfig.JfsSetting, mountPath string, options []string) string {
	cmd := ""
	if jfsSetting.IsCe {
		klog.V(5).Infof("ceMount: mount %v at %v", jfsSetting.Source, mountPath)
		mountArgs := []string{jfsConfig.CeMountPath, jfsSetting.Source, mountPath}
		if !util.ContainsString(options, "metrics") {
			options = append(options, "metrics=0.0.0.0:9567")
		}
		mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
		cmd = strings.Join(mountArgs, " ")
	} else {
		klog.V(5).Infof("Mount: mount %v at %v", jfsSetting.Source, mountPath)
		mountArgs := []string{jfsConfig.JfsMountPath, jfsSetting.Source, mountPath}
		options = append(options, "foreground")
		if len(options) > 0 {
			mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
		}
		cmd = strings.Join(mountArgs, " ")
	}
	return cmd
}

func (p *PodMount) JUmount(volumeId, target string) error {
	podName := GeneratePodNameByVolumeId(volumeId)
	lock := jfsConfig.GetPodLock(podName)
	lock.Lock()
	defer lock.Unlock()

	// check mount pod is need to delete
	klog.V(5).Infof("JUmount: Delete target ref [%s] and check mount pod [%s] is need to delete or not.", target, podName)

	pod, err := p.K8sClient.GetPod(GeneratePodNameByVolumeId(volumeId), jfsConfig.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		klog.Errorf("JUmount: Get pod of volumeId %s err: %v", volumeId, err)
		return err
	}

	// if mount pod not exists.
	if pod == nil {
		klog.V(5).Infof("JUmount: Mount pod of volumeId %v not exists.", volumeId)
		return nil
	}

	key := util.GetReferenceKey(target)
	klog.V(6).Infof("JUmount: Target %v hash of target %v", target, key)

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		po, err := p.K8sClient.GetPod(pod.Name, pod.Namespace)
		if err != nil {
			return err
		}
		annotation := po.Annotations
		if _, ok := annotation[key]; !ok {
			klog.V(5).Infof("JUmount: Target ref [%s] in pod [%s] already not exists.", target, pod.Name)
			return nil
		}
		delete(annotation, key)
		po.Annotations = annotation
		return p.K8sClient.UpdatePod(po)
	})
	if err != nil {
		klog.Errorf("JUmount: Remove ref of volumeId %s err: %v", volumeId, err)
		return err
	}

	deleteMountPod := func(podName, namespace string) error {
		po, err := p.K8sClient.GetPod(podName, namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			klog.Errorf("JUmount: Get mount pod %s err %v", podName, err)
			return err
		}

		if hasRef(po) {
			klog.V(5).Infof("JUmount: pod %s still has juicefs- refs.", podName)
			return nil
		}

		klog.V(5).Infof("JUmount: pod %s has no juicefs- refs. delete it.", podName)
		if err := p.K8sClient.DeletePod(po); err != nil {
			klog.V(5).Infof("JUmount: Delete pod of volumeId %s error: %v", volumeId, err)
			return err
		}
		return nil
	}

	newPod, err := p.K8sClient.GetPod(pod.Name, pod.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("JUmount: Get mount pod %s err %v", podName, err)
		return err
	}
	if hasRef(newPod) {
		return nil
	}
	// if pod annotations has no "juicefs-" prefix, delete pod
	return deleteMountPod(pod.Name, pod.Namespace)
}

func (p *PodMount) waitUntilMount(jfsSetting *jfsConfig.JfsSetting, volumeId, target, mountPath, cmd string) error {
	podName := GeneratePodNameByVolumeId(volumeId)
	lock := jfsConfig.GetPodLock(podName)
	lock.Lock()
	defer lock.Unlock()
	klog.V(5).Infof("waitUtilMount: Mount pod cmd: %v", cmd)
	podResource := corev1.ResourceRequirements{}
	config := make(map[string]string)
	env := make(map[string]string)
	labels := make(map[string]string)
	anno := make(map[string]string)
	var serviceAccount string
	if jfsSetting != nil {
		podResource = parsePodResources(
			jfsSetting.MountPodCpuLimit,
			jfsSetting.MountPodMemLimit,
			jfsSetting.MountPodCpuRequest,
			jfsSetting.MountPodMemRequest,
		)
		config = jfsSetting.Configs
		env = jfsSetting.Envs
		labels = jfsSetting.MountPodLabels
		anno = jfsSetting.MountPodAnnotations
		serviceAccount = jfsSetting.MountPodServiceAccount
		if serviceAccount == "" {
			serviceAccount = jfsConfig.PodServiceAccountName
		}
	}

	key := util.GetReferenceKey(target)
	needCreate := false
	isDeletedErr := true
	for i := 0; i < 120; i++ {
		// wait for old pod deleted
		if oldPod, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace); err == nil && oldPod.DeletionTimestamp != nil {
			klog.V(6).Infof("waitUtilMount: wait for old mount pod deleted.")
			time.Sleep(time.Millisecond * 500)
			continue
		} else if err != nil {
			if k8serrors.IsNotFound(err) {
				needCreate = true
				isDeletedErr = false
				klog.V(5).Infof("waitUtilMount: Need to create pod %s.", podName)
				newPod := NewMountPod(podName, cmd, mountPath, podResource, config, env, labels, anno, serviceAccount)
				if newPod.Annotations == nil {
					newPod.Annotations = make(map[string]string)
				}
				newPod.Annotations[key] = target
				if _, err = p.K8sClient.CreatePod(newPod); err != nil {
					klog.Errorf("Create pod %s err and retry: %v", podName, err)
					return err
				}
				break
			}
			klog.Errorf("Get pod %s err and retry: %v", podName, err)
			return err
		}
		isDeletedErr = false
		break
	}
	if isDeletedErr {
		return status.Errorf(codes.Internal, "Mount %v failed: mount pod %s has been deleting for 1 min", volumeId, podName)
	}

	// Wait until the mount pod is ready
	for i := 0; i < 60; i++ {
		pod, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
		if err != nil {
			return status.Errorf(codes.Internal, "waitUtilMount: Get pod %v failed: %v", volumeId, err)
		}
		if util.IsPodReady(pod) {
			klog.V(5).Infof("waitUtilMount: Pod %v is successful", volumeId)
			if !needCreate {
				// add volumeId ref
				return p.AddRefOfMount(target, podName)
			}
			return nil
		}
		time.Sleep(time.Millisecond * 500)
	}
	return status.Errorf(codes.Internal, "Mount %v failed: mount pod isn't ready in 30 seconds", volumeId)
}

func (p *PodMount) AddRefOfMount(target string, podName string) error {
	klog.V(5).Infof("addRefOfMount: Add target ref in mount pod. mount pod: [%s], target: [%s]", podName, target)
	// add volumeId ref in pod annotation
	key := util.GetReferenceKey(target)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		exist, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
		if err != nil {
			return err
		}
		if exist.DeletionTimestamp != nil {
			return status.Errorf(codes.Internal, "addRefOfMount: Mount pod [%s] has been deleted.", podName)
		}
		annotation := exist.Annotations
		if _, ok := annotation[key]; ok {
			klog.V(5).Infof("addRefOfMount: Target ref [%s] in pod [%s] already exists.", target, podName)
			return nil
		}
		if annotation == nil {
			annotation = make(map[string]string)
		}
		annotation[key] = target
		exist.Annotations = annotation
		return p.K8sClient.UpdatePod(exist)
	})
	if err != nil {
		klog.Errorf("addRefOfMount: Add target ref in mount pod %s error: %v", podName, err)
		return err
	}
	return nil
}
