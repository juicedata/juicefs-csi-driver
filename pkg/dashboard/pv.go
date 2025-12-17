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
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var pvLog = klog.NewKlogr().WithName("pv")

type PVCWithMountPod struct {
	PVC       corev1.PersistentVolumeClaim `json:"PVC,omitempty"`
	MountPods []corev1.Pod                 `json:"MountPods,omitempty"`
}

func (api *API) listPodPVsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		pvs, err := api.podSvc.ListPodPVs(c, pod)
		if err != nil {
			c.String(500, "get pod pv error: %v", err)
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
		pvcs, err := api.podSvc.ListPodPVCs(c, pod)
		if err != nil {
			c.String(500, "get pod pvcs error: %v", err)
			return
		}
		c.IndentedJSON(200, pvcs)
	}
}

func (api *API) listSCsHandler() gin.HandlerFunc {
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
		required := func(sc *storagev1.StorageClass) bool {
			scName := types.NamespacedName{
				Name: sc.Name,
			}
			return (nameFilter == "" || strings.Contains(scName.Name, nameFilter)) && (sc.Provisioner == config.DriverName)
		}
		scList := storagev1.StorageClassList{}
		err = api.cachedReader.List(c, &scList)
		if err != nil {
			c.String(500, "list storageClass error: %v", err)
			return
		}
		var scs []*storagev1.StorageClass
		for i := range scList.Items {
			sc := &scList.Items[i]
			if required(sc) {
				scs = append(scs, sc)
			}
		}
		result := &ListSCResult{len(scs), scs}
		if descend {
			sort.Sort(Reverse(result))
		} else {
			sort.Sort(result)
		}

		startIndex := (current - 1) * pageSize
		if startIndex >= uint64(len(scs)) {
			c.IndentedJSON(200, result)
			return
		}
		endIndex := startIndex + pageSize
		if endIndex > uint64(len(scs)) {
			endIndex = uint64(len(scs))
		}
		result.SCs = scs[startIndex:endIndex]
		c.IndentedJSON(200, result)
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

type ListSCResult struct {
	Total int                       `json:"total"`
	SCs   []*storagev1.StorageClass `json:"scs"`
}

func (r ListSCResult) Len() int {
	return r.Total
}

func (r ListSCResult) Less(i, j int) bool {
	return (&r.SCs[i].CreationTimestamp).Before(&r.SCs[j].CreationTimestamp)
}

func (r ListSCResult) Swap(i, j int) {
	r.SCs[i], r.SCs[j] = r.SCs[j], r.SCs[i]
}

func (api *API) listPVsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := api.pvSvc.ListPVs(c)
		if err != nil {
			c.String(500, "list pvs error %v", err)
			return
		}
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
		if pv == nil || pv.Spec.CSI == nil || pv.Spec.CSI.Driver != config.DriverName {
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
		pvc, err := api.getPVC(namespace, name)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}
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
		if sc == nil || sc.Provisioner != config.DriverName {
			c.AbortWithStatus(404)
			return
		}
		c.Set("sc", sc)
	}
}

