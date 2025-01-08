// Copyright 2025 Juicedata Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pods

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type podService struct {
	client     client.Client
	k8sClient  *k8sclient.K8sClient
	kubeconfig *rest.Config

	sysNamespace string
}

func (s *podService) listPVCsOfPod(ctx context.Context, pod *corev1.Pod) ([]corev1.PersistentVolumeClaim, error) {
	pvcs := make([]corev1.PersistentVolumeClaim, 0)
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}
		pvc := corev1.PersistentVolumeClaim{}
		if err := s.client.Get(ctx, types.NamespacedName{Name: v.PersistentVolumeClaim.ClaimName, Namespace: pod.Namespace}, &pvc); err != nil {
			return nil, err
		}
		pvcs = append(pvcs, pvc)
	}
	return pvcs, nil
}

func (s *podService) listPVsOfPVC(ctx context.Context, pvcs []corev1.PersistentVolumeClaim) ([]corev1.PersistentVolume, error) {
	pvs := make([]corev1.PersistentVolume, 0)
	for _, pvc := range pvcs {
		if pvc.Spec.VolumeName == "" {
			continue
		}
		pv := corev1.PersistentVolume{}
		if err := s.client.Get(ctx, types.NamespacedName{Name: pvc.Spec.VolumeName}, &pv); err != nil {
			return nil, err
		}
		pvs = append(pvs, pv)
	}
	return pvs, nil
}

func (s *podService) listMountPodOfPV(ctx context.Context, pod *corev1.Pod, pvs []corev1.PersistentVolume) ([]corev1.Pod, error) {
	mountPods := make([]corev1.Pod, 0)
	for _, pv := range pvs {
		var pods corev1.PodList
		err := s.client.List(ctx, &pods, &client.ListOptions{
			LabelSelector: utils.LabelSelectorOfMount(pv),
		})
		if err != nil {
			continue
		}
		for i, item := range pods.Items {
			for _, v := range item.Annotations {
				if strings.Contains(v, string(pod.UID)) {
					mountPods = append(mountPods, pods.Items[i])
					break
				}
			}
		}
	}
	return mountPods, nil
}

func (s *podService) getCSINode(ctx context.Context, nodeName string) (*corev1.Pod, error) {
	pods, err := s.ListCSINodePod(ctx, nodeName)
	if err != nil {
		return nil, err
	}
	if len(pods) == 0 {
		return nil, fmt.Errorf("csi node not found")
	}
	return &pods[0], nil
}

func (s *podService) getPodNode(ctx context.Context, pod *corev1.Pod) (*corev1.Node, error) {
	node := corev1.Node{}
	if err := s.client.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

func (s *podService) ListAppPods(c *gin.Context) (*ListAppPodResult, error) {
	pageSize, err := strconv.ParseInt(c.Query("pageSize"), 10, 64)
	if err != nil || pageSize == 0 {
		pageSize = 10
	}
	namespaceFilter := c.Query("namespace")
	continueToken := c.Query("continue")

	labelSelector := labels.SelectorFromSet(map[string]string{common.UniqueId: ""})
	podLists := corev1.PodList{}
	if err := s.client.List(c, &podLists, &client.ListOptions{
		LabelSelector: labelSelector,
		Limit:         pageSize,
		Continue:      continueToken,
		Namespace:     namespaceFilter,
	}); err != nil {
		c.String(500, "list pods error %v", err)
		return nil, err
	}
	pods := make([]PodExtra, 0, len(podLists.Items))
	for _, pod := range podLists.Items {
		pods = append(pods, PodExtra{Pod: &pod})
	}
	result := &ListAppPodResult{
		Pods:     pods,
		Continue: podLists.Continue,
	}
	return result, nil
}

func (s *podService) ListSysPods(c *gin.Context) (*ListSysPodResult, error) {
	pageSize, err := strconv.ParseInt(c.Query("pageSize"), 10, 64)
	if err != nil || pageSize == 0 {
		pageSize = 10
	}
	continueToken := c.Query("continue")

	nameFilter := c.Query("name")
	if nameFilter != "" {
		pod := corev1.Pod{}
		if err := s.client.Get(c, types.NamespacedName{Name: nameFilter, Namespace: s.sysNamespace}, &pod); err != nil {
			return nil, client.IgnoreNotFound(err)
		}
		result := &ListSysPodResult{
			Pods: []PodExtra{{Pod: &pod}},
		}
		return result, nil
	}

	labelSelector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      common.PodTypeKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{common.PodTypeValue, "juicefs-cache-group-worker", "juicefs-csi-driver"},
			},
		},
	}

	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		c.String(500, "convert label selector error %v", err)
		return nil, err
	}

	podLists := corev1.PodList{}
	if err := s.client.List(c, &podLists, &client.ListOptions{
		LabelSelector: selector,
		Namespace:     s.sysNamespace,
		Limit:         pageSize,
		Continue:      continueToken,
	}); err != nil {
		c.String(500, "list sys pods error %v", err)
		return nil, err
	}
	pods := make([]PodExtra, 0, len(podLists.Items))
	for _, pod := range podLists.Items {
		pods = append(pods, PodExtra{Pod: &pod})
	}
	result := &ListSysPodResult{
		Pods:     pods,
		Continue: podLists.Continue,
	}
	return result, nil
}

