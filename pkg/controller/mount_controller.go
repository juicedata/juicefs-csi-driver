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
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

var (
	mountCtrlLog = klog.NewKlogr().WithName("mount-controller")
)

type MountController struct {
	*k8sclient.K8sClient
}

func NewMountController(client *k8sclient.K8sClient) *MountController {
	return &MountController{client}
}

func (m MountController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	mountCtrlLog.V(1).Info("Receive pod", "name", request.Name, "namespace", request.Namespace)
	mountPod, err := m.GetPod(ctx, request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		mountCtrlLog.Error(err, "get pod error", "name", request.Name)
		return reconcile.Result{}, err
	}
	if mountPod == nil {
		mountCtrlLog.V(1).Info("pod has been deleted.", "name", request.Name)
		return reconcile.Result{}, nil
	}

	// Scenario 1: Handle pending mount pod without nodeName (not scheduled yet)
	// This handles the case where mount pod is stuck in Pending state due to custom scheduler
	if mountPod.DeletionTimestamp == nil &&
		mountPod.Status.Phase == corev1.PodPending &&
		mountPod.Spec.NodeName == "" {
		return m.handlePendingMountPod(ctx, mountPod)
	}

	// Scenario 2: Handle mount pod being deleted (original logic)
	// check mount pod deleted
	if mountPod.DeletionTimestamp == nil {
		mountCtrlLog.V(1).Info("pod is not deleted and not pending", "name", mountPod.Name)
		return reconcile.Result{}, nil
	}
	if !util.ContainsString(mountPod.GetFinalizers(), common.Finalizer) {
		// do nothing
		return reconcile.Result{}, nil
	}

	// check csi node exist or not
	nodeName := mountPod.Spec.NodeName
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{common.CSINodeLabelKey: common.CSINodeLabelValue},
	}
	fieldSelector := fields.Set{
		"spec.nodeName": nodeName,
	}
	csiPods, err := m.ListPod(ctx, config.Namespace, &labelSelector, &fieldSelector)
	if err != nil {
		mountCtrlLog.Error(err, "list pod by label and field error", "labels", common.CSINodeLabelValue, "node", nodeName)
		return reconcile.Result{}, err
	}
	if len(csiPods) > 0 {
		mountCtrlLog.V(1).Info("csi node exists.", "node", nodeName)
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil // requeue to avoid mount pod is removed before csi node when node is cordoned
	}

	mountCtrlLog.Info("csi node did not exist. remove finalizer of pod", "node", nodeName, "name", mountPod.Name)
	// remove finalizer
	err = resource.RemoveFinalizer(ctx, m.K8sClient, mountPod, common.Finalizer)
	if err != nil {
		mountCtrlLog.Error(err, "remove finalizer of pod error", "name", mountPod.Name)
	}

	return reconcile.Result{}, err
}