func (api *API) listPVCsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := api.pvcSvc.ListPVCs(c)
		if err != nil {
			c.String(500, "list pvcs error %v", err)
			return
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) listPVCsBasicHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := api.pvcSvc.ListPVCsBasicInfo(c)
		if err != nil {
			c.String(500, "list pvcs basic info error %v", err)
			return
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) listPVCWithSelectorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := &config.Config{}
		var cm corev1.ConfigMap
		err := c.ShouldBindJSON(&cm)
		if err != nil {
			if err := config.LoadFromConfigMap(c, api.client); err != nil {
				pvLog.Error(err, "load config error")
				c.JSON(200, []PVCWithMountPod{})
			}
			cfg = config.GlobalConfig
		} else {
			cfg.Unmarshal([]byte(cm.Data["config.yaml"]))
		}

		mountPods, err := api.podSvc.ListMountPods(c)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		mountPodMaps := make(map[string][]corev1.Pod)
		for _, po := range mountPods {
			mountPodMaps[po.Labels[common.PodUniqueIdLabelKey]] = append(mountPodMaps[po.Labels[common.PodUniqueIdLabelKey]], po)
		}

		results := make([][]PVCWithMountPod, len(cfg.MountPodPatch))
		for i, patch := range cfg.MountPodPatch {
			if IsPVCSelectorEmpty(patch.PVCSelector) {
				continue
			}
			if patch.PVCSelector.MatchName != "" {
				results[i] = []PVCWithMountPod{}
				pvc, err := api.client.CoreV1().PersistentVolumeClaims("").Get(c, patch.PVCSelector.MatchName, metav1.GetOptions{})
				if err != nil {
					pvLog.Error(err, "get pvc error", "name", patch.PVCSelector.MatchName)
					continue
				}
				if pvc != nil {
					results[i] = []PVCWithMountPod{{
						PVC:       *pvc,
						MountPods: []corev1.Pod{},
					}}
					if m, ok := mountPodMaps[utils.GetUniqueOfPVC(*pvc)]; ok {
						results[i][0].MountPods = m
					}
				}
				continue
			}
			if patch.PVCSelector.MatchStorageClassName != "" {
				pvcs, err := api.pvcSvc.ListPVCsByStorageClass(c, patch.PVCSelector.MatchStorageClassName)
				if err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
				pmp := []PVCWithMountPod{}
				for _, pvc := range pvcs {
					mps := []corev1.Pod{}
					if m, ok := mountPodMaps[utils.GetUniqueOfPVC(pvc)]; ok {
						mps = m
					}
					pmp = append(pmp, PVCWithMountPod{
						PVC:       pvc,
						MountPods: mps,
					})
				}
				results[i] = pmp
				continue
			}
			if (len(patch.PVCSelector.LabelSelector.MatchLabels) == 0) &&
				(len(patch.PVCSelector.LabelSelector.MatchExpressions) == 0) {
				continue
			}
			selector, err := metav1.LabelSelectorAsSelector(&patch.PVCSelector.LabelSelector)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			pvcs, err := api.client.CoreV1().PersistentVolumeClaims("").List(context.Background(), metav1.ListOptions{
				LabelSelector: selector.String(),
			})
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			pmp := []PVCWithMountPod{}
			for _, pvc := range pvcs.Items {
				mps := []corev1.Pod{}
				if m, ok := mountPodMaps[utils.GetUniqueOfPVC(pvc)]; ok {
					mps = m
				}
				pmp = append(pmp, PVCWithMountPod{
					PVC:       pvc,
					MountPods: mps,
				})
			}
			results[i] = pmp
		}
		c.IndentedJSON(200, results)
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

func (api *API) getPVCByUniqueId() gin.HandlerFunc {
	return func(c *gin.Context) {
		uniqueId := c.Param("uniqueid")

		selectPV, err := api.pvSvc.GetPVByUniqueId(c, uniqueId)
		if err != nil {
			c.String(400, "get pv error: %v", err)
			return
		}
		if selectPV == nil || selectPV.Spec.ClaimRef == nil {
			c.String(404, "not found")
			return
		}
		var pvc corev1.PersistentVolumeClaim
		err = api.cachedReader.Get(c, types.NamespacedName{Namespace: selectPV.Spec.ClaimRef.Namespace, Name: selectPV.Spec.ClaimRef.Name}, &pvc)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				c.String(404, "not found")
				return
			}
			c.String(500, "get pvc error: %v", err)
			return
		}
		c.IndentedJSON(200, pvc)
	}
}

