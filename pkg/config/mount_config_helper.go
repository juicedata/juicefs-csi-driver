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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"

	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

// Helper functions for DaemonSet node affinity configuration
// The main MountConfig struct is defined in mount_config.go

// GetDaemonSetNodeAffinity retrieves the DaemonSet node affinity configuration for a given StorageClass
// It first checks for a StorageClass-specific configuration, then falls back to default
func GetDaemonSetNodeAffinity(ctx context.Context, client *k8sclient.K8sClient, storageClassName string) (*corev1.NodeAffinity, error) {
	log := klog.NewKlogr().WithName("mount-config")
	
	// Try to get the ConfigMap
	configMap, err := client.GetConfigMap(ctx, MountConfigMapName, Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.V(1).Info("Mount ConfigMap not found, using no node affinity", 
				"configMap", MountConfigMapName, "namespace", Namespace)
			return nil, nil // No ConfigMap means no node affinity restrictions
		}
		return nil, fmt.Errorf("failed to get Mount ConfigMap: %v", err)
	}

	// Try to get StorageClass-specific configuration
	if configData, exists := configMap.Data[storageClassName]; exists {
		log.V(1).Info("Found StorageClass-specific mount configuration", 
			"storageClass", storageClassName)
		return parseDaemonSetNodeAffinity(configData)
	}

	// Fall back to default configuration
	if configData, exists := configMap.Data[DefaultConfigKey]; exists {
		log.V(1).Info("Using default mount configuration for StorageClass", 
			"storageClass", storageClassName)
		return parseDaemonSetNodeAffinity(configData)
	}

	log.V(1).Info("No mount configuration found for StorageClass", 
		"storageClass", storageClassName)
	return nil, nil
}

// parseDaemonSetNodeAffinity parses the configuration string to extract NodeAffinity
func parseDaemonSetNodeAffinity(configData string) (*corev1.NodeAffinity, error) {
	config := &MountConfig{}
	if err := yaml.Unmarshal([]byte(configData), config); err != nil {
		return nil, fmt.Errorf("failed to parse mount configuration: %v", err)
	}
	return config.NodeAffinity, nil
}

// LoadDaemonSetNodeAffinity loads node affinity for a StorageClass from ConfigMap
// This is called when creating or updating a DaemonSet for mount pods
func LoadDaemonSetNodeAffinity(ctx context.Context, client *k8sclient.K8sClient, jfsSetting *JfsSetting) error {
	log := klog.NewKlogr().WithName("mount-config")
	
	// This should only be called when mount sharing is enabled
	// If we're here without mount sharing, it's a programming error
	if !StorageClassShareMount {
		log.Error(nil, "LoadDaemonSetNodeAffinity called but StorageClassShareMount is false - this should not happen")
		return fmt.Errorf("LoadDaemonSetNodeAffinity called without mount sharing enabled")
	}

	// Skip if node affinity already set (from StorageClass parameters)
	if jfsSetting.StorageClassNodeAffinity != nil {
		log.V(1).Info("Node affinity already set from StorageClass parameters")
		return nil
	}

	// Get StorageClass name from PV if available
	storageClassName := ""
	if jfsSetting.PV != nil && jfsSetting.PV.Spec.StorageClassName != "" {
		storageClassName = jfsSetting.PV.Spec.StorageClassName
	} else {
		// For static provisioning or when PV is not available,
		// use the unique ID as the key in ConfigMap
		storageClassName = jfsSetting.UniqueId
	}

	nodeAffinity, err := GetDaemonSetNodeAffinity(ctx, client, storageClassName)
	if err != nil {
		log.Error(err, "Failed to get DaemonSet configuration", 
			"storageClass", storageClassName)
		// Don't fail mount if ConfigMap is misconfigured
		// Just proceed without node affinity
		return nil
	}

	jfsSetting.StorageClassNodeAffinity = nodeAffinity
	if nodeAffinity != nil {
		log.Info("Loaded node affinity from ConfigMap for DaemonSet deployment", 
			"storageClass", storageClassName)
	}

	return nil
}