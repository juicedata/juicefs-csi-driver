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

package controller

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	podmount "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

const (
	defaultCheckoutTimeout   = 1 * time.Second
	defaultTargetMountCounts = 5
)

type PodDriver struct {
	Client   *k8sclient.K8sClient
	handlers map[podStatus]podHandler
	mit      mountInfoTable
	mount.SafeFormatAndMount
}

func NewPodDriver(client *k8sclient.K8sClient, mounter mount.SafeFormatAndMount) *PodDriver {
	return newPodDriver(client, mounter)
}

func newPodDriver(client *k8sclient.K8sClient, mounter mount.SafeFormatAndMount) *PodDriver {
	driver := &PodDriver{
		Client:             client,
		handlers:           map[podStatus]podHandler{},
		SafeFormatAndMount: mounter,
	}
	driver.handlers[podReady] = driver.podReadyHandler
	driver.handlers[podError] = driver.podErrorHandler
	driver.handlers[podPending] = driver.podPendingHandler
	driver.handlers[podDeleted] = driver.podDeletedHandler
	return driver
}

type podHandler func(ctx context.Context, pod *corev1.Pod) error
type podStatus string

const (
	podReady   podStatus = "podReady"
	podError   podStatus = "podError"
	podDeleted podStatus = "podDeleted"
	podPending podStatus = "podPending"
)

func (p *PodDriver) SetMountInfo(mit mountInfoTable) {
	p.mit = mit
}

func (p *PodDriver) Run(ctx context.Context, current *corev1.Pod) error {
	podStatus := getPodStatus(current)
	klog.V(6).Infof("[PodDriver] start handle pod %s/%s, status: %s", current.Namespace, current.Name, podStatus)
	// check refs in mount pod annotation first, delete ref that target pod is not found
	err := p.checkAnnotations(ctx, current)
	if err != nil {
		return err
	}

	if podStatus != podError && podStatus != podDeleted {
		return p.handlers[podStatus](ctx, current)
	}

	// resourceVersion of kubelet may be different from apiserver
	// so we need get latest pod resourceVersion from apiserver
	pod, err := p.Client.GetPod(ctx, current.Name, current.Namespace)
	if err != nil {
		return err
	}
	// set mount pod status in mit again, maybe deleted
	p.mit.setPodStatus(pod)
	return p.handlers[getPodStatus(pod)](ctx, pod)
}

// getPodStatus get pod status
func getPodStatus(pod *corev1.Pod) podStatus {
	if pod == nil {
		return podError
	}
	if pod.DeletionTimestamp != nil {
		return podDeleted
	}
	if util.IsPodError(pod) {
		return podError
	}
	if util.IsPodReady(pod) {
		return podReady
	}
	return podPending
}

