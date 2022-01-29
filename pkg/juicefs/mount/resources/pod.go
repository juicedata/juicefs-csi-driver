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

package resources

import (
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func GenerateNameByVolumeId(volumeId string) string {
	return fmt.Sprintf("juicefs-%s-%s", config.NodeName, volumeId)
}

func NewMountPod(jfsSetting *config.JfsSetting) *corev1.Pod {
	podName := GenerateNameByVolumeId(jfsSetting.VolumeId)
	resourceRequirements := parsePodResources(
		jfsSetting.MountPodCpuLimit,
		jfsSetting.MountPodMemLimit,
		jfsSetting.MountPodCpuRequest,
		jfsSetting.MountPodMemRequest,
	)

	cmd := quoteForShell(getCommand(jfsSetting))
	statCmd := "stat -c %i " + jfsSetting.MountPath

	pod := generateJuicePod(jfsSetting)
	// add cache-dir host path volume
	cacheVolumes, cacheVolumeMounts := getCacheDirVolumes(cmd)
	pod.Spec.Volumes = append(pod.Spec.Volumes, cacheVolumes...)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, cacheVolumeMounts...)

	pod.Name = podName
	pod.Spec.ServiceAccountName = jfsSetting.MountPodServiceAccount
	controllerutil.AddFinalizer(pod, config.Finalizer)
	pod.Spec.PriorityClassName = config.JFSMountPriorityName
	pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	pod.Spec.Containers[0].Env = []corev1.EnvVar{{
		Name:  "JFS_FOREGROUND",
		Value: "1",
	}}
	pod.Spec.Containers[0].Resources = resourceRequirements
	pod.Spec.Containers[0].Command = []string{"sh", "-c", cmd}
	pod.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{Command: []string{"sh", "-c", fmt.Sprintf(
				"if [ x$(%v) = x1 ]; then exit 0; else exit 1; fi ", statCmd)},
			}},
		InitialDelaySeconds: 1,
		PeriodSeconds:       1,
	}
	pod.Spec.Containers[0].Command = []string{"sh", "-c", cmd}
	pod.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{
		PreStop: &corev1.Handler{
			Exec: &corev1.ExecAction{Command: []string{"sh", "-c", fmt.Sprintf(
				"umount %s && rmdir %s", jfsSetting.MountPath, jfsSetting.MountPath)}},
		},
	}

	for k, v := range jfsSetting.MountPodLabels {
		pod.Labels[k] = v
	}
	for k, v := range jfsSetting.MountPodAnnotations {
		pod.Annotations[k] = v
	}
	if jfsSetting.DeletedDelay != "" {
		pod.Annotations[config.DeleteDelayTimeKey] = jfsSetting.DeletedDelay
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

	if !strings.Contains(cmd, "cache-dir") {
		cacheVolumes = append(cacheVolumes, corev1.Volume{
			Name: "jfs-default-cache",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/jfsCache",
					Type: &hostPathType,
				},
			},
		})
		cacheVolumeMounts = append(cacheVolumeMounts, corev1.VolumeMount{
			Name:             "jfs-default-cache",
			MountPath:        "/var/jfsCache",
			MountPropagation: &mountPropagation,
		})
		return cacheVolumes, cacheVolumeMounts
	}

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
				Name:             name,
				MountPath:        cacheDir,
				MountPropagation: &mountPropagation,
			}
			cacheVolumeMounts = append(cacheVolumeMounts, volumeMount)
		}
	}

	return cacheVolumes, cacheVolumeMounts
}

func quoteForShell(cmd string) string {
	if strings.Contains(cmd, "(") {
		cmd = strings.ReplaceAll(cmd, "(", "\\(")
	}
	if strings.Contains(cmd, ")") {
		cmd = strings.ReplaceAll(cmd, ")", "\\)")
	}
	return cmd
}

func getCommand(jfsSetting *config.JfsSetting) string {
	cmd := ""
	options := jfsSetting.Options
	if jfsSetting.IsCe {
		klog.V(5).Infof("ceMount: mount %v at %v", jfsSetting.Source, jfsSetting.MountPath)
		mountArgs := []string{config.CeMountPath, jfsSetting.Source, jfsSetting.MountPath}
		if !util.ContainsString(options, "metrics") {
			options = append(options, "metrics=0.0.0.0:9567")
		}
		mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
		cmd = strings.Join(mountArgs, " ")
	} else {
		klog.V(5).Infof("Mount: mount %v at %v", jfsSetting.Source, jfsSetting.MountPath)
		mountArgs := []string{config.JfsMountPath, jfsSetting.Source, jfsSetting.MountPath}
		options = append(options, "foreground")
		if jfsSetting.EncryptRsaKey != "" {
			mountArgs = append(mountArgs, "--rsa-key=/root/.rsa/rsa-key.pem")
		}
		if len(options) > 0 {
			mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
		}
		cmd = strings.Join(mountArgs, " ")
	}
	return cmd
}

func getInitContainer(setting *config.JfsSetting) corev1.Container {
	isPrivileged := true
	secretName := GenerateNameByVolumeId(setting.VolumeId)
	formatCmd := setting.FormatCmd
	container := corev1.Container{
		Name:  "jfs-format",
		Image: config.MountImage,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &isPrivileged,
		},
	}
	if setting.InitConfig != "" {
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      "init_config",
				MountPath: "/root/.juicefs",
			},
		)
	}
	if setting.EncryptRsaKey != "" {
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      "rsa-key",
				MountPath: "/root/.rsa",
			},
		)
		if setting.IsCe {
			formatCmd = formatCmd + " --encrypt-rsa-key=/root/.rsa/rsa-key.pem"
		}
	}

	container.Command = []string{"sh", "-c", formatCmd}
	container.EnvFrom = append(container.EnvFrom, corev1.EnvFromSource{
		SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{
			Name: secretName,
		}},
	})
	return container
}