func (s *podService) ListCSINodePod(ctx context.Context, nodeName string) ([]corev1.Pod, error) {
	labelSelector := labels.SelectorFromSet(map[string]string{
		"app.kubernetes.io/name": "juicefs-csi-driver",
		"app":                    "juicefs-csi-node",
	})
	fieldSelector := fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName})

	pods := corev1.PodList{}
	if err := s.client.List(ctx, &pods, &client.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func (s *podService) ListPodPVs(ctx context.Context, pod *corev1.Pod) ([]corev1.PersistentVolume, error) {
	pvcs, err := s.listPVCsOfPod(ctx, pod)
	if err != nil {
		return nil, err
	}
	pvs, err := s.listPVsOfPVC(ctx, pvcs)
	if err != nil {
		return nil, err
	}
	return pvs, nil
}

func (s *podService) ListPodPVCs(ctx context.Context, pod *corev1.Pod) ([]corev1.PersistentVolumeClaim, error) {
	pvcs, err := s.listPVCsOfPod(ctx, pod)
	if err != nil {
		return nil, err
	}
	return pvcs, nil
}

func (s *podService) ListAppPodMountPods(ctx context.Context, pod *corev1.Pod) ([]corev1.Pod, error) {
	pvs, err := s.ListPodPVs(ctx, pod)
	if err != nil {
		return nil, err
	}
	mountPods, err := s.listMountPodOfPV(ctx, pod, pvs)
	if err != nil {
		return nil, err
	}
	return mountPods, nil
}

func (s *podService) ListNodeMountPods(ctx context.Context, nodeName string) ([]corev1.Pod, error) {
	labelSelector := labels.SelectorFromSet(map[string]string{
		"app.kubernetes.io/name": "juicefs-mount",
	})
	fieldSelector := fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName})

	mountpods := corev1.PodList{}
	if err := s.client.List(ctx, &mountpods, &client.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}); err != nil {
		return nil, err
	}
	return mountpods.Items, nil
}

func (s *podService) ListMountPodAppPods(ctx context.Context, mountPod *corev1.Pod) ([]corev1.Pod, error) {
	labelSelector := labels.SelectorFromSet(map[string]string{
		common.UniqueId: "",
	})
	fieldSelector := fields.SelectorFromSet(fields.Set{"spec.nodeName": mountPod.Spec.NodeName})
	nodeAppPods := corev1.PodList{}
	if err := s.client.List(ctx, &nodeAppPods, &client.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}); err != nil {
		return nil, err
	}
	podsUIDMap := make(map[string]corev1.Pod)
	for _, pod := range nodeAppPods.Items {
		podsUIDMap[string(pod.UID)] = pod
	}
	appPods := make([]corev1.Pod, 0)
	for _, v := range mountPod.Annotations {
		if uid := utils.GetTargetUID(v); uid != "" {
			if pod, ok := podsUIDMap[uid]; ok {
				appPods = append(appPods, pod)
			}
		}
	}
	return appPods, nil
}
