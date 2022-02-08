/*
Copyright 2022 Juicedata Inc

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
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func (r *Builder) NewJobForCreateVolume() *batchv1.Job {
	jobName := GenJobNameByVolumeId(r.jfsSetting.VolumeId) + "-createvol"
	job := r.newJob(jobName)
	job.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", r.getCreateVolumeCmd()}
	return job
}

func (r *Builder) NewJobForDeleteVolume() *batchv1.Job {
	jobName := GenJobNameByVolumeId(r.jfsSetting.VolumeId) + "-delvol"
	job := r.newJob(jobName)
	job.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", r.getDeleteVolumeCmd()}
	return job
}

func GenJobNameByVolumeId(volumeId string) string {
	return fmt.Sprintf("juicefs-%s", volumeId)
}

func (r *Builder) newJob(jobName string) *batchv1.Job {
	secretName := jobName + "-secret"
	r.jfsSetting.SecretName = secretName
	podTemplate := r.generateJuicePod()
	ttlSecond := int32(1)
	podTemplate.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{
		PreStop: &corev1.Handler{
			Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "umount /mnt/jfs && rmdir /mnt/jfs"}},
		},
	}
	podTemplate.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: config.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: config.Namespace,
				},
				Spec: podTemplate.Spec,
			},
			TTLSecondsAfterFinished: &ttlSecond,
		},
	}
	return &job
}

func (r *Builder) getCreateVolumeCmd() string {
	cmd := r.getJobCommand()
	return fmt.Sprintf("%s && if [ ! -d /mnt/jfs/%s ]; then mkdir -m 777 /mnt/jfs/%s; fi;", cmd, r.jfsSetting.SubPath, r.jfsSetting.SubPath)
}

func (r *Builder) getDeleteVolumeCmd() string {
	cmd := r.getJobCommand()
	var jfsPath string
	if r.jfsSetting.IsCe {
		jfsPath = config.CeCliPath
	} else {
		jfsPath = config.CliPath
	}
	return fmt.Sprintf("%s && if [ -d /mnt/jfs/%s ]; then %s rmr /mnt/jfs/%s; fi;", cmd, r.jfsSetting.SubPath, jfsPath, r.jfsSetting.SubPath)
}

func (r *Builder) getJobCommand() string {
	var cmd string
	if r.jfsSetting.IsCe {
		args := []string{config.CeMountPath, r.jfsSetting.Source, "/mnt/jfs", "-d"}
		if len(r.jfsSetting.Options) != 0 {
			args = append(args, "-o", strings.Join(r.jfsSetting.Options, ","))
		}
		cmd = strings.Join(args, " ")
	} else {
		args := []string{config.CliPath, "mount", r.jfsSetting.Source, "/mnt/jfs", "-b"}
		if r.jfsSetting.EncryptRsaKey != "" {
			args = append(args, "--rsa-key=/root/.rsa/rsa-key.pem")
		}
		if len(r.jfsSetting.Options) > 0 {
			args = append(args, "-o", strings.Join(r.jfsSetting.Options, ","))
		}
		cmd = strings.Join(args, " ")
	}
	return cmd
}
