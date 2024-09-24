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
	"path"
	"runtime"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	podmount "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

const (
	defaultCheckoutTimeout   = 1 * time.Second
	defaultTargetMountCounts = 5
)

var (
	podDriverLog = klog.NewKlogr().WithName("pod-driver")
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
	driver.handlers[podComplete] = driver.podCompleteHandler
	return driver
}

type podHandler func(ctx context.Context, pod *corev1.Pod) (Result, error)
type podStatus string
type Result struct {
	RequeueImmediately bool
	RequeueAfter       time.Duration
}

const (
	podReady    podStatus = "podReady"
	podError    podStatus = "podError"
	podDeleted  podStatus = "podDeleted"
	podPending  podStatus = "podPending"
	podComplete podStatus = "podComplete"
)

func (p *PodDriver) SetMountInfo(mit mountInfoTable) {
	p.mit = mit
}

func (p *PodDriver) Run(ctx context.Context, current *corev1.Pod) (Result, error) {
	if current == nil {
		return Result{}, nil
	}
	log := klog.NewKlogr().WithName("pod-driver").WithValues("podName", current.Name)
	ctx = util.WithLog(ctx, log)
	ps := getPodStatus(current)
	log.V(1).Info("start handle pod", "namespace", current.Namespace, "status", ps)
	// check refs in mount pod annotation first, delete ref that target pod is not found
	err := p.checkAnnotations(ctx, current)
	if err != nil {
		return Result{}, err
	}

	if ps != podError && ps != podDeleted {
		return p.handlers[ps](ctx, current)
	}

	// resourceVersion of kubelet may be different from apiserver
	// so we need get latest pod resourceVersion from apiserver
	pod, err := p.Client.GetPod(ctx, current.Name, current.Namespace)
	if err != nil {
		return Result{}, err
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
	if resource.IsPodComplete(pod) {
		return podComplete
	}
	if resource.IsPodError(pod) {
		return podError
	}
	if resource.IsPodReady(pod) {
		return podReady
	}
	return podPending
}

// checkAnnotations
// 1. check refs in mount pod annotation
// 2. delete ref that target pod is not found
func (p *PodDriver) checkAnnotations(ctx context.Context, pod *corev1.Pod) error {
	log := util.GenLog(ctx, podDriverLog, "")
	// check refs in mount pod, the corresponding pod exists or not
	hashVal := pod.Labels[common.PodJuiceHashLabelKey]
	if hashVal == "" {
		return fmt.Errorf("pod %s/%s has no hash label", pod.Namespace, pod.Name)
	}
	lock := config.GetPodLock(hashVal)
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
				log.Info("get app pod deleted in annotations of mount pod, remove its ref.", "appId", targetUid)
				delAnnotations = append(delAnnotations, k)
				continue
			}
			existTargets++
		}
	}

	if existTargets != 0 && pod.Annotations[common.DeleteDelayAtKey] != "" {
		delAnnotations = append(delAnnotations, common.DeleteDelayAtKey)
	}
	if len(delAnnotations) != 0 {
		// check mount pod reference key, if it is not the latest, return conflict
		newPod, err := p.Client.GetPod(ctx, pod.Name, pod.Namespace)
		if err != nil {
			return err
		}
		if len(resource.GetAllRefKeys(*newPod)) != len(resource.GetAllRefKeys(*pod)) {
			return apierrors.NewConflict(schema.GroupResource{
				Group:    pod.GroupVersionKind().Group,
				Resource: pod.GroupVersionKind().Kind,
			}, pod.Name, fmt.Errorf("can not patch pod"))
		}
		if err := resource.DelPodAnnotation(ctx, p.Client, pod, delAnnotations); err != nil {
			return err
		}
	}
	if existTargets == 0 && pod.DeletionTimestamp == nil {
		var shouldDelay bool
		shouldDelay, err := resource.ShouldDelay(ctx, pod, p.Client)
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
			if len(resource.GetAllRefKeys(*newPod)) != 0 {
				return apierrors.NewConflict(schema.GroupResource{
					Group:    pod.GroupVersionKind().Group,
					Resource: pod.GroupVersionKind().Kind,
				}, pod.Name, fmt.Errorf("can not delete pod"))
			}
			// if there are no refs or after delay time, delete it
			log.Info("There are no refs in pod annotation, delete it")
			if err := p.Client.DeletePod(ctx, pod); err != nil {
				log.Error(err, "Delete pod")
				return err
			}
			// for old version
			// delete related secret
			secretName := pod.Name + "-secret"
			log.V(1).Info("delete related secret of pod", "secretName", secretName)
			if err := p.Client.DeleteSecret(ctx, secretName, pod.Namespace); !apierrors.IsNotFound(err) && err != nil {
				log.V(1).Info("Delete secret error", "secretName", secretName)
			}
		}
	}
	return nil
}

