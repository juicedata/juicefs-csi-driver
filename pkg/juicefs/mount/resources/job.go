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

package resources

import (
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func NewJobForCreateVolume(jfsSetting *config.JfsSetting) *batchv1.Job {
	job := newJob(jfsSetting)
	job.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", getCreateVolumeCmd(*jfsSetting)}
	return job
}

func NewJobForDeleteVolume(jfsSetting *config.JfsSetting) *batchv1.Job {
	job := newJob(jfsSetting)
	job.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", getDeleteVolumeCmd(*jfsSetting)}
	return job
}

func newJob(jfsSetting *config.JfsSetting) *batchv1.Job {
	jobName := GenerateNameByVolumeId(jfsSetting.VolumeId)
	podTemplate := generateJuicePod(jfsSetting)
	ttlSecond := int32(1)
	podTemplate.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{
		PreStop: &corev1.Handler{
			Exec: &corev1.ExecAction{Command: []string{"sh", "-c", fmt.Sprintf(
				"umount %s && rmdir %s", jfsSetting.MountPath, jfsSetting.MountPath)}},
		},
	}
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

func getCreateVolumeCmd(jfsSetting config.JfsSetting) string {
	cmd := getJobCommand(jfsSetting)
	return fmt.Sprintf("%s && if [ ! -d /mnt/jfs/%s ]; then mkdir -m 777 /mnt/jfs/%s; fi;", cmd, jfsSetting.SubPath, jfsSetting.SubPath)
}

func getDeleteVolumeCmd(jfsSetting config.JfsSetting) string {
	cmd := getJobCommand(jfsSetting)
	var jfsPath string
	if jfsSetting.IsCe {
		jfsPath = config.CeCliPath
	} else {
		jfsPath = config.CliPath
	}
	return fmt.Sprintf("%s && if [ -d /mnt/jfs/%s ]; then %s rmr /mnt/jfs/%s; fi;", cmd, jfsSetting.SubPath, jfsPath, jfsSetting.SubPath)
}

func getJobCommand(jfsSetting config.JfsSetting) string {
	var cmd string
	if jfsSetting.IsCe {
		args := []string{config.CeMountPath, jfsSetting.Source, jfsSetting.MountPath, "-d"}
		if len(jfsSetting.Options) != 0 {
			args = append(args, "-o", strings.Join(jfsSetting.Options, ","))
		}
		cmd = strings.Join(args, " ")
	} else {
		args := []string{config.JfsMountPath, jfsSetting.Source, jfsSetting.MountPath, "-d"}
		if jfsSetting.EncryptRsaKey != "" {
			args = append(args, "--rsa-key=/root/.rsa/rsa-key.pem")
		}
		if len(jfsSetting.Options) > 0 {
			args = append(args, "-o", strings.Join(jfsSetting.Options, ","))
		}
		cmd = strings.Join(args, " ")
	}
	return cmd
}
