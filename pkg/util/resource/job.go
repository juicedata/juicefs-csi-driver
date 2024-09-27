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

package resource

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var log = klog.NewKlogr().WithName("job-util")

func IsJobCompleted(job *batchv1.Job) bool {
	for _, cond := range job.Status.Conditions {
		if cond.Status == corev1.ConditionTrue && cond.Type == batchv1.JobComplete {
			return true
		}
	}
	return false
}

func IsJobFailed(job *batchv1.Job) bool {
	for _, cond := range job.Status.Conditions {
		if cond.Status == corev1.ConditionTrue && cond.Type == batchv1.JobFailed {
			return true
		}
	}
	return false
}

func IsJobShouldBeRecycled(job *batchv1.Job) bool {
	// job not completed or not failed, should not be recycled
	if !IsJobCompleted(job) && !IsJobFailed(job) {
		return false
	}
	// job not set ttl, should be recycled
	if job.Spec.TTLSecondsAfterFinished == nil {
		return true
	}

	// job completionTime is nil, may be failed, should not be recycled. (will be recycled after ttl)
	if job.Status.CompletionTime == nil {
		return false
	}

	// job exits after ttl time, should be recycled (should not happen)
	ttlTime := job.Status.CompletionTime.Add(time.Duration(*job.Spec.TTLSecondsAfterFinished) * time.Second)
	return ttlTime.Before(time.Now())
}

func WaitForJobComplete(ctx context.Context, client *k8s.K8sClient, name string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// Wait until the mount point is ready
	log.Info("waiting for job complete", "name", name)
	timer := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("job %s is not complete eventually", name)
		case <-timer.C:
			job, err := client.GetJob(waitCtx, name, config.Namespace)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return fmt.Errorf("job %s is not complete eventually", name)
				}
				if k8serrors.IsNotFound(err) {
					return nil
				}
			}
			if IsJobCompleted(job) {
				return nil
			}
		}
	}
}
