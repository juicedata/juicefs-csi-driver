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

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/security"
)

const (
	CCIANNOKey    = "virtual-kubelet.io/burst-to-cci"
	CCIANNOValue  = "enforce"
	CCIDriverName = "juicefs.csi.everest.io"
	CCIDriverType = "gpath"
)

type CCIBuilder struct {
	ServerlessBuilder
	pvc corev1.PersistentVolumeClaim
	app corev1.Pod
}

var _ SidecarInterface = &CCIBuilder{}

func NewCCIBuilder(setting *config.JfsSetting, capacity int64, app corev1.Pod, pvc corev1.PersistentVolumeClaim) SidecarInterface {
	return &CCIBuilder{
		ServerlessBuilder: ServerlessBuilder{PodBuilder: PodBuilder{
			BaseBuilder: BaseBuilder{
				jfsSetting: setting,
				capacity:   capacity,
			},
		}},
		pvc: pvc,
		app: app,
	}
}

// NewMountSidecar generates a pod with a juicefs sidecar in serverless mode
// 1. no hostpath
// 2. without privileged container
// 3. no propagationBidirectional
// 4. with env JFS_NO_UMOUNT=1
// 5. annotations for CCI
func (r *CCIBuilder) NewMountSidecar() *corev1.Pod {
	pod := r.genCommonJuicePod(r.genNonPrivilegedContainer)

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
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, []corev1.EnvVar{
		{Name: "JFS_NO_UMOUNT", Value: "1"},
		{Name: "JFS_FOREGROUND", Value: "1"},
		{Name: "JUICEFS_CLIENT_SIDERCAR_CONTAINER", Value: "true"},
		{Name: "JUICEFS_CLIENT_PATH", Value: "/jfs"},
	}...)

	// generate volumes and volumeMounts only used in CCI serverless sidecar
	volumes, volumeMounts := r.genCCIServerlessVolumes()
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

func (r *CCIBuilder) OverwriteVolumes(volume *corev1.Volume, mountPath string) {
	driverType := CCIDriverType
	volume.VolumeSource = corev1.VolumeSource{
		CSI: &corev1.CSIVolumeSource{
			Driver:           CCIDriverName,
			FSType:           &driverType,
			VolumeAttributes: map[string]string{"mountpoint": mountPath},
		},
	}
}

func (r *CCIBuilder) OverwriteVolumeMounts(mount *corev1.VolumeMount) {
	none := corev1.MountPropagationNone
	mount.MountPropagation = &none
}

// genCCIServerlessVolumes generates volumes and volumeMounts for serverless sidecar
// 1. jfs-check-mount: secret volume, used to check if the mount point is mounted
func (r *CCIBuilder) genCCIServerlessVolumes() ([]corev1.Volume, []corev1.VolumeMount) {
	var mode int32 = 0755
	secretName := r.jfsSetting.SecretName

	volumes := []corev1.Volume{
		{
			Name: "jfs-check-mount",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretName,
					DefaultMode: ptr.To(mode),
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "jfs-check-mount",
			MountPath: checkMountScriptPath,
			SubPath:   checkMountScriptName,
		},
	}

	return volumes, volumeMounts
}

func (r *CCIBuilder) genNonPrivilegedContainer() corev1.Container {
	rootUser := int64(0)
	return corev1.Container{
		Name:  common.MountContainerName,
		Image: r.BaseBuilder.jfsSetting.Attr.Image,
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"SYS_ADMIN",
					"MKNOD",
				},
			},
			RunAsUser: &rootUser,
		},
		Env: []corev1.EnvVar{
			{
				Name:  common.JfsInsideContainer,
				Value: "1",
			},
		},
	}
}
