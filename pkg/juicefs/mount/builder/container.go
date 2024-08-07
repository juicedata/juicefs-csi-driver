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
	"path/filepath"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	utilpointer "k8s.io/utils/pointer"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/security"
)

type ContainerBuilder struct {
	PodBuilder
}

var _ SidecarInterface = &ContainerBuilder{}

func NewContainerBuilder(setting *config.JfsSetting, capacity int64) SidecarInterface {
	return &ContainerBuilder{PodBuilder{
		BaseBuilder: BaseBuilder{
			jfsSetting: setting,
			capacity:   capacity,
		}},
	}
}

// NewMountSidecar generates a pod with a juicefs sidecar
// exactly the same spec as Mount Pod
// except fuse passfd path
func (r *ContainerBuilder) NewMountSidecar() *corev1.Pod {
	pod := r.NewMountPod("")
	// no annotation and label for sidecar
	pod.Annotations = map[string]string{}
	pod.Labels = map[string]string{}

	volumes, volumeMounts := r.genSidecarVolumes()
	pod.Spec.Volumes = append(pod.Spec.Volumes, volumes...)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, volumeMounts...)

	// delete fuse passfd path
	for i, vm := range pod.Spec.Containers[0].VolumeMounts {
		if vm.Name == JfsFuseFdPathName {
			pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts[:i], pod.Spec.Containers[0].VolumeMounts[i+1:]...)
			break
		}
	}
	for i, v := range pod.Spec.Volumes {
		if v.Name == JfsFuseFdPathName {
			pod.Spec.Volumes = append(pod.Spec.Volumes[:i], pod.Spec.Volumes[i+1:]...)
			break
		}
	}

	// check mount & create subpath & set quota
	capacity := strconv.FormatInt(r.capacity, 10)
	subpath := r.jfsSetting.SubPath
	community := "ce"
	if !r.jfsSetting.IsCe {
		community = "ee"
	}
	quotaPath := r.getQuotaPath()
	name := r.jfsSetting.Name
	if pod.Spec.Containers[0].Lifecycle == nil {
		pod.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{}
	}
	pod.Spec.Containers[0].Lifecycle.PostStart = &corev1.Handler{
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

	// overwrite subdir
	r.overwriteSubdirWithSubPath()

	mountCmd := r.genMountCommand()
	cmd := mountCmd
	initCmd := r.genInitCommand()
	if initCmd != "" {
		cmd = strings.Join([]string{initCmd, mountCmd}, "\n")
	}
	pod.Spec.Containers[0].Command = []string{"sh", "-c", cmd}
	return pod
}

func (r *ContainerBuilder) OverwriteVolumeMounts(mount *corev1.VolumeMount) {
	// do not overwrite volumeMounts
}

func (r *ContainerBuilder) OverwriteVolumes(volume *corev1.Volume, mountPath string) {
	// overwrite original volume and use juicefs volume mountpoint instead
	hostMount := filepath.Join(config.MountPointPath, mountPath)
	volume.VolumeSource = corev1.VolumeSource{
		HostPath: &corev1.HostPathVolumeSource{
			Path: hostMount,
		},
	}
}

// genSidecarVolumes generates volumes and volumeMounts for sidecar container
// extra volumes and volumeMounts are used to check mount status
func (r *ContainerBuilder) genSidecarVolumes() (volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) {
	var mode int32 = 0755
	secretName := r.jfsSetting.SecretName
	volumes = []corev1.Volume{{
		Name: "jfs-check-mount",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretName,
				DefaultMode: utilpointer.Int32Ptr(mode),
			},
		},
	}}
	volumeMounts = []corev1.VolumeMount{{
		Name:      "jfs-check-mount",
		MountPath: checkMountScriptPath,
		SubPath:   checkMountScriptName,
	}}
	return
}
