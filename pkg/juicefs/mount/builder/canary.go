/*
Copyright 2024 Juicedata Inc

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

package builder

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type CanaryJobSpec struct {
	Name                    string
	Namespace               string
	Image                   string
	NodeName                string
	Command                 []string
	VolumeMounts            []corev1.VolumeMount
	Volumes                 []corev1.Volume
	TTLSecondsAfterFinished int32
	ServiceAccountName      string
	Labels                  map[string]string
}

func NewCanaryJobFromSpec(spec CanaryJobSpec) *batchv1.Job {
	ttlSecond := spec.TTLSecondsAfterFinished
	if ttlSecond == 0 {
		ttlSecond = 1800
	}
	labels := map[string]string{
		common.CanaryJobLabelKey: spec.Name,
	}
	for k, v := range spec.Labels {
		labels[k] = v
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spec.Name,
					Namespace: spec.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:        spec.Image,
						Name:         "canary",
						Command:      spec.Command,
						VolumeMounts: spec.VolumeMounts,
					}},
					NodeName:           spec.NodeName,
					RestartPolicy:      corev1.RestartPolicyNever,
					Volumes:            spec.Volumes,
					ServiceAccountName: spec.ServiceAccountName,
				},
			},
			Parallelism:             util.ToPtr(int32(1)),
			Completions:             util.ToPtr(int32(1)),
			BackoffLimit:            util.ToPtr(int32(0)),
			TTLSecondsAfterFinished: util.ToPtr(ttlSecond),
		},
	}
}

// NewCanaryJob
// restart: pull image ahead
// !restart: for download binary
func NewCanaryJob(ctx context.Context, client *k8s.K8sClient, mountPod *corev1.Pod, restart bool) (*batchv1.Job, error) {
	setting, err := config.GenSettingAttrWithMountPod(ctx, client, mountPod)
	if err != nil {
		return nil, err
	}
	attr := setting.Attr
	volumeId := mountPod.Labels[common.PodUniqueIdLabelKey]
	name := GenJobNameByVolumeId(volumeId+"-"+config.NodeName) + "-canary"
	if _, err := client.GetJob(ctx, name, config.Namespace); err == nil {
		log.Info("canary job already exists, delete it first", "name", name)
		if err := client.DeleteJob(ctx, name, config.Namespace); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "delete canary job error", "name", name)
			return nil, err
		}
	}

	log.Info("create canary job", "image", attr.Image, "name", name)
	var (
		mounts  []corev1.VolumeMount
		volumes []corev1.Volume
	)
	for _, v := range mountPod.Spec.Volumes {
		if v.Name == config.JfsFuseFdPathName {
			volumes = append(volumes, v)
		}
	}
	for _, c := range mountPod.Spec.Containers[0].VolumeMounts {
		if c.Name == config.JfsFuseFdPathName {
			mounts = append(mounts, c)
		}
	}
	cmd := []string{}
	if !restart {
		ce := util.ContainSubString(mountPod.Spec.Containers[0].Command, "format")
		if ce {
			cmd = []string{"sh", "-c", "cp /usr/local/bin/juicefs /tmp/juicefs"}
		} else {
			cmd = []string{"sh", "-c", "cp /usr/bin/juicefs /tmp/juicefs && cp /usr/local/juicefs/mount/jfsmount /tmp/jfsmount"}
		}
	}
	job := NewCanaryJobFromSpec(CanaryJobSpec{
		Name:         name,
		Namespace:    config.Namespace,
		Image:        attr.Image,
		NodeName:     mountPod.Spec.NodeName,
		Command:      cmd,
		VolumeMounts: mounts,
		Volumes:      volumes,
	})
	job.Spec.Template.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{
		PreStop: &corev1.LifecycleHandler{
			Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "umount /mnt/jfs -l && rmdir /mnt/jfs"}},
		},
	}
	job.Spec.Template.Spec.NodeName = ""
	job.Spec.Template.Spec.NodeSelector = config.CSIPod.Spec.NodeSelector
	job.Spec.Template.Spec.Affinity = config.CSIPod.Spec.Affinity.DeepCopy()
	job.Spec.Template.Spec.Tolerations = util.CopySlice(config.CSIPod.Spec.Tolerations)
	job.Spec.Template.Spec.PriorityClassName = ""
	job.Spec.Template.Spec.PreemptionPolicy = nil
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	return job, nil
}
