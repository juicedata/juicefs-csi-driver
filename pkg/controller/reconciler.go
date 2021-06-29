package controller

import (
	"context"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PodReconciler struct {
	client.Client
}

func (p PodReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(6).Infof("Receive event. name: %s, namespace: %s", request.Name, request.Namespace)

	// fetch pod
	pod := &corev1.Pod{}
	requeue, err := p.fetchPod(ctx, request.NamespacedName, pod)
	if err != nil || requeue {
		return ctrl.Result{}, err
	}

	// check label
	if value, ok := pod.Labels[juicefs.PodTypeKey]; !ok || value != juicefs.PodTypeValue {
		klog.V(6).Info("Pod %s is not juicefs mount pod. ignore.", pod.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	// check nodeName
	if pod.Spec.NodeName != juicefs.NodeName {
		klog.V(6).Infof("Pod %s is not this node: %s. ignore.", pod.Name, juicefs.NodeName)
		return reconcile.Result{Requeue: true}, nil
	}

	podDriver := NewPodDriver(p.Client)
	return podDriver.Run(ctx, pod)
}

func (p *PodReconciler) fetchPod(ctx context.Context, name types.NamespacedName, pod *corev1.Pod) (bool, error) {
	if err := p.Get(ctx, name, pod); err != nil {
		klog.V(6).Infof("Get pod namespace %s name %s failed: %v", name.Namespace, name.Name, err)
		return true, err
	}
	return false, nil
}
