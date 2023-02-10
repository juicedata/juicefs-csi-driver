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
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (a *AppController) umountFuseSidecars(pod *corev1.Pod) (err error) {
	for _, cn := range pod.Spec.Containers {
		if strings.Contains(cn.Name, config.MountContainerName) {
			if e := a.umountFuseSidecar(pod, cn); e != nil {
				return e
			}
		}
	}
	return
}

func (a *AppController) umountFuseSidecar(pod *corev1.Pod, fuseContainer corev1.Container) (err error) {
	if fuseContainer.Name == "" {
		return
	}

	// get prestop
	if fuseContainer.Lifecycle == nil || fuseContainer.Lifecycle.PreStop == nil || fuseContainer.Lifecycle.PreStop.Exec == nil {
		klog.Infof("[AppController] no prestop in container %s of pod [%s] in [%s]", config.MountContainerName, pod.Name, pod.Namespace)
		return nil
	}
	cmd := fuseContainer.Lifecycle.PreStop.Exec.Command

	klog.Infof("[AppController] exec cmd [%s] in container %s of pod [%s] in [%s]", cmd, config.MountContainerName, pod.Name, pod.Namespace)
	stdout, stderr, err := a.K8sClient.ExecuteInContainer(pod.Name, pod.Namespace, fuseContainer.Name, cmd)
	if err != nil {
		if strings.Contains(stderr, "not mounted") ||
			strings.Contains(stderr, "mountpoint not found") ||
			strings.Contains(stderr, "no mount point specified") {
			// if mount point not mounted, do not retry
			return nil
		}
		if strings.Contains(err.Error(), "exit code 137") {
			klog.Warningf("[AppController] exec with exit code 137, ignore it. error: %v", err)
			return nil
		}
		klog.Errorf("[AppController] error: %v; exec stdout: %s; exec stderr: %s", err, stdout, stderr)
	}
	return
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
				klog.V(6).Infof("[pod.onCreate] skip due to shouldRequeue false. pod: [%s] in [%s]", pod.Name, pod.Namespace)
				return false
			}

			klog.V(6).Infof("[pod.onCreate] pod [%s] in [%s] requeue", pod.Name, pod.Namespace)
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			podNew, ok := updateEvent.ObjectNew.(*corev1.Pod)
			if !ok {
				klog.V(6).Infof("[pod.onUpdate] can not turn into pod, Skip. object: %v", updateEvent.ObjectNew)
				return false
			}

			podOld, ok := updateEvent.ObjectOld.(*corev1.Pod)
			if !ok {
				klog.V(6).Infof("[pod.onUpdate] can not turn into pod, Skip. object: %v", updateEvent.ObjectOld)
				return false
			}

			if podNew.GetResourceVersion() == podOld.GetResourceVersion() {
				klog.V(6).Infof("[pod.onUpdate] Skip due to resourceVersion not changed, pod: [%s] in [%s]", podNew.Name, podNew.Namespace)
				return false
			}

			// ignore if it's not fluid label pod
			if !ShouldInQueue(podNew) {
				klog.V(6).Infof("[pod.onUpdate] skip due to shouldRequeue false. pod: [%s] in [%s]", podNew.Name, podNew.Namespace)
				return false
			}

			klog.V(6).Infof("[pod.onUpdate] pod [%s] in [%s] requeue", podNew.Name, podNew.Namespace)
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
		klog.V(6).Infof("Sidecar inject disabled in pod [%s] in [%s] labels %v, skip.", pod.Name, pod.Namespace, pod.Labels)
		return false
	}

	// ignore if restartPolicy is Always
	if pod.Spec.RestartPolicy == corev1.RestartPolicyAlways {
		klog.V(6).Infof("pod [%s] in [%s] restart policy always, skip.", pod.Name, pod.Namespace)
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
		klog.V(6).Infof("There are no juicefs sidecar in pod [%s] in [%s].", pod.Name, pod.Namespace)
		return false
	}

	// ignore if pod status is not running
	if pod.Status.Phase != corev1.PodRunning || len(pod.Status.ContainerStatuses) < 2 {
		klog.V(6).Infof("Pod [%s] in [%s] status is not running or containerStatus less than 2.", pod.Name, pod.Namespace)
		return false
	}

	// reconcile if all app containers exit 0 and fuse container not exit
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !strings.Contains(containerStatus.Name, config.MountContainerName) {
			klog.V(6).Infof("container %s in pod [%s] in [%s] status: %v", containerStatus.Name, pod.Name, pod.Namespace, containerStatus)
			if containerStatus.State.Terminated == nil {
				klog.V(6).Infof("container %s in pod [%s] in [%s] not exited", containerStatus.Name, pod.Name, pod.Namespace)
				// container not exist
				return false
			}
		}
		if strings.Contains(containerStatus.Name, config.MountContainerName) {
			if containerStatus.State.Running == nil {
				klog.V(6).Infof("juicefs fuse client in pod [%s] in [%s] not running", pod.Name, pod.Namespace)
				return false
			}
		}
	}
	return true
}
