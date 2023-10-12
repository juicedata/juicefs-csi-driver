/*
Copyright 2022 Juicedata Inc

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

const (
	JfsDirName      = "jfs-dir"
	UpdateDBDirName = "updatedb"
	UpdateDBCfgFile = "/etc/updatedb.conf"
)

type BaseBuilder struct {
	jfsSetting *config.JfsSetting
	capacity   int64
}

// genPodTemplate generates a pod template from csi pod
func (r *BaseBuilder) genPodTemplate(baseCnGen func() corev1.Container) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.jfsSetting.Attr.Namespace,
			Labels: map[string]string{
				config.PodTypeKey:          config.PodTypeValue,
				config.PodUniqueIdLabelKey: r.jfsSetting.UniqueId,
			},
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			Containers:         []corev1.Container{baseCnGen()},
			NodeName:           config.NodeName,
			HostNetwork:        r.jfsSetting.Attr.HostNetwork,
			HostAliases:        r.jfsSetting.Attr.HostAliases,
			HostPID:            r.jfsSetting.Attr.HostPID,
			HostIPC:            r.jfsSetting.Attr.HostIPC,
			DNSConfig:          r.jfsSetting.Attr.DNSConfig,
			DNSPolicy:          r.jfsSetting.Attr.DNSPolicy,
			ServiceAccountName: r.jfsSetting.ServiceAccountName,
			ImagePullSecrets:   r.jfsSetting.Attr.ImagePullSecrets,
			PreemptionPolicy:   r.jfsSetting.Attr.PreemptionPolicy,
			Tolerations:        r.jfsSetting.Attr.Tolerations,
		},
	}
}

// genCommonJuicePod generates a pod with common settings
func (r *BaseBuilder) genCommonJuicePod(cnGen func() corev1.Container) *corev1.Pod {
	pod := r.genPodTemplate(cnGen)
	// labels & annotations
	pod.ObjectMeta.Labels, pod.ObjectMeta.Annotations = r._genMetadata()
	pod.Spec.ServiceAccountName = r.jfsSetting.ServiceAccountName
	pod.Spec.PriorityClassName = config.JFSMountPriorityName
	pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	gracePeriod := int64(10)
	pod.Spec.TerminationGracePeriodSeconds = &gracePeriod
	controllerutil.AddFinalizer(pod, config.Finalizer)

	volumes, volumeMounts := r._genJuiceVolumes()
	pod.Spec.Volumes = volumes
	pod.Spec.Containers[0].VolumeMounts = volumeMounts
	pod.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: r.jfsSetting.SecretName,
			},
		},
	}}
	pod.Spec.Containers[0].Resources = r.jfsSetting.Resources
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
			{Name: "metrics", ContainerPort: r.genMetricsPort()},
		}
	}
	if initContainer := r.genInitContainer(cnGen); initContainer != nil {
		// initContainer should have the same volumeMounts as mount container
		initContainer.VolumeMounts = pod.Spec.Containers[0].VolumeMounts
		initContainer.EnvFrom = []corev1.EnvFromSource{{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: r.jfsSetting.SecretName,
				},
			},
		}}
		pod.Spec.InitContainers = []corev1.Container{*initContainer}
	}
	return pod
}

// genMountCommand generates mount command
func (r *BaseBuilder) genMountCommand() string {
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

// genInitContainer: generate init container
func (r *BaseBuilder) genInitContainer(cnGen func() corev1.Container) *corev1.Container {
	if r.jfsSetting.SubPath == "" && !util.ContainsString(r.jfsSetting.Options, "read-only") && !util.ContainsString(r.jfsSetting.Options, "ro") && !config.Webhook {
		// do not need initContainer if no subpath
		return nil
	}
	container := cnGen()
	container.Name = "jfs-init"

	initCmds := []string{
		r.genInitCommand(),
	}
	// create subpath if readonly mount or in webhook mode
	if util.ContainsString(r.jfsSetting.Options, "read-only") || util.ContainsString(r.jfsSetting.Options, "ro") || config.Webhook {
		// generate mount command
		initCmds = append(initCmds,
			r.getJobCommand(),
			fmt.Sprintf("if [ ! -d /mnt/jfs/%s ]; then mkdir -m 777 /mnt/jfs/%s; fi;", r.jfsSetting.SubPath, r.jfsSetting.SubPath),
			"umount /mnt/jfs",
		)
		// set quota in webhook mode
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
				cmdArgs := []string{
					config.CliPath, "quota; if [ $? -eq 0 ]; then",
					config.CliPath,
					"quota", "set", r.jfsSetting.Name,
					"--path", targetPath,
					"--capacity", capacity,
					"; fi",
				}
				setQuotaCmd = strings.Join(cmdArgs, " ")
			}
			initCmds = append(initCmds, setQuotaCmd)
		}
	}
	container.Command = []string{"sh", "-c", strings.Join(initCmds, "\n")}
	return &container
}

// genInitCommand generates init command
func (r *BaseBuilder) genInitCommand() string {
	formatCmd := r.jfsSetting.FormatCmd
	if r.jfsSetting.EncryptRsaKey != "" {
		if r.jfsSetting.IsCe {
			formatCmd = formatCmd + " --encrypt-rsa-key=/root/.rsa/rsa-key.pem"
		}
	}

	return formatCmd
}

// genJobCommand generates job command
func (r *BaseBuilder) getJobCommand() string {
	var cmd string
	options := util.StripReadonlyOption(r.jfsSetting.Options)
	if r.jfsSetting.IsCe {
		args := []string{config.CeMountPath, "${metaurl}", "/mnt/jfs"}
		if len(options) != 0 {
			args = append(args, "-o", strings.Join(options, ","))
		}
		cmd = strings.Join(args, " ")
	} else {
		args := []string{config.JfsMountPath, r.jfsSetting.Source, "/mnt/jfs"}
		if r.jfsSetting.EncryptRsaKey != "" {
			options = append(options, "rsa-key=/root/.rsa/rsa-key.pem")
		}
		options = append(options, "background")
		args = append(args, "-o", strings.Join(options, ","))
		cmd = strings.Join(args, " ")
	}
	return util.QuoteForShell(cmd)
}

// genMetricsPort generates metrics port
func (r *BaseBuilder) genMetricsPort() int32 {
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

// _genMetadata generates labels & annotations
func (r *BaseBuilder) _genMetadata() (labels map[string]string, annotations map[string]string) {
	labels = map[string]string{
		config.PodTypeKey:          config.PodTypeValue,
		config.PodUniqueIdLabelKey: r.jfsSetting.UniqueId,
	}
	annotations = map[string]string{}

	for k, v := range r.jfsSetting.MountPodLabels {
		labels[k] = v
	}
	for k, v := range r.jfsSetting.MountPodAnnotations {
		annotations[k] = v
	}
	if r.jfsSetting.DeletedDelay != "" {
		annotations[config.DeleteDelayTimeKey] = r.jfsSetting.DeletedDelay
	}
	annotations[config.JuiceFSUUID] = r.jfsSetting.UUID
	annotations[config.UniqueId] = r.jfsSetting.UniqueId
	if r.jfsSetting.CleanCache {
		annotations[config.CleanCache] = "true"
	}
	return
}

// _genJuiceVolumes generates volumes & volumeMounts
// 1. if encrypt_rsa_key is set, mount secret to /root/.rsa
// 2. if init_config is set, mount secret to /root/.config
// 3. configs in secret
func (r *BaseBuilder) _genJuiceVolumes() ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	secretName := r.jfsSetting.SecretName

	if r.jfsSetting.EncryptRsaKey != "" {
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
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "rsa-key",
				MountPath: "/root/.rsa",
			},
		)
	}
	if r.jfsSetting.InitConfig != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "init-config",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{{
					Key:  "init_config",
					Path: r.jfsSetting.Name + ".conf",
				}},
			}},
		})
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "init-config",
				MountPath: "/root/.juicefs",
			},
		)
	}
	i := 1
	for k, v := range r.jfsSetting.Configs {
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
	return volumes, volumeMounts
}