// checkAnnotations
// 1. check refs in mount pod annotation
// 2. delete ref that target pod is not found
func (p *PodDriver) checkAnnotations(ctx context.Context, pod *corev1.Pod) error {
	// check refs in mount pod, the corresponding pod exists or not
	lock := config.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	delAnnotations := []string{}
	var existTargets int
	for k, target := range pod.Annotations {
		if k == util.GetReferenceKey(target) {
			targetUid := getPodUid(target)
			// Only it is not in pod lists can be seen as deleted
			_, exists := p.mit.deletedPods[targetUid]
			if !exists {
				// target pod is deleted
				klog.V(5).Infof("[PodDriver] get app pod %s deleted in annotations of mount pod, remove its ref.", targetUid)
				delAnnotations = append(delAnnotations, k)
				continue
			}
			existTargets++
		}
	}

	if existTargets != 0 && pod.Annotations[config.DeleteDelayAtKey] != "" {
		delAnnotations = append(delAnnotations, config.DeleteDelayAtKey)
	}
	if len(delAnnotations) != 0 {
		// check mount pod reference key, if it is not the latest, return conflict
		newPod, err := p.Client.GetPod(ctx, pod.Name, pod.Namespace)
		if err != nil {
			return err
		}
		if len(util.GetAllRefKeys(*newPod)) != len(util.GetAllRefKeys(*pod)) {
			return apierrors.NewConflict(schema.GroupResource{
				Group:    pod.GroupVersionKind().Group,
				Resource: pod.GroupVersionKind().Kind,
			}, pod.Name, fmt.Errorf("can not patch pod"))
		}
		if err := util.DelPodAnnotation(ctx, p.Client, pod, delAnnotations); err != nil {
			return err
		}
	}
	if existTargets == 0 && pod.DeletionTimestamp == nil {
		var shouldDelay bool
		shouldDelay, err := util.ShouldDelay(ctx, pod, p.Client)
		if err != nil {
			return err
		}
		if !shouldDelay {
			// check mount pod resourceVersion, if it is not the latest, return conflict
			newPod, err := p.Client.GetPod(ctx, pod.Name, pod.Namespace)
			if err != nil {
				return err
			}
			// check mount pod reference key, if it is not none, return conflict
			if len(util.GetAllRefKeys(*newPod)) != 0 {
				return apierrors.NewConflict(schema.GroupResource{
					Group:    pod.GroupVersionKind().Group,
					Resource: pod.GroupVersionKind().Kind,
				}, pod.Name, fmt.Errorf("can not delete pod"))
			}
			// if there are no refs or after delay time, delete it
			klog.V(5).Infof("There are no refs in pod %s annotation, delete it", pod.Name)
			if err := p.Client.DeletePod(ctx, pod); err != nil {
				klog.Errorf("Delete pod %s error: %v", pod.Name, err)
				return err
			}
			// delete related secret
			secretName := pod.Name + "-secret"
			klog.V(6).Infof("delete related secret of pod: %s", secretName)
			if err := p.Client.DeleteSecret(ctx, secretName, pod.Namespace); err != nil {
				klog.V(5).Infof("Delete secret %s error: %v", secretName, err)
			}
		}
	}
	return nil
}

// podErrorHandler handles mount pod error status
func (p *PodDriver) podErrorHandler(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		return nil
	}
	lock := config.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	// check resource err
	if util.IsPodResourceError(pod) {
		klog.V(5).Infof("[podErrorHandler]waitUtilMount: Pod %s failed because of resource.", pod.Name)
		if util.IsPodHasResource(*pod) {
			// if pod is failed because of resource, delete resource and deploy pod again.
			_ = util.RemoveFinalizer(ctx, p.Client, pod, config.Finalizer)
			klog.V(5).Infof("Delete it and deploy again with no resource.")
			if err := p.Client.DeletePod(ctx, pod); err != nil {
				klog.Errorf("delete po:%s err:%v", pod.Name, err)
				return nil
			}
			isDeleted := false
			// wait pod delete for 1min
			for {
				_, err := p.Client.GetPod(ctx, pod.Name, pod.Namespace)
				if err == nil {
					klog.V(6).Infof("pod %s %s still exists wait.", pod.Name, pod.Namespace)
					time.Sleep(time.Microsecond * 500)
					continue
				}
				if apierrors.IsNotFound(err) {
					isDeleted = true
					break
				}
				if apierrors.IsTimeout(err) {
					break
				}
				if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
					break
				}
				klog.Errorf("[podErrorHandler] get mountPod err:%v, podName: %s", err, pod.Name)
			}
			if !isDeleted {
				klog.Errorf("[podErrorHandler] Old pod %s %s deleting timeout", pod.Name, config.Namespace)
				return nil
			}
			var newPod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        pod.Name,
					Namespace:   pod.Namespace,
					Labels:      pod.Labels,
					Annotations: pod.Annotations,
				},
				Spec: pod.Spec,
			}
			controllerutil.AddFinalizer(newPod, config.Finalizer)
			util.DeleteResourceOfPod(newPod)
			_, err := p.Client.CreatePod(ctx, newPod)
			if err != nil {
				klog.Errorf("[podErrorHandler] create pod:%s err:%v", pod.Name, err)
			}
		} else {
			klog.V(5).Infof("mountPod PodResourceError, but pod no resource, do nothing.")
		}
	}
	klog.Errorf("[podErrorHandler]: Pod %s/%s with error: %s, %s", pod.Namespace, pod.Name, pod.Status.Reason, pod.Status.Message)
	return nil
}

