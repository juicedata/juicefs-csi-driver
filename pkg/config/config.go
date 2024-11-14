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
	"context"
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
	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var (
	log                    = klog.NewKlogr().WithName("config")
	WebPort                = MustGetWebPort() // web port used by metrics
	ByProcess              = false            // csi driver runs juicefs in process or not
	FormatInPod            = false            // put format/auth in pod (only in k8s)
	Provisioner            = false            // provisioner in controller
	CacheClientConf        = false            // cache client config files and use directly in mount containers
	MountManager           = false            // manage mount pod in controller (only in k8s)
	Webhook                = false            // inject juicefs client as sidecar in pod (only in k8s)
	ValidatingWebhook      = false            // start validating webhook, applicable to ee only
	Immutable              = false            // csi driver is running in an immutable environment
	StorageClassShareMount = false            // share mount pod for the same storage class

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
	ShutdownSockPath      = "/tmp/juicefs-csi-shutdown.sock"
	JfsFuseFdPathName     = "jfs-fuse-fd"

	DefaultCEMountImage = "juicedata/mount:ce-nightly" // mount pod ce image, override by ENV
	DefaultEEMountImage = "juicedata/mount:ee-nightly" // mount pod ee image, override by ENV
)

// env auto set by the csi side
var CSISetEnvMap = map[string]interface{}{
	"_JFS_META_SID":                     nil,
	"JFS_NO_UMOUNT":                     nil,
	"JFS_NO_UPDATE":                     nil,
	"JFS_FOREGROUND":                    nil,
	"JFS_SUPER_COMM":                    nil,
	"JFS_INSIDE_CONTAINER":              nil,
	"JUICEFS_CLIENT_PATH":               nil,
	"JUICEFS_CLIENT_SIDERCAR_CONTAINER": nil,
	"JFS_NO_CHECK_OBJECT_STORAGE":       nil,
}

// opts auto set by the csi side
var CSISetOptsMap = map[string]interface{}{
	"no-update":  nil,
	"foreground": nil,
	"metrics":    nil,
	"rsa-key":    nil,
}

// volume set by the csi side
var interVolumesPrefix = []string{
	"rsa-key",
	"init-config",
	"config-",
	"jfs-dir",
	"update-db",
	"cachedir-",
}

