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
	"os"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
)

var (
	ByProcess    = false // csi driver runs juicefs in process or not
	FormatInPod  = false // put format/auth in pod (only in k8s)
	Provisioner  = false // provisioner in controller
	MountManager = false // manage mount pod in controller (only in k8s)
	Webhook      = false // inject juicefs client as sidecar in pod (only in k8s)
	Immutable    = false // csi driver is running in an immutable environment

	NodeName           = ""
	Namespace          = ""
	PodName            = ""
	CEMountImage       = "juicedata/mount:ce-nightly" // mount pod ce image
	EEMountImage       = "juicedata/mount:ee-nightly" // mount pod ee image
	MountLabels        = ""
	HostIp             = ""
	KubeletPort        = ""
	ReconcileTimeout   = 5 * time.Minute
	ReconcilerInterval = 5

	CSIPod = corev1.Pod{}

	MountPointPath           = "/var/lib/juicefs/volume"
	JFSConfigPath            = "/var/lib/juicefs/config"
	JFSMountPriorityName     = "system-node-critical"
	JFSMountPreemptionPolicy = ""

	TmpPodMountBase = "/tmp"
	PodMountBase    = "/jfs"
	MountBase       = "/var/lib/jfs"
	FsType          = "juicefs"
	CliPath         = "/usr/bin/juicefs"
	CeCliPath       = "/usr/local/bin/juicefs"
	CeMountPath     = "/bin/mount.juicefs"
	JfsMountPath    = "/sbin/mount.juicefs"
	JfsGoBinaryPath = os.Getenv("JFS_MOUNT_PATH")
	JfsChannel      = os.Getenv("JFSCHAN")
)

const (
	// DriverName to be registered
	DriverName           = "csi.juicefs.com"
	CSINodeLabelKey      = "app"
	CSINodeLabelValue    = "juicefs-csi-node"
	PodTypeKey           = "app.kubernetes.io/name"
	PodTypeValue         = "juicefs-mount"
	PodUniqueIdLabelKey  = "volume-id"
	PodJuiceHashLabelKey = "juicefs-hash"
	Finalizer            = "juicefs.com/finalizer"
	JuiceFSUUID          = "juicefs-uuid"
	UniqueId             = "juicefs-uniqueid"
	CleanCache           = "juicefs-clean-cache"
	MountContainerName   = "jfs-mount"
	JuiceFSMountPod      = "juicefs-mountpod"
	JobTypeValue         = "juicefs-job"

	// CSI Secret
	ProvisionerSecretName           = "csi.storage.k8s.io/provisioner-secret-name"
	ProvisionerSecretNamespace      = "csi.storage.k8s.io/provisioner-secret-namespace"
	PublishSecretName               = "csi.storage.k8s.io/node-publish-secret-name"
	PublishSecretNamespace          = "csi.storage.k8s.io/node-publish-secret-namespace"
	ControllerExpandSecretName      = "csi.storage.k8s.io/controller-expand-secret-name"
	ControllerExpandSecretNamespace = "csi.storage.k8s.io/controller-expand-secret-namespace"

	// webhook
	WebhookName          = "juicefs-admission-webhook"
	True                 = "true"
	False                = "false"
	inject               = ".juicefs.com/inject"
	injectSidecar        = ".sidecar" + inject
	InjectSidecarDone    = "done" + injectSidecar
	InjectSidecarDisable = "disable" + injectSidecar

	// config in pv
	MountPodCpuLimitKey    = "juicefs/mount-cpu-limit"
	MountPodMemLimitKey    = "juicefs/mount-memory-limit"
	MountPodCpuRequestKey  = "juicefs/mount-cpu-request"
	MountPodMemRequestKey  = "juicefs/mount-memory-request"
	mountPodLabelKey       = "juicefs/mount-labels"
	mountPodAnnotationKey  = "juicefs/mount-annotations"
	mountPodServiceAccount = "juicefs/mount-service-account"
	mountPodImageKey       = "juicefs/mount-image"
	deleteDelay            = "juicefs/mount-delete-delay"
	cleanCache             = "juicefs/clean-cache"
	cachePVC               = "juicefs/mount-cache-pvc"
	mountPodHostPath       = "juicefs/host-path"

	// DeleteDelayTimeKey mount pod annotation
	DeleteDelayTimeKey = "juicefs-delete-delay"
	DeleteDelayAtKey   = "juicefs-delete-at"

	// default value
	DefaultMountPodCpuLimit   = "2000m"
	DefaultMountPodMemLimit   = "5Gi"
	DefaultMountPodCpuRequest = "1000m"
	DefaultMountPodMemRequest = "1Gi"
)

var PodLocks [1024]sync.Mutex

func GetPodLock(podName string) *sync.Mutex {
	h := fnv.New32a()
	h.Write([]byte(podName))
	index := int(h.Sum32())
	return &PodLocks[index%1024]
}
