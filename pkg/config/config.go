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
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
)

var (
	WebPort           = MustGetWebPort() // web port used by metrics
	ByProcess         = false            // csi driver runs juicefs in process or not
	FormatInPod       = false            // put format/auth in pod (only in k8s)
	Provisioner       = false            // provisioner in controller
	CacheClientConf   = false            // cache client config files and use directly in mount containers
	MountManager      = false            // manage mount pod in controller (only in k8s)
	Webhook           = false            // inject juicefs client as sidecar in pod (only in k8s)
	ValidatingWebhook = false            // start validating webhook, applicable to ee only
	Immutable         = false            // csi driver is running in an immutable environment

	DriverName               = "csi.juicefs.com"
	NodeName                 = ""
	Namespace                = ""
	PodName                  = ""
	HostIp                   = ""
	KubeletPort              = ""
	ReconcileTimeout         = 5 * time.Minute
	ReconcilerInterval       = 5
	SecretReconcilerInterval = 1 * time.Hour

	CSIPod = corev1.Pod{}

	MountPointPath           = "/var/lib/juicefs/volume"
	JFSConfigPath            = "/var/lib/juicefs/config"
	JFSMountPriorityName     = "system-node-critical"
	JFSMountPreemptionPolicy = ""

	TmpPodMountBase       = "/tmp"
	PodMountBase          = "/jfs"
	MountBase             = "/var/lib/jfs"
	FsType                = "juicefs"
	CliPath               = "/usr/bin/juicefs"
	CeCliPath             = "/usr/local/bin/juicefs"
	CeMountPath           = "/bin/mount.juicefs"
	JfsMountPath          = "/sbin/mount.juicefs"
	DefaultClientConfPath = "/root/.juicefs"
	ROConfPath            = "/etc/juicefs"

	DefaultCEMountImage = "juicedata/mount:ce-nightly" // mount pod ce image, override by ENV
	DefaultEEMountImage = "juicedata/mount:ee-nightly" // mount pod ee image, override by ENV
)

