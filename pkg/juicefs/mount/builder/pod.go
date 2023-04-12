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

package builder

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func (r *Builder) NewMountPod(podName string) *corev1.Pod {
	resourceRequirements := r.jfsSetting.Resources

	cmd := r.getCommand()

	pod := r.generateJuicePod()

	metricsPort := r.getMetricsPort()

	// add cache-dir host path volume
	cacheVolumes, cacheVolumeMounts := r.getCacheDirVolumes(corev1.MountPropagationBidirectional)
	pod.Spec.Volumes = append(pod.Spec.Volumes, cacheVolumes...)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, cacheVolumeMounts...)

	pod.Name = podName
	pod.Spec.ServiceAccountName = r.jfsSetting.ServiceAccountName
	controllerutil.AddFinalizer(pod, config.Finalizer)
	pod.Spec.PriorityClassName = config.JFSMountPriorityName
	pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	pod.Spec.Containers[0].Env = []corev1.EnvVar{{
		Name:  "JFS_FOREGROUND",
		Value: "1",
	}}
	pod.Spec.Containers[0].Resources = resourceRequirements
	pod.Spec.Containers[0].Command = []string{"sh", "-c", cmd}
	pod.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{
		PreStop: &corev1.Handler{
			Exec: &corev1.ExecAction{Command: []string{"sh", "-c", fmt.Sprintf(
				"umount %s && rmdir %s", r.jfsSetting.MountPath, r.jfsSetting.MountPath)}},
		},
	}

	if r.jfsSetting.Attr.HostNetwork {
		// When using hostNetwork, the MountPod will use a random port for metrics.
		// Before inducing any auxiliary method to detect that random port, the
		// best way is to avoid announcing any port about that.
		pod.Spec.Containers[0].Ports = []corev1.ContainerPort{}
	} else {
		pod.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: metricsPort},
		}
	}

	if config.Webhook {
		pod.Spec.Containers[0].Lifecycle.PostStart = &corev1.Handler{
			Exec: &corev1.ExecAction{Command: []string{"bash", "-c",
				fmt.Sprintf("time %s %s >> /proc/1/fd/1", checkMountScriptPath, r.jfsSetting.MountPath)}},
		}
	}

	gracePeriod := int64(10)
	pod.Spec.TerminationGracePeriodSeconds = &gracePeriod

	for k, v := range r.jfsSetting.MountPodLabels {
		pod.Labels[k] = v
	}
	for k, v := range r.jfsSetting.MountPodAnnotations {
		pod.Annotations[k] = v
	}
	if r.jfsSetting.DeletedDelay != "" {
		pod.Annotations[config.DeleteDelayTimeKey] = r.jfsSetting.DeletedDelay
	}
	pod.Annotations[config.JuiceFSUUID] = r.jfsSetting.UUID
	pod.Annotations[config.UniqueId] = r.jfsSetting.UniqueId
	if r.jfsSetting.CleanCache {
		pod.Annotations[config.CleanCache] = "true"
	}
	return pod
}

