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
	"crypto/sha256"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func (r *Builder) NewJobForCreateVolume() *batchv1.Job {
	jobName := GenJobNameByVolumeId(r.jfsSetting.VolumeId) + "-createvol"
	job := r.newJob(jobName)
	jobCmd := r.getCreateVolumeCmd()
	job.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", jobCmd}
	klog.Infof("create volume job cmd: %s", jobCmd)
	return job
}

func (r *Builder) NewJobForDeleteVolume() *batchv1.Job {
	jobName := GenJobNameByVolumeId(r.jfsSetting.VolumeId) + "-delvol"
	job := r.newJob(jobName)
	jobCmd := r.getDeleteVolumeCmd()
	job.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", jobCmd}
	klog.Infof("delete volume job cmd: %s", jobCmd)
	return job
}

func (r *Builder) NewJobForCleanCache() *batchv1.Job {
	jobName := GenJobNameByVolumeId(r.jfsSetting.VolumeId) + "-cleancache-" + util.RandStringRunes(6)
	job := r.newCleanJob(jobName)
	return job
}

func GenJobNameByVolumeId(volumeId string) string {
	h := sha256.New()
	h.Write([]byte(volumeId))
	return fmt.Sprintf("juicefs-%x", h.Sum(nil))[:16]
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
			Namespace: r.jfsSetting.Attr.Namespace,
			Labels: map[string]string{
				config.PodTypeKey: config.JobTypeValue,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: r.jfsSetting.Attr.Namespace,
				},
				Spec: podTemplate.Spec,
			},
			TTLSecondsAfterFinished: &ttlSecond,
		},
	}
	return &job
}

func (r *Builder) newCleanJob(jobName string) *batchv1.Job {
	podTemplate := r.generateCleanCachePod()
	ttlSecond := int32(5)
	podTemplate.Spec.RestartPolicy = corev1.RestartPolicyNever
	podTemplate.Spec.NodeName = config.NodeName
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.jfsSetting.Attr.Namespace,
			Labels: map[string]string{
				config.PodTypeKey: config.JobTypeValue,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: r.jfsSetting.Attr.Namespace,
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
	options := util.StripReadonlyOption(r.jfsSetting.Options)
	if r.jfsSetting.IsCe {
		args := []string{config.CeMountPath, "${metaurl}", "/mnt/jfs"}
		if len(options) != 0 {
			args = append(args, "-o", strings.Join(options, ","))
		}
		cmd = strings.Join(args, " ")
	} else {
		args := []string{config.JfsMountPath, r.jfsSetting.Source, "/mnt/jfs"}
		if r.jfsSetting.EncryptRsaKey != "" {
			options = append(options, "rsa-key=/root/.rsa/rsa-key.pem")
		}
		options = append(options, "background")
		args = append(args, "-o", strings.Join(options, ","))
		cmd = strings.Join(args, " ")
	}
	return util.QuoteForShell(cmd)
}
