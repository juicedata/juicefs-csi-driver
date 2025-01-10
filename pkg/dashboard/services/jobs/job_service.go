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
	"strconv"

	"github.com/gin-gonic/gin"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
)

type jobService struct {
	client       client.Client
	sysNamespace string
}

func (c *jobService) ListAllBatchJobs(ctx *gin.Context) (*ListJobResult, error) {
	pageSize, err := strconv.ParseInt(ctx.Query("pageSize"), 10, 64)
	if err != nil || pageSize == 0 {
		pageSize = 10
	}
	continueToken := ctx.Query("continue")

	nameFilter := ctx.Query("name")
	if nameFilter != "" {
		job := batchv1.Job{}
		if err := c.client.Get(ctx, types.NamespacedName{Name: nameFilter, Namespace: c.sysNamespace}, &job); err != nil {
			return nil, client.IgnoreNotFound(err)
		}
		result := &ListJobResult{
			Jobs: []batchv1.Job{job},
		}
		return result, nil
	}
	jobs := batchv1.JobList{}
	labelSelector := labels.SelectorFromSet(map[string]string{
		common.PodTypeKey: common.JobTypeValue,
		common.JfsJobKind: common.KindOfUpgrade,
	})
	if err := c.client.List(ctx, &jobs, &client.ListOptions{
		LabelSelector: labelSelector,
		Namespace:     c.sysNamespace,
		Limit:         pageSize,
		Continue:      continueToken,
	}); err != nil {
		return nil, err
	}
	return &ListJobResult{
		Continue: jobs.Continue,
		Jobs:     jobs.Items,
	}, nil
}
