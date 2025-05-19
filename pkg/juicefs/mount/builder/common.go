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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/security"
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
				common.PodTypeKey:          common.PodTypeValue,
				common.PodUniqueIdLabelKey: r.jfsSetting.UniqueId,
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
			ServiceAccountName: r.jfsSetting.Attr.ServiceAccountName,
			ImagePullSecrets:   r.jfsSetting.Attr.ImagePullSecrets,
			PreemptionPolicy:   r.jfsSetting.Attr.PreemptionPolicy,
			Tolerations:        r.jfsSetting.Attr.Tolerations,
		},
	}
}

// genCommonJuicePod generates a pod with common settings
func (r *BaseBuilder) genCommonJuicePod(cnGen func() corev1.Container) *corev1.Pod {
	// gen again to update the mount pod spec
	if err := config.GenPodAttrWithCfg(r.jfsSetting, nil); err != nil {
		builderLog.Error(err, "genCommonJuicePod gen pod attr failed, mount pod may not be the expected config")
	}
	pod := r.genPodTemplate(cnGen)
	// labels & annotations
	pod.ObjectMeta.Labels, pod.ObjectMeta.Annotations = r._genMetadata()
	pod.Spec.ServiceAccountName = r.jfsSetting.Attr.ServiceAccountName
	pod.Spec.PriorityClassName = config.JFSMountPriorityName
	pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	pod.Spec.Hostname = r.jfsSetting.VolumeId
	gracePeriod := int64(10)
	if r.jfsSetting.Attr.TerminationGracePeriodSeconds != nil {
		gracePeriod = *r.jfsSetting.Attr.TerminationGracePeriodSeconds
	}
	pod.Spec.TerminationGracePeriodSeconds = &gracePeriod
	controllerutil.AddFinalizer(pod, common.Finalizer)

	volumes, volumeMounts := r._genJuiceVolumes()
	pod.Spec.Volumes = volumes
	pod.Spec.Containers[0].VolumeMounts = volumeMounts
	// set env key from secret
	for _, key := range r.GetEnvKey() {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
			Name: key,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: r.jfsSetting.SecretName,
					},
					Key: key,
				},
			},
		})
	}
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, r.jfsSetting.Attr.Env...)

	pod.Spec.Containers[0].Resources = r.jfsSetting.Attr.Resources
	// if image support passFd from csi, do not set umount preStop
	if r.jfsSetting.Attr.Lifecycle == nil {
		if !util.SupportFusePass(pod.Spec.Containers[0].Image) || config.Webhook {
			pod.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{
				PreStop: &corev1.LifecycleHandler{
					Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "+e", fmt.Sprintf(
						"umount %s -l; rmdir %s; exit 0", r.jfsSetting.MountPath, r.jfsSetting.MountPath)}},
				},
			}
		}
	} else {
		pod.Spec.Containers[0].Lifecycle = r.jfsSetting.Attr.Lifecycle
	}

	pod.Spec.Containers[0].StartupProbe = r.jfsSetting.Attr.StartupProbe
	pod.Spec.Containers[0].LivenessProbe = r.jfsSetting.Attr.LivenessProbe
	pod.Spec.Containers[0].ReadinessProbe = r.jfsSetting.Attr.ReadinessProbe

	if r.jfsSetting.Attr.HostNetwork || !r.jfsSetting.IsCe {
		// When using hostNetwork, the MountPod will use a random port for metrics.
		// Before inducing any auxiliary method to detect that random port, the
		// best way is to avoid announcing any port about that.
		// Enterprise edition does not have metrics port.
		pod.Spec.Containers[0].Ports = []corev1.ContainerPort{}
	} else {
		pod.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: r.genMetricsPort()},
		}
	}
	return pod
}

