/*
 * Copyright 2026 Juicedata Inc
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
	"sort"

	"github.com/gin-gonic/gin"
	juicefsiov1 "github.com/juicedata/juicefs-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (api *API) listSyncs() gin.HandlerFunc {
	return func(c *gin.Context) {
		syncs := juicefsiov1.SyncList{}
		if err := api.cachedReader.List(c, &syncs); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, syncs.Items)
	}
}

func (api *API) createSync() gin.HandlerFunc {
	return func(c *gin.Context) {
		var sync juicefsiov1.Sync
		if err := c.ShouldBindJSON(&sync); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := api.mgrClient.Create(c, &sync); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, sync)
	}
}

func (api *API) getSync() gin.HandlerFunc {
	return func(c *gin.Context) {
		sync := juicefsiov1.Sync{}
		if err := api.cachedReader.Get(c, types.NamespacedName{Namespace: c.Param("namespace"), Name: c.Param("name")}, &sync); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, sync)
	}
}

func (api *API) deleteSync() gin.HandlerFunc {
	return func(c *gin.Context) {
		sync := juicefsiov1.Sync{}
		if err := api.mgrClient.Get(c, types.NamespacedName{Namespace: c.Param("namespace"), Name: c.Param("name")}, &sync); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if err := api.mgrClient.Delete(c, &sync); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "succeed"})
	}
}

func (api *API) listSyncPods() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		sync := juicefsiov1.Sync{}
		if err := api.cachedReader.Get(c, types.NamespacedName{Namespace: namespace, Name: c.Param("name")}, &sync); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		pods := corev1.PodList{}
		if err := api.cachedReader.List(c, &pods, client.InNamespace(namespace)); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		result := []warmUpPod{}
		for _, pod := range pods.Items {
			owner := metav1.GetControllerOf(&pod)
			if owner != nil && owner.UID == sync.UID && owner.Kind == "Sync" && owner.APIVersion == "juicefs.io/v1" {
				result = append(result, warmUpPod{Name: pod.Name, Phase: pod.Status.Phase, CreationTimestamp: pod.CreationTimestamp})
			}
		}
		sort.Slice(result, func(i, j int) bool { return result[i].CreationTimestamp.After(result[j].CreationTimestamp.Time) })
		c.JSON(200, result)
	}
}

func (api *API) isSyncPod(ctx context.Context, pod *corev1.Pod) bool {
	owner := metav1.GetControllerOf(pod)
	if owner == nil || owner.Kind != "Sync" || owner.APIVersion != "juicefs.io/v1" {
		return false
	}
	sync := juicefsiov1.Sync{}
	return api.cachedReader.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: owner.Name}, &sync) == nil && sync.UID == owner.UID
}
