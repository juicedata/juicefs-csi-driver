/*
 Copyright 2022 Juicedata Inc

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
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type AppController struct {
	*k8sclient.K8sClient
}

func NewAppController(client *k8sclient.K8sClient) *AppController {
	return &AppController{client}
}

func (a *AppController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(6).Infof("Receive pod %s %s", request.Name, request.Namespace)
	pod, err := a.K8sClient.GetPod(ctx, request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		klog.Errorf("get pod %s error: %v", request.Name, err)
		return reconcile.Result{}, err
	}
	if pod == nil {
		klog.V(6).Infof("pod %s not found.", request.Name)
		return reconcile.Result{}, nil
	}

	if !ShouldInQueue(pod) {
		klog.V(6).Infof("pod %s namespace %s should not in queue", request.Name, request.Namespace)
		return reconcile.Result{}, nil
	}
	// umount fuse sidecars
	err = a.umountFuseSidecars(pod)
	if err != nil {
		klog.Errorf("umount juicefs sidecar in pod %s namespace %s error: %v", pod.Name, pod.Namespace, err)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (a *AppController) umountFuseSidecars(pod *corev1.Pod) (err error) {
	for _, cn := range pod.Spec.Containers {
		if strings.Contains(cn.Name, config.MountContainerName) {
			if e := a.umountFuseSidecar(pod, cn); e != nil {
				return
			}
		}
	}
	return
}

func (a *AppController) umountFuseSidecar(pod *corev1.Pod, fuseContainer corev1.Container) (err error) {
	if fuseContainer.Name == "" {
		return
	}

	cmd := []string{}
	// get prestop
	if fuseContainer.Lifecycle != nil && fuseContainer.Lifecycle.PreStop != nil && fuseContainer.Lifecycle.PreStop.Exec != nil {
		cmd = fuseContainer.Lifecycle.PreStop.Exec.Command
	}

	klog.Infof("[AppController] exec cmd [%s] in container %s of pod %s namespace %s", cmd, config.MountContainerName, pod.Name, pod.Namespace)
	stdout, stderr, err := a.K8sClient.ExecuteInContainer(pod.Name, pod.Namespace, fuseContainer.Name, cmd)
	if err != nil {
		klog.Errorf("[AppController] exec stdout: %s; exec stderr: %s; error: %v", stdout, stderr, err)
		if strings.Contains(stderr, "not mounted") {
			// if mount point not mounted, do not retry
			return nil
		}
		if strings.Contains(err.Error(), "exit code 137") {
			klog.Warningf("[AppController] exec with exit code 137, ignore it. error: %v", err)
			return nil
		}
		klog.Errorf("[AppController] exec error: %v", err)
		return err
	}
	return err
}
func (a *AppController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("app", mgr, controller.Options{Reconciler: a})
	if err != nil {
		return err
	}

	return c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				klog.V(6).Infof("[pod.onCreate] can not turn into pod, Skip. object: %v", event.Object)
				return false
			}

			if !ShouldInQueue(pod) {
				klog.V(6).Info("[pod.onCreate] skip due to shouldRequeue false. pod: %s", fmt.Sprintf("%s-%s", pod.Name, pod.Namespace))
				return false
			}

			klog.V(6).Infof("[pod.onCreate] pod %s requeue", fmt.Sprintf("%s-%s", pod.Name, pod.Namespace))
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) (needUpdate bool) {
			podNew, ok := updateEvent.ObjectNew.(*corev1.Pod)
			if !ok {
				klog.V(6).Infof("[pod.onUpdate] can not turn into pod, Skip. object: %v", updateEvent.ObjectNew)
				return needUpdate
			}

			podOld, ok := updateEvent.ObjectOld.(*corev1.Pod)
			if !ok {
				klog.V(6).Info("[pod.onUpdate] can not turn into pod, Skip. object: %v", updateEvent.ObjectOld)
				return needUpdate
			}

			if podNew.GetResourceVersion() == podOld.GetResourceVersion() {
				klog.V(6).Info("[pod.onUpdate] Skip due to resourceVersion not changed", fmt.Sprintf("%s-%s", podNew.Name, podNew.Namespace))
				return needUpdate
			}

			// ignore if it's not fluid label pod
			if !ShouldInQueue(podNew) {
				klog.V(6).Info("[pod.onUpdate] skip due to shouldRequeue false. pod: %s", fmt.Sprintf("%s-%s", podNew.Name, podNew.Namespace))
				return false
			}

			klog.V(6).Infof("[pod.onUpdate] pod %s requeue", fmt.Sprintf("%s-%s", podNew.Name, podNew.Namespace))
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			// ignore delete event
			return false
		},
	})
}

func ShouldInQueue(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	// ignore if it's not fluid label pod
	if util.CheckExpectValue(pod.Labels, config.InjectSidecarDisable, config.True) {
		klog.V(6).Infof("Serverless not enable in pod %s labels %v.", pod.Name, pod.Labels)
		return false
	}

	// ignore if restartPolicy is Always
	if pod.Spec.RestartPolicy == corev1.RestartPolicyAlways {
		klog.V(6).Infof("pod %s restart policy always", pod.Name)
		return false
	}

	// ignore if no fuse container
	exist := false
	for _, cn := range pod.Spec.Containers {
		if strings.Contains(cn.Name, config.MountContainerName) {
			exist = true
			break
		}
	}
	if !exist {
		klog.V(6).Infof("There are no juicefs sidecar in pod %s namespace %s.", pod.Name, pod.Namespace)
		return false
	}

	// ignore if pod status is not running
	if pod.Status.Phase != corev1.PodRunning || len(pod.Status.ContainerStatuses) < 2 {
		klog.V(6).Infof("Pod %s namespace %s status is not running or containerStatus less than 2.", pod.Name, pod.Namespace)
		return false
	}

	// reconcile if all app containers exit 0 and fuse container not exit
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !strings.Contains(containerStatus.Name, config.MountContainerName) {
			klog.V(6).Infof("container %s in pod %s status: %v", containerStatus.Name, pod.Name, containerStatus)
			if containerStatus.State.Terminated == nil {
				klog.V(6).Infof("container %s in pod %s not exited", containerStatus.Name, pod.Name)
				// container not exist
				return false
			}
		}
		if strings.Contains(containerStatus.Name, config.MountContainerName) {
			if containerStatus.State.Running == nil {
				klog.V(6).Infof("juicefs fuse client in pod %s not running", pod.Name)
				return false
			}
		}
	}
	return true
}