func (r *Builder) getCacheDirVolumes(mountPropagation corev1.MountPropagationMode) ([]corev1.Volume, []corev1.VolumeMount) {
	cacheVolumes := []corev1.Volume{}
	cacheVolumeMounts := []corev1.VolumeMount{}

	hostPathType := corev1.HostPathDirectoryOrCreate

	for idx, cacheDir := range r.jfsSetting.CacheDirs {
		name := fmt.Sprintf("cachedir-%d", idx)

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

	return cacheVolumes, cacheVolumeMounts
}

func (r *Builder) getCommand() string {
	cmd := ""
	options := r.jfsSetting.Options
	if r.jfsSetting.IsCe {
		klog.V(5).Infof("ceMount: mount %v at %v", util.StripPasswd(r.jfsSetting.Source), r.jfsSetting.MountPath)
		mountArgs := []string{config.CeMountPath, "${metaurl}", r.jfsSetting.MountPath}
		if !util.ContainsPrefix(options, "metrics=") {
			if r.jfsSetting.Attr.HostNetwork {
				// Pick up a random (useable) port for hostNetwork MountPods.
				options = append(options, "metrics=0.0.0.0:0")
			} else {
				options = append(options, "metrics=0.0.0.0:9567")
			}
		}
		mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
		cmd = strings.Join(mountArgs, " ")
	} else {
		klog.V(5).Infof("Mount: mount %v at %v", util.StripPasswd(r.jfsSetting.Source), r.jfsSetting.MountPath)
		mountArgs := []string{config.JfsMountPath, r.jfsSetting.Source, r.jfsSetting.MountPath}
		mountOptions := []string{"foreground", "no-update"}
		if r.jfsSetting.EncryptRsaKey != "" {
			mountOptions = append(mountOptions, "rsa-key=/root/.rsa/rsa-key.pem")
		}
		mountOptions = append(mountOptions, options...)
		mountArgs = append(mountArgs, "-o", strings.Join(mountOptions, ","))
		cmd = strings.Join(mountArgs, " ")
	}
	return util.QuoteForShell(cmd)
}

func (r *Builder) getInitContainer() corev1.Container {
	isPrivileged := true
	rootUser := int64(0)
	secretName := r.jfsSetting.SecretName
	formatCmd := r.jfsSetting.FormatCmd
	container := corev1.Container{
		Name:  "jfs-format",
		Image: r.jfsSetting.Attr.Image,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &isPrivileged,
			RunAsUser:  &rootUser,
		},
	}
	if r.jfsSetting.EncryptRsaKey != "" {
		if r.jfsSetting.IsCe {
			container.VolumeMounts = append(container.VolumeMounts,
				corev1.VolumeMount{
					Name:      "rsa-key",
					MountPath: "/root/.rsa",
				},
			)
			formatCmd = formatCmd + " --encrypt-rsa-key=/root/.rsa/rsa-key.pem"
		}
	}

	// create subpath if readonly mount or in webhook mode
	if r.jfsSetting.SubPath != "" {
		if util.ContainsString(r.jfsSetting.Options, "read-only") || util.ContainsString(r.jfsSetting.Options, "ro") || config.Webhook {
			// generate mount command
			cmd := r.getJobCommand()
			initCmd := fmt.Sprintf("%s && if [ ! -d /mnt/jfs/%s ]; then mkdir -m 777 /mnt/jfs/%s; fi; umount /mnt/jfs", cmd, r.jfsSetting.SubPath, r.jfsSetting.SubPath)
			formatCmd = fmt.Sprintf("%s && %s", formatCmd, initCmd)
			if config.Webhook && r.capacity > 0 {
				quotaPath := r.jfsSetting.SubPath
				var subdir string
				for _, o := range r.jfsSetting.Options {
					pair := strings.Split(o, "=")
					if len(pair) != 2 {
						continue
					}
					if pair[0] == "subdir" {
						subdir = path.Join("/", pair[1])
					}
				}
				var setQuotaCmd string
				targetPath := path.Join(subdir, quotaPath)
				capacity := strconv.FormatInt(r.capacity, 10)
				if r.jfsSetting.IsCe {
					// juicefs quota; if [ $? -eq 0 ]; then juicefs quota set ${metaurl} --path ${path} --capacity ${capacity}; fi
					cmdArgs := []string{
						config.CeCliPath, "quota; if [ $? -eq 0 ]; then",
						config.CeCliPath,
						"quota", "set", "${metaurl}",
						"--path", targetPath,
						"--capacity", capacity,
						"; fi",
					}
					setQuotaCmd = strings.Join(cmdArgs, " ")
				} else {
					jfsPath := config.JfsGoBinaryPath
					if config.JfsChannel != "" {
						jfsPath += "." + config.JfsChannel
					}
					cmdArgs := []string{
						jfsPath, "quota; if [ $? -eq 0 ]; then",
						jfsPath,
						"quota", "set", r.jfsSetting.Name,
						"--path", targetPath,
						"--capacity", capacity,
						"; fi",
					}
					setQuotaCmd = strings.Join(cmdArgs, " ")
				}
				formatCmd = fmt.Sprintf("%s && %s", formatCmd, setQuotaCmd)
			}
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

func (r *Builder) getMetricsPort() int32 {
	port := int64(9567)
	options := r.jfsSetting.Options

	for _, option := range options {
		if strings.HasPrefix(option, "metrics=") {
			re := regexp.MustCompile(`metrics=.*:([0-9]{1,6})`)
			match := re.FindStringSubmatch(option)
			if len(match) > 0 {
				port, _ = strconv.ParseInt(match[1], 10, 32)
			}
		}
	}

	return int32(port)
}
