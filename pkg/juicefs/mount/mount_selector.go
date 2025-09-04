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
	"context"

	"k8s.io/klog/v2"
	k8sMount "k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

// MountSelector dynamically selects the appropriate mount interface based on configuration
type MountSelector struct {
	log klog.Logger
	k8sMount.SafeFormatAndMount
	K8sClient    *k8sclient.K8sClient
	processMount MntInterface
	podMount     MntInterface
	daemonMount  MntInterface
}

var _ MntInterface = &MountSelector{}

// NewMountSelector creates a new mount selector that chooses the appropriate mount implementation
func NewMountSelector(client *k8sclient.K8sClient, mounter k8sMount.SafeFormatAndMount) MntInterface {
	return &MountSelector{
		log:          klog.NewKlogr().WithName("mount-selector"),
		SafeFormatAndMount: mounter,
		K8sClient:    client,
		processMount: nil, // Created on demand
		podMount:     nil, // Created on demand
		daemonMount:  nil, // Created on demand
	}
}

// selectMount chooses the appropriate mount interface based on JfsSetting configuration
func (m *MountSelector) selectMount(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) MntInterface {
	log := util.GenLog(ctx, m.log, "selectMount")
	
	// Load mount configuration from ConfigMap if not already loaded
	if jfsSetting.MountMode == "" {
		if err := jfsConfig.LoadMountConfig(ctx, m.K8sClient, jfsSetting); err != nil {
			log.Error(err, "Failed to load mount configuration, using default")
		}
	}
	
	// Select mount implementation based on mode
	switch {
	case jfsConfig.ByProcess:
		log.V(1).Info("Using process mount")
		if m.processMount == nil {
			m.processMount = NewProcessMount(m.SafeFormatAndMount)
		}
		return m.processMount
		
	case jfsConfig.ShouldUseDaemonSet(jfsSetting):
		log.Info("Using DaemonSet mount", "uniqueId", jfsSetting.UniqueId)
		if m.daemonMount == nil {
			m.daemonMount = NewDaemonSetMount(m.K8sClient, m.SafeFormatAndMount)
		}
		return m.daemonMount
		
	case jfsConfig.ShouldUseSharedPod(jfsSetting):
		log.Info("Using shared pod mount", "uniqueId", jfsSetting.UniqueId)
		if m.podMount == nil {
			m.podMount = NewPodMount(m.K8sClient, m.SafeFormatAndMount)
		}
		return m.podMount
		
	default:
		log.V(1).Info("Using per-PVC pod mount", "volumeId", jfsSetting.VolumeId)
		if m.podMount == nil {
			m.podMount = NewPodMount(m.K8sClient, m.SafeFormatAndMount)
		}
		return m.podMount
	}
}

// JMount mounts JuiceFS volume
func (m *MountSelector) JMount(ctx context.Context, appInfo *jfsConfig.AppInfo, jfsSetting *jfsConfig.JfsSetting) error {
	log := util.GenLog(ctx, m.log, "JMount")
	mnt := m.selectMount(ctx, jfsSetting)
	
	// Try to mount using the selected mount type
	err := mnt.JMount(ctx, appInfo, jfsSetting)
	
	// If it's a DaemonSet scheduling error, fall back to shared pod mount
	if IsDaemonSetSchedulingError(err) {
		log.Info("DaemonSet cannot schedule on this node, falling back to shared pod mount", 
			"error", err, "uniqueId", jfsSetting.UniqueId)
		
		// Override the mount mode to shared-pod for this specific mount
		originalMode := jfsSetting.MountMode
		jfsSetting.MountMode = string(jfsConfig.MountModeSharedPod)
		
		// Use shared pod mount as fallback
		if m.podMount == nil {
			m.podMount = NewPodMount(m.K8sClient, m.SafeFormatAndMount)
		}
		
		err = m.podMount.JMount(ctx, appInfo, jfsSetting)
		
		// Restore original mode (in case it's used elsewhere)
		jfsSetting.MountMode = originalMode
		
		if err != nil {
			log.Error(err, "Fallback to shared pod mount also failed")
			return err
		}
		
		log.Info("Successfully mounted using shared pod fallback", "uniqueId", jfsSetting.UniqueId)
		return nil
	}
	
	return err
}

