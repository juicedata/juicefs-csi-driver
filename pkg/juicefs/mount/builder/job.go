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
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
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
	podTemplate.Spec.Affinity = config.CSIPod.Spec.Affinity.DeepCopy()
	podTemplate.Spec.Tolerations = util.CopySlice(config.CSIPod.Spec.Tolerations)
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
	supFusePass := util.SupportFusePass(mountpod)
	command := fmt.Sprintf(`set -x
supFusePass=%t
if [ $supFusePass = true ]; then
  attempt=1
  while [ $attempt -le 5 ]; do
    if inode=$(timeout 1 stat -c %%i %s 2>/dev/null) && [ "$inode" = "1" ]; then
      echo "fuse mount point is normal, exit 0"
      exit 0
    fi
    sleep 1
    attempt=$((attempt+1))
  done
fi

if [ $(cat /sys/fs/fuse/connections/%d/waiting) -eq 0 ]; then
  echo "fuse connections 'waiting' is zero, skip"
fi

echo "fuse mount point is hung or deadlocked, aborting..."
echo 1 > /sys/fs/fuse/connections/%d/abort
`, supFusePass, mntPath, devMinor, devMinor)

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
								command,
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

// NewJobForSnapshot creates a Job to create a snapshot using juicefs clone
func (r *JobBuilder) NewJobForSnapshot(snapshotID, sourceVolumeID string, secrets map[string]string) *batchv1.Job {
	jobName := fmt.Sprintf("juicefs-snapshot-%s", snapshotID[:8])
	ttlSecond := int32(60) // Clean up after 1 minute
	backoffLimit := int32(2)

	// Determine mount image based on CE/EE
	mountImage := config.DefaultEEMountImage
	if r.jfsSetting.IsCe {
		mountImage = config.DefaultCEMountImage
	}

	cmd := fmt.Sprintf(`
set -ex
echo "=========================================="
echo "JuiceFS Snapshot Creation"
echo "Snapshot: %s"
echo "Source Volume: %s"
echo "=========================================="

echo "Mounting JuiceFS at root level..."
/usr/local/bin/juicefs mount -d "%s" /jfs --no-syslog
sleep 2

echo "Creating snapshot directory..."
mkdir -p /jfs/.snapshots/%s

echo "Cloning volume to snapshot..."
juicefs clone /jfs/%s /jfs/.snapshots/%s/%s

echo "=========================================="
echo "Snapshot created successfully!"
echo "=========================================="

umount /jfs || true
`, snapshotID, sourceVolumeID, secrets["metaurl"], sourceVolumeID, sourceVolumeID, sourceVolumeID, snapshotID)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.jfsSetting.Attr.Namespace,
			Labels: map[string]string{
				common.PodTypeKey: common.JobTypeValue,
				"app":             "juicefs-snapshot",
				"snapshot":        snapshotID,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: util.ToPtr(ttlSecond),
			BackoffLimit:            util.ToPtr(backoffLimit),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "juicefs-snapshot",
						"job": jobName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "snapshot",
							Image:   mountImage,
							Command: []string{"sh", "-c", cmd},
							SecurityContext: &corev1.SecurityContext{
								Privileged: util.ToPtr(true),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SYS_ADMIN"},
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ACCESS_KEY", Value: secrets["access-key"]},
								{Name: "SECRET_KEY", Value: secrets["secret-key"]},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    k8sresource.MustParse("100m"),
									corev1.ResourceMemory: k8sresource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    k8sresource.MustParse("1"),
									corev1.ResourceMemory: k8sresource.MustParse("512Mi"),
								},
							},
						},
					},
				},
			},
		},
	}
}

// NewJobForRestore creates a Job to restore a snapshot using juicefs clone
func (r *JobBuilder) NewJobForRestore(snapshotID, sourceVolumeID, targetVolumeID string, secrets map[string]string) *batchv1.Job {
	jobName := fmt.Sprintf("juicefs-restore-%s", targetVolumeID[:8])
	ttlSecond := int32(300) // Clean up after 5 minutes
	backoffLimit := int32(3)

	// Determine mount image based on CE/EE
	mountImage := config.DefaultEEMountImage
	if r.jfsSetting.IsCe {
		mountImage = config.DefaultCEMountImage
	}

	cmd := fmt.Sprintf(`
set -ex
echo "==========================================)"
echo "JuiceFS Snapshot Restore"
echo "Time: $(date)"
echo "Snapshot: %s"
echo "Source Volume: %s"
echo "Target Volume: %s"
echo "=========================================="

echo "Mounting JuiceFS at root level..."
/usr/local/bin/juicefs mount -d %s /jfs --no-syslog
sleep 2

echo "Preparing target directory..."
# If target exists and is empty, remove it so clone can create it
# If target has files, clone will fail (which is correct - we shouldn't overwrite)
if [ -d "/jfs/%s" ]; then
	if [ -z "$(ls -A /jfs/%s)" ]; then
		echo "Target directory exists but is empty, removing it..."
		rmdir /jfs/%s
	else
		echo "Target directory exists and has files, aborting!"
		exit 1
	fi
fi

echo "Cloning snapshot to new volume using native juicefs clone..."
juicefs clone /jfs/.snapshots/%s/%s /jfs/%s

echo "=========================================="
echo "Restore completed successfully!"
echo "Time: $(date)"
echo "=========================================="

umount /jfs || true
`, snapshotID, sourceVolumeID, targetVolumeID, secrets["metaurl"], targetVolumeID, targetVolumeID, targetVolumeID, sourceVolumeID, snapshotID, targetVolumeID)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: r.jfsSetting.Attr.Namespace,
			Labels: map[string]string{
				common.PodTypeKey: common.JobTypeValue,
				"app":             "juicefs-restore",
				"snapshot":        snapshotID,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: util.ToPtr(ttlSecond),
			BackoffLimit:            util.ToPtr(backoffLimit),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "juicefs-restore",
						"job": jobName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "restore",
							Image:   mountImage,
							Command: []string{"sh", "-c", cmd},
							SecurityContext: &corev1.SecurityContext{
								Privileged: util.ToPtr(true),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SYS_ADMIN"},
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ACCESS_KEY", Value: secrets["access-key"]},
								{Name: "SECRET_KEY", Value: secrets["secret-key"]},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    k8sresource.MustParse("100m"),
									corev1.ResourceMemory: k8sresource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    k8sresource.MustParse("1"),
									corev1.ResourceMemory: k8sresource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
		},
	}
}
