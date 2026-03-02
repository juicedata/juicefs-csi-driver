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

package builder

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/security"
)

type ServerlessBuilder struct {
	PodBuilder
	app corev1.Pod
	pvc corev1.PersistentVolumeClaim
}

var _ SidecarInterface = &ServerlessBuilder{}

func NewServerlessBuilder(setting *config.JfsSetting, capacity int64, app corev1.Pod, pvc corev1.PersistentVolumeClaim) SidecarInterface {
	return &ServerlessBuilder{
		PodBuilder: PodBuilder{
			BaseBuilder: BaseBuilder{
				jfsSetting: setting,
				capacity:   capacity,
			},
		},
		app: app,
		pvc: pvc,
	}
}

// NewMountSidecar generates a pod with a juicefs sidecar in serverless mode
// 1. no hostpath
// 2. use emptyDir as mount point
// 3. with privileged container (the serverless cluster must have this permission.)
// 4. no initContainer
func (r *ServerlessBuilder) NewMountSidecar() *corev1.Pod {
	pod := r.genCommonJuicePod(r.genCommonContainer)

	// no annotation and label for sidecar
	pod.Annotations = map[string]string{}
	pod.Labels = map[string]string{}

	// check mount & create subpath & set quota
	capacity := strconv.FormatInt(r.capacity, 10)
	subpath := r.jfsSetting.SubPath
	community := "ce"
	if !r.jfsSetting.IsCe {
		community = "ee"
	}
	quotaPath := r.getQuotaPath()
	name := r.jfsSetting.Name
	pod.Spec.Containers[0].Lifecycle.PostStart = &corev1.LifecycleHandler{
		Exec: &corev1.ExecAction{Command: []string{"bash", "-c",
			fmt.Sprintf("time subpath=%s name=%s capacity=%s community=%s quotaPath=%s %s '%s' >> /proc/1/fd/1",
				security.EscapeBashStr(subpath),
				security.EscapeBashStr(name),
				capacity,
				community,
				security.EscapeBashStr(quotaPath),
				checkMountScriptPath,
				security.EscapeBashStr(r.jfsSetting.MountPath),
			)}},
	}
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []corev1.EnvVar{{Name: "JFS_NO_UMOUNT", Value: "1"}, {Name: "JFS_FOREGROUND", Value: "1"}}...)
	// generate volumes and volumeMounts only used in serverless sidecar
	volumes, volumeMounts := r.genServerlessVolumes()
	pod.Spec.Volumes = append(pod.Spec.Volumes, volumes...)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, volumeMounts...)

	// add cache-dir PVC volume
	cacheVolumes, cacheVolumeMounts := r.genCacheDirVolumes()
	pod.Spec.Volumes = append(pod.Spec.Volumes, cacheVolumes...)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, cacheVolumeMounts...)

	// command
	mountCmd := r.genMountCommand()
	initCmd := r.genInitCommand()
	cmd := strings.Join([]string{initCmd, mountCmd}, "\n")
	pod.Spec.Containers[0].Command = []string{"sh", "-c", cmd}

	return pod
}

func (r *ServerlessBuilder) OverwriteVolumes(volume *corev1.Volume, mountPath string) {
	// overwrite original volume and use juicefs volume mountpoint instead
	volume.VolumeSource = corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}
}

func (r *ServerlessBuilder) OverwriteVolumeMounts(mount *corev1.VolumeMount) {
	hostToContainer := corev1.MountPropagationHostToContainer
	mount.MountPropagation = &hostToContainer
}

