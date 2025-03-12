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

package jobs

import (
	"github.com/gin-gonic/gin"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
)

type ListJobResult struct {
	Total    int           `json:"total"`
	Continue string        `json:"continue"`
	Jobs     []batchv1.Job `json:"jobs"`
}

type JobService interface {
	ListAllBatchJobs(ctx *gin.Context) (*ListJobResult, error)
}

func NewJobService(client client.Client, enableManager bool) JobService {
	svc := &jobService{
		client:       client,
		sysNamespace: config.Namespace,
	}
	if enableManager {
		return &CacheJobService{
			jobService: svc,
			jobIndexes: utils.NewTimeIndexes[batchv1.Job](),
		}
	}
	return svc
}
