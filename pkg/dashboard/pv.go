/*
Copyright 2023 The Kubernetes Authors.

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

package dashboard

import (
	"context"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (api *API) listPodPVsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		c.IndentedJSON(200, api.listPVsOfPod(c, pod))
	}
}

func (api *API) listPVsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pvs := make(map[string]*corev1.PersistentVolume)
		api.pvsLock.RLock()
		for name, pv := range api.pvs {
			pvs[name.String()] = pv
		}
		api.pvsLock.RUnlock()
		c.IndentedJSON(200, pvs)
	}
}

func (api *API) listPVsOfPod(ctx context.Context, pod *corev1.Pod) map[string]*corev1.PersistentVolume {
	pvs := make(map[string]*corev1.PersistentVolume)
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}
		api.pvsLock.RLock()
		pv, ok := api.pvs[types.NamespacedName{Namespace: pod.Namespace, Name: v.PersistentVolumeClaim.ClaimName}]
		if ok {
			pvs[v.PersistentVolumeClaim.ClaimName] = pv
		}
		api.pvsLock.RUnlock()
	}
	return pvs
}
