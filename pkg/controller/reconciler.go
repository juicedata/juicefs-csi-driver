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
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
)

type PodReconciler struct {
	k8sclient.K8sClient
}

func (p PodReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(5).Infof("Receive event. name: %s, namespace: %s", request.Name, request.Namespace)

	// fetch pod
	requeue, pod, err := p.fetchPod(request.NamespacedName)
	if err != nil || requeue {
		return ctrl.Result{}, err
	}

	// check label
	if value, ok := pod.Labels[config.PodTypeKey]; !ok || value != config.PodTypeValue {
		klog.V(6).Infof("Pod %s is not JuiceFS mount pod. ignore.", pod.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	// check nodeName
	if pod.Spec.NodeName != config.NodeName {
		klog.V(6).Infof("Pod %s is not this node: %s. ignore.", pod.Name, config.NodeName)
		return reconcile.Result{Requeue: true}, nil
	}

	podDriver := NewPodDriver(p.K8sClient)
	return podDriver.Run(ctx, pod)
}

func (p *PodReconciler) fetchPod(name types.NamespacedName) (bool, *corev1.Pod, error) {
	if reach, err := p.GetPod(name.Name, name.Namespace); err != nil {
		klog.V(6).Infof("Get pod namespace %s name %s failed: %v", name.Namespace, name.Name, err)
		return true, nil, err
	} else {
		return false, reach, nil
	}
}
