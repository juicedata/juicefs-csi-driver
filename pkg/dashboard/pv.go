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

package dashboard

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

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
		pvs, err := api.listPVsOfPod(c, pod)
		if err != nil {
			c.String(500, "get pod persistent volumes error: %v", err)
			return
		}
		c.IndentedJSON(200, pvs)
	}
}

func (api *API) listPodPVCsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		c.IndentedJSON(200, api.listPVCsOfPod(c, pod))
	}
}

func (api *API) listSCsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		scs := storagev1.StorageClassList{}
		err := api.cachedReader.List(c, &scs)
		if err != nil {
			c.String(500, "get storageClass error: %v", err)
			return
		}
		for _, sc := range scs.Items {
			if sc.Provisioner == config.DriverName {
				scs.Items = append(scs.Items, sc)
			}
		}
		c.IndentedJSON(200, scs.Items)
	}
}

type ListPVPodResult struct {
	Total int                        `json:"total"`
	PVs   []*corev1.PersistentVolume `json:"pvs"`
}

type ListPVCPodResult struct {
	Total int                             `json:"total"`
	PVCs  []*corev1.PersistentVolumeClaim `json:"pvcs"`
}

func (api *API) listPVsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
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
		descend := c.Query("order") != "ascend"
		nameFilter := c.Query("name")
		pvcFilter := c.Query("pvc")
		scFilter := c.Query("sc")
		required := func(pv *corev1.PersistentVolume) bool {
			pvcName := types.NamespacedName{}
			if pv.Spec.ClaimRef != nil {
				pvcName = types.NamespacedName{
					Namespace: pv.Spec.ClaimRef.Namespace,
					Name:      pv.Spec.ClaimRef.Name,
				}
			}
			return (nameFilter == "" || strings.Contains(pv.Name, nameFilter)) &&
				(pvcFilter == "" || strings.Contains(pvcName.String(), pvcFilter)) &&
				(scFilter == "" || strings.Contains(pv.Spec.StorageClassName, scFilter))

		}
		pvs := make([]*corev1.PersistentVolume, 0, api.pvIndexes.length())
		for name := range api.pvIndexes.iterate(c, descend) {
			var pv corev1.PersistentVolume
			if err := api.cachedReader.Get(c, name, &pv); err == nil && required(&pv) {
				pvs = append(pvs, &pv)
			}
		}
		result := &ListPVPodResult{len(pvs), make([]*corev1.PersistentVolume, 0)}
		startIndex := (current - 1) * pageSize
		if startIndex >= uint64(len(pvs)) {
			c.IndentedJSON(200, result)
			return
		}
		endIndex := startIndex + pageSize
		if endIndex > uint64(len(pvs)) {
			endIndex = uint64(len(pvs))
		}
		result.PVs = pvs[startIndex:endIndex]
		c.IndentedJSON(200, result)
	}
}

func (api *API) getPVMiddileware() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		pv, err := api.getPV(c, name)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}
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
		descend := c.Query("order") != "ascend"
		namespaceFilter := c.Query("namespace")
		nameFilter := c.Query("name")
		pvFilter := c.Query("pv")
		scFilter := c.Query("sc")
		required := func(pvc *corev1.PersistentVolumeClaim) bool {
			pvName := ""
			scName := ""
			if pv, ok := api.pairs[types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Name}]; ok {
				pvName = pv.Name
			}
			if pvc.Spec.StorageClassName != nil {
				scName = *pvc.Spec.StorageClassName
			}
			return (namespaceFilter == "" || strings.Contains(pvc.Namespace, namespaceFilter)) &&
				(nameFilter == "" || strings.Contains(pvc.Name, nameFilter)) &&
				(pvFilter == "" || strings.Contains(pvName, pvFilter)) &&
				(scFilter == "" || strings.Contains(scName, scFilter))

		}
		pvcs := make([]*corev1.PersistentVolumeClaim, 0, api.pvcIndexes.length())
		for name := range api.pvcIndexes.iterate(c, descend) {
			if pvc, ok := api.pvcs[name]; ok && required(pvc) {
				pvcs = append(pvcs, pvc)
			}
		}
		result := &ListPVCPodResult{len(pvcs), make([]*corev1.PersistentVolumeClaim, 0)}
		startIndex := (current - 1) * pageSize
		if startIndex >= uint64(len(pvcs)) {
			c.IndentedJSON(200, result)
			return
		}
		endIndex := startIndex + pageSize
		if endIndex > uint64(len(pvcs)) {
			endIndex = uint64(len(pvcs))
		}
		result.PVCs = pvcs[startIndex:endIndex]
		c.IndentedJSON(200, result)
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

func (api *API) getPV(ctx context.Context, name string) (*corev1.PersistentVolume, error) {
	var pv corev1.PersistentVolume
	if err := api.cachedReader.Get(ctx, api.sysNamespaced(name), &pv); err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Errorf("get pv %s error: %v", name, err)
			return nil, nil
		}
		return nil, err
	}
	return &pv, nil
}

func (api *API) getPVC(namespace, name string) *corev1.PersistentVolumeClaim {
	return api.pvcs[types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}]
}

func (api *API) getStorageClass(ctx *gin.Context, name string) (*storagev1.StorageClass, error) {
	var sc storagev1.StorageClass
	err := api.cachedReader.Get(ctx, types.NamespacedName{Name: name}, &sc)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &sc, nil
}

func (api *API) listPVsOfPod(ctx context.Context, pod *corev1.Pod) ([]*corev1.PersistentVolume, error) {
	pvs := make([]*corev1.PersistentVolume, 0)
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}
		pvName := api.pairs[types.NamespacedName{Namespace: pod.Namespace, Name: v.PersistentVolumeClaim.ClaimName}]
		pv, err := api.getPV(ctx, pvName.Name)
		if err != nil {
			return nil, err
		}
		if pv != nil {
			pvs = append(pvs, pv)
		}
	}
	return pvs, nil
}

func (api *API) listPVCsOfPod(ctx context.Context, pod *corev1.Pod) []*corev1.PersistentVolumeClaim {
	pvcs := make([]*corev1.PersistentVolumeClaim, 0)
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}
		pvc, ok := api.pvcs[types.NamespacedName{
			Name:      v.PersistentVolumeClaim.ClaimName,
			Namespace: pod.Namespace,
		}]
		if ok {
			pvcs = append(pvcs, pvc)
		}
	}
	return pvcs
}
