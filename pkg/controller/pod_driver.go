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
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PodDriver struct {
	Client   *k8sclient.K8sClient
	handlers map[podStatus]podHandler
	polling  bool
	mit      *mountInfoTable
	mount.SafeFormatAndMount
}

func NewPodDriver(client *k8sclient.K8sClient, mounter mount.SafeFormatAndMount) *PodDriver {
	return newPodDriver(client, mounter, false)
}

func NewPollingPodDriver(client *k8sclient.K8sClient, mounter mount.SafeFormatAndMount) *PodDriver {
	return newPodDriver(client, mounter, true)
}

func newPodDriver(client *k8sclient.K8sClient, mounter mount.SafeFormatAndMount, polling bool) *PodDriver {
	driver := &PodDriver{
		Client:             client,
		handlers:           map[podStatus]podHandler{},
		polling:            polling,
		mit:                newMountInfoTable(),
		SafeFormatAndMount: mounter,
	}
	driver.handlers[podReady] = driver.podReadyHandler
	driver.handlers[podError] = driver.podErrorHandler
	driver.handlers[podPending] = driver.podPendingHandler
	driver.handlers[podDeleted] = driver.podDeletedHandler
	return driver
}

type podHandler func(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error)
type podStatus string

const (
	podReady   podStatus = "podReady"
	podError   podStatus = "podError"
	podDeleted podStatus = "podDeleted"
	podPending podStatus = "podPending"
)

func (p *PodDriver) Run(ctx context.Context, current *corev1.Pod) (reconcile.Result, error) {
	podStatus := p.getPodStatus(current)

	if !p.polling || (podStatus != podError && podStatus != podDeleted) {
		return p.handlers[podStatus](ctx, current)
	}

	// resourceVersion of kubelet may be different from apiserver
	// so we need get latest pod resourceVersion from apiserver
	pod, err := p.Client.GetPod(current.Name, current.Namespace)
	if err != nil {
		return reconcile.Result{}, nil
	}
	return p.handlers[p.getPodStatus(pod)](ctx, pod)
}