// podDeletedHandler handles mount pod that will be deleted
func (p *PodDriver) podDeletedHandler(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		klog.Errorf("get nil pod")
		return nil
	}
	klog.V(5).Infof("Pod %s in namespace %s is to be deleted.", pod.Name, pod.Namespace)

	// pod with no finalizer
	if !util.ContainsString(pod.GetFinalizers(), config.Finalizer) {
		// do nothing
		return nil
	}

	// remove finalizer of pod
	if err := util.RemoveFinalizer(ctx, p.Client, pod, config.Finalizer); err != nil {
		klog.Errorf("[podDeletedHandler] remove pod %s finalizer err:%v", pod.Name, err)
		return err
	}

	// pod with resource error
	if util.IsPodResourceError(pod) {
		klog.V(6).Infof("The pod is PodResourceError, podDeletedHandler skip delete the pod:%s", pod.Name)
		return nil
	}

	// get mount point
	sourcePath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		klog.Error(err)
		return nil
	}

	// check if it needs to create new one
	klog.V(6).Infof("Annotations:%v", pod.Annotations)
	if pod.Annotations == nil {
		return nil
	}
	annotation := pod.Annotations
	existTargets := make(map[string]string)

	for k, v := range pod.Annotations {
		// annotation is checked in beginning, don't double-check here
		if k == util.GetReferenceKey(v) {
			existTargets[k] = v
		}
	}

	if len(existTargets) == 0 {
		// do not need to create new one, umount
		util.UmountPath(ctx, sourcePath)
		// clean mount point
		err = util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
			klog.V(5).Infof("Clean mount point : %s", sourcePath)
			return mount.CleanupMountPoint(sourcePath, p.SafeFormatAndMount.Interface, false)
		})
		if err != nil {
			klog.Errorf("[podDeletedHandler] Clean mount point %s error: %v, podName:%s", sourcePath, err, pod.Name)
		}
		// cleanup cache should always complete, don't set timeout
		go p.CleanUpCache(context.TODO(), pod)
		return nil
	}

	lock := config.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	// create
	klog.V(5).Infof("pod targetPath not empty, need create pod:%s", pod.Name)

	// check pod delete
	for {
		po, err := p.Client.GetPod(ctx, pod.Name, pod.Namespace)
		if err == nil && po.DeletionTimestamp != nil {
			klog.V(6).Infof("pod %s %s is being deleted, waiting", pod.Name, pod.Namespace)
			time.Sleep(time.Millisecond * 500)
			continue
		}
		if err != nil {
			if apierrors.IsTimeout(err) {
				break
			}
			if apierrors.IsNotFound(err) {
				// umount mount point before recreate mount pod
				err := util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
					exist, _ := mount.PathExists(sourcePath)
					if !exist {
						return fmt.Errorf("%s not exist", sourcePath)
					}
					return nil
				})
				if err == nil {
					klog.Infof("start to umount: %s", sourcePath)
					util.UmountPath(ctx, sourcePath)
				}
				// create pod
				var newPod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        pod.Name,
						Namespace:   pod.Namespace,
						Labels:      pod.Labels,
						Annotations: annotation,
					},
					Spec: pod.Spec,
				}
				controllerutil.AddFinalizer(newPod, config.Finalizer)
				klog.Infof("Need to create pod %s %s", pod.Name, pod.Namespace)
				if err := p.OverwirteMountPodResourcesWithPVC(ctx, newPod); err != nil {
					klog.Errorf("Overwrite mount pod resources with pvc error %v", err)
				}
				_, err = p.Client.CreatePod(ctx, newPod)
				if err != nil {
					klog.Errorf("[podDeletedHandler] Create pod:%s err:%v", pod.Name, err)
				}
				return nil
			}
			klog.Errorf("[podDeletedHandler] Get pod: %s err:%v", pod.Name, err)
			return nil
		}

		// pod is created elsewhere
		if po.Annotations == nil {
			po.Annotations = make(map[string]string)
		}
		for k, v := range existTargets {
			// add exist target in annotation
			po.Annotations[k] = v
		}
		if err := util.ReplacePodAnnotation(ctx, p.Client, pod, po.Annotations); err != nil {
			klog.Errorf("[podDeletedHandler] Update pod %s %s error: %v", po.Name, po.Namespace, err)
		}
		return err
	}
	err = fmt.Errorf("old pod %s %s deleting timeout", pod.Name, config.Namespace)
	klog.V(5).Infof(err.Error())
	return err
}

