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

package pvs

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type pvService struct {
	client client.Client
}

func (s *pvService) listPVs(ctx context.Context, limit int64, continueToken string) ([]corev1.PersistentVolume, string, error) {
	pvLists := corev1.PersistentVolumeList{}
	opts := &client.ListOptions{
		Limit:    limit,
		Continue: continueToken,
	}
	if err := s.client.List(ctx, &pvLists, opts); err != nil {
		return nil, "", err
	}
	pvs := make([]corev1.PersistentVolume, 0)
	for _, pv := range pvLists.Items {
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
			pvs = append(pvs, pv)
		}
	}
	nextContinue := pvLists.Continue
	var err error
	if len(pvs) != 0 && int64(len(pvs)) < limit && nextContinue != "" {
		var nextPvs []corev1.PersistentVolume
		nextPvs, nextContinue, err = s.listPVs(ctx, limit-int64(len(pvs)), nextContinue)
		if err != nil {
			return nil, "", err
		}
		pvs = append(pvs, nextPvs...)
	}
	return pvs, nextContinue, nil
}

func (s *pvService) ListPVs(c *gin.Context) (*ListPVPodResult, error) {
	pageSize, err := strconv.ParseInt(c.Query("pageSize"), 10, 64)
	if err != nil || pageSize == 0 {
		pageSize = 10
	}
	continueToken := c.Query("continue")
	pvs, nextContinue, err := s.listPVs(c, pageSize, continueToken)
	if err != nil {
		return nil, err
	}
	result := &ListPVPodResult{
		PVs:           pvs,
		ContinueToken: nextContinue,
	}
	return result, nil
}

func (s *pvService) GetPVByUniqueId(c *gin.Context, uniqueId string) (*corev1.PersistentVolume, error) {
	pv := &corev1.PersistentVolume{}
	pvs := corev1.PersistentVolumeList{}
	if err := s.client.List(c, &pvs, &client.ListOptions{}); err != nil {
		return nil, err
	}
	for _, p := range pvs.Items {
		if p.Spec.CSI != nil && p.Spec.CSI.VolumeHandle == uniqueId {
			pv = &p
			break
		}
	}
	return pv, nil
}

func (s pvService) ListAllPVs(ctx context.Context) ([]corev1.PersistentVolume, error) {
	pvLists := corev1.PersistentVolumeList{}
	if err := s.client.List(ctx, &pvLists); err != nil {
		return nil, err
	}
	result := make([]corev1.PersistentVolume, 0)
	for _, pv := range pvLists.Items {
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
			result = append(result, pv)
		}
	}
	return result, nil
}