func IsInterVolume(name string) bool {
	for _, prefix := range interVolumesPrefix {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

var PodLocks [1024]sync.Mutex

func GetPodLockKey(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}
	podHashVal := pod.Labels[common.PodJuiceHashLabelKey]
	if podHashVal == "" {
		return pod.Name
	}
	return podHashVal
}

func GetPodLock(podHashVal string) *sync.Mutex {
	h := fnv.New32a()
	h.Write([]byte(podHashVal))
	index := h.Sum32() % 1024
	return &PodLocks[index]
}

func MustGetWebPort() int {
	value, exists := os.LookupEnv("JUICEFS_CSI_WEB_PORT")
	if exists {
		port, err := strconv.Atoi(value)
		if err == nil {
			return port
		}
		log.Error(err, "Fail to parse JUICEFS_CSI_WEB_PORT", "port", value)
	}
	return 8080
}

type PVCSelector struct {
	metav1.LabelSelector
	MatchStorageClassName string `json:"matchStorageClassName,omitempty"`
	MatchName             string `json:"matchName,omitempty"`
}

type MountPatchCacheDirType string

var (
	MountPatchCacheDirTypeHostPath MountPatchCacheDirType = "HostPath"
	MountPatchCacheDirTypePVC      MountPatchCacheDirType = "PVC"
)

type MountPatchCacheDir struct {
	Type MountPatchCacheDirType `json:"type,omitempty"`

	// required for HostPath type
	Path string `json:"path,omitempty"`

	// required for PVC type
	Name string `json:"name,omitempty"`
}

type MountPodPatch struct {
	// used to specify the selector for the PVC that will be patched
	// omit will patch for all PVC
	PVCSelector *PVCSelector `json:"pvcSelector,omitempty"`

	CEMountImage string               `json:"ceMountImage,omitempty"`
	EEMountImage string               `json:"eeMountImage,omitempty"`
	CacheDirs    []MountPatchCacheDir `json:"cacheDirs,omitempty"`

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
	Volumes                       []corev1.Volume              `json:"volumes,omitempty"`
	VolumeDevices                 []corev1.VolumeDevice        `json:"volumeDevices,omitempty"`
	VolumeMounts                  []corev1.VolumeMount         `json:"volumeMounts,omitempty"`
	Env                           []corev1.EnvVar              `json:"env,omitempty"`
	MountOptions                  []string                     `json:"mountOptions,omitempty"`
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
	if mpp.PVCSelector.MatchStorageClassName != "" {
		if pvc.Spec.StorageClassName == nil {
			return false
		}
		return *pvc.Spec.StorageClassName == mpp.PVCSelector.MatchStorageClassName
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
	vok := make(map[string]bool)
	if mp.Volumes != nil {
		if mpp.Volumes == nil {
			mpp.Volumes = []corev1.Volume{}
		}
		for _, v := range mp.Volumes {
			if IsInterVolume(v.Name) {
				log.Info("applyConfig: volume uses an internal volume name, ignore", "volume", v.Name)
				continue
			}
			found := false
			for _, vv := range mpp.Volumes {
				if vv.Name == v.Name {
					found = true
					break
				}
			}
			if found {
				log.Info("applyConfig: volume already exists, ignore", "volume", v.Name)
				continue
			}
			vok[v.Name] = true
			mpp.Volumes = append(mpp.Volumes, v)
		}
	}
	if mp.VolumeMounts != nil {
		if mpp.VolumeMounts == nil {
			mpp.VolumeMounts = []corev1.VolumeMount{}
		}
		for _, vm := range mp.VolumeMounts {
			if !vok[vm.Name] {
				log.Info("applyConfig: volumeMount not exists in volumes, ignore", "volume", vm.Name)
				continue
			}
			mpp.VolumeMounts = append(mpp.VolumeMounts, vm)
		}
	}
	if mp.VolumeDevices != nil {
		if mpp.VolumeDevices == nil {
			mpp.VolumeDevices = []corev1.VolumeDevice{}
		}
		for _, vm := range mp.VolumeDevices {
			if !vok[vm.Name] {
				log.Info("applyConfig: volumeDevices not exists in volumes, ignore", "volume", vm.Name)
				continue
			}
			mpp.VolumeDevices = append(mpp.VolumeDevices, vm)
		}
	}
	if mp.Env != nil {
		mpp.Env = mp.Env
	}
	if mp.MountOptions != nil {
		mpp.MountOptions = mp.MountOptions
	}
	if mp.CacheDirs != nil {
		mpp.CacheDirs = mp.CacheDirs
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
	log.V(1).Info("volume using patch", "volumeId", setting.VolumeId, "patch", patch)
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
	log.V(1).Info("config loaded", "global config", *GlobalConfig)
	return err
}

func LoadFromConfigMap(ctx context.Context, client *k8s.K8sClient) error {
	cmName := os.Getenv("JUICEFS_CONFIG_NAME")
	if cmName == "" {
		cmName = "juicefs-csi-driver-config"
	}
	sysNamespace := os.Getenv("SYS_NAMESPACE")
	if sysNamespace == "" {
		sysNamespace = "kube-system"
	}
	cm, err := client.GetConfigMap(ctx, cmName, sysNamespace)
	if err != nil {
		return err
	}

	cfg := newCfg()

	// compatible with old version
	if os.Getenv("ENABLE_NODE_SELECTOR") == "1" {
		cfg.EnableNodeSelector = true
	}

	err = cfg.Unmarshal([]byte(cm.Data["config.yaml"]))
	if err != nil {
		return err
	}

	GlobalConfig = cfg
	log.V(1).Info("config loaded", "global config", *GlobalConfig)
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
					log.Info("fsnotify watcher closed")
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

				log.Info("config file updated, reload config", "config file", configPath)
				err := LoadConfig(configPath)
				if err != nil {
					log.Error(err, "fail to reload config")
					continue
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					continue
				}
				log.Error(err, "fsnotify error")
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
				log.Error(err, "fail to load config")
			}
		}
	}()

	return nil
}
