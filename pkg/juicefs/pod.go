/*
Copyright 2021 Juicedata Inc

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

package juicefs

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ConfigSecretVolume struct {
	SecretName string
	Key        string
	ConfigPath string
}

func GeneratePodNameByVolumeId(volumeId string) string {
	return fmt.Sprintf("juicefs-%s-%s", NodeName, volumeId)
}

func NewMountPod(podName, cmd, mountPath string, resourceRequirements corev1.ResourceRequirements,
	configs, env map[string]string) *corev1.Pod {
	isPrivileged := true
	mp := corev1.MountPropagationBidirectional
	dir := corev1.HostPathDirectory
	statCmd := "stat -c %i " + mountPath

	volumeMounts := []corev1.VolumeMount{{
		Name:             "jfs-dir",
		MountPath:        mountBase,
		MountPropagation: &mp,
	}, {
		Name:             "jfs-root-dir",
		MountPath:        "/root/.juicefs",
		MountPropagation: &mp,
	}}

	volumes := []corev1.Volume{{
		Name: "jfs-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: MountPointPath,
				Type: &dir,
			},
		},
	}, {
		Name: "jfs-root-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: JFSConfigPath,
				Type: &dir,
			},
		},
	}}

	// add cache-dir host path volume
	if strings.Contains(cmd, "cache-dir") {
		cacheVolumes, cacheVolumeMounts := getCacheDirVolumes(cmd)
		volumes = append(volumes, cacheVolumes...)
		volumeMounts = append(volumeMounts, cacheVolumeMounts...)
	}

	var pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: Namespace,
			Labels: map[string]string{
				PodTypeKey: PodTypeValue,
			},
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "jfs-mount",
				Image:   MountImage,
				Command: []string{"sh", "-c", cmd},
				SecurityContext: &corev1.SecurityContext{
					Privileged: &isPrivileged,
				},
				Resources: resourceRequirements,
				Env: []corev1.EnvVar{{
					Name:  "JFS_FOREGROUND",
					Value: "1",
				}},
				Ports: []corev1.ContainerPort{{
					Name:          "metrics",
					ContainerPort: 9567,
				}},
				VolumeMounts: volumeMounts,
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", fmt.Sprintf(
							"if [ x$(%v) = x1 ]; then exit 0; else exit 1; fi ", statCmd)},
						}},
					InitialDelaySeconds: 1,
					PeriodSeconds:       1,
				},
			}},
			Volumes: volumes,
			NodeName: NodeName,
		},
	}
	controllerutil.AddFinalizer(pod, Finalizer)
	if JFSMountPriorityName != "" {
		pod.Spec.PriorityClassName = JFSMountPriorityName
	}
	i := 1
	for k, v := range configs {
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      fmt.Sprintf("config-%v", i),
				MountPath: v,
			})
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: fmt.Sprintf("config-%v", i),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: k,
				},
			},
		})
		i++
	}
	for k, v := range env {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	return pod
}

func parsePodResources(mountPodCpuLimit, mountPodMemLimit, mountPodCpuRequest, mountPodMemRequest string) corev1.ResourceRequirements {
	podLimit := map[corev1.ResourceName]resource.Quantity{}
	podRequest := map[corev1.ResourceName]resource.Quantity{}
	if mountPodCpuLimit != "" {
		podLimit[corev1.ResourceCPU] = resource.MustParse(mountPodCpuLimit)
	}
	if mountPodMemLimit != "" {
		podLimit[corev1.ResourceMemory] = resource.MustParse(mountPodMemLimit)
	}
	if mountPodCpuRequest != "" {
		podRequest[corev1.ResourceCPU] = resource.MustParse(mountPodCpuRequest)
	}
	if mountPodMemRequest != "" {
		podRequest[corev1.ResourceMemory] = resource.MustParse(mountPodMemRequest)
	}
	return corev1.ResourceRequirements{
		Limits:   podLimit,
		Requests: podRequest,
	}
}

func getCacheDirVolumes(cmd string) ([]corev1.Volume, []corev1.VolumeMount) {
	var cacheVolumes []corev1.Volume
	var cacheVolumeMounts []corev1.VolumeMount

	cmdSplits := strings.Split(cmd, " -o ")
	if len(cmdSplits) != 2 {
		return cacheVolumes, cacheVolumeMounts
	}

	hostPathType := corev1.HostPathDirectoryOrCreate
	mountPropagation := corev1.MountPropagationBidirectional

	for _, optSubStr := range strings.Split(cmdSplits[1], ",") {
		optValStr := strings.TrimSpace(optSubStr)
		if !strings.HasPrefix(optValStr, "cache-dir") {
			continue
		}
		optValPair := strings.Split(optValStr, "=")
		if len(optValPair) != 2 {
			continue
		}
		cacheDirs := strings.Split(strings.TrimSpace(optValPair[1]), ":")

		for _, cacheDir := range cacheDirs {
			dirTrimPrefix := strings.TrimPrefix(cacheDir, "/")
			name := strings.ReplaceAll(dirTrimPrefix, "/", "-")

			hostPath := corev1.HostPathVolumeSource{
				Path: cacheDir,
				Type: &hostPathType,
			}
			hostPathVolume := corev1.Volume{
				Name: name,
				VolumeSource: corev1.VolumeSource{
					HostPath: &hostPath,
				},
			}
			cacheVolumes = append(cacheVolumes, hostPathVolume)

			volumeMount := corev1.VolumeMount{
				Name: name,
				MountPath: cacheDir,
				MountPropagation: &mountPropagation,
			}
			cacheVolumeMounts = append(cacheVolumeMounts, volumeMount)
		}
	}

	return cacheVolumes, cacheVolumeMounts
}
