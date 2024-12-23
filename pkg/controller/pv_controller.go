/*
 Copyright 2024 Juicedata Inc

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var (
	pvCtrlLog = klog.NewKlogr().WithName("pv-controller")

	// used secret set
	watchedSecrets = map[string]struct{}{}
)

type PVController struct {
	*k8sclient.K8sClient
}

func NewPVController(client *k8sclient.K8sClient) *PVController {
	return &PVController{client}
}

func (m *PVController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	pvCtrlLog.V(1).Info("Receive pv", "name", request.Name)
	pv, err := m.GetPersistentVolume(ctx, request.Name)
	if err != nil {
		pvCtrlLog.Error(err, "Failed to get pv", "name", request.Name)
		return reconcile.Result{}, err
	}
	if pv.Spec.CSI != nil && pv.Spec.CSI.NodePublishSecretRef != nil {
		secretName := pv.Spec.CSI.NodePublishSecretRef.Name
		secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Namespace
		watchedSecrets[fmt.Sprintf("%s/%s", secretNamespace, secretName)] = struct{}{}
		// for first time, we need to refresh the secret init config in pv controller
		if err := refreshSecretInitConfig(ctx, m.K8sClient, secretName, secretNamespace); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func shouldPVInQueue(pv *corev1.PersistentVolume) bool {
	if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
		if pv.Spec.CSI.NodePublishSecretRef == nil {
			return false
		}
		secretName := pv.Spec.CSI.NodePublishSecretRef.Name
		secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Namespace
		if _, ok := watchedSecrets[fmt.Sprintf("%s/%s", secretNamespace, secretName)]; !ok {
			return true
		}
		return false
	}
	return false
}

func (m *PVController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("pv", mgr, controller.Options{Reconciler: m})
	if err != nil {
		return err
	}

	// list all pv and add secret to watch list
	// only run once
	pvs, err := m.ListPersistentVolumes(context.Background(), nil, nil)
	if err != nil {
		return err
	}
	for _, pv := range pvs {
		if shouldPVInQueue(&pv) {
			if pv.Spec.CSI != nil && pv.Spec.CSI.NodePublishSecretRef != nil {
				secretName := pv.Spec.CSI.NodePublishSecretRef.Name
				secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Namespace
				if _, ok := watchedSecrets[secretName]; !ok {
					watchedSecrets[fmt.Sprintf("%s/%s", secretNamespace, secretName)] = struct{}{}
				}
			}
		}
	}

	return c.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolume{}, &handler.TypedEnqueueRequestForObject[*corev1.PersistentVolume]{}, predicate.TypedFuncs[*corev1.PersistentVolume]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.PersistentVolume]) bool {
			pv := event.Object
			return shouldPVInQueue(pv)
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.PersistentVolume]) bool {
			pvNew, pvOld := updateEvent.ObjectNew, updateEvent.ObjectOld
			if pvNew.GetResourceVersion() == pvOld.GetResourceVersion() {
				pvCtrlLog.V(1).Info("pv.onUpdateFunc Skip due to resourceVersion not changed")
				return false
			}
			return shouldPVInQueue(pvNew)
		},
	}))
}