// GetMountRef gets mount references
func (m *MountSelector) GetMountRef(ctx context.Context, target, podName string) (int, error) {
	// For GetMountRef, we need to determine which mount type is being used
	// This is a bit tricky without the JfsSetting, so we check what exists
	
	// First check if it's a DaemonSet
	if dsName := m.getDaemonSetNameFromPodName(podName); dsName != "" {
		if m.daemonMount == nil {
			m.daemonMount = NewDaemonSetMount(m.K8sClient, m.SafeFormatAndMount)
		}
		return m.daemonMount.GetMountRef(ctx, target, dsName)
	}
	
	// Otherwise use pod mount
	if m.podMount == nil {
		m.podMount = NewPodMount(m.K8sClient, m.SafeFormatAndMount)
	}
	return m.podMount.GetMountRef(ctx, target, podName)
}

// UmountTarget unmounts target
func (m *MountSelector) UmountTarget(ctx context.Context, target, podName string) error {
	// Determine which mount type is being used
	if dsName := m.getDaemonSetNameFromPodName(podName); dsName != "" {
		if m.daemonMount == nil {
			m.daemonMount = NewDaemonSetMount(m.K8sClient, m.SafeFormatAndMount)
		}
		return m.daemonMount.UmountTarget(ctx, target, dsName)
	}
	
	// Otherwise use pod mount
	if m.podMount == nil {
		m.podMount = NewPodMount(m.K8sClient, m.SafeFormatAndMount)
	}
	return m.podMount.UmountTarget(ctx, target, podName)
}

// JUmount unmounts JuiceFS volume
func (m *MountSelector) JUmount(ctx context.Context, target, podName string) error {
	// Try to find if it's mounted by DaemonSet
	if m.daemonMount == nil {
		m.daemonMount = NewDaemonSetMount(m.K8sClient, m.SafeFormatAndMount)
	}
	
	// Check if DaemonSet has this target
	dsList, err := m.K8sClient.ListDaemonSet(ctx, jfsConfig.Namespace, nil)
	if err == nil {
		key := util.GetReferenceKey(target)
		for _, ds := range dsList {
			if ds.Annotations != nil && ds.Annotations[key] == target {
				return m.daemonMount.JUmount(ctx, target, ds.Name)
			}
		}
	}
	
	// Otherwise use pod mount
	if m.podMount == nil {
		m.podMount = NewPodMount(m.K8sClient, m.SafeFormatAndMount)
	}
	return m.podMount.JUmount(ctx, target, podName)
}


// JCreateVolume creates JuiceFS volume (CE only)
func (m *MountSelector) JCreateVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	// Volume creation always uses pod mount
	if m.podMount == nil {
		m.podMount = NewPodMount(m.K8sClient, m.SafeFormatAndMount)
	}
	return m.podMount.JCreateVolume(ctx, jfsSetting)
}

// JDeleteVolume deletes JuiceFS volume (CE only)
func (m *MountSelector) JDeleteVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	// Volume deletion always uses pod mount
	if m.podMount == nil {
		m.podMount = NewPodMount(m.K8sClient, m.SafeFormatAndMount)
	}
	return m.podMount.JDeleteVolume(ctx, jfsSetting)
}

// AddRefOfMount adds reference of mount
func (m *MountSelector) AddRefOfMount(ctx context.Context, target string, podName string) error {
	// Determine which mount type is being used
	if dsName := m.getDaemonSetNameFromPodName(podName); dsName != "" {
		if m.daemonMount == nil {
			m.daemonMount = NewDaemonSetMount(m.K8sClient, m.SafeFormatAndMount)
		}
		return m.daemonMount.AddRefOfMount(ctx, target, dsName)
	}
	
	// Otherwise use pod mount
	if m.podMount == nil {
		m.podMount = NewPodMount(m.K8sClient, m.SafeFormatAndMount)
	}
	return m.podMount.AddRefOfMount(ctx, target, podName)
}

// CleanCache cleans cache
func (m *MountSelector) CleanCache(ctx context.Context, image string, id string, volumeId string, cacheDirs []string) error {
	// For now, delegate to pod mount
	if m.podMount == nil {
		m.podMount = NewPodMount(m.K8sClient, m.SafeFormatAndMount)
	}
	return m.podMount.CleanCache(ctx, image, id, volumeId, cacheDirs)
}

// getDaemonSetNameFromPodName tries to determine if this pod is managed by a DaemonSet
func (m *MountSelector) getDaemonSetNameFromPodName(podName string) string {
	// DaemonSet pods have a specific naming pattern
	// This is a simple heuristic, could be improved
	if len(podName) > 7 && podName[len(podName)-7:len(podName)-6] == "-" {
		// Try to get the DaemonSet name
		// Format: juicefs-<uniqueid>-mount-ds-<hash>
		if len(podName) > 10 && podName[len(podName)-10:len(podName)-6] == "-ds-" {
			return podName[:len(podName)-6]
		}
	}
	return ""
}