// podPendingHandler handles mount pod that is pending
func (p *PodDriver) podPendingHandler(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		return nil
	}
	lock := config.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	// check resource err
	if util.IsPodResourceError(pod) {
		klog.V(5).Infof("waitUtilMount: Pod %s failed because of resource.", pod.Name)
		if util.IsPodHasResource(*pod) {
			// if pod is failed because of resource, delete resource and deploy pod again.
			_ = util.RemoveFinalizer(ctx, p.Client, pod, config.Finalizer)
			klog.V(5).Infof("Delete it and deploy again with no resource.")
			if err := p.Client.DeletePod(ctx, pod); err != nil {
				klog.Errorf("delete po:%s err:%v", pod.Name, err)
				return nil
			}
			isDeleted := false
			// wait pod delete for 1min
			for {
				_, err := p.Client.GetPod(ctx, pod.Name, pod.Namespace)
				if err == nil {
					klog.V(6).Infof("pod %s %s still exists wait.", pod.Name, pod.Namespace)
					time.Sleep(time.Microsecond * 500)
					continue
				}
				if apierrors.IsNotFound(err) {
					isDeleted = true
					break
				}
				if apierrors.IsTimeout(err) {
					break
				}
				if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
					break
				}
				klog.Errorf("[podPendingHandler] get mountPod err:%v, podName: %s", err, pod.Name)
			}
			if !isDeleted {
				klog.Errorf("[podPendingHandler] Old pod %s %s deleting timeout", pod.Name, config.Namespace)
				return nil
			}
			var newPod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        pod.Name,
					Namespace:   pod.Namespace,
					Labels:      pod.Labels,
					Annotations: pod.Annotations,
				},
				Spec: pod.Spec,
			}
			controllerutil.AddFinalizer(newPod, config.Finalizer)
			util.DeleteResourceOfPod(newPod)
			_, err := p.Client.CreatePod(ctx, newPod)
			if err != nil {
				klog.Errorf("[podPendingHandler] create pod:%s err:%v", pod.Name, err)
			}
		} else {
			klog.V(5).Infof("mountPod PodResourceError, but pod no resource, do nothing.")
		}
	}
	return nil
}

// podReadyHandler handles mount pod that is ready
func (p *PodDriver) podReadyHandler(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		klog.Errorf("[podReadyHandler] get nil pod")
		return nil
	}

	if pod.Annotations == nil {
		return nil
	}
	// get mount point
	mntPath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		klog.Error(err)
		return nil
	}

	e := util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
		_, e := os.Stat(mntPath)
		return e
	})

	if e != nil {
		klog.Errorf("[podReadyHandler] stat mntPath: %s, podName: %s, err: %v, don't do recovery", mntPath, pod.Name, e)
		return nil
	}

	// recovery for each target
	for k, target := range pod.Annotations {
		if k == util.GetReferenceKey(target) {
			mi := p.mit.resolveTarget(target)
			if mi == nil {
				klog.Errorf("[podReadyHandler] pod %s target %s resolve fail", pod.Name, target)
				continue
			}

			p.recoverTarget(ctx, pod.Name, mntPath, mi.baseTarget, mi)
			for _, ti := range mi.subPathTarget {
				p.recoverTarget(ctx, pod.Name, mntPath, ti, mi)
			}
		}
	}

	return nil
}

