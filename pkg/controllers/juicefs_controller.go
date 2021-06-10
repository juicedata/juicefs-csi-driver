/*
Copyright 2021.

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

package controllers

import (
	"context"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// JuicefsReconciler reconciles a Juicefs object
type JuicefsReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mount.juicefs.com,resources=juicemount,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mount.juicefs.com,resources=juicemount/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mount.juicefs.com,resources=juicemount/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the JuiceMount object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *JuicefsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// reconcile pod
	result, err := r.reconcilePod(ctx, req)
	if err != nil {
		klog.Error(err)
	}

	// reconcile mount
	result, err = r.reconcileMount(ctx, req)
	if err != nil {
		klog.Error(err)
	}

	return result, nil
}

func (r *JuicefsReconciler) reconcileMount(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// fetching jfsMount instance
	jfsMountInstance := &mountv1.JuiceMount{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, jfsMountInstance)
	if kerrors.IsNotFound(err) {
		klog.V(5).Infof("jfsMount instance not found for %v.", req.NamespacedName)
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to fetch jfsMount %s", req.NamespacedName)
	}

	switch jfsMountInstance.Status.MountStatus {
	case mountv1.JMountInit:
		// create jfsMount pod if instance status is init
		klog.V(5).Infof("create pod of jfsMount %v.", jfsMountInstance.Name)
		mountPod := NewMountPod(jfsMountInstance)
		err = r.Client.Create(ctx, mountPod)
		if err != nil {
			klog.Errorf("create pod of jfsMount %v error: %v", jfsMountInstance.Name, err)
		}
		jfsMountInstance.Status.MountStatus = mountv1.JMountRunning
		break

	case mountv1.JMountFailed:
		// check jfsMount pod if instance status is failed
		klog.V(5).Infof("check pod of jfsMount %v.", jfsMountInstance.Name)

		mountPods := &corev1.PodList{}
		err := r.Client.List(context.TODO(), mountPods, client.InNamespace(req.Namespace),
			client.MatchingLabels{podMountRef: jfsMountInstance.Name})
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "get pod of juicefsMount %v error: %v", jfsMountInstance.Name, err)
		}

		if len(mountPods.Items) == 0 {
			klog.V(5).Infof("pod of jfsMount instance %v not found for %v. create now.", jfsMountInstance.Name)
			mountPod := NewMountPod(jfsMountInstance)
			err = r.Client.Create(ctx, mountPod)
			if err != nil {
				klog.Errorf("create pod of jfsMount %v error: %v", jfsMountInstance.Name, err)
			}
			jfsMountInstance.Status.MountStatus = mountv1.JMountRunning
			break
		}

	case mountv1.JMountSuccess:
		// do nothing if status is success
		klog.V(5).Infof("jfsMount %v status is success, do nothing.", jfsMountInstance.Name)
		return reconcile.Result{}, nil
	}

	// update mount instance status
	if err = r.Client.Update(ctx, jfsMountInstance); err != nil {
		klog.V(5).Infof("update jfsMount instance err: %v", err)
		return ctrl.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *JuicefsReconciler) reconcilePod(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// fetching pod
	mountPod := &corev1.Pod{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, mountPod)
	if kerrors.IsNotFound(err) {
		klog.V(5).Infof("jfsMount pod not found for %v.", req.NamespacedName)
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to fetch jfsMount pod %s", req.NamespacedName)
	}

	if len(mountPod.OwnerReferences) != 1 {
		return ctrl.Result{}, errors.Wrapf(err, "jfsMount pod ownerRef isn't 1 %s", req.NamespacedName)
	}

	// get pod owner mount instance
	jfsMountName := mountPod.OwnerReferences[0].Name
	jfsInstance := &mountv1.JuiceMount{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      jfsMountName,
	}, jfsInstance)
	if kerrors.IsNotFound(err) {
		klog.V(5).Infof("jfsMount not found for pod %v.", req.NamespacedName)
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to fetch jfsMount for pod %s", req.NamespacedName)
	}

	switch mountPod.Status.Phase {
	case corev1.PodRunning:
		// update jfsMount instance status when pod is ready
		for _, cn := range mountPod.Status.ContainerStatuses {
			if cn.State.Running == nil {
				jfsInstance.Status.MountStatus = mountv1.JMountRunning
				break
			}
		}
		jfsInstance.Status.MountStatus = mountv1.JMountSuccess
		break
	case corev1.PodFailed:
	case corev1.PodUnknown:
	case corev1.PodReasonUnschedulable:
		// update jfsMount instance status when pod is error
		jfsInstance.Status.MountStatus = mountv1.JMountFailed
		break
	}

	if err = r.Client.Update(ctx, jfsInstance); err != nil {
		klog.V(5).Infof("update jfsMount instance err: %v", err)
		return ctrl.Result{}, err
	}
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JuicefsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mountv1.JuiceMount{}).For(&corev1.Pod{}).
		Complete(r)
}
