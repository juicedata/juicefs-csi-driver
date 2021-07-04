/*

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
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PodDriver struct {
	Client   client.Client
	handlers map[podStatus]podHandler
}

func NewPodDriver(client client.Client) *PodDriver {
	driver := &PodDriver{
		Client:   client,
		handlers: map[podStatus]podHandler{},
	}
	driver.handlers[podReady] = driver.podReadyHandler
	driver.handlers[podError] = driver.podErrorHandler
	driver.handlers[podRunning] = driver.podRunningHandler
	driver.handlers[podDeleted] = driver.podDeletedHandler
	return driver
}

type podHandler func(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error)
type podStatus string

const (
	podReady   podStatus = "podReady"
	podError   podStatus = "podError"
	podDeleted podStatus = "podDeleted"
	podRunning podStatus = "podRunning"
)

func (p *PodDriver) Run(ctx context.Context, current *corev1.Pod) (reconcile.Result, error) {
	return p.handlers[p.getPodStatus(current)](ctx, current)
}

func (p *PodDriver) getPodStatus(pod *corev1.Pod) podStatus {
	if pod == nil {
		return podError
	}
	if pod.DeletionTimestamp != nil {
		return podDeleted
	}
	if util.IsPodError(pod) {
		return podError
	}
	if util.IsPodReady(pod) {
		return podReady
	}
	return podRunning
}

func (p *PodDriver) podErrorHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	// todo
	return reconcile.Result{}, nil
}

func (p *PodDriver) podDeletedHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	klog.V(5).Infof("Get pod %s in namespace %s is to be deleted.", pod.Name, pod.Namespace)
	if !util.ContainsString(pod.GetFinalizers(), juicefs.Finalizer) {
		// do nothing
		return reconcile.Result{}, nil
	}

	// todo

	klog.V(5).Infof("Remove finalizer of pod %s namespace %s", pod.Name, pod.Namespace)
	controllerutil.RemoveFinalizer(pod, juicefs.Finalizer)
	if err := p.Client.Update(ctx, pod); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (p *PodDriver) podReadyHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	// do nothing
	return reconcile.Result{}, nil
}

func (p *PodDriver) podRunningHandler(ctx context.Context, pod *corev1.Pod) (reconcile.Result, error) {
	// requeue
	return reconcile.Result{Requeue: true}, nil
}