func (p *PodDriver) podCompleteHandler(ctx context.Context, pod *corev1.Pod) (Result, error) {
	if pod == nil {
		return Result{}, nil
	}
	log := util.GenLog(ctx, podDriverLog, "podCompleteHandler")
	hashVal := pod.Labels[common.PodJuiceHashLabelKey]
	if hashVal == "" {
		return Result{}, fmt.Errorf("pod %s/%s has no hash label", pod.Namespace, pod.Name)
	}
	lock := config.GetPodLock(hashVal)
	lock.Lock()
	defer lock.Unlock()

	needCreate, err := p.needCreateMountPod(ctx, pod.Labels[common.PodUniqueIdLabelKey], hashVal)
	if err != nil {
		return Result{}, err
	}
	if !needCreate {
		return Result{}, nil
	}
	newPodName := podmount.GenPodNameByUniqueId(pod.Labels[common.PodUniqueIdLabelKey], true)
	log.Info("need to create a new one", "newPodName", newPodName)
	newPod, err := p.newMountPod(ctx, pod, newPodName)
	if err != nil {
		return Result{}, err
	}
	// get sid
	sid := passfd.GlobalFds.GetSid(hashVal)
	if sid != 0 {
		env := []corev1.EnvVar{}
		oldEnv := newPod.Spec.Containers[0].Env
		for _, v := range oldEnv {
			if v.Name != "_JFS_META_SID" {
				env = append(env, v)
			}
		}
		env = append(env, corev1.EnvVar{
			Name:  "_JFS_META_SID",
			Value: fmt.Sprintf("%d", sid),
		})
		newPod.Spec.Containers[0].Env = env
	}

	_, err = p.Client.CreatePod(ctx, newPod)
	if err != nil {
		log.Error(err, "Create pod")
		return Result{}, err
	}

	// delete the old one
	log.Info("delete the old complete mount pod")
	err = p.Client.DeletePod(ctx, pod)
	return Result{}, err
}

