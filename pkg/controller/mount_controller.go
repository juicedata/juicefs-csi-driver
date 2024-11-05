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

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

var (
	mountCtrlLog = klog.NewKlogr().WithName("mount-controller")
)

type MountController struct {
	*k8sclient.K8sClient
}

func NewMountController(client *k8sclient.K8sClient) *MountController {
	return &MountController{client}
}

func (m MountController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	mountCtrlLog.V(1).Info("Receive pod", "name", request.Name, "namespace", request.Namespace)
	mountPod, err := m.GetPod(ctx, request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		mountCtrlLog.Error(err, "get pod error", "name", request.Name)
		return reconcile.Result{}, err
	}
	if mountPod == nil {
		mountCtrlLog.V(1).Info("pod has been deleted.", "name", request.Name)
		return reconcile.Result{}, nil
	}

	// check mount pod deleted
	if mountPod.DeletionTimestamp == nil {
		mountCtrlLog.V(1).Info("pod is not deleted", "name", mountPod.Name)
		return reconcile.Result{}, nil
	}
	if !util.ContainsString(mountPod.GetFinalizers(), common.Finalizer) {
		// do nothing
		return reconcile.Result{}, nil
	}

	// check csi node exist or not
	nodeName := mountPod.Spec.NodeName
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{common.CSINodeLabelKey: common.CSINodeLabelValue},
	}
	fieldSelector := fields.Set{
		"spec.nodeName": nodeName,
	}
	csiPods, err := m.ListPod(ctx, config.Namespace, &labelSelector, &fieldSelector)
	if err != nil {
		mountCtrlLog.Error(err, "list pod by label and field error", "labels", common.CSINodeLabelValue, "node", nodeName)
		return reconcile.Result{}, err
	}
	if len(csiPods) > 0 {
		mountCtrlLog.V(1).Info("csi node exists.", "node", nodeName)
		return reconcile.Result{}, nil
	}

	mountCtrlLog.Info("csi node did not exist. remove finalizer of pod", "node", nodeName, "name", mountPod.Name)
	// remove finalizer
	err = resource.RemoveFinalizer(ctx, m.K8sClient, mountPod, common.Finalizer)
	if err != nil {
		mountCtrlLog.Error(err, "remove finalizer of pod error", "name", mountPod.Name)
	}

	return reconcile.Result{}, err
}

func (m *MountController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("mount", mgr, controller.Options{Reconciler: m})
	if err != nil {
		return err
	}

	return c.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}, &handler.TypedEnqueueRequestForObject[*corev1.Pod]{}, predicate.TypedFuncs[*corev1.Pod]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.Pod]) bool {
			pod := event.Object
			mountCtrlLog.V(1).Info("watch pod created", "name", pod.GetName())
			// check mount pod deleted
			if pod.DeletionTimestamp == nil {
				mountCtrlLog.V(1).Info("pod is not deleted", "name", pod.Name)
				return false
			}
			if !util.ContainsString(pod.GetFinalizers(), common.Finalizer) {
				return false
			}
			return true
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.Pod]) bool {
			podNew, podOld := updateEvent.ObjectNew, updateEvent.ObjectOld
			if podNew.GetResourceVersion() == podOld.GetResourceVersion() {
				mountCtrlLog.V(1).Info("pod.onUpdateFunc Skip due to resourceVersion not changed")
				return false
			}
			// check mount pod deleted
			if podNew.DeletionTimestamp == nil {
				mountCtrlLog.V(1).Info("pod is not deleted", "name", podNew.Name)
				return false
			}
			if !util.ContainsString(podNew.GetFinalizers(), common.Finalizer) {
				return false
			}
			return true
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.Pod]) bool {
			pod := deleteEvent.Object
			mountCtrlLog.V(1).Info("watch pod deleted", "name", pod.GetName())
			// check mount pod deleted
			if pod.DeletionTimestamp == nil {
				mountCtrlLog.V(1).Info("pod is not deleted", "name", pod.Name)
				return false
			}
			if !util.ContainsString(pod.GetFinalizers(), common.Finalizer) {
				// do nothing
				return false
			}
			return true
		},
	}))
}
