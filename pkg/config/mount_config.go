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

const (
	MountConfigMapName = "juicefs-mount-config"
	DefaultConfigKey   = "default"
)

// DeploymentMode constants
const (
	DeploymentModeSharedPod = "sharedPod"
	DeploymentModeDaemonSet = "daemonset"
)

// MountConfig represents the complete mount configuration
type MountConfig struct {
	// DeploymentMode specifies how shared mount pods are deployed: sharedPod or daemonset
	// Only applicable when MountShareMode is enabled (fsShareMount or storageClassShareMount)
	DeploymentMode string `yaml:"deploymentMode,omitempty"`
	
	// NodeAffinity is used when DeploymentMode is daemonset
	NodeAffinity *corev1.NodeAffinity `yaml:"nodeAffinity,omitempty"`
	
	// Additional mount pod configurations can be added here in the future
	// For example:
	// Resources    *corev1.ResourceRequirements `yaml:"resources,omitempty"`
	// Tolerations  []corev1.Toleration          `yaml:"tolerations,omitempty"`
	// Labels       map[string]string            `yaml:"labels,omitempty"`
	// Annotations  map[string]string            `yaml:"annotations,omitempty"`
}

// GetMountConfig retrieves the mount configuration for a given StorageClass
func GetMountConfig(ctx context.Context, client *k8sclient.K8sClient, storageClassName string) (*MountConfig, error) {
	log := klog.NewKlogr().WithName("mount-config")
	
	// Start with global defaults
	defaultConfig := &MountConfig{
		DeploymentMode: getDefaultDeploymentMode(),
	}
	
	// Try to get the ConfigMap
	configMap, err := client.GetConfigMap(ctx, MountConfigMapName, Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.V(1).Info("Mount ConfigMap not found, using global defaults", 
				"configMap", MountConfigMapName, "namespace", Namespace, "deploymentMode", defaultConfig.DeploymentMode)
			return defaultConfig, nil
		}
		return nil, fmt.Errorf("failed to get mount ConfigMap: %v", err)
	}

	// Try to get StorageClass-specific configuration
	if configData, exists := configMap.Data[storageClassName]; exists {
		log.V(1).Info("Found StorageClass-specific mount configuration", 
			"storageClass", storageClassName)
		config, err := parseMountConfig(configData)
		if err != nil {
			log.Error(err, "Failed to parse StorageClass-specific configuration, using defaults",
				"storageClass", storageClassName)
			return defaultConfig, nil
		}
		// Fill in any missing values with defaults
		if config.DeploymentMode == "" {
			config.DeploymentMode = defaultConfig.DeploymentMode
		}
		return config, nil
	}

	// Try default configuration from ConfigMap
	if configData, exists := configMap.Data[DefaultConfigKey]; exists {
		log.V(1).Info("Using default mount configuration from ConfigMap for StorageClass", 
			"storageClass", storageClassName)
		config, err := parseMountConfig(configData)
		if err != nil {
			log.Error(err, "Failed to parse default configuration, using global defaults")
			return defaultConfig, nil
		}
		// Fill in any missing values with defaults
		if config.DeploymentMode == "" {
			config.DeploymentMode = defaultConfig.DeploymentMode
		}
		return config, nil
	}

	log.V(1).Info("No mount configuration found in ConfigMap, using global defaults", 
		"storageClass", storageClassName, "deploymentMode", defaultConfig.DeploymentMode)
	return defaultConfig, nil
}

// parseMountConfig parses the configuration string into a MountConfig
func parseMountConfig(configData string) (*MountConfig, error) {
	config := &MountConfig{}
	if err := yaml.Unmarshal([]byte(configData), config); err != nil {
		return nil, fmt.Errorf("failed to parse mount configuration: %v", err)
	}
	
	// Validate deployment mode
	if config.DeploymentMode != "" && 
		config.DeploymentMode != "sharedPod" && 
		config.DeploymentMode != "daemonset" {
		return nil, fmt.Errorf("invalid deployment mode: %s", config.DeploymentMode)
	}
	
	return config, nil
}

// getDefaultDeploymentMode returns the default deployment mode
func getDefaultDeploymentMode() string {
	// Default to sharedPod when sharing is enabled
	// DaemonSet mode is only used when explicitly configured via ConfigMap
	return "sharedPod"
}

// LoadMountConfig loads mount configuration for a JfsSetting
func LoadMountConfig(ctx context.Context, client *k8sclient.K8sClient, jfsSetting *JfsSetting) error {
	log := klog.NewKlogr().WithName("mount-config")
	
	// Get StorageClass name from PV if available
	storageClassName := ""
	if jfsSetting.PV != nil && jfsSetting.PV.Spec.StorageClassName != "" {
		storageClassName = jfsSetting.PV.Spec.StorageClassName
	} else {
		// For static provisioning or when PV is not available,
		// use the unique ID as the key in ConfigMap
		storageClassName = jfsSetting.UniqueId
	}

	config, err := GetMountConfig(ctx, client, storageClassName)
	if err != nil {
		log.Error(err, "Failed to get mount configuration, using defaults", 
			"storageClass", storageClassName)
		// Don't fail mount if ConfigMap is misconfigured
		// Just proceed with defaults
		config = &MountConfig{
			DeploymentMode: getDefaultDeploymentMode(),
		}
	}

	// Store the deployment mode and configuration in JfsSetting
	jfsSetting.DeploymentMode = config.DeploymentMode
	if config.DeploymentMode == "daemonset" && config.NodeAffinity != nil {
		jfsSetting.StorageClassNodeAffinity = config.NodeAffinity
		log.Info("Loaded mount configuration", 
			"storageClass", storageClassName,
			"deploymentMode", config.DeploymentMode,
			"hasNodeAffinity", true)
	} else {
		log.Info("Loaded mount configuration", 
			"storageClass", storageClassName,
			"deploymentMode", config.DeploymentMode)
	}

	return nil
}

// ShouldUseDaemonSet checks if DaemonSet should be used for the given JfsSetting
func ShouldUseDaemonSet(jfsSetting *JfsSetting) bool {
	return jfsSetting.MountShareMode != "" && jfsSetting.DeploymentMode == "daemonset"
}

// ShouldUseSharedPod checks if shared pod should be used for the given JfsSetting
func ShouldUseSharedPod(jfsSetting *JfsSetting) bool {
	return jfsSetting.MountShareMode != "" && jfsSetting.DeploymentMode != "daemonset"
}

// ShouldUsePVCPod checks if per-PVC pod should be used for the given JfsSetting
func ShouldUsePVCPod(jfsSetting *JfsSetting) bool {
	return jfsSetting.MountShareMode == ""
}