// podErrorHandler handles mount pod error status
func (p *PodDriver) podErrorHandler(ctx context.Context, pod *corev1.Pod) (Result, error) {
	if pod == nil {
		return Result{}, nil
	}
	log := util.GenLog(ctx, podDriverLog, "podErrorHandler")
	hashVal := pod.Labels[common.PodJuiceHashLabelKey]
	if hashVal == "" {
		return Result{}, fmt.Errorf("pod %s/%s has no hash label", pod.Namespace, pod.Name)
	}
	lock := config.GetPodLock(hashVal)
	lock.Lock()
	defer lock.Unlock()

	// check resource err
	if resource.IsPodResourceError(pod) {
		log.Info("Pod failed because of resource.")
		if resource.IsPodHasResource(*pod) {
			// if pod is failed because of resource, delete resource and deploy pod again.
			_ = resource.RemoveFinalizer(ctx, p.Client, pod, common.Finalizer)
			log.Info("Delete it and deploy again with no resource.")
			if err := p.Client.DeletePod(ctx, pod); err != nil {
				log.Error(err, "delete pod err")
				return Result{}, nil
			}
			isDeleted := false
			// wait pod delete for 1min
			for {
				_, err := p.Client.GetPod(ctx, pod.Name, pod.Namespace)
				if err == nil {
					log.V(1).Info("pod still exists wait.")
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
				log.Error(err, "get mountPod err")
			}
			if !isDeleted {
				log.Info("Old pod deleting timeout")
				return Result{RequeueImmediately: true}, nil
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
			controllerutil.AddFinalizer(newPod, common.Finalizer)
			resource.DeleteResourceOfPod(newPod)
			err := mkrMp(ctx, *newPod)
			if err != nil {
				log.Error(err, "mkdir mount point of pod")
			}
			_, err = p.Client.CreatePod(ctx, newPod)
			if err != nil {
				log.Error(err, "create pod")
			}
		} else {
			log.Info("mountPod PodResourceError, but pod no resource, do nothing.")
		}
	}
	log.Info("Pod is error", "reason", pod.Status.Reason, "message", pod.Status.Message)
	return Result{}, nil
}

// podDeletedHandler handles mount pod that will be deleted
func (p *PodDriver) podDeletedHandler(ctx context.Context, pod *corev1.Pod) (Result, error) {
	if pod == nil {
		podDriverLog.Info("get nil pod")
		return Result{}, nil
	}
	log := util.GenLog(ctx, podDriverLog, "podDeletedHandler")
	log.Info("Pod is to be deleted.")

	// pod with no finalizer
	if !util.ContainsString(pod.GetFinalizers(), common.Finalizer) {
		log.V(1).Info("Pod has no finalizer, skip deleting")
		// do nothing
		return Result{}, nil
	}

	// remove finalizer of pod
	if err := resource.RemoveFinalizer(ctx, p.Client, pod, common.Finalizer); err != nil {
		log.Error(err, "remove pod finalizer error")
		return Result{}, err
	}

	go p.checkMountPodStuck(pod)

	// pod with resource error
	if resource.IsPodResourceError(pod) {
		log.V(1).Info("The pod is PodResourceError, skip delete the pod")
		return Result{}, nil
	}

	// get mount point
	sourcePath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		log.Error(err, "get mount point error")
		return Result{}, err
	}

	// check if it needs to create new one
	log.V(1).Info("Get pod annotations", "annotations", pod.Annotations)
	if pod.Annotations == nil {
		return Result{}, nil
	}
	existTargets := make(map[string]string)

	hashVal := pod.Labels[common.PodJuiceHashLabelKey]
	if hashVal == "" {
		return Result{}, fmt.Errorf("pod %s/%s has no hash label", pod.Namespace, pod.Name)
	}

	for k, v := range pod.Annotations {
		// annotation is checked in beginning, don't double-check here
		if k == util.GetReferenceKey(v) {
			existTargets[k] = v
		}
	}

	if len(existTargets) == 0 {
		// do not need to create new one, umount
		_ = util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
			util.UmountPath(ctx, sourcePath)
			return nil
		})
		// clean mount point
		err = util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
			log.Info("Clean mount point", "mountPath", sourcePath)
			return mount.CleanupMountPoint(sourcePath, p.SafeFormatAndMount.Interface, false)
		})
		if err != nil {
			log.Error(err, "Clean mount point error", "mountPath", sourcePath)
		}
		// cleanup cache should always complete, don't set timeout
		go p.CleanUpCache(context.TODO(), pod)
		// stop fuse fd and clean up socket
		go passfd.GlobalFds.StopFd(context.TODO(), hashVal)
		return Result{}, nil
	}

	lock := config.GetPodLock(hashVal)
	lock.Lock()
	defer lock.Unlock()

	// create
	log.Info("pod targetPath not empty, need to create pod")

	// check pod delete
	_, err = p.Client.GetPod(ctx, pod.Name, pod.Namespace)
	if err == nil || apierrors.IsNotFound(err) {
		needCreate, err := p.needCreateMountPod(ctx, pod.Labels[common.PodUniqueIdLabelKey], hashVal)
		if err != nil {
			return Result{}, err
		}
		if needCreate {
			// create pod
			newPodName := podmount.GenPodNameByUniqueId(pod.Labels[common.PodUniqueIdLabelKey], true)
			log.Info("need to create a new one", "newPodName", newPodName)
			// delete tmp file
			log.Info("delete tmp state file because it is not smoothly upgrade")
			_ = util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
				return os.Remove(path.Join("/tmp", hashVal, "state1.json"))
			})
			newPod, err := p.newMountPod(ctx, pod, newPodName)
			if err == nil {
				_, err = p.Client.CreatePod(ctx, newPod)
				if err != nil {
					log.Error(err, "Create pod")
				}
			}
			return Result{RequeueImmediately: true}, err
		}
	}
	if err != nil {
		if apierrors.IsTimeout(err) {
			err = fmt.Errorf("old pod %s %s deleting timeout", pod.Name, config.Namespace)
			log.Error(err, "delete pod error")
			return Result{}, err
		}
		log.Error(err, "Get pod error")
		return Result{}, err
	}
	return Result{}, nil
}

