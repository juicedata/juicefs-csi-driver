/*
 Copyright 2024 Juicedata Inc

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

package common

const (
	// DriverName to be registered
	CSINodeLabelKey        = "app"
	CSINodeLabelValue      = "juicefs-csi-node"
	PodTypeKey             = "app.kubernetes.io/name"
	PodTypeValue           = "juicefs-mount"
	PodUniqueIdLabelKey    = "volume-id"
	PodJuiceHashLabelKey   = "juicefs-hash"
	PodUpgradeUUIDLabelKey = "juicefs-upgrade-uuid"
	Finalizer              = "juicefs.com/finalizer"
	JuiceFSUUID            = "juicefs-uuid"
	UniqueId               = "juicefs-uniqueid"
	CleanCache             = "juicefs-clean-cache"
	MountContainerName     = "jfs-mount"
	JobTypeValue           = "juicefs-job"
	JfsInsideContainer     = "JFS_INSIDE_CONTAINER"
	MaxParallelUpgradeNum  = 50

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
	MountPodLabelKey       = "juicefs/mount-labels"
	MountPodAnnotationKey  = "juicefs/mount-annotations"
	MountPodServiceAccount = "juicefs/mount-service-account"
	MountPodImageKey       = "juicefs/mount-image"
	DeleteDelay            = "juicefs/mount-delete-delay"
	CleanCacheKey          = "juicefs/clean-cache"
	CachePVC               = "juicefs/mount-cache-pvc"
	CacheEmptyDir          = "juicefs/mount-cache-emptydir"
	CacheInlineVolume      = "juicefs/mount-cache-inline-volume"
	MountPodHostPath       = "juicefs/host-path"

	// DeleteDelayTimeKey mount pod annotation
	DeleteDelayTimeKey = "juicefs-delete-delay"
	DeleteDelayAtKey   = "juicefs-delete-at"

	// default value
	DefaultMountPodCpuLimit   = "2000m"
	DefaultMountPodMemLimit   = "5Gi"
	DefaultMountPodCpuRequest = "1000m"
	DefaultMountPodMemRequest = "1Gi"

	// secret labels
	JuicefsSecretLabelKey = "juicefs/secret"

	PodInfoName      = "csi.storage.k8s.io/pod.name"
	PodInfoNamespace = "csi.storage.k8s.io/pod.namespace"

	// smooth upgrade
	JfsFuseFsPathInPod      = "/tmp"
	JfsFuseFsPathInHost     = "/var/run/juicefs-csi"
	JfsCommEnv              = "JFS_SUPER_COMM"
	JfsUpgradeJobLabelKey   = "app.kubernetes.io/name"
	JfsUpgradeJobLabelValue = "juicefs-upgrade"
	JfsUpgradeNodeName      = "juicefs-upgrade-node"
	JfsUpgradeUniqueIds     = "juicefs-upgrade-uniqueids"
	JfsUpgradeRecreateName  = "juicefs-upgrade-recreate"
	JfsUpgradePodLabelKey   = "juicefs-job-name"
)

func GenUpgradeJobName() string {
	return "juicefs-upgrade-job"
}
