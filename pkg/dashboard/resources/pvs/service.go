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

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ListPVPodResult struct {
	Total         int                       `json:"total"`
	ContinueToken string                    `json:"continueToken"`
	PVs           []corev1.PersistentVolume `json:"pvs"`
}

type PVService interface {
	ListPVs(c *gin.Context) (*ListPVPodResult, error)
	ListAllPVs(ctx context.Context) ([]corev1.PersistentVolume, error)

	GetPVByUniqueId(c *gin.Context, uniqueId string) (*corev1.PersistentVolume, error)
}

func NewPVService(client client.Client, enableManager bool) PVService {
	svc := &pvService{
		client: client,
	}
	if enableManager {
		return &CachePVService{
			pvService: svc,
			pvIndexes: utils.NewTimeIndexes[corev1.PersistentVolume](),
		}
	}
	return svc
}