func (p *PodDriver) getPodStatus(pod *corev1.Pod) podStatus {
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

func (p *PodDriver) podErrorHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	if pod == nil {
		return reconcile.Result{}, nil
	}
	lock := config.JLock.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	// check resource err
	if util.IsPodResourceError(pod) {
		klog.V(5).Infof("waitUtilMount: Pod is failed because of resource.")
		if util.IsPodHasResource(*pod) {
			// if pod is failed because of resource, delete resource and deploy pod again.
			controllerutil.RemoveFinalizer(pod, config.Finalizer)
			if err := p.Client.UpdatePod(pod); err != nil {
				klog.Errorf("Update pod err:%v", err)
				return reconcile.Result{Requeue: true}, nil
			}
			klog.V(5).Infof("Delete it and deploy again with no resource.")
			if err := p.Client.DeletePod(pod); err != nil {
				klog.V(5).Infof("delete po:%s err:%v", pod.Name, err)
				return reconcile.Result{Requeue: true}, nil
			}
			// wait pod delete
			for i := 0; i < 30; i++ {
				oldPod, err := p.Client.GetPod(pod.Name, pod.Namespace)
				if err == nil {
					if controllerutil.ContainsFinalizer(oldPod, config.Finalizer) {
						controllerutil.RemoveFinalizer(oldPod, config.Finalizer)
						if err := p.Client.UpdatePod(oldPod); err != nil {
							klog.Errorf("Update pod err:%v", err)
						}
					}
					klog.V(5).Infof("pod %s %s still exists wait.", pod.Name, pod.Namespace)
					time.Sleep(time.Second * 5)
					continue
				}
				if apierrors.IsNotFound(err) {
					break
				}
				klog.V(5).Infof("get mountPod err:%v", err)
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
			klog.V(5).Infof("Deploy again with no resource.")
			_, err := p.Client.CreatePod(newPod)
			if err != nil {
				klog.Errorf("create pod:%s err:%v", pod.Name, err)
			}
		} else {
			klog.V(5).Infof("mountPod PodResourceError, but pod no resource, do nothing.")
		}
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func (p *PodDriver) podDeletedHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	if pod == nil {
		klog.Errorf("get nil pod")
		return reconcile.Result{}, nil
	}
	klog.V(5).Infof("Get pod %s in namespace %s is to be deleted.", pod.Name, pod.Namespace)

	lock := config.JLock.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	// pod with no finalizer
	if !util.ContainsString(pod.GetFinalizers(), config.Finalizer) {
		// do nothing
		return reconcile.Result{}, nil
	}

	// pod with resource error
	if util.IsPodResourceError(pod) {
		klog.V(5).Infof("The pod is PodResourceError, podDeletedHandler skip delete the pod:%s", pod.Name)
		return reconcile.Result{}, nil
	}

	// remove finalizer of pod
	klog.V(5).Infof("Remove finalizer of pod %s namespace %s", pod.Name, pod.Namespace)
	controllerutil.RemoveFinalizer(pod, config.Finalizer)
	if err := p.Client.UpdatePod(pod); err != nil {
		klog.Errorf("Update pod err:%v", err)
		return reconcile.Result{Requeue: true}, err
	}

	// get mount point
	sourcePath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		klog.Error(err)
		return reconcile.Result{}, nil
	}

	// check if it needs to do recovery
	klog.V(6).Infof("Annotations:%v", pod.Annotations)
	if pod.Annotations == nil {
		return reconcile.Result{}, nil
	}
	annotation := pod.Annotations
	existTargets := make([]string, 0)

	e := doWithinTime(ctx, func() error {
		for k, v := range pod.Annotations {
			if strings.HasPrefix(k, "juicefs-") {
				// check if target exist
				if exist, _ := mount.PathExists(v); exist {
					existTargets = append(existTargets, v)
					return nil
				}
				klog.V(5).Infof("Target %s didn't exist.", v)
				delete(annotation, k)
			}
		}
		return nil
	})

	if e != nil {
		return reconcile.Result{}, nil
	}

	if len(existTargets) == 0 {
		e := doWithinTime(ctx, func() error {
			// do not need recovery, clean mount point
			klog.V(5).Infof("Clean mount point : %s", sourcePath)
			return mount.CleanupMountPoint(sourcePath, p.SafeFormatAndMount.Interface, false)
		})

		if e != nil {
			klog.V(5).Infof("Clean mount point %s error: %v", sourcePath, e)
		}
		return reconcile.Result{}, nil
	}

	// create the pod even if getting err
	defer func() {
		// check pod delete
		for i := 0; i < 30; i++ {
			if _, err := p.Client.GetPod(pod.Name, pod.Namespace); err == nil {
				klog.Infof("pod %s %s still exists, wait to create", pod.Name, pod.Namespace)
				time.Sleep(time.Second * 5)
			} else {
				if apierrors.IsNotFound(err) {
					break
				}
				klog.Errorf("get pod err:%v", err) // create pod even if get err
				break
			}

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
		_, err := p.Client.CreatePod(newPod)
		if err != nil {
			klog.Errorf("create pod:%s err:%v", pod.Name, err)
		}
	}()

	e = doWithinTime(ctx, func() error {
		// umount mount point before recreate mount pod
		klog.Infof("start to umount: %s", sourcePath)
		out, e := exec.Command("umount", sourcePath).CombinedOutput()
		if e != nil {
			if !strings.Contains(string(out), "not mounted") && !strings.Contains(string(out), "mountpoint not found") {
				klog.V(5).Infof("Unmount %s failed: %q, try to lazy unmount", sourcePath, err)
				output, err2 := exec.Command("umount", "-l", sourcePath).CombinedOutput()
				if err2 != nil {
					klog.Errorf("could not lazy unmount %q: %v, output: %s", sourcePath, err2, string(output))
				}
				return err2
			}
		}
		return e
	})

	if e != nil {
		klog.Errorf("[podDeleteHandler] umount mountPath: %s err: %v", sourcePath, err)
		return reconcile.Result{}, nil
	}

	// create
	klog.V(5).Infof("pod targetPath not empty, need create pod:%s", pod.Name)
	return reconcile.Result{}, nil
}

func (p *PodDriver) podPendingHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	// requeue
	return reconcile.Result{Requeue: true}, nil
}

func (p *PodDriver) podReadyHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	if pod == nil {
		klog.Errorf("[podReadyHandler] get nil pod")
		return reconcile.Result{}, nil
	}
	lock := config.JLock.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	if pod.Annotations == nil {
		return reconcile.Result{}, nil
	}
	// get mount point
	mntPath, volumeId, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		klog.Error(err)
		return reconcile.Result{}, nil
	}

	e := doWithinTime(ctx, func() error {
		_, e := os.Stat(mntPath)
		return e
	})

	if e != nil {
		klog.Errorf("[podReadyHandler] stat mntPath:%s err:%v, don't do recovery", mntPath, err)
		return reconcile.Result{}, nil
	}

	if !p.polling {
		if err := p.mit.parse(); err != nil {
			klog.Errorf("podReadyHandler ParseMountInfo: %v", err)
			return reconcile.Result{}, nil
		}
	}

	_ = doWithinTime(ctx, func() error {
		// recovery for each target
		for k, target := range pod.Annotations {
			if k == util.GetReferenceKey(target) {
				mi := p.mit.resolveTarget(target)
				if mi == nil {
					klog.Errorf("pod %s target %s resolve fail", pod.Name, target)
					continue
				}

				p.recoverTarget(volumeId, pod.Name, mntPath, mi.baseTarget, mi)
				for _, ti := range mi.subPathTarget {
					p.recoverTarget(volumeId, pod.Name, mntPath, ti, mi)
				}
			}
		}
		return nil
	})

	return reconcile.Result{}, nil
}

func (p *PodDriver) recoverTarget(volumeId, podName, sourcePath string, ti *targetItem, mi *mountItem) {
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
		if !p.polling {
			klog.V(6).Infof("pod %s target %s is normal mounted", podName, ti.target)
		}

	case targetStatusNotMount:
		if !p.polling {
			klog.V(5).Infof("pod %s target %s is not mounted", podName, ti.target)
		}

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
		p.umountTarget(ti.target, ti.count-1)
		if ti.subpath != "" {
			sourcePath += "/" + ti.subpath
			_, err := os.Stat(sourcePath)
			if err != nil {
				klog.Errorf("pod %s target %s, stat volPath:%s err:%v, don't do recovery", podName, ti.target, sourcePath, err)
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

func (p *PodDriver) umountTarget(target string, count int) {
	for i := 0; i < count; i++ {
		// ignore error
		p.Unmount(target)
	}
}

func doWithinTime(ctx context.Context, f func() error) error {
	doneCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	doneCh := make(chan error)
	go func() {
		doneCh <- f()
	}()

	select {
	case <-doneCtx.Done():
		return status.Error(codes.Internal, "context timeout")
	case res := <-doneCh:
		return res
	}
}
