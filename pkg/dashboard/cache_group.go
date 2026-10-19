/**
 * Copyright 2024 Juicedata Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dashboard

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	juicefsiov1 "github.com/juicedata/juicefs-operator/api/v1"
	operatorcommon "github.com/juicedata/juicefs-operator/pkg/common"
	operatorutils "github.com/juicedata/juicefs-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (api *API) listCacheGroups() gin.HandlerFunc {
	return func(c *gin.Context) {
		cgs := juicefsiov1.CacheGroupList{}
		if err := api.cachedReader.List(c, &cgs); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, cgs.Items)
	}
}

func validateCg(ctx context.Context, client client.Client, cg juicefsiov1.CacheGroup) error {
	if cg.Spec.SecretRef == nil {
		return fmt.Errorf("secretRef is required")
	}
	secret := cg.Spec.SecretRef.Name
	if secret == "" {
		return fmt.Errorf("secretRef name is required")
	}
	if err := client.Get(ctx, types.NamespacedName{
		Namespace: cg.Namespace,
		Name:      secret,
	}, &corev1.Secret{}); err != nil {
		if operatorutils.IsNotFound(err) {
			return fmt.Errorf("secret %s not found", secret)
		}
	}
	if cg.Spec.Worker.Template.NodeSelector == nil {
		return fmt.Errorf("worker node selector is required")
	}

	return nil
}

func (api *API) createCacheGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var cg juicefsiov1.CacheGroup
		if err := c.BindJSON(&cg); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateCg(c, api.mgrClient, cg); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := api.mgrClient.Create(c, &cg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, cg)
	}
}

func (api *API) deleteCacheGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		namespace := c.Param("namespace")
		cg := juicefsiov1.CacheGroup{}
		if err := api.cachedReader.Get(c, types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}, &cg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if err := api.mgrClient.Delete(c, &cg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "succeed"})
	}
}

func (api *API) updateCacheGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		namespace := c.Param("namespace")
		cg := juicefsiov1.CacheGroup{}
		if err := api.cachedReader.Get(c, types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}, &cg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		var body juicefsiov1.CacheGroup
		if err := c.BindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := validateCg(c, api.mgrClient, cg); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := api.mgrClient.Update(c, &body); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, cg)
	}
}

func (api *API) getCacheGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		namespace := c.Param("namespace")
		cg := juicefsiov1.CacheGroup{}
		if err := api.cachedReader.Get(c, types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}, &cg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, cg)
	}
}

func (api *API) listCacheGroupWorkers() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		namespace := c.Param("namespace")
		pageSize, err := strconv.ParseUint(c.Query("pageSize"), 10, 64)
		if err != nil || pageSize == 0 {
			c.String(400, "invalid page size")
			return
		}
		current, err := strconv.ParseUint(c.Query("current"), 10, 64)
		if err != nil || current == 0 {
			c.String(400, "invalid current page")
			return
		}
		nameFilter := c.Query("name")
		nodeFilter := c.Query("node")
		unreadyOnly := c.Query("unready") == "1"
		sortBy := c.Query("sortBy")
		order := c.Query("order")
		cg := juicefsiov1.CacheGroup{}
		if err := api.cachedReader.Get(c, types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}, &cg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		s, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				operatorcommon.LabelCacheGroup: cg.Name,
				operatorcommon.LabelAppType:    operatorcommon.LabelWorkerValue,
			},
		})
		if err != nil {
			c.String(500, "parse label selector error %v", err)
			return
		}

		listOptions := client.ListOptions{
			Namespace:     namespace,
			LabelSelector: s,
		}
		workers := corev1.PodList{}
		if err := api.cachedReader.List(c, &workers, &listOptions); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		items, total := filterSortAndPaginateCacheGroupWorkers(
			workers.Items, nameFilter, nodeFilter, unreadyOnly, sortBy, order, current, pageSize,
		)
		c.JSON(200, gin.H{"items": items, "total": total})
	}
}

func isCacheGroupWorkerUnready(pod *corev1.Pod) bool {
	ready := false
	containersReady := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = true
		}
		if condition.Type == corev1.ContainersReady && condition.Status == corev1.ConditionTrue {
			containersReady = true
		}
	}
	return !ready || !containersReady
}

func filterSortAndPaginateCacheGroupWorkers(workers []corev1.Pod, nameFilter, nodeFilter string, unreadyOnly bool, sortBy, order string, current, pageSize uint64) ([]corev1.Pod, int) {
	filtered := make([]corev1.Pod, 0, len(workers))
	for _, worker := range workers {
		if nameFilter != "" && !strings.Contains(worker.Name, nameFilter) {
			continue
		}
		if nodeFilter != "" && !strings.Contains(worker.Spec.NodeName, nodeFilter) {
			continue
		}
		if unreadyOnly && !isCacheGroupWorkerUnready(&worker) {
			continue
		}
		filtered = append(filtered, worker)
	}

	if sortBy != "name" && sortBy != "node" && sortBy != "time" {
		sortBy = "time"
	}
	descending := order == "descend"
	sort.SliceStable(filtered, func(i, j int) bool {
		left, right := filtered[i], filtered[j]
		var comparison int
		switch sortBy {
		case "name":
			comparison = strings.Compare(left.Name, right.Name)
		case "node":
			comparison = strings.Compare(left.Spec.NodeName, right.Spec.NodeName)
		default:
			comparison = left.CreationTimestamp.Time.Compare(right.CreationTimestamp.Time)
		}
		if comparison == 0 {
			return left.Name < right.Name
		}
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})

	total := len(filtered)
	startIndex := (current - 1) * pageSize
	if startIndex >= uint64(total) {
		return []corev1.Pod{}, total
	}
	endIndex := min(startIndex+pageSize, uint64(total))
	return filtered[startIndex:endIndex], total
}

func ensureNodeSelectorLabels(labels, selector map[string]string) (map[string]string, bool) {
	if labels == nil {
		labels = make(map[string]string)
	}
	updated := false
	for key, value := range selector {
		if labels[key] != value {
			labels[key] = value
			updated = true
		}
	}
	return labels, updated
}

func (api *API) addWorker() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		namespace := c.Param("namespace")

		var body struct {
			NodeName string `json:"nodeName"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		cg := juicefsiov1.CacheGroup{}
		if err := api.cachedReader.Get(c, types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}, &cg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		if cg.Spec.Worker.Template.NodeSelector == nil {
			c.JSON(400, gin.H{"error": "cache group node selector is not set"})
			return
		}

		node := corev1.Node{}
		if err := api.cachedReader.Get(c, types.NamespacedName{
			Name: body.NodeName,
		}, &node); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		var shouldUpdate bool
		node.Labels, shouldUpdate = ensureNodeSelectorLabels(
			node.Labels, cg.Spec.Worker.Template.NodeSelector,
		)
		if shouldUpdate {
			if _, err := api.client.CoreV1().Nodes().Update(c, &node, metav1.UpdateOptions{}); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
		}

		c.JSON(200, gin.H{"message": "succeed"})
	}
}

func (api *API) removeWorker() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		namespace := c.Param("namespace")

		var body struct {
			NodeName string `json:"nodeName"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		cg := juicefsiov1.CacheGroup{}
		if err := api.cachedReader.Get(c, types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}, &cg); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		if cg.Spec.Worker.Template.NodeSelector == nil {
			c.JSON(400, gin.H{"error": "cache group node selector is not set"})
			return
		}

		node := corev1.Node{}
		if err := api.cachedReader.Get(c, types.NamespacedName{
			Name: body.NodeName,
		}, &node); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		shouldUpdate := false
		for k := range cg.Spec.Worker.Template.NodeSelector {
			if _, ok := node.Labels[k]; ok {
				shouldUpdate = true
				delete(node.Labels, k)
			}
		}

		if shouldUpdate {
			if _, err := api.client.CoreV1().Nodes().Update(c, &node, metav1.UpdateOptions{}); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
		}

		c.JSON(200, gin.H{"message": "succeed"})
	}
}

func (api *API) getCacheWorkerBytes() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		workerName := c.Param("workerName")

		worker := corev1.Pod{}
		if err := api.cachedReader.Get(c, types.NamespacedName{
			Namespace: namespace,
			Name:      workerName,
		}, &worker); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		result, err := operatorutils.GetWorkerCacheBlocksBytes(c, worker, "/mnt/jfs")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"result": result})
	}
}
