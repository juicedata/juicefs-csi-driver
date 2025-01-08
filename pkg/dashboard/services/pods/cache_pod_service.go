/*
 Copyright 2025 Juicedata Inc

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

package pods

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	podLog = klog.NewKlogr().WithName("PodService/Cache")
)

type CachePodService struct {
	*podService

	csiNodeLock  sync.RWMutex
	csiNodeIndex map[string]types.NamespacedName
	sysIndexes   *utils.TimeOrderedIndexes[corev1.Pod]
	appIndexes   *utils.TimeOrderedIndexes[corev1.Pod]
	PairLock     sync.RWMutex
	Pairs        map[types.NamespacedName]types.NamespacedName
}

func (s *CachePodService) filterPVsOfPod(pod PodExtra, filter string) bool {
	if filter == "" {
		return true
	}
	for _, pv := range pod.Pvs {
		if strings.Contains(pv.Name, filter) {
			return true
		}
	}
	return false
}

func (s *CachePodService) filterMountPodsOfPod(pod PodExtra, filter string) bool {
	for _, mountPod := range pod.MountPods {
		if strings.Contains(mountPod.Name, filter) {
			return true
		}
	}
	return false
}

func (s *CachePodService) filterCSINodeOfPod(pod PodExtra, filter string) bool {
	if filter == "" {
		return true
	}
	if pod.Spec.NodeName == "" {
		return false
	}

	return strings.Contains(pod.CsiNode.Name, filter)
}

func (s *CachePodService) ListAppPods(c *gin.Context) (*ListAppPodResult, error) {
	pageSize, err := strconv.ParseUint(c.Query("pageSize"), 10, 64)
	if err != nil || pageSize == 0 {
		c.String(400, "invalid page size")
		return nil, err
	}
	current, err := strconv.ParseUint(c.Query("current"), 10, 64)
	if err != nil || current == 0 {
		c.String(400, "invalid current page")
		return nil, err
	}
	descend := c.Query("order") != "ascend"
	nameFilter := c.Query("name")
	namespaceFilter := c.Query("namespace")
	pvFilter := c.Query("pv")
	mountpodFilter := c.Query("mountpod")
	csiNodeFilter := c.Query("csinode")

	pods := make([]PodExtra, 0, s.appIndexes.Length())
	for name := range s.appIndexes.Iterate(c, descend) {
		var pod corev1.Pod
		if err := s.client.Get(c, name, &pod); err == nil &&
			(nameFilter == "" || strings.Contains(pod.Name, nameFilter)) &&
			(namespaceFilter == "" || strings.Contains(pod.Namespace, namespaceFilter)) &&
			(utils.IsAppPod(&pod) || utils.IsAppPodShouldList(c, s.client, &pod)) {
			extraPods := PodExtra{Pod: &pod}
			extraPods.Pvcs, err = s.listPVCsOfPod(c, &pod)
			if err != nil {
				podLog.Error(err, "list pvs of pod error", "namespace", pod.Namespace, "name", pod.Name)
				continue
			}
			extraPods.Pvs, err = s.listPVsOfPVC(c, extraPods.Pvcs)
			if err != nil {
				podLog.Error(err, "list pvcs of pod error", "namespace", pod.Namespace, "name", pod.Name)
				continue
			}
			extraPods.MountPods, err = s.listMountPodOfPV(c, &pod, extraPods.Pvs)
			if err != nil {
				podLog.Error(err, "list mountpods of pod error", "namespace", pod.Namespace, "name", pod.Name)
				continue
			}
			extraPods.CsiNode, err = s.getCSINode(c, pod.Spec.NodeName)
			if err != nil {
				podLog.Error(err, "list csi nodes of pod error", "namespace", pod.Namespace, "name", pod.Name)
				continue
			}
			extraPods.Node, err = s.getPodNode(c, &pod)
			if err != nil {
				podLog.Error(err, "list nodes of pod error", "namespace", pod.Namespace, "name", pod.Name)
				continue
			}
			pods = append(pods, extraPods)
		}
	}
	if pvFilter != "" || mountpodFilter != "" || csiNodeFilter != "" {
		filterdPods := make([]PodExtra, 0, len(pods))
		for _, pod := range pods {
			if s.filterPVsOfPod(pod, pvFilter) && s.filterMountPodsOfPod(pod, mountpodFilter) && s.filterCSINodeOfPod(pod, csiNodeFilter) {
				filterdPods = append(filterdPods, pod)
			}
		}
		pods = filterdPods
	}
	result := &ListAppPodResult{
		Total: len(pods),
		Pods:  make([]PodExtra, 0),
	}
	startIndex := (current - 1) * pageSize
	if startIndex >= uint64(len(pods)) {
		return result, nil
	}
	endIndex := startIndex + pageSize
	if endIndex > uint64(len(pods)) {
		endIndex = uint64(len(pods))
	}
	result.Pods = pods[startIndex:endIndex]
	for _, pod := range result.Pods {
		if pod.Spec.NodeName != "" {
			var node corev1.Node
			err := s.client.Get(c, types.NamespacedName{Name: pod.Spec.NodeName}, &node)
			if err != nil {
				podLog.Error(err, "get node error", "node", pod.Spec.NodeName)
			} else {
				pod.Node = &node
			}
		}
	}
	return result, nil
}

func (s *CachePodService) ListSysPods(c *gin.Context) (*ListSysPodResult, error) {
	pageSize, err := strconv.ParseUint(c.Query("pageSize"), 10, 64)
	if err != nil || pageSize == 0 {
		pageSize = 10
	}
	current, err := strconv.ParseUint(c.Query("current"), 10, 64)
	if err != nil || current == 0 {
		c.String(400, "invalid current page")
		return nil, fmt.Errorf("invalid current page")
	}
	descend := c.Query("order") == "descend"
	nameFilter := c.Query("name")
	namespaceFilter := c.Query("namespace")
	nodeFilter := c.Query("node")
	required := func(pod *corev1.Pod) bool {
		return (nameFilter == "" || strings.Contains(pod.Name, nameFilter)) &&
			(namespaceFilter == "" || strings.Contains(pod.Namespace, namespaceFilter)) &&
			(nodeFilter == "" || strings.Contains(pod.Spec.NodeName, nodeFilter))

	}
	pods := make([]*corev1.Pod, 0, s.sysIndexes.Length())
	for name := range s.sysIndexes.Iterate(c, descend) {
		var pod corev1.Pod
		if err := s.client.Get(c, name, &pod); err == nil && required(&pod) {
			pods = append(pods, &pod)
		}
	}
	result := &ListSysPodResult{
		Total: len(pods),
		Pods:  make([]PodExtra, 0),
	}
	startIndex := (current - 1) * pageSize
	if startIndex >= uint64(len(pods)) {
		return result, nil
	}
	endIndex := startIndex + pageSize
	if endIndex > uint64(len(pods)) {
		endIndex = uint64(len(pods))
	}
	for i := startIndex; i < endIndex; i++ {
		var node corev1.Node
		err := s.client.Get(c, types.NamespacedName{Name: pods[i].Spec.NodeName}, &node)
		if err != nil {
			podLog.Error(err, "get node error", "node", pods[i].Spec.NodeName)
			continue
		}
		var csiNode *corev1.Pod
		csiNode, err = s.getCSINode(c, pods[i].Spec.NodeName)
		if err != nil {
			podLog.Error(err, "get csi node error", "node", pods[i].Spec.NodeName)
		}
		result.Pods = append(result.Pods, PodExtra{
			Pod:     pods[i],
			Node:    &node,
			CsiNode: csiNode,
		})
	}
	return result, nil
}

func (c *CachePodService) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pod := &corev1.Pod{}
	if err := c.client.Get(ctx, req.NamespacedName, pod); err != nil {
		podLog.Error(err, "get pod failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	if !utils.IsSysPod(pod) && !utils.IsAppPod(pod) && !utils.IsAppPodShouldList(ctx, c.client, pod) {
		return reconcile.Result{}, nil
	}
	if pod.DeletionTimestamp != nil {
		c.appIndexes.RemoveIndex(req.NamespacedName)
		if utils.IsCsiNode(pod) {
			c.csiNodeLock.Lock()
			delete(c.csiNodeIndex, pod.Spec.NodeName)
			c.csiNodeLock.Unlock()
		}
		podLog.V(1).Info("pod deleted", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	indexes := c.appIndexes
	if utils.IsSysPod(pod) {
		indexes = c.sysIndexes
		if utils.IsCsiNode(pod) && pod.Spec.NodeName != "" {
			c.csiNodeLock.Lock()
			c.csiNodeIndex[pod.Spec.NodeName] = types.NamespacedName{
				Namespace: pod.GetNamespace(),
				Name:      pod.GetName(),
			}
			c.csiNodeLock.Unlock()
		}
	}
	indexes.AddIndex(
		pod,
		func(p *corev1.Pod) metav1.ObjectMeta { return p.ObjectMeta },
		func(name types.NamespacedName) (*corev1.Pod, error) {
			var pod corev1.Pod
			err := c.client.Get(ctx, name, &pod)
			return &pod, err
		},
	)
	podLog.V(1).Info("pod created", "namespacedName", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *CachePodService) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("pod", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod := rawObj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return err
	}

	return ctr.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}, &handler.TypedEnqueueRequestForObject[*corev1.Pod]{}, predicate.TypedFuncs[*corev1.Pod]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.Pod]) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.Pod]) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.Pod]) bool {
			pod := deleteEvent.Object
			var indexes *utils.TimeOrderedIndexes[corev1.Pod]
			if utils.IsAppPod(pod) {
				indexes = c.appIndexes
			} else if utils.IsSysPod(pod) {
				indexes = c.sysIndexes
			}
			if indexes != nil {
				indexes.RemoveIndex(types.NamespacedName{
					Namespace: pod.GetNamespace(),
					Name:      pod.GetName(),
				})
				if utils.IsCsiNode(pod) {
					c.csiNodeLock.Lock()
					delete(c.csiNodeIndex, pod.Spec.NodeName)
					c.csiNodeLock.Unlock()
				}
				podLog.V(1).Info("pod deleted", "namespace", pod.GetNamespace(), "name", pod.GetName())
				return false
			}
			return true
		},
	}))
}
