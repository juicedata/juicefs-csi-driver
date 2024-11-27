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

	"github.com/gin-gonic/gin"
	juicefsiov1 "github.com/juicedata/juicefs-cache-group-operator/api/v1"
	operatorcommon "github.com/juicedata/juicefs-cache-group-operator/pkg/common"
	operatorutils "github.com/juicedata/juicefs-cache-group-operator/pkg/utils"
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
			},
		})
		if err != nil {
			c.String(500, "parse label selector error %v", err)
			return
		}

		listOptions := client.ListOptions{
			LabelSelector: s,
		}
		workers := corev1.PodList{}
		if err := api.cachedReader.List(c, &workers, &listOptions); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, workers.Items)
	}
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

		shouldUpdate := false
		for k, v := range cg.Spec.Worker.Template.NodeSelector {
			if _, ok := node.Labels[k]; ok {
				continue
			} else {
				shouldUpdate = true
				node.Labels[k] = v
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
