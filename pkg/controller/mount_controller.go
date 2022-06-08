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
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type MountController struct {
	*k8sclient.K8sClient
}

func (m MountController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(6).Infof("Receive pod %s %s", request.Name, request.Namespace)
	mountPod, err := m.GetPod(request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		klog.Errorf("get pod %s error: %v", request.Name, err)
		return reconcile.Result{}, err
	}
	if mountPod == nil {
		klog.V(6).Infof("pod %s has been deleted.", request.Name)
		return reconcile.Result{}, err
	}

	// check mount pod deleted
	if mountPod.DeletionTimestamp == nil {
		klog.V(6).Infof("pod %s is not deleted", mountPod.Name)
		return reconcile.Result{}, err
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
	csiPods, err := m.ListPod(config.Namespace, labelSelector)
	if err != nil {
		klog.Errorf("list pod by label %s error: %v", config.CSINodeLabelValue, err)
		return reconcile.Result{}, err
	}
	for _, csiPod := range csiPods {
		if csiPod.Spec.NodeName == nodeName {
			klog.V(6).Infof("csi node in %s exists.", nodeName)
			return reconcile.Result{}, nil
		}
	}

	klog.Infof("csi node in %s did not exist. remove finalizer of pod %s", nodeName, mountPod.Name)
	// remove finalizer
	err = util.RemoveFinalizer(m.K8sClient, mountPod, config.Finalizer)
	if err != nil {
		klog.Errorf("remove finalizer of pod %s error: %v", mountPod.Name, err)
	}

	return reconcile.Result{}, err
}
