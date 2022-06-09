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

package config

import (
	"hash/fnv"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sync"

	corev1 "k8s.io/api/core/v1"
)

var (
	ByProcess    = false // csi driver runs juicefs in process or not
	PodManager   = false // enable pod manager or not (only in k8s)
	FormatInPod  = false // put format/auth in pod (only in k8s)
	Provisioner  = false // provisioner in controller
	MountManager = false // manage mount pod in controller (only in k8s)

	NodeName    = ""
	Namespace   = ""
	PodName     = ""
	MountImage  = ""
	MountLabels = ""
	HostIp      = ""
	KubeletPort = ""

	CSIPod            = corev1.Pod{}
	ContainerResource = corev1.ResourceRequirements{}

	MountPointPath       = "/var/lib/juicefs/volume"
	JFSConfigPath        = "/var/lib/juicefs/config"
	JFSMountPriorityName = "system-node-critical"

	PodMountBase = "/jfs"
	MountBase    = "/var/lib/jfs"
	FsType       = "juicefs"
	CliPath      = "/usr/bin/juicefs"
	CeCliPath    = "/usr/local/bin/juicefs"
	CeMountPath  = "/bin/mount.juicefs"
	JfsMountPath = "/sbin/mount.juicefs"

	ReconcilerInterval = 5
)

const (
	CSINodeLabelKey   = "app"
	CSINodeLabelValue = "juicefs-csi-node"
	PodTypeKey        = "app.kubernetes.io/name"
	PodTypeValue      = "juicefs-mount"
	Finalizer         = "juicefs.com/finalizer"
	JuiceFSUUID       = "juicefs-uuid"
	UniqueId          = "juicefs-uniqueid"
	CleanCache        = "juicefs-clean-cache"

	// config in pv
	mountPodCpuLimitKey    = "juicefs/mount-cpu-limit"
	mountPodMemLimitKey    = "juicefs/mount-memory-limit"
	mountPodCpuRequestKey  = "juicefs/mount-cpu-request"
	mountPodMemRequestKey  = "juicefs/mount-memory-request"
	mountPodLabelKey       = "juicefs/mount-labels"
	mountPodAnnotationKey  = "juicefs/mount-annotations"
	mountPodServiceAccount = "juicefs/mount-service-account"
	deleteDelay            = "juicefs/mount-delete-delay"
	cleanCache             = "juicefs/clean-cache"

	// DeleteDelayTimeKey mount pod annotation
	DeleteDelayTimeKey = "juicefs-delete-delay"
	DeleteDelayAtKey   = "juicefs-delete-at"
)

var PodLocks [1024]sync.Mutex

func GetPodLock(podName string) *sync.Mutex {
	h := fnv.New32a()
	h.Write([]byte(podName))
	index := int(h.Sum32())
	return &PodLocks[index%1024]
}

func GeneratePodTemplate() *corev1.Pod {
	isPrivileged := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Labels: map[string]string{
				PodTypeKey: PodTypeValue,
			},
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "jfs-mount",
				Image: MountImage,
				SecurityContext: &corev1.SecurityContext{
					Privileged: &isPrivileged,
				},
				Env: []corev1.EnvVar{},
			}},
			NodeName:           NodeName,
			HostNetwork:        CSIPod.Spec.HostNetwork,
			HostAliases:        CSIPod.Spec.HostAliases,
			HostPID:            CSIPod.Spec.HostPID,
			HostIPC:            CSIPod.Spec.HostIPC,
			DNSConfig:          CSIPod.Spec.DNSConfig,
			DNSPolicy:          CSIPod.Spec.DNSPolicy,
			ServiceAccountName: CSIPod.Spec.ServiceAccountName,
			ImagePullSecrets:   CSIPod.Spec.ImagePullSecrets,
			PreemptionPolicy:   CSIPod.Spec.PreemptionPolicy,
			Tolerations:        CSIPod.Spec.Tolerations,
		},
	}
}