// podPendingHandler handles mount pod that is pending
func (p *PodDriver) podPendingHandler(ctx context.Context, pod *corev1.Pod) (Result, error) {
	if pod == nil {
		return Result{}, nil
	}
	log := util.GenLog(ctx, podDriverLog, "podPendingHandler")
	hashVal := pod.Labels[common.PodJuiceHashLabelKey]
	if hashVal == "" {
		return Result{}, fmt.Errorf("pod %s/%s has no hash label", pod.Namespace, pod.Name)
	}
	lock := config.GetPodLock(hashVal)
	lock.Lock()
	defer lock.Unlock()

	// check resource err
	if resource.IsPodResourceError(pod) {
		log.Info("Pod failed because of resource.")
		if resource.IsPodHasResource(*pod) {
			// if pod is failed because of resource, delete resource and deploy pod again.
			_ = resource.RemoveFinalizer(ctx, p.Client, pod, common.Finalizer)
			log.Info("Delete it and deploy again with no resource.")
			if err := p.Client.DeletePod(ctx, pod); err != nil {
				log.Error(err, "delete pod error")
				return Result{}, nil
			}
			isDeleted := false
			// wait pod delete for 1min
			for {
				_, err := p.Client.GetPod(ctx, pod.Name, pod.Namespace)
				if err == nil {
					log.V(1).Info("pod still exists wait.")
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
				log.Error(err, "get mountPod err")
			}
			if !isDeleted {
				log.Info("Old pod deleting timeout")
				return Result{}, nil
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
			controllerutil.AddFinalizer(newPod, common.Finalizer)
			resource.DeleteResourceOfPod(newPod)
			err := mkrMp(ctx, *newPod)
			if err != nil {
				log.Error(err, "mkdir mount point of pod error")
			}
			_, err = p.Client.CreatePod(ctx, newPod)
			if err != nil {
				log.Error(err, "create pod error")
			}
		} else {
			log.Info("mountPod PodResourceError, but pod no resource, do nothing.")
		}
	}
	return Result{}, nil
}

// podReadyHandler handles mount pod that is ready
func (p *PodDriver) podReadyHandler(ctx context.Context, pod *corev1.Pod) (Result, error) {
	if pod == nil {
		return Result{}, nil
	}
	log := util.GenLog(ctx, podDriverLog, "podReadyHandler")

	if pod.Annotations == nil {
		return Result{}, nil
	}
	// get mount point
	mntPath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		log.Error(err, "get mount point error")
		return Result{}, err
	}

	lock := config.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	supFusePass := util.SupportFusePass(pod.Spec.Containers[0].Image)
	podHashVal := pod.Labels[common.PodJuiceHashLabelKey]

	err = resource.WaitUtilMountReady(ctx, pod.Name, mntPath, defaultCheckoutTimeout)
	if err != nil {
		if supFusePass {
			log.Error(err, "pod is not ready within 60s")
			// mount pod hang probably, close fd and delete it
			log.Info("close fd and delete pod")
			passfd.GlobalFds.CloseFd(podHashVal)
			// umount it
			_ = util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
				util.UmountPath(ctx, mntPath)
				return nil
			})
			return Result{RequeueImmediately: true}, p.Client.DeletePod(ctx, pod)
		}
		log.Error(err, "pod is err, don't do recovery")
		return Result{}, err
	}

	return Result{}, p.recover(ctx, pod, mntPath)
}

