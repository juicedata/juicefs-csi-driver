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

package mount

import (
	"fmt"
	"strings"

	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func GenerateNameByVolumeId(volumeId string, simple bool) string {
	if simple {
		return fmt.Sprintf("juicefs-%s-%s-simple", config.NodeName, volumeId)
	}
	return fmt.Sprintf("juicefs-%s-%s", config.NodeName, volumeId)
}

func hasRef(pod *corev1.Pod) bool {
	for k, target := range pod.Annotations {
		if k == util.GetReferenceKey(target) {
			return true
		}
	}
	return false
}

func NewMountPod(jfsSetting *config.JfsSetting) *corev1.Pod {
	podName := GenerateNameByVolumeId(jfsSetting.VolumeId, jfsSetting.Simple)
	secretName := GenerateNameByVolumeId(jfsSetting.VolumeId, jfsSetting.Simple)
	resourceRequirements := parsePodResources(
		jfsSetting.MountPodCpuLimit,
		jfsSetting.MountPodMemLimit,
		jfsSetting.MountPodCpuRequest,
		jfsSetting.MountPodMemRequest,
	)

	cmd := quoteForShell(getCommand(jfsSetting))
	mp := corev1.MountPropagationBidirectional
	dir := corev1.HostPathDirectoryOrCreate
	statCmd := "stat -c %i " + jfsSetting.MountPath

	volumes := getVolumes(jfsSetting)
	volumeMounts := getVolumeMounts(jfsSetting)

	// add cache-dir host path volume
	if strings.Contains(cmd, "cache-dir") {
		cacheVolumes, cacheVolumeMounts := getCacheDirVolumes(cmd)
		volumes = append(volumes, cacheVolumes...)
		volumeMounts = append(volumeMounts, cacheVolumeMounts...)
	} else {
		volumes = append(volumes, corev1.Volume{
			Name: "jfs-default-cache",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/jfsCache",
					Type: &dir,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:             "jfs-default-cache",
			MountPath:        "/var/jfsCache",
			MountPropagation: &mp,
		})
	}
	pod := generatePodTemplate()
	pod.Name = podName
	pod.Spec.ServiceAccountName = jfsSetting.MountPodServiceAccount
	if jfsSetting.Simple {
		pod.Annotations[config.PodSimpleAnnotationKey] = ""
	}
	controllerutil.AddFinalizer(pod, config.Finalizer)
	pod.Spec.PriorityClassName = config.JFSMountPriorityName
	pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	pod.Spec.Volumes = volumes
	if jfsSetting.FormatCmd != "" {
		pod.Spec.InitContainers = []corev1.Container{getInitContainer(jfsSetting)}
	}
	pod.Spec.Containers[0].VolumeMounts = volumeMounts
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

	i := 1
	for k, v := range jfsSetting.Configs {
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      fmt.Sprintf("config-%v", i),
			MountPath: v,
		})
		if len(pod.Spec.InitContainers) != 0 {
			pod.Spec.InitContainers[0].VolumeMounts = append(pod.Spec.InitContainers[0].VolumeMounts, corev1.VolumeMount{
				Name:      fmt.Sprintf("config-%v", i),
				MountPath: v,
			})
		}
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
	pod.Spec.Containers[0].EnvFrom = append(pod.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{
		SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{
			Name: secretName,
		}},
	})
	for k, v := range jfsSetting.MountPodLabels {
		pod.Labels[k] = v
	}
	for k, v := range jfsSetting.MountPodAnnotations {
		pod.Annotations[k] = v
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

func generatePodTemplate() *corev1.Pod {
	isPrivileged := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.Namespace,
			Labels: map[string]string{
				config.PodTypeKey: config.PodTypeValue,
			},
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "jfs-mount",
				Image: config.MountImage,
				SecurityContext: &corev1.SecurityContext{
					Privileged: &isPrivileged,
				},
				Env: []corev1.EnvVar{{
					Name:  "JFS_FOREGROUND",
					Value: "1",
				}},
			}},
			NodeName:         config.NodeName,
			HostNetwork:      config.CSINodePod.Spec.HostNetwork,
			HostAliases:      config.CSINodePod.Spec.HostAliases,
			HostPID:          config.CSINodePod.Spec.HostPID,
			HostIPC:          config.CSINodePod.Spec.HostIPC,
			DNSConfig:        config.CSINodePod.Spec.DNSConfig,
			DNSPolicy:        config.CSINodePod.Spec.DNSPolicy,
			ImagePullSecrets: config.CSINodePod.Spec.ImagePullSecrets,
			PreemptionPolicy: config.CSINodePod.Spec.PreemptionPolicy,
			Tolerations:      config.CSINodePod.Spec.Tolerations,
		},
	}
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

func getVolumes(setting *config.JfsSetting) []corev1.Volume {
	dir := corev1.HostPathDirectoryOrCreate
	secretName := GenerateNameByVolumeId(setting.VolumeId, setting.Simple)

	volumes := []corev1.Volume{{
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
	if setting.EncryptRsaKey != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "rsa-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
					Items: []corev1.KeyToPath{{
						Key:  "encrypt_rsa_key",
						Path: "rsa-key.pem",
					}},
				},
			},
		})
	}
	if setting.InitConfig != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "init_config",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{{
					Key:  "init_config",
					Path: setting.Name + ".conf",
				}},
			}},
		})
	}
	return volumes
}

func getVolumeMounts(setting *config.JfsSetting) []corev1.VolumeMount {
	mp := corev1.MountPropagationBidirectional
	volumeMounts := []corev1.VolumeMount{{
		Name:             "jfs-dir",
		MountPath:        config.PodMountBase,
		MountPropagation: &mp,
	}, {
		Name:             "jfs-root-dir",
		MountPath:        "/root/.juicefs",
		MountPropagation: &mp,
	}}
	if setting.EncryptRsaKey != "" {
		if !setting.IsCe {
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{
					Name:      "rsa-key",
					MountPath: "/root/.rsa",
				},
			)
		}
	}
	if setting.InitConfig != "" {
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "init_config",
				MountPath: "/root/.juicefs",
			},
		)
	}
	return volumeMounts
}

func getInitContainer(setting *config.JfsSetting) corev1.Container {
	secretName := GenerateNameByVolumeId(setting.VolumeId, setting.Simple)
	formatCmd := setting.FormatCmd
	container := corev1.Container{
		Name:  "jfs-format",
		Image: config.MountImage,
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

func NewSecret(setting *config.JfsSetting) corev1.Secret {
	name := GenerateNameByVolumeId(setting.VolumeId, setting.Simple)
	data := make(map[string]string)
	if setting.MetaUrl != "" {
		data["metaurl"] = setting.MetaUrl
	}
	if setting.SecretKey != "" {
		data["secretkey"] = setting.SecretKey
	}
	if setting.SecretKey2 != "" {
		data["secretkey2"] = setting.SecretKey2
	}
	if setting.Token != "" {
		data["token"] = setting.Token
	}
	if setting.Passphrase != "" {
		data["passphrase"] = setting.Passphrase
	}
	if setting.EncryptRsaKey != "" {
		data["encrypt_rsa_key"] = setting.EncryptRsaKey
	}
	if setting.InitConfig != "" {
		data["init_config"] = setting.InitConfig
	}
	for k, v := range setting.Envs {
		data[k] = v
	}
	klog.V(6).Infof("secret data: %v", data)
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.Namespace,
		},
		StringData: data,
	}
	return secret
}
