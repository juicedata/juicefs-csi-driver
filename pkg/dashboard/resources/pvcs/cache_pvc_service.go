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

package pvcs

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	corev1 "k8s.io/api/core/v1"
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
	pvcLog = klog.NewKlogr().WithName("PVCService/Cache")
)

type CachePVCService struct {
	*pvcService

	pvcIndexes *utils.TimeOrderedIndexes[corev1.PersistentVolumeClaim]
}

func (s *CachePVCService) ListPVCs(c *gin.Context) (*ListPVCResult, error) {
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
	namespaceFilter := c.Query("namespace")
	nameFilter := c.Query("name")
	pvFilter := c.Query("pv")
	scFilter := c.Query("sc")
	required := func(pvc *corev1.PersistentVolumeClaim) bool {
		pvName := ""
		scName := ""
		if pvc.Spec.VolumeName != "" {
			pvName = pvc.Spec.VolumeName
		}
		if pvc.Spec.StorageClassName != nil {
			scName = *pvc.Spec.StorageClassName
		}
		return (namespaceFilter == "" || strings.Contains(pvc.Namespace, namespaceFilter)) &&
			(nameFilter == "" || strings.Contains(pvc.Name, nameFilter)) &&
			(pvFilter == "" || strings.Contains(pvName, pvFilter)) &&
			(scFilter == "" || strings.Contains(scName, scFilter))

	}
	pvcs := make([]corev1.PersistentVolumeClaim, 0, s.pvcIndexes.Length())
	for name := range s.pvcIndexes.Iterate(c, descend) {
		var pvc corev1.PersistentVolumeClaim
		if err := s.client.Get(c, name, &pvc); err == nil && required(&pvc) {
			pvcs = append(pvcs, pvc)
		}
	}
	result := &ListPVCResult{
		Total: len(pvcs),
		PVCs:  make([]corev1.PersistentVolumeClaim, 0),
	}
	startIndex := (current - 1) * pageSize
	if startIndex >= uint64(len(pvcs)) {
		return result, nil
	}
	endIndex := startIndex + pageSize
	if endIndex > uint64(len(pvcs)) {
		endIndex = uint64(len(pvcs))
	}
	result.PVCs = pvcs[startIndex:endIndex]
	return result, nil
}

func (s *CachePVCService) ListAllPVCs(ctx context.Context, pvs []corev1.PersistentVolume) ([]corev1.PersistentVolumeClaim, error) {
	pvcs := make([]corev1.PersistentVolumeClaim, 0, s.pvcIndexes.Length())
	for name := range s.pvcIndexes.Iterate(ctx, false) {
		var pvc corev1.PersistentVolumeClaim
		if err := s.client.Get(ctx, name, &pvc); err == nil {
			pvcs = append(pvcs, pvc)
		}
	}
	return pvcs, nil
}

func (s *CachePVCService) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := s.client.Get(ctx, req.NamespacedName, pvc); err != nil {
		pvcLog.Error(err, "get pvc failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	if pvc.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	// check is juicefs pvc
	pvName := pvc.Spec.VolumeName
	if pvName == "" {
		// wait for pv bound
		return reconcile.Result{}, nil
	}
	pv := &corev1.PersistentVolume{}
	if err := s.client.Get(ctx, types.NamespacedName{Name: pvName}, pv); err != nil {
		pvcLog.Error(err, "get pv failed", "name", pvName)
		return reconcile.Result{}, nil
	}
	if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != config.DriverName {
		return reconcile.Result{}, nil
	}
	s.pvcIndexes.AddIndex(
		pvc,
		func(p *corev1.PersistentVolumeClaim) metav1.ObjectMeta { return p.ObjectMeta },
		func(name types.NamespacedName) (*corev1.PersistentVolumeClaim, error) {
			var p corev1.PersistentVolumeClaim
			err := s.client.Get(ctx, name, &p)
			return &p, err
		},
	)
	return reconcile.Result{}, nil
}

func (s *CachePVCService) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("pvc", mgr, controller.Options{Reconciler: s})
	if err != nil {
		return err
	}

	return ctr.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolumeClaim{}, &handler.TypedEnqueueRequestForObject[*corev1.PersistentVolumeClaim]{}, predicate.TypedFuncs[*corev1.PersistentVolumeClaim]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.PersistentVolumeClaim]) bool {
			pvc := event.Object
			// bound pvc should be added by pv controller
			return pvc.Status.Phase == corev1.ClaimPending || pvc.Status.Phase == corev1.ClaimBound
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.PersistentVolumeClaim]) bool {
			oldPvc, newPvc := updateEvent.ObjectOld, updateEvent.ObjectNew
			if oldPvc.Status.Phase == corev1.ClaimBound && newPvc.Status.Phase != corev1.ClaimBound {
				return false
			}
			if oldPvc.Status.Phase == corev1.ClaimPending && newPvc.Status.Phase == corev1.ClaimBound {
				// pvc bound
				return true
			}
			return false
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.PersistentVolumeClaim]) bool {
			pvc := deleteEvent.Object
			name := types.NamespacedName{
				Namespace: pvc.GetNamespace(),
				Name:      pvc.GetName(),
			}
			pvcLog.V(1).Info("watch pvc deleted", "name", name)
			s.pvcIndexes.RemoveIndex(name)
			return false
		},
	}))
}