// genMountCommand generates mount command
func (r *BaseBuilder) genMountCommand() string {
	cmd := ""
	options := []string{}
	subdir := r.jfsSetting.SubPath
	if !config.StorageClassShareMount {
		for _, option := range r.jfsSetting.Options {
			if strings.HasPrefix(option, "subdir=") {
				s := strings.Split(option, "=")
				if len(s) != 2 {
					continue
				}
				subdir = path.Join(s[1], r.jfsSetting.SubPath)
				continue
			}
			options = append(options, option)
		}
		if subdir != "" {
			options = append(options, fmt.Sprintf("subdir=%s", subdir))
		}
	} else {
		options = r.jfsSetting.Options
	}
	if r.jfsSetting.IsCe {
		mountArgs := []string{"exec", config.CeMountPath, "${metaurl}", security.EscapeBashStr(r.jfsSetting.MountPath)}
		if !util.ContainsPrefix(options, "metrics=") {
			if r.jfsSetting.Attr.HostNetwork {
				// Pick up a random (useable) port for hostNetwork MountPods.
				options = append(options, "metrics=0.0.0.0:0")
			} else {
				options = append(options, "metrics=0.0.0.0:9567")
			}
		}
		mountArgs = append(mountArgs, "-o", security.EscapeBashStr(strings.Join(options, ",")))
		cmd = strings.Join(mountArgs, " ")
	} else {
		mountArgs := []string{"exec", config.JfsMountPath, security.EscapeBashStr(r.jfsSetting.Source), security.EscapeBashStr(r.jfsSetting.MountPath)}
		mountOptions := []string{"foreground", "no-update"}
		if r.jfsSetting.EncryptRsaKey != "" {
			mountOptions = append(mountOptions, "rsa-key=/root/.rsa/rsa-key.pem")
		}
		mountOptions = append(mountOptions, options...)
		mountArgs = append(mountArgs, "-o", security.EscapeBashStr(strings.Join(mountOptions, ",")))
		cmd = strings.Join(mountArgs, " ")
	}
	return util.QuoteForShell(cmd)
}

// genInitCommand generates init command
func (r *BaseBuilder) genInitCommand() string {
	formatCmd := r.jfsSetting.FormatCmd
	if r.jfsSetting.EncryptRsaKey != "" {
		if r.jfsSetting.IsCe {
			formatCmd = formatCmd + " --encrypt-rsa-key=/root/.rsa/rsa-key.pem"
		}
	}
	if r.jfsSetting.InitConfig != "" {
		confPath := filepath.Join(config.ROConfPath, r.jfsSetting.Name+".conf")
		args := []string{"cp", confPath, r.jfsSetting.ClientConfPath}
		formatCmd = strings.Join(args, " ")
	}
	return formatCmd
}

func (r *BaseBuilder) getQuotaPath() string {
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
	targetPath := path.Join(subdir, quotaPath)
	return targetPath
}

// genJobCommand generates job command
func (r *BaseBuilder) getJobCommand() string {
	var cmd string
	options := util.StripReadonlyOption(r.jfsSetting.Options)
	if r.jfsSetting.IsCe {
		args := []string{config.CeMountPath, "${metaurl}", "/mnt/jfs"}
		if len(options) != 0 {
			args = append(args, "-o", security.EscapeBashStr(strings.Join(options, ",")))
		}
		cmd = strings.Join(args, " ")
	} else {
		args := []string{config.JfsMountPath, security.EscapeBashStr(r.jfsSetting.Source), "/mnt/jfs"}
		if r.jfsSetting.EncryptRsaKey != "" {
			options = append(options, "rsa-key=/root/.rsa/rsa-key.pem")
		}
		options = append(options, "background")
		args = append(args, "-o", security.EscapeBashStr(strings.Join(options, ",")))
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

func GenMetadata(jfsSetting *config.JfsSetting) (labels map[string]string, annotations map[string]string) {
	labels = map[string]string{}
	annotations = map[string]string{}
	if jfsSetting.DeletedDelay != "" {
		annotations[common.DeleteDelayTimeKey] = jfsSetting.DeletedDelay
	}
	if jfsSetting.CleanCache {
		annotations[common.CleanCache] = "true"
	}
	for k, v := range jfsSetting.Attr.Labels {
		labels[k] = v
	}
	for k, v := range jfsSetting.Attr.Annotations {
		if k != common.JfsUpgradeProcess {
			// new pod do not need upgradeProcess annotation
			annotations[k] = v
		}
	}
	// inter labels & annotations
	annotations[common.JuiceFSUUID] = jfsSetting.UUID
	annotations[common.UniqueId] = jfsSetting.UniqueId
	labels[common.PodJuiceHashLabelKey] = jfsSetting.HashVal
	labels[common.PodUpgradeUUIDLabelKey] = jfsSetting.UpgradeUUID
	labels[common.PodTypeKey] = common.PodTypeValue
	labels[common.PodUniqueIdLabelKey] = jfsSetting.UniqueId
	return
}

// _genMetadata generates labels & annotations
func (r *BaseBuilder) _genMetadata() (labels map[string]string, annotations map[string]string) {
	return GenMetadata(r.jfsSetting)
}

// _genJuiceVolumes generates volumes & volumeMounts
// 1. if encrypt_rsa_key is set, mount secret to /root/.rsa
// 2. if initconfig is set, mount secret to /etc/juicefs
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
					Key:  "initconfig",
					Path: r.jfsSetting.Name + ".conf",
				}},
			}},
		})
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "init-config",
				MountPath: config.ROConfPath,
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