func (api *API) getPVCWithPVHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pvc")
		if !ok {
			c.String(404, "not found")
			return
		}
		pvc := obj.(*corev1.PersistentVolumeClaim)
		result := make(map[string]interface{})
		result["PVC"] = pvc
		if pvc.Spec.VolumeName != "" {
			pv, err := api.getPV(c, pvc.Spec.VolumeName)
			if err != nil {
				c.AbortWithStatus(500)
				return
			}
			if pv != nil {
				result["PV"] = pv
				s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name": "juicefs-csi-driver",
						"app":                    "juicefs-csi-node",
					},
				})
				var pods corev1.PodList
				err = api.cachedReader.List(c, &pods, &client.ListOptions{
					LabelSelector: s,
				})
				if err != nil {
					c.String(500, "list pods error %v", err)
					return
				}
				result["UniqueId"] = pv.Spec.CSI.VolumeHandle
				if len(pods.Items) > 0 {
					if isShareMount(&pods.Items[0]) {
						result["UniqueId"] = pv.Spec.StorageClassName
					}
				}
			}
		}
		c.IndentedJSON(200, result)
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
	if err := api.cachedReader.Get(ctx, types.NamespacedName{Name: name}, &pv); err != nil {
		if k8serrors.IsNotFound(err) {
			pvLog.Error(err, "get pv error", "name", name)
			return nil, nil
		}
		return nil, err
	}
	return &pv, nil
}

func (api *API) getPVC(namespace, name string) (*corev1.PersistentVolumeClaim, error) {
	var pvc corev1.PersistentVolumeClaim
	if err := api.cachedReader.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, &pvc); err != nil {
		if k8serrors.IsNotFound(err) {
			pvLog.Error(err, "get pvc error", "namespace", namespace, "name", name)
			return nil, nil
		}
		return nil, err
	}
	return &pvc, nil
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

func (api *API) getPVEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := c.Get("pv")
		if !ok {
			c.String(404, "not found")
			return
		}
		pv := p.(*corev1.PersistentVolume)
		result, err := api.eventSvc.ListEvents(c, "", "PersistentVolume", string(pv.UID))
		if err != nil {
			c.String(500, "list events error %v", err)
			return
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) getPVCEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := c.Get("pvc")
		if !ok {
			c.String(404, "not found")
			return
		}
		pvc := p.(*corev1.PersistentVolumeClaim)
		result, err := api.eventSvc.ListEvents(c, pvc.Namespace, "PersistentVolumeClaim", string(pvc.UID))
		if err != nil {
			c.String(500, "list events error %v", err)
			return
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) getMountPodsOfPV() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pv")
		if !ok {
			c.String(404, "not found")
			return
		}
		pv := obj.(*corev1.PersistentVolume)

		var pods corev1.PodList
		err := api.cachedReader.List(c, &pods, &client.ListOptions{
			LabelSelector: LabelSelectorOfMount(*pv),
		})
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}
		c.IndentedJSON(200, pods.Items)
	}
}

func (api *API) getMountPodsOfPVC() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pvc")
		if !ok {
			c.String(404, "not found")
			return
		}
		pvc := obj.(*corev1.PersistentVolumeClaim)
		if pvc.Spec.VolumeName == "" {
			c.String(404, "not found")
			return
		}
		pvName := pvc.Spec.VolumeName
		var pv corev1.PersistentVolume
		if err := api.cachedReader.Get(c, types.NamespacedName{Name: pvName}, &pv); err != nil {
			if k8serrors.IsNotFound(err) {
				c.String(404, "not found")
			} else {
				c.String(500, "get pv %s error %v", pvName, err)
			}
			return
		}

		var pods corev1.PodList
		err := api.cachedReader.List(c, &pods, &client.ListOptions{
			LabelSelector: LabelSelectorOfMount(pv),
		})
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}
		c.IndentedJSON(200, pods.Items)
	}
}

func (api *API) getPVOfSC() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("sc")
		if !ok {
			c.String(404, "not found")
			return
		}
		sc := obj.(*storagev1.StorageClass)
		var pvList corev1.PersistentVolumeList
		err := api.cachedReader.List(c, &pvList)
		if err != nil {
			c.String(500, "list pvs error %v", err)
			return
		}
		var pvs []*corev1.PersistentVolume
		for i := range pvList.Items {
			pv := &pvList.Items[i]
			if pv.Spec.StorageClassName == sc.Name {
				pvs = append(pvs, pv)
			}
		}
		c.IndentedJSON(200, pvs)
	}
}
