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
	"strings"

	corev1 "k8s.io/api/core/v1"
	utilpointer "k8s.io/utils/pointer"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

const (
	VCIANNOKey                = "vke.volcengine.com/burst-to-vci"
	VCIANNOValue              = "enforce"
	VCIPropagation            = "vci.vke.volcengine.com/config-bidirectional-mount-propagation"
	VCIPropagationConfig      = "vke.al.vci-enable-bidirectional-mount-propagation"
	VCIPropagationConfigValue = "vke.al.vci-enable-bidirectional-mount-propagation"
)

type VCIBuilder struct {
	ServerlessBuilder
	pvc corev1.PersistentVolumeClaim
	app corev1.Pod
}

var _ SidecarInterface = &VCIBuilder{}

func NewVCIBuilder(setting *config.JfsSetting, capacity int64, app corev1.Pod, pvc corev1.PersistentVolumeClaim) SidecarInterface {
	return &VCIBuilder{
		ServerlessBuilder: ServerlessBuilder{PodBuilder{BaseBuilder{
			jfsSetting: setting,
			capacity:   capacity,
		}}},
		pvc: pvc,
		app: app,
	}
}

// NewMountSidecar generates a pod with a juicefs sidecar in serverless mode
// 1. no hostpath
// 2. without privileged container
// 3. no propagationBidirectional
// 4. with env JFS_NO_UMOUNT=1
// 5. annotations for VCI
func (r *VCIBuilder) NewMountSidecar() *corev1.Pod {
	pod := r.genCommonJuicePod(r.genNonPrivilegedContainer)
	// overwrite annotation
	pod.Annotations = map[string]string{
		VCIPropagationConfig: VCIPropagationConfigValue,
		VCIPropagation: fmt.Sprintf(`[{
			"container": "jfs-mount",
			"mountPath" : "%s"
		}]`, r.jfsSetting.MountPath),
	}
	if len(pod.Spec.InitContainers) != 0 {
		pod.Annotations = map[string]string{
			VCIPropagationConfig: VCIPropagationConfigValue,
			VCIPropagation: fmt.Sprintf(`[{
			"container": "jfs-mount",
			"mountPath" : "%s"
		}, {
			"container": "jfs-init",
			"mountPath" : "/mnt/jfs"
		}]`, r.jfsSetting.MountPath),
		}
	}

	pod.Spec.Containers[0].Lifecycle.PostStart = &corev1.Handler{
		Exec: &corev1.ExecAction{Command: []string{"bash", "-c",
			fmt.Sprintf("time %s %s >> /proc/1/fd/1", checkMountScriptPath, r.jfsSetting.MountPath)}},
	}
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []corev1.EnvVar{{Name: "JFS_NO_UMOUNT", Value: "1"}, {Name: "JFS_FOREGROUND", Value: "1"}}...)

	// generate volumes and volumeMounts only used in VCI serverless sidecar
	volumes, volumeMounts := r.genVCIServerlessVolumes()
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

func (r *VCIBuilder) OverwriteVolumes(volume *corev1.Volume, mountPath string) {
	volume.VolumeSource = corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}
}

func (r *VCIBuilder) OverwriteVolumeMounts(mount *corev1.VolumeMount) {
	hostToContainer := corev1.MountPropagationHostToContainer
	mount.MountPropagation = &hostToContainer
}

// genVCIServerlessVolumes generates volumes and volumeMounts for serverless sidecar
// 1. jfs dir: mount point as emptyDir, used to propagate the mount point in the mount container to the business container
// 2. jfs-check-mount: secret volume, used to check if the mount point is mounted
func (r *VCIBuilder) genVCIServerlessVolumes() ([]corev1.Volume, []corev1.VolumeMount) {
	var mode int32 = 0755
	var sharedVolumeName string
	secretName := r.jfsSetting.SecretName

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
					DefaultMode: utilpointer.Int32Ptr(mode),
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      sharedVolumeName,
			MountPath: r.jfsSetting.MountPath,
		},
		{
			Name:      "jfs-check-mount",
			MountPath: checkMountScriptPath,
			SubPath:   checkMountScriptName,
		},
	}

	return volumes, volumeMounts
}

func (r *PodBuilder) genNonPrivilegedContainer() corev1.Container {
	rootUser := int64(0)
	return corev1.Container{
		Name:  config.MountContainerName,
		Image: r.BaseBuilder.jfsSetting.Attr.Image,
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: &rootUser,
		},
		Env: []corev1.EnvVar{},
	}
}
