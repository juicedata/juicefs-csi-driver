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

type MountController struct {
	*k8sclient.K8sClient
}

func NewMountController(client *k8sclient.K8sClient) *MountController {
	return &MountController{client}
}

func (m MountController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(6).Infof("Receive pod %s %s", request.Name, request.Namespace)
	mountPod, err := m.GetPod(ctx, request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		klog.Errorf("get pod %s error: %v", request.Name, err)
		return reconcile.Result{}, err
	}
	if mountPod == nil {
		klog.V(6).Infof("pod %s has been deleted.", request.Name)
		return reconcile.Result{}, nil
	}

	// check mount pod deleted
	if mountPod.DeletionTimestamp == nil {
		klog.V(6).Infof("pod %s is not deleted", mountPod.Name)
		return reconcile.Result{}, nil
	}
	if !util.ContainsString(mountPod.GetFinalizers(), config.Finalizer) {
		// do nothing
		return reconcile.Result{}, nil
	}

	// check csi node exist or not
	nodeName := mountPod.Spec.NodeName
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{config.CSINodeLabelKey: config.CSINodeLabelValue},
	}
	fieldSelector := fields.Set{
		"spec.nodeName": nodeName,
	}
	csiPods, err := m.ListPod(ctx, config.Namespace, &labelSelector, &fieldSelector)
	if err != nil {
		klog.Errorf("list pod by label %s and field %s error: %v", config.CSINodeLabelValue, nodeName, err)
		return reconcile.Result{}, err
	}
	if len(csiPods) > 0 {
		klog.V(6).Infof("csi node in %s exists.", nodeName)
		return reconcile.Result{}, nil
	}

	klog.Infof("csi node in %s did not exist. remove finalizer of pod %s", nodeName, mountPod.Name)
	// remove finalizer
	err = util.RemoveFinalizer(ctx, m.K8sClient, mountPod, config.Finalizer)
	if err != nil {
		klog.Errorf("remove finalizer of pod %s error: %v", mountPod.Name, err)
	}

	return reconcile.Result{}, err
}

func (m *MountController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("mount", mgr, controller.Options{Reconciler: m})
	if err != nil {
		return err
	}

	return c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			pod := event.Object.(*corev1.Pod)
			klog.V(6).Infof("watch pod %s created", pod.GetName())
			// check mount pod deleted
			if pod.DeletionTimestamp == nil {
				klog.V(6).Infof("pod %s is not deleted", pod.Name)
				return false
			}
			if !util.ContainsString(pod.GetFinalizers(), config.Finalizer) {
				return false
			}
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			podNew, ok := updateEvent.ObjectNew.(*corev1.Pod)
			klog.V(6).Infof("watch pod %s updated", podNew.GetName())
			if !ok {
				klog.V(6).Infof("pod.onUpdateFunc Skip object: %v", updateEvent.ObjectNew)
				return false
			}

			podOld, ok := updateEvent.ObjectOld.(*corev1.Pod)
			if !ok {
				klog.V(6).Infof("pod.onUpdateFunc Skip object: %v", updateEvent.ObjectOld)
				return false
			}

			if podNew.GetResourceVersion() == podOld.GetResourceVersion() {
				klog.V(6).Info("pod.onUpdateFunc Skip due to resourceVersion not changed")
				return false
			}
			// check mount pod deleted
			if podNew.DeletionTimestamp == nil {
				klog.V(6).Infof("pod %s is not deleted", podNew.Name)
				return false
			}
			if !util.ContainsString(podNew.GetFinalizers(), config.Finalizer) {
				return false
			}
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			pod := deleteEvent.Object.(*corev1.Pod)
			klog.V(6).Infof("watch pod %s deleted", pod.GetName())
			// check mount pod deleted
			if pod.DeletionTimestamp == nil {
				klog.V(6).Infof("pod %s is not deleted", pod.Name)
				return false
			}
			if !util.ContainsString(pod.GetFinalizers(), config.Finalizer) {
				// do nothing
				return false
			}
			return true
		},
	})
}
