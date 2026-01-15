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

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
)

type ListPVCResult struct {
	Total    int                            `json:"total,omitempty"`
	Continue string                         `json:"continue,omitempty"`
	PVCs     []corev1.PersistentVolumeClaim `json:"pvcs"`
}

type ListPVCWithPodResult struct {
	PVCWithPods []PVCWithPod `json:"pvcWithPod"`
}

type PVCWithPod struct {
	PVC corev1.PersistentVolumeClaim `json:"pvc,omitempty"`
	Pod corev1.Pod                   `json:"pod,omitempty"`
}

type PVCBasicInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

type ListPVCBasicResult struct {
	PVCs []PVCBasicInfo `json:"pvcs"`
}

type PVCService interface {
	ListPVCs(c *gin.Context) (*ListPVCResult, error)
	ListPVCsByStorageClass(c context.Context, scName string) ([]corev1.PersistentVolumeClaim, error)
	ListAllPVCs(ctx context.Context, pvs []corev1.PersistentVolume) ([]corev1.PersistentVolumeClaim, error)
	ListPVCsBasicInfo(ctx context.Context) (*ListPVCBasicResult, error)
}

func NewPVCService(client client.Client, enableManager bool) PVCService {
	svc := &pvcService{
		client: client,
	}
	if enableManager {
		return &CachePVCService{
			pvcService: svc,
			pvcIndexes: utils.NewTimeIndexes[corev1.PersistentVolumeClaim](),
		}
	}
	return svc
}
