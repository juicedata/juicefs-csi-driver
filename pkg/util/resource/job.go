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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

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