const (
	// DriverName to be registered
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
	JobTypeValue         = "juicefs-job"
	JfsInsideContainer   = "JFS_INSIDE_CONTAINER"

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
	cacheEmptyDir          = "juicefs/mount-cache-emptydir"
	cacheInlineVolume      = "juicefs/mount-cache-inline-volume"
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

func GetPodLock(podHashVal string) *sync.Mutex {
	h := fnv.New32a()
	h.Write([]byte(podHashVal))
	index := int(h.Sum32())
	return &PodLocks[index%1024]
}

func MustGetWebPort() int {
	value, exists := os.LookupEnv("JUICEFS_CSI_WEB_PORT")
	if exists {
		port, err := strconv.Atoi(value)
		if err == nil {
			return port
		}
		klog.Errorf("Fail to parse JUICEFS_CSI_WEB_PORT %s: %v", value, err)
	}
	return 8080
}

type PVCSelector struct {
	metav1.LabelSelector
	MatchStorageClassName string `json:"matchStorageClassName,omitempty"`
	MatchName             string `json:"matchName,omitempty"`
}

type MountPodPatch struct {
	// used to specify the selector for the PVC that will be patched
	// omit will patch for all PVC
	PVCSelector *PVCSelector `json:"pvcSelector,omitempty"`

	CEMountImage string `json:"ceMountImage,omitempty"`
	EEMountImage string `json:"eeMountImage,omitempty"`

	Image                         string                       `json:"-"`
	Labels                        map[string]string            `json:"labels,omitempty"`
	Annotations                   map[string]string            `json:"annotations,omitempty"`
	HostNetwork                   *bool                        `json:"hostNetwork,omitempty" `
	HostPID                       *bool                        `json:"hostPID,omitempty" `
	LivenessProbe                 *corev1.Probe                `json:"livenessProbe,omitempty"`
	ReadinessProbe                *corev1.Probe                `json:"readinessProbe,omitempty"`
	StartupProbe                  *corev1.Probe                `json:"startupProbe,omitempty"`
	Lifecycle                     *corev1.Lifecycle            `json:"lifecycle,omitempty"`
	Resources                     *corev1.ResourceRequirements `json:"resources,omitempty"`
	TerminationGracePeriodSeconds *int64                       `json:"terminationGracePeriodSeconds,omitempty"`
}

func (mpp *MountPodPatch) isMatch(pvc *corev1.PersistentVolumeClaim) bool {
	if mpp.PVCSelector == nil {
		return true
	}
	if pvc == nil {
		return false
	}
	if mpp.PVCSelector.MatchName != "" && mpp.PVCSelector.MatchName != pvc.Name {
		return false
	}
	if mpp.PVCSelector.MatchStorageClassName != "" && pvc.Spec.StorageClassName != nil && mpp.PVCSelector.MatchStorageClassName != *pvc.Spec.StorageClassName {
		return false
	}
	selector, err := metav1.LabelSelectorAsSelector(&mpp.PVCSelector.LabelSelector)
	if err != nil {
		return false
	}
	return selector.Matches(labels.Set(pvc.Labels))
}

func (mpp *MountPodPatch) deepCopy() MountPodPatch {
	var copy MountPodPatch
	data, _ := json.Marshal(mpp)
	_ = json.Unmarshal(data, &copy)
	return copy
}

func (mpp *MountPodPatch) merge(mp MountPodPatch) {
	if mp.CEMountImage != "" {
		mpp.CEMountImage = mp.CEMountImage
	}
	if mp.EEMountImage != "" {
		mpp.EEMountImage = mp.EEMountImage
	}
	if mp.HostNetwork != nil {
		mpp.HostNetwork = mp.HostNetwork
	}
	if mp.HostPID != nil {
		mpp.HostPID = mp.HostPID
	}
	if mp.LivenessProbe != nil {
		mpp.LivenessProbe = mp.LivenessProbe
	}
	if mp.ReadinessProbe != nil {
		mpp.ReadinessProbe = mp.ReadinessProbe
	}
	if mp.ReadinessProbe != nil {
		mpp.ReadinessProbe = mp.ReadinessProbe
	}
	if mp.Lifecycle != nil {
		mpp.Lifecycle = mp.Lifecycle
	}
	if mp.Labels != nil {
		mpp.Labels = mp.Labels
	}
	if mp.Annotations != nil {
		mpp.Annotations = mp.Annotations
	}
	if mp.Resources != nil {
		mpp.Resources = mp.Resources
	}
	if mp.TerminationGracePeriodSeconds != nil {
		mpp.TerminationGracePeriodSeconds = mp.TerminationGracePeriodSeconds
	}
}

// TODO: migrate more config for here
type Config struct {
	// arrange mount pod to node with node selector instead nodeName
	EnableNodeSelector bool            `json:"enableNodeSelector,omitempty"`
	MountPodPatch      []MountPodPatch `json:"mountPodPatch"`
}

func (c *Config) Unmarshal(data []byte) error {
	return yaml.Unmarshal(data, c)
}

// GenMountPodPatch generate mount pod patch from jfsSettting
// 1. match pv selector
// 2. parse template value
// 3. return the merged mount pod patch
func (c *Config) GenMountPodPatch(setting JfsSetting) MountPodPatch {
	patch := &MountPodPatch{
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}

	// merge each patch
	for _, mp := range c.MountPodPatch {
		if mp.isMatch(setting.PVC) {
			patch.merge(mp.deepCopy())
		}
	}
	if setting.IsCe {
		patch.Image = patch.CEMountImage
	} else {
		patch.Image = patch.EEMountImage
	}

	data, _ := json.Marshal(patch)
	strData := string(data)
	strData = strings.ReplaceAll(strData, "${MOUNT_POINT}", setting.MountPath)
	strData = strings.ReplaceAll(strData, "${VOLUME_ID}", setting.VolumeId)
	strData = strings.ReplaceAll(strData, "${VOLUME_NAME}", setting.Name)
	strData = strings.ReplaceAll(strData, "${SUB_PATH}", setting.SubPath)
	_ = json.Unmarshal([]byte(strData), patch)
	klog.V(6).Infof("volume %s using patch: %+v", setting.VolumeId, patch)
	return *patch
}

// reset to default value
// used to unit tests
func (c *Config) Reset() {
	*c = Config{}
}

func newCfg() *Config {
	c := &Config{}
	c.Reset()
	return c
}

var GlobalConfig = newCfg()

func LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("fail to read config file %s: %v", configPath, err)
	}

	cfg := newCfg()

	// compatible with old version
	if os.Getenv("ENABLE_NODE_SELECTOR") == "1" {
		cfg.EnableNodeSelector = true
	}

	err = cfg.Unmarshal(data)
	if err != nil {
		return err
	}

	GlobalConfig = cfg
	klog.V(6).Infof("config loaded: %+v", GlobalConfig)
	return err
}

// ConfigReloader reloads config file when it is updated
func StartConfigReloader(configPath string) error {
	// load first
	if err := LoadConfig(configPath); err != nil {
		return err
	}
	fsnotifyWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := fsnotifyWatcher.Add(configPath); err != nil {
		return err
	}

	go func(watcher *fsnotify.Watcher) {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					klog.Errorf("fsnotify watcher closed")
					continue
				}
				if event.Op != fsnotify.Write && event.Op != fsnotify.Remove {
					continue
				}
				// k8s configmaps uses symlinks, we need this workaround to detect the real file change
				if event.Op == fsnotify.Remove {
					_ = watcher.Remove(event.Name)
					// add a new watcher pointing to the new symlink/file
					_ = watcher.Add(configPath)
				}

				klog.Infof("config file %s updated, reload config", configPath)
				err := LoadConfig(configPath)
				if err != nil {
					klog.Errorf("fail to reload config: %v", err)
					continue
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					continue
				}
				klog.Errorf("fsnotify error: %v", err)
			}
		}
	}(fsnotifyWatcher)

	// fallback policy: reload config every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			err = LoadConfig(configPath)
			if err != nil {
				klog.Error(err)
			}
		}
	}()

	return nil
}
