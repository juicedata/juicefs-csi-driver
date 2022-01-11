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
	"strconv"
	"time"

	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PodReconciler struct {
	mount.SafeFormatAndMount
	*k8sclient.K8sClient
}

func (p PodReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(6).Infof("Receive event. name: %s, namespace: %s", request.Name, request.Namespace)

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

	podDriver := NewPodDriver(p.K8sClient, p.SafeFormatAndMount)
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

func StartReconciler() error {
	// gen kubelet client
	port, err := strconv.Atoi(config.KubeletPort)
	if err != nil {
		return err
	}
	kc, err := newKubeletClient(config.HostIp, port)
	if err != nil {
		return err
	}

	// gen podDriver
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		klog.V(5).Infof("Could not create k8s client %v", err)
		return err
	}

	mounter := mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      k8sexec.New(),
	}

	podDriver := NewPollingPodDriver(k8sClient, mounter)

	go doReconcile(kc, podDriver)
	return nil
}

func doReconcile(kc *kubeletClient, driver *PodDriver) {
	for {
		podList, err := kc.GetNodeRunningPods()
		if err != nil {
			klog.Errorf("doReconcile GetNodeRunningPods: %v", err)
			goto finish
		}
		if err := driver.mit.parse(); err != nil {
			klog.Errorf("doReconcile ParseMountInfo: %v", err)
			goto finish
		}
		driver.mit.setPodsStatus(podList)

		for _, pod := range podList.Items {
			// check label
			if value, ok := pod.Labels[config.PodTypeKey]; !ok || value != config.PodTypeValue {
				continue
			}
			if pod.Namespace != config.Namespace {
				continue
			}
			driver.Run(context.Background(), &pod)
		}

	finish:
		time.Sleep(time.Duration(config.ReconcilerInterval) * time.Second)
	}
}
