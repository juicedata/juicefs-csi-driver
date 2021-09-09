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
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	k8sMount "k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type PodMount struct {
	k8sMount.SafeFormatAndMount
	jfsSetting *jfsConfig.JfsSetting
	K8sClient  k8sclient.K8sClient
}

func NewPodMount(setting *jfsConfig.JfsSetting, client k8sclient.K8sClient) Interface {
	mounter := &k8sMount.SafeFormatAndMount{
		Interface: k8sMount.New(""),
		Exec:      k8sexec.New(),
	}
	return &PodMount{*mounter, setting, client}
}

func (p *PodMount) JMount(storage, volumeId, mountPath string, target string, options []string) error {
	cmd := ""
	if p.jfsSetting.IsCe {
		klog.V(5).Infof("ceMount: mount %v at %v", p.jfsSetting.Source, mountPath)
		mountArgs := []string{jfsConfig.CeMountPath, p.jfsSetting.Source, mountPath}
		options = append(options, "metrics=0.0.0.0:9567")
		mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
		cmd = strings.Join(mountArgs, " ")
	} else {
		klog.V(5).Infof("Mount: mount %v at %v", p.jfsSetting.Source, mountPath)
		mountArgs := []string{jfsConfig.JfsMountPath, p.jfsSetting.Source, mountPath}
		options = append(options, "foreground")
		if len(options) > 0 {
			mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
		}
		cmd = strings.Join(mountArgs, " ")
	}

	return p.waitUntilMount(volumeId, target, mountPath, cmd)
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

	key := getReferenceKey(target)
	klog.V(5).Infof("DeleteRefOfMountPod: Target %v hash of target %v", target, key)

loop:
	err = func() error {
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
	}()
	if err != nil && k8serrors.IsConflict(err) {
		// if can't update pod because of conflict, retry
		klog.V(5).Infof("DeleteRefOfMountPod: Update pod conflict, retry.")
		goto loop
	} else if err != nil {
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

func (p *PodMount) waitUntilMount(volumeId, target, mountPath, cmd string) error {
	podName := GeneratePodNameByVolumeId(volumeId)
	klog.V(5).Infof("waitUtilMount: Mount pod cmd: %v", cmd)
	podResource := corev1.ResourceRequirements{}
	config := make(map[string]string)
	env := make(map[string]string)
	if p.jfsSetting != nil {
		podResource = parsePodResources(
			p.jfsSetting.MountPodCpuLimit,
			p.jfsSetting.MountPodMemLimit,
			p.jfsSetting.MountPodCpuRequest,
			p.jfsSetting.MountPodMemRequest,
		)
		config = p.jfsSetting.Configs
		env = p.jfsSetting.Envs
	}

	key := getReferenceKey(target)
	po, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
	if err != nil && k8serrors.IsNotFound(err) {
		// need create
		klog.V(5).Infof("waitUtilMount: Need to create pod %s.", podName)
		newPod := NewMountPod(podName, cmd, mountPath, podResource, config, env)
		if newPod.Annotations == nil {
			newPod.Annotations = make(map[string]string)
		}
		newPod.Annotations[key] = target
		if _, e := p.K8sClient.CreatePod(newPod); e != nil && k8serrors.IsAlreadyExists(e) {
			// add ref of pod when pod exists
			klog.V(5).Infof("waitUtilMount: Pod %s already exist.", podName)
			exist, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
			if err != nil {
				return err
			}
			if exist.DeletionTimestamp != nil {
				return fmt.Errorf(fmt.Sprintf("waitUtilMount: Pod %s has been deleted.", podName))
			}
			klog.V(5).Infof("waitUtilMount: add mount ref in pod of volumeId %q", volumeId)
			if err = p.AddRefOfMount(target, podName); err != nil {
				return err
			}
		} else if e != nil {
			return e
		}
	} else if err != nil {
		return err
	} else if po.DeletionTimestamp != nil {
		return fmt.Errorf(fmt.Sprintf("waitUtilMount: Pod %s has been deleted.", podName))
	}

	// create pod successfully
	// Wait until the mount pod is ready
	for i := 0; i < 30; i++ {
		pod, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
		if err != nil {
			return status.Errorf(codes.Internal, "waitUtilMount: Get pod %v failed: %v", volumeId, err)
		}
		if util.IsPodReady(pod) {
			klog.V(5).Infof("waitUtilMount: Pod %v is successful", volumeId)
			// add volumeId ref in configMap
			klog.V(5).Infof("waitUtilMount: add mount ref in pod of volumeId %q", volumeId)
			return p.AddRefOfMount(target, podName)
		} else if util.IsPodResourceError(pod) {
			klog.V(5).Infof("waitUtilMount: Pod is failed because of resource.")
			if !util.IsPodHasResource(*pod) {
				return status.Errorf(codes.Internal, "Pod %v is failed", volumeId)
			}

			// if pod is failed because of resource, delete resource and deploy pod again.
			klog.V(5).Infof("waitUtilMount: Delete it and deploy again with no resource.")
			if err := p.K8sClient.DeletePod(pod); err != nil {
				return status.Errorf(codes.Internal, "Can't delete Pod %v", volumeId)
			}

			time.Sleep(time.Second * 5)
			newPod := NewMountPod(podName, cmd, mountPath, podResource, config, env)
			newPod.Annotations = pod.Annotations
			util.DeleteResourceOfPod(newPod)
			klog.V(5).Infof("waitUtilMount: Deploy again with no resource.")
			if _, err := p.K8sClient.CreatePod(newPod); err != nil {
				return status.Errorf(codes.Internal, "waitUtilMount: Can't create Pod %v", volumeId)
			}
		}
		time.Sleep(time.Millisecond * 500)
	}
	return status.Errorf(codes.Internal, "Mount %v failed: mount pod isn't ready in 15 seconds", volumeId)
}

func (p *PodMount) AddRefOfMount(target string, podName string) error {
	// add volumeId ref in pod annotation
	key := getReferenceKey(target)

loop:
	err := func() error {
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
		if err := p.K8sClient.UpdatePod(exist); err != nil && k8serrors.IsConflict(err) {
			klog.V(5).Infof("addRefOfMount: Patch pod %s error: %v", podName, err)
			return err
		}
		return nil
	}()
	if err != nil && k8serrors.IsConflict(err) {
		// if can't update pod because of conflict, retry
		klog.V(5).Infof("DeleteRefOfMountPod: Update pod conflict, retry.")
		goto loop
	} else if err != nil {
		return err
	}
	return nil
}

func getReferenceKey(target string) string {
	h := sha256.New()
	h.Write([]byte(target))
	return fmt.Sprintf("juicefs-%x", h.Sum(nil))[:63]
}
