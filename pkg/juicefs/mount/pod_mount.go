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
	"k8s.io/klog/v2"
	k8sexec "k8s.io/utils/exec"
	k8sMount "k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type PodMount struct {
	k8sMount.SafeFormatAndMount
	K8sClient *k8sclient.K8sClient
}

func NewPodMount(client *k8sclient.K8sClient) MntInterface {
	mounter := &k8sMount.SafeFormatAndMount{
		Interface: k8sMount.New(""),
		Exec:      k8sexec.New(),
	}
	return &PodMount{*mounter, client}
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
	// check mount pod is need to delete
	klog.V(5).Infof("DeleteRefOfMountPod: Check mount pod is need to delete or not.")

	pod, err := p.K8sClient.GetPod(GeneratePodNameByVolumeId(volumeId), jfsConfig.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		klog.V(5).Infof("DeleteRefOfMountPod: Get pod of volumeId %s err: %v", volumeId, err)
		return err
	}

	// if mount pod not exists.
	if pod == nil {
		klog.V(5).Infof("DeleteRefOfMountPod: Mount pod of volumeId %v not exists.", volumeId)
		return nil
	}

	klog.V(5).Infof("DeleteRefOfMountPod: Delete target ref [%s] in pod [%s].", target, pod.Name)

	key := util.GetReferenceKey(target)
	klog.V(5).Infof("DeleteRefOfMountPod: Target %v hash of target %v", target, key)

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		jfsConfig.JLock.Lock()
		defer jfsConfig.JLock.Unlock()

		po, err := p.K8sClient.GetPod(pod.Name, pod.Namespace)
		if err != nil {
			return err
		}
		annotation := po.Annotations
		if _, ok := annotation[key]; !ok {
			klog.V(5).Infof("DeleteRefOfMountPod: Target ref [%s] in pod [%s] already not exists.", target, pod.Name)
			return nil
		}
		delete(annotation, key)
		klog.V(5).Infof("DeleteRefOfMountPod: Remove ref of volumeId %v, target %v", volumeId, target)
		po.Annotations = annotation
		return p.K8sClient.UpdatePod(po)
	})
	if err != nil {
		return err
	}

	deleteMountPod := func(podName, namespace string) error {
		jfsConfig.JLock.Lock()
		defer jfsConfig.JLock.Unlock()

		po, err := p.K8sClient.GetPod(podName, namespace)
		if err != nil {
			return err
		}

		if hasRef(po) {
			klog.V(5).Infof("DeleteRefOfMountPod: pod still has juicefs- refs.")
			return nil
		}

		klog.V(5).Infof("DeleteRefOfMountPod: Pod of volumeId %v has not refs, delete it.", volumeId)
		if err := p.K8sClient.DeletePod(po); err != nil {
			klog.V(5).Infof("DeleteRefOfMountPod: Delete pod of volumeId %s error: %v", volumeId, err)
			return err
		}
		return nil
	}

	newPod, err := p.K8sClient.GetPod(pod.Name, pod.Namespace)
	if err != nil {
		return err
	}
	if hasRef(newPod) {
		klog.V(5).Infof("DeleteRefOfMountPod: pod still has juicefs- refs.")
		return nil
	}
	klog.V(5).Infof("DeleteRefOfMountPod: pod has no juicefs- refs.")
	// if pod annotations has no "juicefs-" prefix, delete pod
	return deleteMountPod(pod.Name, pod.Namespace)
}

func (p *PodMount) waitUntilMount(jfsSetting *jfsConfig.JfsSetting, volumeId, target, mountPath, cmd string) error {
	podName := GeneratePodNameByVolumeId(volumeId)
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
	for i := 0; i < 30; i++ {
		// wait for old pod deleted
		if oldPod, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace); err == nil && oldPod.DeletionTimestamp != nil {
			klog.Infof("pod still exists, wait to create")
			time.Sleep(time.Millisecond * 500)
		} else if err != nil {
			if k8serrors.IsNotFound(err) {
				needCreate = true
				break
			}
			klog.Errorf("get pod err:%v", err)
			return err
		}
	}

	if needCreate {
		klog.V(5).Infof("waitUtilMount: Need to create pod %s.", podName)
		newPod := NewMountPod(podName, cmd, mountPath, podResource, config, env, labels, anno, serviceAccount)
		if newPod.Annotations == nil {
			newPod.Annotations = make(map[string]string)
		}
		newPod.Annotations[key] = target
		if _, e := p.K8sClient.CreatePod(newPod); e != nil && k8serrors.IsAlreadyExists(e) {
			return e
		}
	}

	// Wait until the mount pod is ready
	for i := 0; i < 30; i++ {
		pod, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
		if err != nil {
			return status.Errorf(codes.Internal, "waitUtilMount: Get pod %v failed: %v", volumeId, err)
		}
		if util.IsPodReady(pod) {
			klog.V(5).Infof("waitUtilMount: Pod %v is successful", volumeId)
			if !needCreate {
				// add volumeId ref
				klog.V(5).Infof("waitUtilMount: add mount ref in pod of volumeId %q", volumeId)
				return p.AddRefOfMount(target, podName)
			}
			return nil
		}
		time.Sleep(time.Millisecond * 500)
	}
	return status.Errorf(codes.Internal, "Mount %v failed: mount pod isn't ready in 15 seconds", volumeId)
}

func (p *PodMount) AddRefOfMount(target string, podName string) error {
	// add volumeId ref in pod annotation
	key := util.GetReferenceKey(target)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		jfsConfig.JLock.Lock()
		defer jfsConfig.JLock.Unlock()

		exist, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
		if err != nil {
			return err
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
		klog.V(5).Infof("addRefOfMount: Add target ref in mount pod. mount pod: [%s], target: [%s]", podName, target)
		return p.K8sClient.UpdatePod(exist)
	})
	if err != nil {
		return err
	}
	return nil
}
