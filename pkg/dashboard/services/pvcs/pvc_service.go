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

package pvcs

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

type pvcService struct {
	client client.Client
}

var _ PVCService = &pvcService{}

func (s *pvcService) listPVCs(ctx context.Context, pvMap map[string]interface{}, limit int64, continueToken string) ([]corev1.PersistentVolumeClaim, string, error) {
	pvcLists := corev1.PersistentVolumeClaimList{}
	opts := &client.ListOptions{
		Limit:    limit,
		Continue: continueToken,
	}
	if err := s.client.List(ctx, &pvcLists, opts); err != nil {
		return nil, "", err
	}
	pvcs := make([]corev1.PersistentVolumeClaim, 0)
	for _, pvc := range pvcLists.Items {
		if pvc.Spec.VolumeName == "" {
			continue
		}
		if _, ok := pvMap[pvc.Spec.VolumeName]; ok {
			pvcs = append(pvcs, pvc)
		}
	}
	nextContinue := pvcLists.Continue
	var err error
	if len(pvcs) != 0 && int64(len(pvcs)) < limit && nextContinue != "" {
		var nextPVCs []corev1.PersistentVolumeClaim
		nextPVCs, nextContinue, err = s.listPVCs(ctx, pvMap, limit-int64(len(pvcs)), nextContinue)
		if err != nil {
			return nil, "", err
		}
		pvcs = append(pvcs, nextPVCs...)
	}
	return pvcs, nextContinue, nil
}

func (s *pvcService) ListPVCs(c *gin.Context) (*ListPVCResult, error) {
	pageSize, err := strconv.ParseInt(c.Query("pageSize"), 10, 64)
	if err != nil || pageSize == 0 {
		pageSize = 10
	}
	continueToken := c.Query("continue")
	pvMap := make(map[string]interface{})
	pvList := corev1.PersistentVolumeList{}
	if err := s.client.List(c, &pvList); err != nil {
		return nil, err
	}
	for _, pv := range pvList.Items {
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
			if pv.Spec.ClaimRef != nil {
				pvMap[pv.Name] = nil
			}
		}
	}
	pvcs, nextContinue, err := s.listPVCs(c, pvMap, pageSize, continueToken)
	if err != nil {
		return nil, err
	}
	result := &ListPVCResult{
		PVCs:     pvcs,
		Continue: nextContinue,
	}
	return result, nil
}

func (s *pvcService) ListAllPVCs(ctx context.Context, pvs []corev1.PersistentVolume) ([]corev1.PersistentVolumeClaim, error) {
	pvcs := corev1.PersistentVolumeClaimList{}
	if err := s.client.List(ctx, &pvcs); err != nil {
		return nil, err
	}
	pvMap := make(map[string]interface{})
	for _, pv := range pvs {
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
			if pv.Spec.ClaimRef != nil {
				pvMap[fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)] = nil
			}
		}
	}
	result := make([]corev1.PersistentVolumeClaim, 0)
	for _, pvc := range pvcs.Items {
		if pvc.Spec.VolumeName == "" {
			continue
		}
		if _, ok := pvMap[fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name)]; ok {
			result = append(result, pvc)
		}
	}
	return result, nil
}

func (s *pvcService) ListPVCsByStorageClass(c context.Context, scName string) ([]corev1.PersistentVolumeClaim, error) {
	pvcs := corev1.PersistentVolumeClaimList{}
	if err := s.client.List(c, &pvcs, &client.ListOptions{}); err != nil {
		return nil, err
	}
	result := make([]corev1.PersistentVolumeClaim, 0)
	for _, pvc := range pvcs.Items {
		if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName == scName {
			result = append(result, pvc)
		}
	}
	return result, nil
}
