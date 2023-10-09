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
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
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

func (api *API) listSCsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		scs := make([]storagev1.StorageClass, 0)
		rawSc, err := api.k8sClient.ListStorageClasses(c)
		if err != nil {
			c.String(500, "get storageClass error: %v", err)
			return
		}
		for _, sc := range rawSc {
			if sc.Provisioner == config.DriverName {
				scs = append(scs, sc)
			}
		}
		c.IndentedJSON(200, scs)
	}
}

func (api *API) listPVsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pvs := make([]*corev1.PersistentVolume, 0)
		api.pvsLock.RLock()
		for _, pv := range api.pvs {
			pvs = append(pvs, pv)
		}
		api.pvsLock.RUnlock()
		c.IndentedJSON(200, pvs)
	}
}

func (api *API) getPVMiddileware() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		pv := api.getPV(name)
		if pv == nil {
			c.AbortWithStatus(404)
			return
		}
		c.Set("pv", pv)
	}
}

func (api *API) getPVCMiddileware() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		namespace := c.Param("namespace")
		pvc := api.getPVC(namespace, name)
		if pvc == nil {
			c.AbortWithStatus(404)
			return
		}
		c.Set("pvc", pvc)
	}
}

func (api *API) getSCMiddileware() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		sc, err := api.getStorageClass(c, name)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}
		if sc == nil {
			c.AbortWithStatus(404)
			return
		}
		c.Set("sc", sc)
	}
}

func (api *API) listPVCsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pvcs := make([]*corev1.PersistentVolumeClaim, 0)
		api.pvsLock.RLock()
		for _, pv := range api.pvcs {
			pvcs = append(pvcs, pv)
		}
		api.pvsLock.RUnlock()
		c.IndentedJSON(200, pvcs)
	}
}

func (api *API) getPVHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pv, ok := c.Get("pv")
		if !ok {
			c.String(404, "not found")
			return
		}
		c.IndentedJSON(200, pv)
	}
}

func (api *API) getPVCHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pvc, ok := c.Get("pvc")
		if !ok {
			c.String(404, "not found")
			return
		}
		c.IndentedJSON(200, pvc)
	}
}

func (api *API) getSCHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		sc, ok := c.Get("sc")
		if !ok {
			c.String(404, "not found")
			return
		}
		c.IndentedJSON(200, sc)
	}
}

func (api *API) getPV(name string) *corev1.PersistentVolume {
	api.pvsLock.RLock()
	defer api.pvsLock.RUnlock()
	return api.pvs[name]
}

func (api *API) getPVC(namespace, name string) *corev1.PersistentVolumeClaim {
	api.pvsLock.RLock()
	defer api.pvsLock.RUnlock()
	return api.pvcs[types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}]
}

func (api *API) getStorageClass(ctx *gin.Context, name string) (*storagev1.StorageClass, error) {
	sc, err := api.k8sClient.GetStorageClass(ctx, name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return sc, nil
}

func (api *API) listPVsOfPod(ctx context.Context, pod *corev1.Pod) map[string]*corev1.PersistentVolume {
	pvs := make(map[string]*corev1.PersistentVolume)
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}
		api.pvsLock.RLock()
		pvName := api.pairs[types.NamespacedName{Namespace: pod.Namespace, Name: v.PersistentVolumeClaim.ClaimName}]
		pv, ok := api.pvs[pvName]
		if ok {
			pvs[v.PersistentVolumeClaim.ClaimName] = pv
		}
		api.pvsLock.RUnlock()
	}
	return pvs
}
