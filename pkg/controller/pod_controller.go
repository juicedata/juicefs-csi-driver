/*
 Copyright 2023 Juicedata Inc

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
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog/v2"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
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
)

var (
	podCtrlLog = klog.NewKlogr().WithName("pod-controller")
)

type PodController struct {
	*k8sclient.K8sClient
}

func NewPodController(client *k8sclient.K8sClient) *PodController {
	return &PodController{client}
}

func (m *PodController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	podCtrlLog.V(1).Info("Receive pod", "name", request.Name, "namespace", request.Namespace)
	ctx, cancel := context.WithTimeout(ctx, config.ReconcileTimeout)
	defer cancel()
	mountPod, err := m.GetPod(ctx, request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		podCtrlLog.Error(err, "get pod error", "name", request.Name)
		return reconcile.Result{}, err
	}
	if mountPod == nil {
		podCtrlLog.V(1).Info("pod has been deleted.", "name", request.Name)
		return reconcile.Result{}, nil
	}
	if mountPod.Spec.NodeName != config.NodeName && mountPod.Spec.NodeSelector["kubernetes.io/hostname"] != config.NodeName {
		podCtrlLog.V(1).Info("pod is not on node, skipped", "namespace", mountPod.Namespace, "name", mountPod.Name, "node", config.NodeName)
		return reconcile.Result{}, nil
	}

	// get mount info
	mit := newMountInfoTable()
	if err := mit.parse(); err != nil {
		podCtrlLog.Error(err, "doReconcile ParseMountInfo error")
		return reconcile.Result{}, err
	}

	labelSelector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      common.UniqueId,
			Operator: metav1.LabelSelectorOpExists,
		}},
	}
	fieldSelector := &fields.Set{"spec.nodeName": config.NodeName}
	appPodLists, err := m.K8sClient.ListPod(ctx, "", &labelSelector, fieldSelector)
	if err != nil {
		podCtrlLog.Error(err, "reconcile ListPod error")
		return reconcile.Result{}, err
	}

	mountLabelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.PodTypeKey: common.PodTypeValue,
		},
	}
	mountPodLists, err := m.K8sClient.ListPod(ctx, "", &mountLabelSelector, fieldSelector)
	if err != nil {
		podCtrlLog.Error(err, "reconcile ListPod error")
		return reconcile.Result{}, err
	}

	mounter := mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      k8sexec.New(),
	}

	podDriver := NewPodDriver(m.K8sClient, mounter, &corev1.PodList{
		Items: append(appPodLists, mountPodLists...),
	})
	podDriver.SetMountInfo(*mit)

	result, err := podDriver.Run(ctx, mountPod)
	if err != nil {
		podCtrlLog.Error(err, "Driver check pod error", "podName", mountPod.Name)
		return reconcile.Result{}, err
	}
	if mountPod.Annotations[common.DeleteDelayAtKey] != "" {
		// if mount pod set delay deleted, requeue after delay time
		delayAtStr := mountPod.Annotations[common.DeleteDelayAtKey]
		delayAt, err := util.GetTime(delayAtStr)
		if err != nil {
			return reconcile.Result{}, err
		}
		now := time.Now()
		requeueAfter := delayAt.Sub(now)
		if delayAt.Before(now) {
			requeueAfter = 0
		}
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: requeueAfter,
		}, nil
	}
	requeueAfter := result.RequeueAfter
	if !result.RequeueImmediately {
		requeueAfter = 10 * time.Minute
	}
	return reconcile.Result{
		Requeue:      true,
		RequeueAfter: requeueAfter,
	}, nil
}

func (m *PodController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("mount", mgr, controller.Options{Reconciler: m})
	if err != nil {
		return err
	}

	return c.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}, &handler.TypedEnqueueRequestForObject[*corev1.Pod]{}, predicate.TypedFuncs[*corev1.Pod]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.Pod]) bool {
			pod := event.Object
			if pod.Spec.NodeName != config.NodeName && pod.Spec.NodeSelector["kubernetes.io/hostname"] != config.NodeName {
				return false
			}
			podCtrlLog.V(1).Info("watch pod created", "podName", pod.GetName())
			return true
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.Pod]) bool {
			podNew, podOld := updateEvent.ObjectNew, updateEvent.ObjectOld
			if podNew.GetResourceVersion() == podOld.GetResourceVersion() {
				podCtrlLog.V(1).Info("pod.onUpdateFunc Skip due to resourceVersion not changed")
				return false
			}
			return true
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.Pod]) bool {
			pod := deleteEvent.Object
			if pod.Spec.NodeName != config.NodeName && pod.Spec.NodeSelector["kubernetes.io/hostname"] != config.NodeName {
				return false
			}
			podCtrlLog.V(1).Info("watch pod deleted", "podName", pod.GetName())
			return true
		},
	}))
}
