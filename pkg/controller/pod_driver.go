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
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
	"os"
	"os/exec"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"time"
)

type PodDriver struct {
	Client   *k8sclient.K8sClient
	handlers map[podStatus]podHandler
	mount.SafeFormatAndMount
}

func NewPodDriver(client *k8sclient.K8sClient) *PodDriver {
	mounter := &mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      k8sexec.New(),
	}
	driver := &PodDriver{
		Client:             client,
		handlers:           map[podStatus]podHandler{},
		SafeFormatAndMount: *mounter,
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
	return p.handlers[p.getPodStatus(current)](ctx, current)
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
					if pod.Finalizers != nil || len(oldPod.Finalizers) == 0 {
						controllerutil.RemoveFinalizer(oldPod, config.Finalizer)
						if err := p.Client.UpdatePod(oldPod); err != nil {
							klog.Errorf("Update pod err:%v", err)
						}
					}
					klog.V(5).Infof("pod still exists wait.")
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

	// check mount point is broken
	needDeleted := false
	sourcePath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		klog.Error(err)
		return reconcile.Result{}, err
	}
	exists, err := mount.PathExists(sourcePath)
	if err != nil || !exists {
		klog.V(5).Infof("%s is a corrupted mountpoint", sourcePath)
		needDeleted = true
	} else if notMnt, err := p.IsLikelyNotMountPoint(sourcePath); err != nil || notMnt {
		needDeleted = true
	}

	if needDeleted {
		klog.V(5).Infof("Get pod %s in namespace %s is err status, deleting thd pod.", pod.Name, pod.Namespace)
		if err := p.Client.DeletePod(pod); err != nil {
			klog.V(5).Infof("delete po:%s err:%v", pod.Name, err)
			return reconcile.Result{Requeue: true}, nil
		}
	}
	return reconcile.Result{}, nil
}

func (p *PodDriver) podDeletedHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	if pod == nil {
		klog.Errorf("get nil pod")
		return reconcile.Result{}, nil
	}
	klog.V(5).Infof("Get pod %s in namespace %s is to be deleted.", pod.Name, pod.Namespace)

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

	// check if need to do recovery
	klog.V(6).Infof("Annotations:%v", pod.Annotations)
	if pod.Annotations == nil {
		return reconcile.Result{}, nil
	}
	var targets = make([]string, 0)
	for k, v := range pod.Annotations {
		if k == util.GetReferenceKey(v) {
			targets = append(targets, v)
		}
	}
	if len(targets) == 0 {
		// do not need recovery
		return reconcile.Result{}, nil
	}

	// get mount point
	sourcePath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		klog.Error(err)
		return reconcile.Result{}, nil
	}

	// create the pod even if get err
	defer func() {
		// check pod delete
		for i := 0; i < 30; i++ {
			if _, err := p.Client.GetPod(pod.Name, pod.Namespace); err == nil {
				klog.Infof("pod still exists, wait to create")
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
				Annotations: pod.Annotations,
			},
			Spec: pod.Spec,
		}
		controllerutil.AddFinalizer(newPod, config.Finalizer)
		_, err := p.Client.CreatePod(newPod)
		if err != nil {
			klog.Errorf("create pod:%s err:%v", pod.Name, err)
		}
	}()

	// umount mount point before recreate mount pod
	klog.Infof("start umount :%s", sourcePath)
	out, err := exec.Command("umount", sourcePath).CombinedOutput()
	if err != nil {
		if !strings.Contains(string(out), "not mounted") || !strings.Contains(string(out), "mountpoint not found") {
			klog.V(5).Infof("Unmount %s failed: %q, try to lazy unmount", sourcePath, err)
			output, err1 := exec.Command("umount", "-l", sourcePath).CombinedOutput()
			if err1 != nil {
				klog.Errorf("could not lazy unmount %q: %v, output: %s", sourcePath, err1, string(output))
			}
		}
	}
	// create
	klog.V(5).Infof("pod targetPath not empty, need create pod:%s", pod.Name)
	return reconcile.Result{}, nil
}

func (p *PodDriver) podReadyHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	if pod == nil {
		klog.Errorf("[podReadyHandler] get nil pod")
		return reconcile.Result{}, nil
	}
	if pod.Annotations == nil {
		return reconcile.Result{}, nil
	}
	// get mount point
	mntPath, volumeId, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		klog.Error(err)
		return reconcile.Result{}, nil
	}

	// staticPv has no subPath, check sourcePath
	sourcePath := fmt.Sprintf("%s/%s", mntPath, volumeId)
	_, err = os.Stat(sourcePath)
	if err != nil {
		if !os.IsNotExist(err) {
			klog.Errorf("stat volPath:%s err:%v, don't do recovery", sourcePath, err)
			return reconcile.Result{}, nil
		}
		sourcePath = mntPath
		if _, err2 := os.Stat(sourcePath); err2 != nil {
			klog.Errorf("stat volPath:%s err:%v, don't do recovery", sourcePath, err2)
			return reconcile.Result{}, nil
		}
	}

	// recovery for each target
	mountOption := []string{"bind"}
	for k, v := range pod.Annotations {
		if k == util.GetReferenceKey(v) {
			cmd2 := fmt.Sprintf("start exec cmd: mount -o bind %s %s \n", sourcePath, v)
			// check target should do recover
			_, err := os.Stat(v)
			if err == nil {
				klog.V(5).Infof("target path %s is normal, don't need do recover", v)
				continue
			} else if os.IsNotExist(err) {
				klog.V(5).Infof("target %s not exists,  don't do recover", v)
				continue
			}
			klog.V(5).Infof("Get pod %s in namespace %s is ready, %s", pod.Name, pod.Namespace, cmd2)
			if err := p.Mount(sourcePath, v, "none", mountOption); err != nil {
				klog.Errorf("exec cmd: mount -o bind %s %s err:%v", sourcePath, v, err)
			}
		}
	}
	return reconcile.Result{}, nil
}

func (p *PodDriver) podPendingHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	// requeue
	return reconcile.Result{Requeue: true}, nil
}
