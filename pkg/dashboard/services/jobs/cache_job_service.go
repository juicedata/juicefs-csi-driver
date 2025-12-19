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
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	batchv1 "k8s.io/api/batch/v1"
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

	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	jobLog = klog.NewKlogr().WithName("JobService/Cache")
)

type CacheJobService struct {
	*jobService

	jobIndexes *utils.TimeOrderedIndexes[batchv1.Job]
}

func (c *CacheJobService) ListAllBatchJobs(ctx *gin.Context) (*ListJobResult, error) {
	pageSize, err := strconv.ParseUint(ctx.Query("pageSize"), 10, 64)
	if err != nil || pageSize == 0 {
		pageSize = 10
	}
	current, err := strconv.ParseUint(ctx.Query("current"), 10, 64)
	if err != nil || current == 0 {
		current = 1
	}
	descend := ctx.Query("order") == "descend"
	nameFilter := ctx.Query("name")

	required := func(job *batchv1.Job) bool {
		return (nameFilter == "" || strings.Contains(job.Name, nameFilter)) &&
			utils.IsUpgradeJob(job)
	}

	jobs := make([]*batchv1.Job, 0, c.jobIndexes.Length())
	for name := range c.jobIndexes.Iterate(ctx, descend) {
		var job batchv1.Job
		if err := c.client.Get(ctx, name, &job); err == nil && required(&job) {
			jobs = append(jobs, &job)
		}
	}
	result := &ListJobResult{
		Total: len(jobs),
		Jobs:  make([]batchv1.Job, 0),
	}
	startIndex := (current - 1) * pageSize
	if startIndex >= uint64(len(jobs)) {
		return result, nil
	}
	endIndex := startIndex + pageSize
	if endIndex > uint64(len(jobs)) {
		endIndex = uint64(len(jobs))
	}
	for i := startIndex; i < endIndex; i++ {
		result.Jobs = append(result.Jobs, *jobs[i])
	}
	return result, nil
}

func (c *CacheJobService) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	job := &batchv1.Job{}
	if err := c.client.Get(ctx, req.NamespacedName, job); err != nil {
		if apierrors.IsNotFound(err) {
			c.jobIndexes.RemoveIndex(req.NamespacedName)
			return reconcile.Result{}, nil
		}
		jobLog.Error(err, "get job failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, err
	}
	if job.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}
	if utils.IsUpgradeJob(job) {
		c.jobIndexes.AddIndex(
			job,
			func(p *batchv1.Job) metav1.ObjectMeta { return p.ObjectMeta },
			func(name types.NamespacedName) (*batchv1.Job, error) {
				var j batchv1.Job
				err := c.client.Get(ctx, name, &j)
				return &j, err
			},
		)
	}
	jobLog.V(1).Info("job created", "namespacedName", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *CacheJobService) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("job", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}

	return ctr.Watch(source.Kind(mgr.GetCache(), &batchv1.Job{}, &handler.TypedEnqueueRequestForObject[*batchv1.Job]{}, predicate.TypedFuncs[*batchv1.Job]{
		CreateFunc: func(event event.TypedCreateEvent[*batchv1.Job]) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*batchv1.Job]) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*batchv1.Job]) bool {
			job := deleteEvent.Object
			indexes := c.jobIndexes
			if utils.IsUpgradeJob(job) && indexes != nil {
				indexes.RemoveIndex(types.NamespacedName{
					Namespace: job.GetNamespace(),
					Name:      job.GetName(),
				})
				jobLog.V(1).Info("job deleted", "namespace", job.GetNamespace(), "name", job.GetName())
				return false
			}
			return true
		},
	}))
}
