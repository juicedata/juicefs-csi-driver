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
	"github.com/juicedata/juicefs-csi-driver/pkg/controller"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func NewManager() manager.Manager {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		MetricsBindAddress: ":9908",
	})
	if err != nil {
		klog.V(5).Infof("Could not create mgr %v", err)
		os.Exit(1)
	}

	client, err := k8s.NewClient()
	if err != nil {
		klog.V(5).Infof("Could not create k8s client %v", err)
		os.Exit(0)
	}
	err = ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(&controller.PodReconciler{
			K8sClient: client,
		})

	if err != nil {
		klog.V(5).Infof("Could not create controller: %v", err)
		os.Exit(1)
	}
	return mgr
}