// recoverTarget recovers target path
func (p *PodDriver) recoverTarget(ctx context.Context, podName, sourcePath string, ti *targetItem, mi *mountItem) {
	switch ti.status {
	case targetStatusNotExist:
		klog.Errorf("pod %s target %s not exists, item count:%d", podName, ti.target, ti.count)
		if ti.count > 0 {
			// target exist in /proc/self/mountinfo file
			// refer to this case: local target exist, but source which target binded has beed deleted
			// if target is for pod subpath (volumeMount.subpath), this will cause user pod delete failed, so we help kubelet umount it
			if mi.podDeleted {
				p.umountTarget(ti.target, ti.count)
			}
		}

	case targetStatusMounted:
		// normal, most likely happen
		klog.V(6).Infof("pod %s target %s is normal mounted", podName, ti.target)

	case targetStatusNotMount:
		klog.V(5).Infof("pod %s target %s is not mounted", podName, ti.target)

	case targetStatusCorrupt:
		if ti.inconsistent {
			// source paths (found in /proc/self/mountinfo) which target binded is inconsistent
			// some unexpected things happened
			klog.Errorf("pod %s target %s, source inconsistent", podName, ti.target)
			break
		}
		if mi.podDeleted {
			klog.V(6).Infof("pod %s target %s, user pod has been deleted, don't do recovery", podName, ti.target)
			break
		}
		// if not umountTarget, mountinfo file will increase unlimited
		// if we umount all the target items, `mountPropagation` will lose efficacy
		klog.V(5).Infof("umount pod %s target %s before recover and remain mount count %d", podName, ti.target, defaultTargetMountCounts)
		// avoid umount target all, it will cause pod to write files in disk.
		err := p.umountTargetUntilRemain(ctx, mi, ti.target, defaultTargetMountCounts)
		if err != nil {
			klog.Error(err)
			break
		}
		if ti.subpath != "" {
			sourcePath += "/" + ti.subpath
			_, err := os.Stat(sourcePath)
			if err != nil {
				klog.Errorf("pod %s target %s, stat volPath: %s err: %v, don't do recovery", podName, ti.target, sourcePath, err)
				break
			}
		}
		klog.V(5).Infof("pod %s target %s recover volPath:%s", podName, ti.target, sourcePath)
		mountOption := []string{"bind"}
		if err := p.Mount(sourcePath, ti.target, "none", mountOption); err != nil {
			klog.Errorf("exec cmd: mount -o bind %s %s err:%v", sourcePath, ti.target, err)
		}

	case targetStatusUnexpect:
		klog.Errorf("pod %s target %s reslove err:%v", podName, ti.target, ti.err)
	}
}

// umountTarget umount target path
func (p *PodDriver) umountTarget(target string, count int) {
	if count <= 0 {
		return
	}
	klog.V(5).Infof("umount target %d times", count)
	for i := 0; i < count; i++ {
		// ignore error
		p.Unmount(target)
	}
}

