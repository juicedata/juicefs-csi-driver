package resources

import (
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

func HasRef(pod *corev1.Pod) bool {
	for k, target := range pod.Annotations {
		if k == util.GetReferenceKey(target) {
			return true
		}
	}
	return false
}

func generateJuicePod(jfsSetting *config.JfsSetting) *corev1.Pod {
	pod := config.GeneratePodTemplate()

	volumes := getVolumes()
	volumeMounts := getVolumeMounts()
	i := 1
	for k, v := range jfsSetting.Configs {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      fmt.Sprintf("config-%v", i),
			MountPath: v,
		})
		volumes = append(volumes, corev1.Volume{
			Name: fmt.Sprintf("config-%v", i),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: k,
				},
			},
		})
		i++
	}

	pod.Spec.Volumes = volumes
	pod.Spec.Containers[0].VolumeMounts = volumeMounts
	return pod
}

func getVolumes() []corev1.Volume {
	dir := corev1.HostPathDirectoryOrCreate
	return []corev1.Volume{{
		Name: "jfs-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: config.MountPointPath,
				Type: &dir,
			},
		}}, {
		Name: "jfs-root-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: config.JFSConfigPath,
				Type: &dir,
			},
		},
	}}
}

func getVolumeMounts() []corev1.VolumeMount {
	mp := corev1.MountPropagationBidirectional
	return []corev1.VolumeMount{{
		Name:             "jfs-dir",
		MountPath:        config.PodMountBase,
		MountPropagation: &mp,
	}, {
		Name:             "jfs-root-dir",
		MountPath:        "/root/.juicefs",
		MountPropagation: &mp,
	}}
}