// handlePendingMountPod handles pending mount pod that is not scheduled yet (nodeName is empty)
// It checks if all referenced app pods are deleted, and if so, deletes the mount pod
func (m *MountController) handlePendingMountPod(ctx context.Context, mountPod *corev1.Pod) (reconcile.Result, error) {
	mountCtrlLog.V(1).Info("Handling pending mount pod", "name", mountPod.Name, "namespace", mountPod.Namespace)

	// Get target node name from mount pod's nodeSelector
	// This helps optimize pod search by limiting to a specific node
	targetNodeName := ""
	if mountPod.Spec.NodeSelector != nil {
		targetNodeName = mountPod.Spec.NodeSelector["kubernetes.io/hostname"]
	}

	// Check all references in mount pod annotations
	var existingRefs int
	for k, target := range mountPod.Annotations {
		if k == util.GetReferenceKey(target) {
			// Extract pod UID from target path
			// Target format: /var/lib/kubelet/pods/<pod-uid>/volumes/kubernetes.io~csi/<volume-id>/mount
			targetUid := getPodUid(target)
			if targetUid == "" {
				mountCtrlLog.V(1).Info("Could not extract pod UID from target", "target", target)
				continue
			}

			// Find pod by UID, optimized by searching only on target node
			targetPod, err := m.GetPodByUidAndNode(ctx, targetUid, targetNodeName)
			if err != nil {
				mountCtrlLog.Error(err, "Failed to search for app pod by UID", "appPodUid", targetUid)
				return reconcile.Result{}, err
			}

			if targetPod != nil {
				mountCtrlLog.V(1).Info("Referenced app pod still exists",
					"appPod", targetPod.Name,
					"appPodNamespace", targetPod.Namespace,
					"appPodUid", targetUid,
					"mountPod", mountPod.Name)
				existingRefs++
			} else {
				mountCtrlLog.Info("Referenced app pod has been deleted",
					"appPodUid", targetUid,
					"mountPod", mountPod.Name)
			}
		}
	}

	// If no app pods reference this mount pod anymore, delete it
	if existingRefs == 0 {
		mountCtrlLog.Info("No app pods reference this pending mount pod, deleting it",
			"mountPod", mountPod.Name,
			"namespace", mountPod.Namespace)

		if err := m.DeletePod(ctx, mountPod); err != nil {
			mountCtrlLog.Error(err, "Failed to delete pending mount pod", "mountPod", mountPod.Name)
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// Still has references, check again after 10 seconds
	mountCtrlLog.V(1).Info("Mount pod still has references",
		"mountPod", mountPod.Name,
		"existingRefs", existingRefs)
	return reconcile.Result{RequeueAfter: 10 * time.Minute}, nil
}

// GetPodByUidAndNode finds a pod by its UID, optimized by label and optional node filter
func (m *MountController) GetPodByUidAndNode(ctx context.Context, uid string, nodeName string) (*corev1.Pod, error) {
	// Use label selector to filter app pods (those with juicefs-uniqueid label)
	labelSelector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      common.UniqueId,
			Operator: metav1.LabelSelectorOpExists,
		}},
	}
	labelSelectorStr, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return nil, err
	}

	listOptions := metav1.ListOptions{
		LabelSelector: labelSelectorStr.String(),
	}

	// Add node filter if nodeName is provided (optimized path)
	if nodeName != "" {
		listOptions.FieldSelector = fields.Set{"spec.nodeName": nodeName}.AsSelector().String()
	}

	// List pods with filters
	pods, err := m.K8sClient.CoreV1().Pods("").List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	if nodeName != "" {
		mountCtrlLog.V(1).Info("Searching for pod by UID on specific node with label filter",
			"uid", uid,
			"nodeName", nodeName,
			"filteredPods", len(pods.Items))
	} else {
		mountCtrlLog.V(1).Info("Searching for pod by UID across all nodes with label filter",
			"uid", uid,
			"filteredPods", len(pods.Items))
	}

	// Find pod with matching UID
	for i := range pods.Items {
		if string(pods.Items[i].UID) == uid {
			return &pods.Items[i], nil
		}
	}

	// Pod not found
	return nil, nil
}

func shouldInQueue(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if v, ok := pod.Labels[common.PodTypeKey]; !ok || v != common.PodTypeValue {
		return false
	}

	return util.ContainsString(pod.GetFinalizers(), common.Finalizer)
}

func (m *MountController) SetupWithManager(mgr ctrl.Manager) error {
	mountCtrlLog.V(1).Info("SetupWithManager", "name", "mount-controller")
	c, err := controller.New("mount", mgr, controller.Options{Reconciler: m})
	if err != nil {
		return err
	}

	return c.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}, &handler.TypedEnqueueRequestForObject[*corev1.Pod]{}, predicate.TypedFuncs[*corev1.Pod]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.Pod]) bool {
			pod := event.Object
			mountCtrlLog.V(1).Info("watch pod created", "name", pod.GetName())

			// Scenario 1: Mount pod being deleted (original logic)
			if pod.DeletionTimestamp != nil {
				return shouldInQueue(pod)
			}

			// Scenario 2: Pending mount pod not scheduled yet (new logic for handling orphaned pending pods)
			if pod.Status.Phase == corev1.PodPending && pod.Spec.NodeName == "" {
				return shouldInQueue(pod)
			}

			return false
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.Pod]) bool {
			podNew, podOld := updateEvent.ObjectNew, updateEvent.ObjectOld
			if podNew.GetResourceVersion() == podOld.GetResourceVersion() {
				mountCtrlLog.V(1).Info("pod.onUpdateFunc Skip due to resourceVersion not changed")
				return false
			}

			// Scenario 1: Mount pod being deleted (original logic)
			if podNew.DeletionTimestamp != nil {
				return shouldInQueue(podNew)
			}

			// Scenario 2: Pending mount pod not scheduled yet
			if podNew.Status.Phase == corev1.PodPending && podNew.Spec.NodeName == "" {
				return shouldInQueue(podNew)
			}

			return false
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.Pod]) bool {
			pod := deleteEvent.Object
			mountCtrlLog.V(1).Info("watch pod deleted", "name", pod.GetName())
			// Only handle pods being deleted with finalizer
			if pod.DeletionTimestamp == nil {
				return false
			}
			return shouldInQueue(pod)
		},
	}))
}