func (p *PodDriver) recover(ctx context.Context, pod *corev1.Pod, mntPath string) error {
	log := util.GenLog(ctx, podDriverLog, "recover")
	for k, target := range pod.Annotations {
		if k == util.GetReferenceKey(target) {
			mi := p.mit.resolveTarget(ctx, target)
			if mi == nil {
				log.Info("pod target resolve fail", "target", target)
				continue
			}

			if err := p.recoverTarget(ctx, pod.Name, mntPath, mi.baseTarget, mi); err != nil {
				return err
			}
			for _, ti := range mi.subPathTarget {
				if err := p.recoverTarget(ctx, pod.Name, mntPath, ti, mi); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// recoverTarget recovers target path
func (p *PodDriver) recoverTarget(ctx context.Context, podName, sourcePath string, ti *targetItem, mi *mountItem) error {
	log := util.GenLog(ctx, podDriverLog, "recover")
	switch ti.status {
	case targetStatusNotExist:
		log.Info("pod target not exists", "target", ti.target, "item count", ti.count)
		if ti.count > 0 {
			// target exist in /proc/self/mountinfo file
			// refer to this case: local target exist, but source which target binded has beed deleted
			// if target is for pod subpath (volumeMount.subpath), this will cause user pod delete failed, so we help kubelet umount it
			if mi.podDeleted {
				log.Info("umount target", "count", ti.count)
				p.umountTarget(ti.target, ti.count)
			}
		}

	case targetStatusMounted:
		// normal, most likely happen
		log.V(1).Info("target is normal mounted", "target", ti.target)

	case targetStatusNotMount:
		log.Info("target is not mounted", "target", ti.target)

	case targetStatusCorrupt:
		if ti.inconsistent {
			// source paths (found in /proc/self/mountinfo) which target binded is inconsistent
			// some unexpected things happened
			log.Info("pod source inconsistent", "target", ti.target)
			break
		}
		if mi.podDeleted {
			log.V(1).Info("user pod has been deleted, don't do recovery", "target", ti.target)
			break
		}
		// if not umountTarget, mountinfo file will increase unlimited
		// if we umount all the target items, `mountPropagation` will lose efficacy
		log.Info("umount pod target before recover", "target", ti.target, "remain mount count", defaultTargetMountCounts)
		// avoid umount target all, it will cause pod to write files in disk.
		err := p.umountTargetUntilRemain(ctx, mi, ti.target, defaultTargetMountCounts)
		if err != nil {
			log.Error(err, "umount target until remain error")
			return err
		}
		if ti.subpath != "" {
			sourcePath += "/" + ti.subpath
			err = util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
				_, err = os.Stat(sourcePath)
				return err
			})
			if err != nil {
				log.Error(err, "stat volPath error, don't do recovery", "target", ti.target, "mountPath", sourcePath)
				break
			}
		}
		log.Info("recover volPath", "target", ti.target, "mountPath", sourcePath)
		mountOption := []string{"bind"}
		if err := p.Mount(sourcePath, ti.target, "none", mountOption); err != nil {
			ms := fmt.Sprintf("exec cmd: mount -o bind %s %s err:%v", sourcePath, ti.target, err)
			log.Error(err, "bind mount error")
			return fmt.Errorf(ms)
		}

	case targetStatusUnexpect:
		log.Error(ti.err, "resolve target error")
		return fmt.Errorf("pod %s target %s reslove err: %v", podName, ti.target, ti.err)
	}
	return nil
}

// umountTarget umount target path
func (p *PodDriver) umountTarget(target string, count int) {
	if count <= 0 {
		return
	}
	for i := 0; i < count; i++ {
		// ignore error
		_ = p.Unmount(target)
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

		mi := mit.resolveTarget(ctx, basemi.baseTarget.target)
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

		_ = util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
			util.UmountPath(subCtx, target)
			return nil
		})
	}
}

// CleanUpCache clean up cache
func (p *PodDriver) CleanUpCache(ctx context.Context, pod *corev1.Pod) {
	log := util.GenLog(ctx, podDriverLog, "CleanUpCache")
	if pod.Annotations[common.CleanCache] != "true" {
		return
	}
	uuid := pod.Annotations[common.JuiceFSUUID]
	uniqueId := pod.Annotations[common.UniqueId]
	if uuid == "" && uniqueId == "" {
		// no necessary info, return
		log.Info("Can't get uuid and uniqueId from pod annotation. skip cache clean.")
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
			log.Info("Get pod error. Skip clean cache.", "error", err)
			return
		}
		time.Sleep(time.Microsecond * 500)
	}

	if !isDeleted {
		log.Info("Mount pod not deleted in 3 min. Skip clean cache.")
		return
	}
	log.Info("Cleanup cache of volume", "uniqueId", uniqueId, "node", config.NodeName)
	podMnt := podmount.NewPodMount(p.Client, p.SafeFormatAndMount)
	cacheDirs := []string{}
	for _, dir := range pod.Spec.Volumes {
		if strings.HasPrefix(dir.Name, "cachedir-") && dir.HostPath != nil {
			cacheDirs = append(cacheDirs, dir.HostPath.Path)
		}
	}
	image := pod.Spec.Containers[0].Image
	if err := podMnt.CleanCache(ctx, image, uuid, uniqueId, cacheDirs); err != nil {
		log.Error(err, "Cleanup cache of volume error", "uniqueId", uniqueId)
	}
}

