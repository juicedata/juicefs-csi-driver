/*
 Copyright 2023 Juicedata Inc

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
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
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

type PodController struct {
	*k8sclient.K8sClient
}

func NewPodController(client *k8sclient.K8sClient) *PodController {
	return &PodController{client}
}

func (m *PodController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
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
	if mountPod.Spec.NodeName != config.NodeName && mountPod.Spec.NodeSelector["kubernetes.io/hostname"] != config.NodeName {
		klog.V(6).Infof("pod %s/%s is not on node %s, skipped", mountPod.Namespace, mountPod.Name, config.NodeName)
		return reconcile.Result{}, nil
	}

	// get mount info
	mit := newMountInfoTable()
	if err := mit.parse(); err != nil {
		klog.Errorf("doReconcile ParseMountInfo: %v", err)
		return reconcile.Result{}, err
	}

	// get app pod list
	relatedPVs := []*corev1.PersistentVolume{}
	pvcNamespaces := []string{}
	pvs, err := m.K8sClient.ListPersistentVolumes(ctx, nil, nil)
	if err != nil {
		klog.Errorf("doReconcile ListPV: %v", err)
		return reconcile.Result{}, err
	}
	uniqueId := mountPod.Annotations[config.UniqueId]
	for _, p := range pvs {
		p := p
		if p.Spec.CSI == nil || p.Spec.CSI.Driver != config.DriverName {
			continue
		}
		if uniqueId != "" && (p.Spec.CSI.VolumeHandle == uniqueId || p.Spec.StorageClassName == uniqueId) {
			relatedPVs = append(relatedPVs, &p)
		}
	}
	if len(relatedPVs) == 0 {
		return reconcile.Result{}, fmt.Errorf("can not get pv by uniqueId %s, mount pod: %s", uniqueId, mountPod.Name)
	}
	for _, pv := range relatedPVs {
		if pv != nil {
			if pv.Spec.ClaimRef != nil {
				pvcNamespaces = append(pvcNamespaces, pv.Spec.ClaimRef.Namespace)
			}
		}
	}

	if len(pvcNamespaces) == 0 {
		klog.Errorf("can not get pvc based on mount pod %s/%s: %v", mountPod.Namespace, mountPod.Name, err)
		return reconcile.Result{}, err
	}

	labelSelector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      config.UniqueId,
			Operator: metav1.LabelSelectorOpExists,
		}},
	}
	podLists := []corev1.Pod{}
	for _, pvcNamespace := range pvcNamespaces {
		podList, err := m.K8sClient.ListPod(ctx, pvcNamespace, &labelSelector, nil)
		if err != nil {
			klog.Errorf("doReconcile ListPod: %v", err)
			return reconcile.Result{}, err
		}
		podLists = append(podLists, podList...)
	}

	mounter := mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      k8sexec.New(),
	}

	podDriver := NewPodDriver(m.K8sClient, mounter)
	podDriver.SetMountInfo(*mit)
	podDriver.mit.setPodsStatus(&corev1.PodList{Items: podLists})

	err = podDriver.Run(ctx, mountPod)
	if err != nil {
		klog.Errorf("Driver check pod %s error: %v", mountPod.Name, err)
		return reconcile.Result{}, err
	}
	if mountPod.Annotations[config.DeleteDelayAtKey] != "" {
		// if mount pod set delay deleted, requeue after delay time
		delayAtStr := mountPod.Annotations[config.DeleteDelayAtKey]
		delayAt, err := util.GetTime(delayAtStr)
		if err != nil {
			return reconcile.Result{}, err
		}
		now := time.Now()
		requeueAfter := delayAt.Sub(now)
		if delayAt.Before(now) {
			requeueAfter = 0
		}
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: requeueAfter,
		}, nil
	}
	return reconcile.Result{
		Requeue:      true,
		RequeueAfter: 10 * time.Minute,
	}, nil
}

func (m *PodController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("mount", mgr, controller.Options{Reconciler: m})
	if err != nil {
		return err
	}

	return c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			pod := event.Object.(*corev1.Pod)
			klog.V(6).Infof("watch pod %s created", pod.GetName())
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
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			pod := deleteEvent.Object.(*corev1.Pod)
			klog.V(6).Infof("watch pod %s deleted", pod.GetName())
			return true
		},
	})
}
