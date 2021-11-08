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
	"k8s.io/klog"
	k8sMount "k8s.io/utils/mount"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type PodDriver struct {
	Client   k8sclient.K8sClient
	handlers map[podStatus]podHandler
	Mounter  util.MountInter
}

func NewPodDriver(client k8sclient.K8sClient) *PodDriver {
	driver := &PodDriver{
		Client:   client,
		handlers: map[podStatus]podHandler{},
		Mounter:  k8sMount.New(""),
	}
	driver.handlers[podReady] = driver.podReadyHandler
	driver.handlers[podError] = driver.podErrorHandler
	driver.handlers[podRunning] = driver.podRunningHandler
	driver.handlers[podDeleted] = driver.podDeletedHandler
	return driver
}

type podHandler func(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error)
type podStatus string

const (
	podReady   podStatus = "podReady"
	podError   podStatus = "podError"
	podDeleted podStatus = "podDeleted"
	podRunning podStatus = "podRunning"
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
	return podRunning
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
				return reconcile.Result{}, nil
			}
			klog.V(5).Infof("waitUtilMount: Delete it and deploy again with no resource.")
			if err := p.Client.DeletePod(pod); err != nil {
				klog.V(5).Infof("delete po:%s err:%v", pod.Name, err)
				return reconcile.Result{}, nil
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
			klog.V(5).Infof("waitUtilMount: Deploy again with no resource.")
			_, err := p.Client.CreatePod(newPod)
			if err != nil {
				klog.Errorf("create pod:%s err:%v", pod.Name, err)
			}
		} else {
			klog.V(5).Infof("mountPod PodResourceError, but pod no resource")
		}
		return reconcile.Result{}, nil
	}

	klog.V(5).Infof("Get pod %s in namespace %s is err status, deleting thd pod.", pod.Name, pod.Namespace)
	if err := p.Client.DeletePod(pod); err != nil {
		klog.V(5).Infof("delete po:%s err:%v", pod.Name, err)
		return reconcile.Result{}, nil
	}
	return reconcile.Result{}, nil
}

func (p *PodDriver) podDeletedHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	klog.V(5).Infof("Get pod %s in namespace %s is to be deleted.", pod.Name, pod.Namespace)
	if !util.ContainsString(pod.GetFinalizers(), config.Finalizer) {
		// do nothing
		return reconcile.Result{}, nil
	}
	if util.IsPodResourceError(pod) {
		klog.V(5).Infof("the pod is PodResourceError, podDeletedHandler skip delete the pod:%s", pod.Name)
		return reconcile.Result{}, nil
	}
	// todo
	klog.V(5).Infof("Remove finalizer of pod %s namespace %s", pod.Name, pod.Namespace)
	controllerutil.RemoveFinalizer(pod, config.Finalizer)
	if err := p.Client.UpdatePod(pod); err != nil {
		klog.Errorf("Update pod err:%v", err)
		return reconcile.Result{}, err
	}
	// do recovery
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
	cmd := pod.Spec.Containers[0].Command
	if cmd == nil || len(cmd) < 3 {
		klog.Errorf("get error pod command:%v", cmd)
		return reconcile.Result{}, nil
	}
	sourcePath, _, err := util.ParseMntPath(cmd[2])
	if err != nil {
		klog.Error(err)
		return reconcile.Result{}, nil
	}

	klog.Infof("start umount :%s", sourcePath)
	if err := p.Mounter.Unmount(sourcePath); err != nil {
		klog.Errorf("umount %s err:%v\n", sourcePath, err)
		//return reconcile.Result{}, nil
	}
	// create
	klog.V(5).Infof("pod targetPath not empty, need create thd pod:%s", pod.Name)
	return reconcile.Result{}, nil
}

func (p *PodDriver) podReadyHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	// bind target
	// do recovery
	if pod.Annotations == nil {
		return reconcile.Result{}, nil
	}
	cmd := pod.Spec.Containers[0].Command
	if cmd == nil || len(cmd) < 3 {
		klog.Errorf("get error pod command:%v, don't do recovery", cmd)
		return reconcile.Result{}, nil
	}
	mntPath, volumeId, err := util.ParseMntPath(cmd[2])
	if err != nil {
		klog.Error(err)
		return reconcile.Result{}, nil
	}
	// staticPv no subPath
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
				klog.V(5).Infof("target:%s not exists ,  don't do recover", v)
				continue
			}
			klog.V(5).Infof("Get pod %s in namespace %s is ready, %s", pod.Name, pod.Namespace, cmd2)
			if err := p.Mounter.Mount(sourcePath, v, "none", mountOption); err != nil {
				klog.Errorf("exec cmd: mount -o bind %s %s err:%v", sourcePath, v, err)
			}
		}
	}
	return reconcile.Result{}, nil
}

func (p *PodDriver) podRunningHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	// requeue
	return reconcile.Result{Requeue: true}, nil
}
