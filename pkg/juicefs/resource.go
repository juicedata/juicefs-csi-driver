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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func GeneratePodNameByVolumeId(volumeId string) string {
	return fmt.Sprintf("%s-%s", NodeName, volumeId)
}

func NewMountPod(podName, cmd, mountPath string) *corev1.Pod {
	isPrivileged := true
	mp := corev1.MountPropagationBidirectional
	dir := corev1.HostPathDirectory
	statCmd := "stat -c %i " + mountPath
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
				Name:            "jfs-mount",
				Image:           MountImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"sh", "-c", cmd},
				SecurityContext: &corev1.SecurityContext{
					Privileged: &isPrivileged,
				},
				Resources: parsePodResources(),
				Env: []corev1.EnvVar{{
					Name:  "JFS_FOREGROUND",
					Value: "1",
				}},
				Ports: []corev1.ContainerPort{{
					Name:          "metrics",
					ContainerPort: 9567,
				}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:             "jfs-dir",
					MountPath:        mountBase,
					MountPropagation: &mp,
				}, {
					Name:             "jfs-root-dir",
					MountPath:        "/root/.juicefs",
					MountPropagation: &mp,
				}},
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", fmt.Sprintf(
							"if [ $(%v) == 1 ]; then exit 0; else exit 1; fi ", statCmd)},
						}},
					InitialDelaySeconds: 1,
					PeriodSeconds:       1,
				},
			}},
			Volumes: []corev1.Volume{{
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
			}},
			NodeName: NodeName,
		},
	}
	controllerutil.AddFinalizer(pod, Finalizer)
	if JFSMountPriorityName != "" {
		pod.Spec.PriorityClassName = JFSMountPriorityName
	}
	return pod
}

func parsePodResources() corev1.ResourceRequirements {
	podLimit := map[corev1.ResourceName]resource.Quantity{}
	podRequest := map[corev1.ResourceName]resource.Quantity{}
	if MountPodCpuLimit != "" {
		podLimit[corev1.ResourceCPU] = resource.MustParse(MountPodCpuLimit)
	}
	if MountPodMemLimit != "" {
		podLimit[corev1.ResourceMemory] = resource.MustParse(MountPodMemLimit)
	}
	if MountPodCpuRequest != "" {
		podRequest[corev1.ResourceCPU] = resource.MustParse(MountPodCpuRequest)
	}
	if MountPodMemRequest != "" {
		podRequest[corev1.ResourceMemory] = resource.MustParse(MountPodMemRequest)
	}
	return corev1.ResourceRequirements{
		Limits:   podLimit,
		Requests: podRequest,
	}
}