// umountTargetUntilRemain umount target path with remaining count
func (p *PodDriver) umountTargetUntilRemain(ctx context.Context, basemi *mountItem, target string, remainCount int) error {
	subCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	for {
		// parse mountinfo everytime before umount target
		mit := newMountInfoTable()
		if err := mit.parse(); err != nil {
			return fmt.Errorf("umountTargetWithRemain ParseMountInfo: %v", err)
		}

		mi := mit.resolveTarget(basemi.baseTarget.target)
		if mi == nil {
			return fmt.Errorf("pod target %s resolve fail", target)
		}
		count := mi.baseTarget.count
		if mi.baseTarget.target != target {
			for _, t := range mi.subPathTarget {
				if t.target == target {
					count = t.count
				}
			}
		}
		// return if target count in mountinfo is less than remainCount
		if count < remainCount {
			return nil
		}

		util.UmountPath(subCtx, target)
		select {
		case <-subCtx.Done():
			return fmt.Errorf("umountTargetWithRemain timeout")
		}
	}
}

// CleanUpCache clean up cache
func (p *PodDriver) CleanUpCache(ctx context.Context, pod *corev1.Pod) {
	if pod.Annotations[config.CleanCache] != "true" {
		return
	}
	uuid := pod.Annotations[config.JuiceFSUUID]
	uniqueId := pod.Annotations[config.UniqueId]
	if uuid == "" && uniqueId == "" {
		// no necessary info, return
		klog.Errorf("[CleanUpCache] Can't get uuid and uniqueId from pod %s annotation. skip cache clean.", pod.Name)
		return
	}

	// wait for pod deleted.
	isDeleted := false
	getCtx, getCancel := context.WithTimeout(ctx, 3*time.Minute)
	defer getCancel()
	for {
		if _, err := p.Client.GetPod(getCtx, pod.Name, pod.Namespace); err != nil {
			if apierrors.IsNotFound(err) {
				isDeleted = true
				break
			}
			if apierrors.IsTimeout(err) {
				break
			}
			klog.V(5).Infof("[CleanUpCache] Get pod %s error %v. Skip clean cache.", pod.Name, err)
			return
		}
		time.Sleep(time.Microsecond * 500)
	}

	if !isDeleted {
		klog.Errorf("[CleanUpCache] Mount pod %s not deleted in 3 min. Skip clean cache.", pod.Name)
		return
	}
	klog.V(5).Infof("[CleanUpCache] Cleanup cache of volume %s in node %s", uniqueId, config.NodeName)
	podMnt := podmount.NewPodMount(p.Client, p.SafeFormatAndMount)
	cacheDirs := []string{}
	for _, dir := range pod.Spec.Volumes {
		if strings.HasPrefix(dir.Name, "cachedir-") && dir.HostPath != nil {
			cacheDirs = append(cacheDirs, dir.HostPath.Path)
		}
	}
	image := pod.Spec.Containers[0].Image
	if err := podMnt.CleanCache(ctx, image, uuid, uniqueId, cacheDirs); err != nil {
		klog.V(5).Infof("[CleanUpCache] Cleanup cache of volume %s error %v", uniqueId, err)
	}
}

func (p *PodDriver) OverwirteMountPodResourcesWithPVC(ctx context.Context, pod *corev1.Pod) error {
	pvName := pod.Annotations[config.UniqueId]
	pv, err := p.Client.GetPersistentVolume(ctx, pvName)
	if err != nil {
		klog.Errorf("Get pv %s error: %v", pvName, err)
		return err
	}
	pvc, err := p.Client.GetPersistentVolumeClaim(ctx, pv.Spec.ClaimRef.Name, pv.Spec.ClaimRef.Namespace)
	if err != nil {
		klog.Errorf("Get pvc %s/%s error: %v", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name, err)
		return err
	}

	cpuLimit := pvc.Annotations[config.MountPodCpuLimitKey]
	memoryLimit := pvc.Annotations[config.MountPodMemLimitKey]
	cpuRequest := pvc.Annotations[config.MountPodCpuRequestKey]
	memoryRequest := pvc.Annotations[config.MountPodMemRequestKey]

	resources, err := config.ParsePodResources(cpuLimit, memoryLimit, cpuRequest, memoryRequest)
	if err != nil {
		return fmt.Errorf("parse pvc resources error: %v", err)
	}
	pod.Spec.Containers[0].Resources = resources
	return nil
}