func (p *PodDriver) applyConfigPatch(ctx context.Context, pod *corev1.Pod) error {
	attr, err := config.GenPodAttrWithMountPod(ctx, p.Client, pod)
	if err != nil {
		return err
	}
	// update pod spec
	pod.Labels = attr.Labels
	pod.Annotations = attr.Annotations
	pod.Spec.HostAliases = attr.HostAliases
	pod.Spec.HostNetwork = attr.HostNetwork
	pod.Spec.HostPID = attr.HostPID
	pod.Spec.HostIPC = attr.HostIPC
	pod.Spec.TerminationGracePeriodSeconds = attr.TerminationGracePeriodSeconds
	pod.Spec.Containers[0].Image = attr.Image
	pod.Spec.Containers[0].LivenessProbe = attr.LivenessProbe
	pod.Spec.Containers[0].ReadinessProbe = attr.ReadinessProbe
	pod.Spec.Containers[0].StartupProbe = attr.StartupProbe
	pod.Spec.Containers[0].Lifecycle = attr.Lifecycle
	pod.Spec.Containers[0].Image = attr.Image
	pod.Spec.Containers[0].Resources = attr.Resources
	return nil
}

// checkMountPodStuck check mount pod is stuck or not
// maybe fuse deadlock issue, they symptoms are:
//
// 1. pod in terminating state
// 2. after max(pod.spec.terminationGracePeriodSeconds*2, 1min), the pod is still alive
// 3. /sys/fs/fuse/connections/$dev_minor/waiting > 0
//
// if those conditions are true, we need to manually abort the fuse
// connection so the pod doesn't get stuck.
// we can do this to abort fuse connection:
//
//	echo 1 >> /sys/fs/fuse/connections/$dev_minor/abort
func (p *PodDriver) checkMountPodStuck(pod *corev1.Pod) {
	if pod == nil || getPodStatus(pod) != podDeleted {
		return
	}
	log := klog.NewKlogr().WithName("abortFuse").WithValues("podName", pod.Name)
	mountPoint, _, _ := util.GetMountPathOfPod(*pod)
	defer func() {
		if runtime.GOOS == "linux" {
			util.DevMinorTableDelete(mountPoint)
		}
	}()

	timeout := 1 * time.Minute
	if pod.Spec.TerminationGracePeriodSeconds != nil {
		gracePeriod := time.Duration(*pod.Spec.TerminationGracePeriodSeconds) * 2
		if gracePeriod > timeout {
			timeout = gracePeriod
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			log.Info("mount pod may be stuck in terminating state, create a job to abort fuse connection")
			if runtime.GOOS == "linux" {
				if devMinor, ok := util.DevMinorTableLoad(mountPoint); ok {
					if err := p.doAbortFuse(pod, devMinor); err != nil {
						log.Error(err, "abort fuse connection error")
					}
				} else {
					log.Info("can't find devMinor of mountPoint", "mount point", mountPoint)
				}
			}
			return
		default:
			newPod, err := p.Client.GetPod(ctx, pod.Name, pod.Namespace)
			if apierrors.IsNotFound(err) || getPodStatus(newPod) != podDeleted {
				return
			}
			time.Sleep(10 * time.Second)
		}
	}
}

func (p *PodDriver) doAbortFuse(mountpod *corev1.Pod, devMinor uint32) error {
	log := klog.NewKlogr().WithName("abortFuse").WithValues("podName", mountpod.Name)
	job := builder.NewFuseAbortJob(mountpod, devMinor)
	if _, err := p.Client.CreateJob(context.Background(), job); err != nil {
		log.Error(err, "create fuse abort job error")
		return err
	}

	// wait for job to finish
	waitCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	for {
		if waitCtx.Err() == context.Canceled || waitCtx.Err() == context.DeadlineExceeded {
			log.Error(waitCtx.Err(), "fuse abort job timeout", "namespace", job.Namespace, "name", job.Name)
			break
		}
		job, err := p.Client.GetJob(waitCtx, job.Name, job.Namespace)
		if apierrors.IsNotFound(err) {
			break
		}
		if err != nil {
			log.Error(err, "get fuse abort job error", "namespace", job.Namespace, "name", job.Name)
			time.Sleep(10 * time.Second)
			continue
		}
		if resource.IsJobCompleted(job) {
			log.Info("fuse abort job completed", "namespace", job.Namespace, "name", job.Name)
			break
		}
		time.Sleep(10 * time.Second)
	}
	return nil
}

