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

package pvs

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	pvLog = klog.NewKlogr().WithName("PvService/Cache")
)

type CachePVService struct {
	*pvService
	pvIndexes *utils.TimeOrderedIndexes[corev1.PersistentVolume]
}

// ListPVs implements PVService.
func (s *CachePVService) ListPVs(c *gin.Context) (*ListPVPodResult, error) {
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
	pvs := make([]corev1.PersistentVolume, 0, s.pvIndexes.Length())
	for name := range s.pvIndexes.Iterate(c, descend) {
		var pv corev1.PersistentVolume
		if err := s.client.Get(c, name, &pv); err == nil && required(&pv) {
			pvs = append(pvs, pv)
		}
	}
	result := &ListPVPodResult{
		Total: len(pvs),
		PVs:   make([]corev1.PersistentVolume, 0),
	}
	startIndex := (current - 1) * pageSize
	if startIndex >= uint64(len(pvs)) {
		return result, nil
	}
	endIndex := startIndex + pageSize
	if endIndex > uint64(len(pvs)) {
		endIndex = uint64(len(pvs))
	}
	result.PVs = pvs[startIndex:endIndex]
	return result, nil
}

func (s *CachePVService) GetPVByUniqueId(c *gin.Context, uniqueId string) (*corev1.PersistentVolume, error) {
	for name := range s.pvIndexes.Iterate(c, false) {
		var pv corev1.PersistentVolume
		if err := s.client.Get(c, name, &pv); err == nil {
			if pv.Spec.CSI.VolumeHandle == uniqueId {
				return &pv, nil
			}
		}
	}
	return nil, fmt.Errorf("pv not found")
}

func (s *CachePVService) ListAllPVs(ctx context.Context) ([]corev1.PersistentVolume, error) {
	pvs := make([]corev1.PersistentVolume, 0, s.pvIndexes.Length())
	for name := range s.pvIndexes.Iterate(ctx, false) {
		var pv corev1.PersistentVolume
		if err := s.client.Get(ctx, name, &pv); err == nil {
			pvs = append(pvs, pv)
		}
	}
	return pvs, nil
}

func (c *CachePVService) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pv := &corev1.PersistentVolume{}
	if err := c.client.Get(ctx, req.NamespacedName, pv); err != nil {
		if k8serrors.IsNotFound(err) {
			c.pvIndexes.RemoveIndex(req.NamespacedName)
			return reconcile.Result{}, nil
		}
		pvLog.Error(err, "get pv failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, err
	}
	if pv.DeletionTimestamp != nil {
		pvLog.V(1).Info("watch pv deleted", "namespacedName", req.NamespacedName)
		c.pvIndexes.RemoveIndex(req.NamespacedName)
		return reconcile.Result{}, nil
	}
	c.pvIndexes.AddIndex(
		pv,
		func(p *corev1.PersistentVolume) metav1.ObjectMeta { return p.ObjectMeta },
		func(name types.NamespacedName) (*corev1.PersistentVolume, error) {
			var p corev1.PersistentVolume
			err := c.client.Get(ctx, name, &p)
			return &p, err
		},
	)
	if pv.Spec.ClaimRef != nil {
		pvcName := types.NamespacedName{
			Namespace: pv.Spec.ClaimRef.Namespace,
			Name:      pv.Spec.ClaimRef.Name,
		}
		var pvc corev1.PersistentVolumeClaim
		if err := c.client.Get(ctx, pvcName, &pvc); err != nil {
			pvLog.Error(err, "get pvc failed", "name", pvcName)
			return reconcile.Result{}, nil
		}
	}
	pvLog.V(1).Info("pv created", "namespacedName", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *CachePVService) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("pv", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}
	return ctr.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolume{}, &handler.TypedEnqueueRequestForObject[*corev1.PersistentVolume]{}, predicate.TypedFuncs[*corev1.PersistentVolume]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.PersistentVolume]) bool {
			pv := event.Object
			return pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.PersistentVolume]) bool {
			return false
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.PersistentVolume]) bool {
			pv := deleteEvent.Object
			return pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName
		},
	}))
}