// genServerlessVolumes generates volumes and volumeMounts for serverless sidecar
// 1. jfs dir: mount point as emptyDir, used to propagate the mount point in the mount container to the business container
// 2. jfs-check-mount: secret volume, used to check if the mount point is mounted
func (r *ServerlessBuilder) genServerlessVolumes() ([]corev1.Volume, []corev1.VolumeMount) {
	mp := corev1.MountPropagationBidirectional

	var mode int32 = 0755
	secretName := r.jfsSetting.SecretName
	var sharedVolumeName string
	// get shared volume name
	for _, volume := range r.app.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == r.pvc.Name {
			sharedVolumeName = volume.Name
		}
	}
	volumes := []corev1.Volume{
		{
			Name: "jfs-check-mount",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretName,
					DefaultMode: ptr.To(mode),
					Items: []corev1.KeyToPath{
						{Key: checkMountScriptName, Path: checkMountScriptName},
					},
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:             sharedVolumeName,
			MountPath:        r.jfsSetting.MountPath,
			MountPropagation: &mp,
		},
		// Mount the entire secret directory instead of using subPath to avoid
		// race condition with projected volumes (kubernetes/kubernetes#63726).
		{
			Name:      "jfs-check-mount",
			MountPath: checkMountScriptDir,
			ReadOnly:  true,
		},
	}

	return volumes, volumeMounts
}

// genCacheDirVolumes: in serverless, only support PVC and emptyDir cache, do not support hostpath cache
func (r *ServerlessBuilder) genCacheDirVolumes() ([]corev1.Volume, []corev1.VolumeMount) {
	cacheVolumes := []corev1.Volume{}
	cacheVolumeMounts := []corev1.VolumeMount{}

	for i, cache := range r.jfsSetting.CachePVCs {
		name := fmt.Sprintf("cachedir-pvc-%d", i)
		pvcVolume := corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: cache.PVCName,
					ReadOnly:  false,
				},
			},
		}
		cacheVolumes = append(cacheVolumes, pvcVolume)
		volumeMount := corev1.VolumeMount{
			Name:      name,
			ReadOnly:  false,
			MountPath: cache.Path,
		}
		cacheVolumeMounts = append(cacheVolumeMounts, volumeMount)
	}

	if r.jfsSetting.CacheEmptyDir != nil {
		name := "cachedir-empty-dir"
		emptyVolume := corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium:    corev1.StorageMedium(r.jfsSetting.CacheEmptyDir.Medium),
					SizeLimit: &r.jfsSetting.CacheEmptyDir.SizeLimit,
				},
			},
		}
		cacheVolumes = append(cacheVolumes, emptyVolume)
		volumeMount := corev1.VolumeMount{
			Name:      name,
			ReadOnly:  false,
			MountPath: r.jfsSetting.CacheEmptyDir.Path,
		}
		cacheVolumeMounts = append(cacheVolumeMounts, volumeMount)
	}

	if r.jfsSetting.CacheInlineVolumes != nil {
		for i, inlineVolume := range r.jfsSetting.CacheInlineVolumes {
			name := fmt.Sprintf("cachedir-inline-volume-%d", i)
			cacheVolumes = append(cacheVolumes, corev1.Volume{
				Name:         name,
				VolumeSource: corev1.VolumeSource{CSI: inlineVolume.CSI},
			})
			volumeMount := corev1.VolumeMount{
				Name:      name,
				ReadOnly:  false,
				MountPath: inlineVolume.Path,
			}
			cacheVolumeMounts = append(cacheVolumeMounts, volumeMount)
		}
	}

	// generic ephemeral volumes
	for i, ephemeral := range r.jfsSetting.CacheEphemeral {
		name := fmt.Sprintf("cachedir-ephemeral-%d", i)
		cacheVolumes = append(cacheVolumes, corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				Ephemeral: &corev1.EphemeralVolumeSource{
					VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes:      ephemeral.AccessModes,
							StorageClassName: ephemeral.StorageClassName,
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: ephemeral.Storage,
								},
							},
						},
					},
				},
			},
		})
		cacheVolumeMounts = append(cacheVolumeMounts, corev1.VolumeMount{
			Name:      name,
			ReadOnly:  false,
			MountPath: ephemeral.Path,
		})
	}

	return cacheVolumes, cacheVolumeMounts
}
