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
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/security"
)

var log = klog.NewKlogr().WithName("job-builder")

const DefaultJobTTLSecond = int32(5)

type JobBuilder struct {
	PodBuilder
}

func NewJobBuilder(setting *config.JfsSetting, capacity int64) *JobBuilder {
	return &JobBuilder{PodBuilder{
		BaseBuilder: BaseBuilder{
			jfsSetting: setting,
			capacity:   capacity,
		}},
	}
}

func (r *JobBuilder) NewJobForCreateVolume() *batchv1.Job {
	jobName := GenJobNameByVolumeId(r.jfsSetting.VolumeId) + "-createvol"
	job := r.newJob(jobName)
	jobCmd := r.getCreateVolumeCmd()
	initCmd := r.genInitCommand()
	cmd := strings.Join([]string{initCmd, jobCmd}, "\n")
	job.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", cmd}

	builderLog.Info("create volume job", "command", jobCmd)
	return job
}

func (r *JobBuilder) NewJobForDeleteVolume() *batchv1.Job {
	jobName := GenJobNameByVolumeId(r.jfsSetting.VolumeId) + "-delvol"
	job := r.newJob(jobName)
	jobCmd := r.getDeleteVolumeCmd()
	initCmd := r.genInitCommand()
	cmd := strings.Join([]string{initCmd, jobCmd}, "\n")
	job.Spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", cmd}
	builderLog.Info("delete volume job", "command", jobCmd)
	return job
}

func (r *JobBuilder) NewJobForCleanCache() *batchv1.Job {
	jobName := GenJobNameByVolumeId(r.jfsSetting.VolumeId) + "-cleancache-" + util.RandStringRunes(6)
	job := r.newCleanJob(jobName)
	return job
}

func GenJobNameByVolumeId(volumeId string) string {
	h := sha256.New()
	h.Write([]byte(volumeId))
	return fmt.Sprintf("juicefs-%x", h.Sum(nil))[:16]
}

func (r *JobBuilder) newJob(jobName string) *batchv1.Job {
	secretName := jobName + "-secret"
	r.jfsSetting.SecretName = secretName
	podTemplate := r.genCommonJuicePod(r.genCommonContainer)
	ttlSecond := DefaultJobTTLSecond
	podTemplate.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{
		PreStop: &corev1.LifecycleHandler{
			Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "umount /mnt/jfs -l && rmdir /mnt/jfs"}},
		},
	}
	// set node name to empty to let k8s scheduler to choose a node
	podTemplate.Spec.NodeName = ""
	// set NodeSelector/Affinity/Tolerations follow the csi-node
	podTemplate.Spec.NodeSelector = config.CSIPod.Spec.NodeSelector
	podTemplate.Spec.Affinity = config.CSIPod.Spec.Affinity
	podTemplate.Spec.Tolerations = config.CSIPod.Spec.Tolerations
	// set priority class name to empty to make job use default priority class
	podTemplate.Spec.PriorityClassName = ""
	podTemplate.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.jfsSetting.Attr.Namespace,
			Labels: map[string]string{
				common.PodTypeKey: common.JobTypeValue,
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

func (r *JobBuilder) newCleanJob(jobName string) *batchv1.Job {
	podTemplate := r.genCleanCachePod()
	ttlSecond := DefaultJobTTLSecond
	podTemplate.Spec.RestartPolicy = corev1.RestartPolicyNever
	podTemplate.Spec.NodeName = config.NodeName
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.jfsSetting.Attr.Namespace,
			Labels: map[string]string{
				common.PodTypeKey: common.JobTypeValue,
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

func (r *JobBuilder) getCreateVolumeCmd() string {
	cmd := r.getJobCommand()
	subpath := security.EscapeBashStr(r.jfsSetting.SubPath)
	return fmt.Sprintf("%s && if [ ! -d /mnt/jfs/%s ]; then mkdir -m 777 /mnt/jfs/%s; fi;", cmd, subpath, subpath)
}

func (r *JobBuilder) getDeleteVolumeCmd() string {
	cmd := r.getJobCommand()
	var jfsPath string
	if r.jfsSetting.IsCe {
		jfsPath = config.CeCliPath
	} else {
		jfsPath = config.CliPath
	}
	subpath := security.EscapeBashStr(r.jfsSetting.SubPath)
	return fmt.Sprintf("%s && if [ -d /mnt/jfs/%s ]; then %s rmr /mnt/jfs/%s; fi;", cmd, subpath, jfsPath, subpath)
}

func NewFuseAbortJob(mountpod *corev1.Pod, devMinor uint32, mntPath string) *batchv1.Job {
	jobName := fmt.Sprintf("%s-abort-fuse", GenJobNameByVolumeId(mountpod.Name))
	ttlSecond := DefaultJobTTLSecond
	privileged := true
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: mountpod.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlSecond,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "fuse-abort",
							Image:           mountpod.Spec.Containers[0].Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"sh",
								"-c",
								fmt.Sprintf(`
                                    attempt=1
									while [ $attempt -le 5 ]; do
										if timeout 1 stat "%s"; then
											echo "fuse mount success, exit 0"
											exit 0
										fi
											sleep 1
											attempt=$((attempt+1))
									done

									echo "fuse mount failed, aborting..."

									if [ $(cat /sys/fs/fuse/connections/%d/waiting) -gt 0 ]; then 
										echo 1 > /sys/fs/fuse/connections/%d/abort 	
									fi
                                `, mntPath, devMinor, devMinor),
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "fuse-connections",
									MountPath: "/sys/fs/fuse/connections",
								},
								{
									Name:      "jfs-dir",
									MountPath: "/jfs",
								},
							},
						},
					},
					NodeName:      mountpod.Spec.NodeName,
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "fuse-connections",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/sys/fs/fuse/connections",
								},
							},
						},
						{
							Name: "jfs-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: config.MountPointPath,
								},
							},
						},
					},
				},
			},
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
	name := GenJobNameByVolumeId(volumeId) + "-canary"
	if _, err := client.GetJob(ctx, name, config.Namespace); err == nil {
		log.Info("canary job already exists, delete it first", "name", name)
		if err := client.DeleteJob(ctx, name, config.Namespace); err != nil {
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
	cmd := ""
	if !restart {
		ce := util.ContainSubString(mountPod.Spec.Containers[0].Command, "format")
		if ce {
			cmd = "cp /usr/local/bin/juicefs /tmp/juicefs"
		} else {
			cmd = "cp /usr/bin/juicefs /tmp/juicefs && cp /usr/local/juicefs/mount/jfsmount /tmp/jfsmount"
		}
	}
	ttl := DefaultJobTTLSecond
	cJob := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: config.Namespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:        attr.Image,
						Name:         "canary",
						Command:      []string{"sh", "-c", cmd},
						VolumeMounts: mounts,
					}},
					NodeName:      mountPod.Spec.NodeName,
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       volumes,
				},
			},
			TTLSecondsAfterFinished: &ttl,
		},
	}
	return &cJob, nil
}