func mkrMp(ctx context.Context, pod corev1.Pod) error {
	log := util.GenLog(ctx, podDriverLog, "mkrMp")
	var shouldMkrMp bool
	if util.SupportFusePass(pod.Spec.Containers[0].Image) {
		cn := pod.Spec.Containers[0]
		if cn.Lifecycle != nil && cn.Lifecycle.PreStop != nil && cn.Lifecycle.PreStop.Exec != nil {
			for _, cmd := range cn.Lifecycle.PreStop.Exec.Command {
				if strings.Contains(cmd, "rmdir") {
					shouldMkrMp = true
				}
			}
		}
	} else {
		shouldMkrMp = true
	}
	if !shouldMkrMp {
		return nil
	}
	log.V(1).Info("Prepare mountpoint for pod")
	// mkdir mountpath
	// get mount point
	var mntPath string
	var err error
	mntPath, _, err = util.GetMountPathOfPod(pod)
	if err != nil {
		log.Error(err, "get mount point error")
		return err
	}
	err = util.DoWithTimeout(ctx, 3*time.Second, func() error {
		exist, _ := mount.PathExists(mntPath)
		if !exist {
			return os.MkdirAll(mntPath, 0777)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (p *PodDriver) needCreateMountPod(ctx context.Context, uniqueId, hashVal string) (bool, error) {
	log := util.GenLog(ctx, podDriverLog, "needCreate")
	labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{
		common.PodTypeKey:           common.PodTypeValue,
		common.PodUniqueIdLabelKey:  uniqueId,
		common.PodJuiceHashLabelKey: hashVal,
	}}
	fieldSelector := &fields.Set{"spec.nodeName": config.NodeName}
	pods, err := p.Client.ListPod(ctx, config.Namespace, labelSelector, fieldSelector)
	if err != nil {
		log.Error(err, "List pod error")
		return false, err
	}
	needCreate := true
	for _, po := range pods {
		if po.DeletionTimestamp == nil && !resource.IsPodComplete(&po) {
			needCreate = false
		}
	}
	return needCreate, nil
}

func (p *PodDriver) newMountPod(ctx context.Context, pod *corev1.Pod, newPodName string) (*corev1.Pod, error) {
	log := util.GenLog(ctx, podDriverLog, "newMountPod")
	hashVal := pod.Labels[common.PodJuiceHashLabelKey]
	// get mount point
	sourcePath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		log.Error(err, "get mount point error")
		return nil, err
	}
	oldSupportFusePass := util.SupportFusePass(pod.Spec.Containers[0].Image)
	var newPod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        newPodName,
			Namespace:   pod.Namespace,
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
		},
		Spec: pod.Spec,
	}
	controllerutil.AddFinalizer(newPod, common.Finalizer)
	if err := p.applyConfigPatch(ctx, newPod); err != nil {
		log.Error(err, "apply config patch error, will ignore")
	}
	if !util.SupportFusePass(newPod.Spec.Containers[0].Image) {
		if oldSupportFusePass {
			// old image support fuse pass and new image do not support, stop fd in csi
			passfd.GlobalFds.StopFd(ctx, hashVal)
		}
		// umount mount point before recreate mount pod
		err := util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
			exist, _ := mount.PathExists(sourcePath)
			if !exist {
				return fmt.Errorf("%s not exist", sourcePath)
			}
			return nil
		})
		if err == nil {
			log.Info("start to umount", "mountPath", sourcePath)
			_ = util.DoWithTimeout(ctx, defaultCheckoutTimeout, func() error {
				util.UmountPath(ctx, sourcePath)
				return nil
			})
		}
	}
	err = mkrMp(ctx, *newPod)
	if err != nil {
		log.Error(err, "mkdir mount point of pod")
	}
	return newPod, nil
}
