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

package apps

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8scontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
)

func NewManager() manager.Manager {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		MetricsBindAddress: ":9908",
	})
	if err != nil {
		klog.V(5).Infof("Could not create mgr %v", err)
		os.Exit(1)
	}

	k8sclient, err := k8s.NewClient()
	if err != nil {
		klog.V(5).Infof("Could not create k8s client %v", err)
		os.Exit(0)
	}

	ctl, err := k8scontroller.New("juicefs", mgr,
		k8scontroller.Options{
			Reconciler: controller.PodReconciler{
				K8sClient: k8sclient,
			},
		},
	)
	// only watch pod with juicefs-mount label
	err = ctl.Watch(
		&source.Kind{Type: &corev1.Pod{}},
		handler.EnqueueRequestsFromMapFunc(objToReconcileRequest(config.PodTypeKey)),
	)

	if err != nil {
		klog.V(5).Infof("Could not create controller: %v", err)
		os.Exit(1)
	}
	return mgr
}

func objToReconcileRequest(objLabelKey string) func(object client.Object) []reconcile.Request {
	return func(object client.Object) []reconcile.Request {
		labels := object.GetLabels()
		objLabelValue, isSet := labels[objLabelKey]
		if !isSet || objLabelValue != config.PodTypeValue {
			return nil
		}
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: object.GetNamespace(),
					Name:      object.GetName(),
				},
			},
		}
	}
}
