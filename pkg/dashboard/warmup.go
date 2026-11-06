/*
 * Copyright 2026 Juicedata Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dashboard

import (
	"context"
	"sort"

	"github.com/gin-gonic/gin"
	juicefsiov1 "github.com/juicedata/juicefs-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (api *API) listWarmUps() gin.HandlerFunc {
	return func(c *gin.Context) {
		warmups := juicefsiov1.WarmUpList{}
		if err := api.cachedReader.List(c, &warmups); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, warmups.Items)
	}
}

func (api *API) createWarmUp() gin.HandlerFunc {
	return func(c *gin.Context) {
		var warmup juicefsiov1.WarmUp
		if err := c.ShouldBindJSON(&warmup); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := api.mgrClient.Create(c, &warmup); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, warmup)
	}
}

func (api *API) getWarmUp() gin.HandlerFunc {
	return func(c *gin.Context) {
		warmup := juicefsiov1.WarmUp{}
		if err := api.cachedReader.Get(c, types.NamespacedName{Namespace: c.Param("namespace"), Name: c.Param("name")}, &warmup); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, warmup)
	}
}

func (api *API) deleteWarmUp() gin.HandlerFunc {
	return func(c *gin.Context) {
		warmup := juicefsiov1.WarmUp{}
		if err := api.mgrClient.Get(c, types.NamespacedName{Namespace: c.Param("namespace"), Name: c.Param("name")}, &warmup); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if err := api.mgrClient.Delete(c, &warmup); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "succeed"})
	}
}

type warmUpPod struct {
	Name              string          `json:"name"`
	Job               string          `json:"job"`
	Phase             corev1.PodPhase `json:"phase"`
	CreationTimestamp metav1.Time     `json:"creationTimestamp"`
}

func (api *API) warmUpPods(ctx context.Context, namespace, name string) ([]warmUpPod, error) {
	warmup := juicefsiov1.WarmUp{}
	if err := api.cachedReader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &warmup); err != nil {
		return nil, err
	}
	owners := map[types.UID]bool{warmup.UID: true}
	if warmup.Spec.Policy.Type == juicefsiov1.PolicyTypeCron {
		cronJobs := batchv1.CronJobList{}
		if err := api.cachedReader.List(ctx, &cronJobs, client.InNamespace(namespace)); err != nil {
			return nil, err
		}
		for _, cronJob := range cronJobs.Items {
			if owner := metav1.GetControllerOf(&cronJob); owner != nil && owner.UID == warmup.UID {
				owners[cronJob.UID] = true
			}
		}
	}
	jobs := batchv1.JobList{}
	if err := api.cachedReader.List(ctx, &jobs, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	jobNames := map[types.UID]string{}
	for _, job := range jobs.Items {
		if owner := metav1.GetControllerOf(&job); owner != nil && owners[owner.UID] {
			jobNames[job.UID] = job.Name
		}
	}
	pods := corev1.PodList{}
	if err := api.cachedReader.List(ctx, &pods, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := []warmUpPod{}
	for _, pod := range pods.Items {
		if owner := metav1.GetControllerOf(&pod); owner != nil {
			if job, ok := jobNames[owner.UID]; ok {
				result = append(result, warmUpPod{Name: pod.Name, Job: job, Phase: pod.Status.Phase, CreationTimestamp: pod.CreationTimestamp})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreationTimestamp.After(result[j].CreationTimestamp.Time) })
	return result, nil
}

func (api *API) listWarmUpPods() gin.HandlerFunc {
	return func(c *gin.Context) {
		pods, err := api.warmUpPods(c, c.Param("namespace"), c.Param("name"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, pods)
	}
}

func (api *API) isWarmUpPod(ctx context.Context, pod *corev1.Pod) bool {
	podOwner := metav1.GetControllerOf(pod)
	if podOwner == nil || podOwner.Kind != "Job" {
		return false
	}
	job := batchv1.Job{}
	if err := api.cachedReader.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: podOwner.Name}, &job); err != nil || job.UID != podOwner.UID {
		return false
	}
	owner := metav1.GetControllerOf(&job)
	if owner == nil {
		return false
	}
	if owner.Kind == "CronJob" {
		cronJob := batchv1.CronJob{}
		if err := api.cachedReader.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: owner.Name}, &cronJob); err != nil || cronJob.UID != owner.UID {
			return false
		}
		owner = metav1.GetControllerOf(&cronJob)
	}
	if owner == nil || owner.Kind != "WarmUp" || owner.APIVersion != "juicefs.io/v1" {
		return false
	}
	warmup := juicefsiov1.WarmUp{}
	return api.cachedReader.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: owner.Name}, &warmup) == nil && warmup.UID == owner.UID
}
