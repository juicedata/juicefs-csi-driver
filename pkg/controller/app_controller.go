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
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var (
	appCtrlLog = klog.NewKlogr().WithName("app-controller")
)

type AppController struct {
	*k8sclient.K8sClient
}

func NewAppController(client *k8sclient.K8sClient) *AppController {
	return &AppController{client}
}

func (a *AppController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	appCtrlLog.V(1).Info("Receive pod", "name", request.Name, "namespace", request.Namespace)
	pod, err := a.K8sClient.GetPod(ctx, request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		appCtrlLog.Error(err, "get pod error", "name", request.Name)
		return reconcile.Result{}, err
	}
	if pod == nil {
		appCtrlLog.V(1).Info("pod not found.", "name", request.Name)
		return reconcile.Result{}, nil
	}

	if !ShouldInQueue(pod) {
		appCtrlLog.V(1).Info("pod should not in queue", "name", request.Name)
		return reconcile.Result{}, nil
	}

	// get a last terminated container finsh time
	// if the time is more than 5 minutes, kill the fuse process
	var appContainerExitedTime time.Time
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !strings.Contains(containerStatus.Name, common.MountContainerName) {
			if containerStatus.State.Terminated != nil {
				if containerStatus.State.Terminated.FinishedAt.After(appContainerExitedTime) {
					appContainerExitedTime = containerStatus.State.Terminated.FinishedAt.Time
				}
			}
		}
	}

	if !appContainerExitedTime.IsZero() && time.Since(appContainerExitedTime) > 5*time.Minute {
		appCtrlLog.V(1).Info("app container exited more than 5 minutes, kill the mount process, app pod will enter an error phase")
		err = a.killFuseProcesss(ctx, pod)
		if err != nil {
			appCtrlLog.Error(err, "kill fuse process error", "name", request.Name)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// umount fuse sidecars
	err = a.umountFuseSidecars(ctx, pod)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (a *AppController) umountFuseSidecars(ctx context.Context, pod *corev1.Pod) (err error) {
	for _, cn := range pod.Spec.Containers {
		if strings.Contains(cn.Name, common.MountContainerName) {
			if e := a.umountFuseSidecar(ctx, pod, cn); e != nil {
				return e
			}
		}
	}
	return
}

func (a *AppController) umountFuseSidecar(ctx context.Context, pod *corev1.Pod, fuseContainer corev1.Container) (err error) {
	if fuseContainer.Name == "" {
		return
	}
	log := klog.NewKlogr().WithName("app-ctrl").WithValues("pod", pod.Name, "namespace", pod.Namespace)

	// get prestop
	if fuseContainer.Lifecycle == nil || fuseContainer.Lifecycle.PreStop == nil || fuseContainer.Lifecycle.PreStop.Exec == nil {
		log.Info("no prestop in container of pod", "cnName", common.MountContainerName)
		return nil
	}
	cmd := fuseContainer.Lifecycle.PreStop.Exec.Command

	log.Info("exec cmd in container of pod", "command", cmd, "cnName", common.MountContainerName)
	stdout, stderr, err := a.K8sClient.ExecuteInContainer(ctx, pod.Name, pod.Namespace, fuseContainer.Name, cmd)
	if err != nil {
		if strings.Contains(stderr, "not mounted") ||
			strings.Contains(stderr, "mountpoint not found") ||
			strings.Contains(stderr, "no mount point specified") {
			// if mount point not mounted, do not retry
			return nil
		}
		if strings.Contains(err.Error(), "exit code 137") {
			log.Error(err, "exec with exit code 137, ignore it.")
			return nil
		}
		log.Error(err, "exec error", "stdout", stdout, "stderr", stderr)
	}
	return
}

func (a *AppController) killFuseProcesss(ctx context.Context, pod *corev1.Pod) error {
	for _, cn := range pod.Spec.Containers {
		if strings.Contains(cn.Name, common.MountContainerName) {
			if e := a.killFuseProcess(ctx, pod, cn); e != nil {
				return e
			}
		}
	}
	return nil
}

func (a *AppController) killFuseProcess(ctx context.Context, pod *corev1.Pod, fuseContainer corev1.Container) error {
	if fuseContainer.Name == "" {
		return nil
	}
	log := klog.NewKlogr().WithName("app-ctrl").WithValues("pod", pod.Name, "namespace", pod.Namespace)
	cmd := []string{"sh", "-c", "pkill mount.juicefs"}
	log.Info("exec cmd in container of pod", "command", cmd, "cnName", common.MountContainerName)
	stdout, stderr, err := a.K8sClient.ExecuteInContainer(ctx, pod.Name, pod.Namespace, fuseContainer.Name, cmd)
	if err != nil {
		return err
	}
	log.Info("exec cmd result", "stdout", stdout, "stderr", stderr)
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
				appCtrlLog.V(1).Info("[pod.onCreate] can not turn into pod, Skip.", "object", event.Object)
				return false
			}

			if !ShouldInQueue(pod) {
				appCtrlLog.V(1).Info("[pod.onCreate] skip due to shouldRequeue false.", "name", pod.Name, "namespace", pod.Namespace)
				return false
			}

			appCtrlLog.V(1).Info("[pod.onCreate] pod in requeue", "name", pod.Name, "namespace", pod.Namespace)
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			podNew, ok := updateEvent.ObjectNew.(*corev1.Pod)
			if !ok {
				appCtrlLog.V(1).Info("[pod.onUpdate] can not turn into pod, Skip.", "object", updateEvent.ObjectNew)
				return false
			}

			podOld, ok := updateEvent.ObjectOld.(*corev1.Pod)
			if !ok {
				appCtrlLog.V(1).Info("[pod.onUpdate] can not turn into pod, Skip.", "object", updateEvent.ObjectOld)
				return false
			}

			if podNew.GetResourceVersion() == podOld.GetResourceVersion() {
				appCtrlLog.V(1).Info("[pod.onUpdate] Skip due to resourceVersion not changed", "name", podNew.Name, "namespace", podNew.Namespace)
				return false
			}

			// ignore if it's not fluid label pod
			if !ShouldInQueue(podNew) {
				appCtrlLog.V(1).Info("[pod.onUpdate] skip due to shouldRequeue false.", "name", podNew.Name, "namespace", podNew.Namespace)
				return false
			}

			appCtrlLog.V(1).Info("[pod.onUpdate] pod requeue", "name", podNew.Name, "namespace", podNew.Namespace)
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

	log := klog.NewKlogr().WithName("app-ctrl").WithValues("pod", pod.Name, "namespace", pod.Namespace)

	// ignore if it's not fluid label pod
	if util.CheckExpectValue(pod.Labels, common.InjectSidecarDisable, common.True) {
		log.V(1).Info("Sidecar inject disabled in pod in labels, skip.", "labels", pod.Labels)
		return false
	}

	// ignore if restartPolicy is Always
	if pod.Spec.RestartPolicy == corev1.RestartPolicyAlways {
		log.V(1).Info("pod restart policy always, skip.")
		return false
	}

	// ignore if no fuse container
	exist := false
	for _, cn := range pod.Spec.Containers {
		if strings.Contains(cn.Name, common.MountContainerName) {
			exist = true
			break
		}
	}
	if !exist {
		log.V(1).Info("There are no juicefs sidecar in pod")
		return false
	}

	// ignore if pod status is not running
	if pod.Status.Phase != corev1.PodRunning || len(pod.Status.ContainerStatuses) < 2 {
		log.V(1).Info("Pod status is not running or containerStatus less than 2.")
		return false
	}

	// reconcile if all app containers exit 0 and fuse container not exit
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !strings.Contains(containerStatus.Name, common.MountContainerName) {
			log.V(1).Info("container status", "container", containerStatus.Name, "status", containerStatus)
			if containerStatus.State.Terminated == nil {
				log.V(1).Info("container not exited", "container", containerStatus.Name)
				// container not exist
				return false
			}
		}
		if strings.Contains(containerStatus.Name, common.MountContainerName) {
			if containerStatus.State.Running == nil {
				log.V(1).Info("juicefs fuse client in pod not running")
				return false
			}
		}
	}
	return true
}
