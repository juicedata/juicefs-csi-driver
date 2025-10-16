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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

func TestParseMountConfig(t *testing.T) {
	tests := []struct {
		name      string
		configStr string
		wantMode  MountMode
		wantErr   bool
	}{
		{
			name: "per-pvc mode",
			configStr: `mode: per-pvc`,
			wantMode: MountModePVC,
		},
		{
			name: "shared-pod mode",
			configStr: `mode: shared-pod`,
			wantMode: MountModeSharedPod,
		},
		{
			name: "daemonset mode with node affinity",
			configStr: `
mode: daemonset
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: test-key
        operator: In
        values:
        - test-value`,
			wantMode: MountModeDaemonSet,
		},
		{
			name: "invalid mode",
			configStr: `mode: invalid-mode`,
			wantErr: true,
		},
		{
			name: "empty config uses default",
			configStr: ``,
			wantMode: "", // Will use default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := parseMountConfig(tt.configStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMountConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && config.Mode != tt.wantMode {
				t.Errorf("parseMountConfig() mode = %v, want %v", config.Mode, tt.wantMode)
			}
		})
	}
}

func TestGetMountConfig(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name             string
		storageClassName string
		configMap        *corev1.ConfigMap
		globalShareMount bool
		wantMode         MountMode
		wantHasAffinity  bool
	}{
		{
			name:             "no configmap, use global per-pvc",
			storageClassName: "test-sc",
			configMap:        nil,
			globalShareMount: false,
			wantMode:         MountModePVC,
		},
		{
			name:             "no configmap, use global shared-pod",
			storageClassName: "test-sc",
			configMap:        nil,
			globalShareMount: true,
			wantMode:         MountModeSharedPod,
		},
		{
			name:             "configmap with default",
			storageClassName: "test-sc",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      MountConfigMapName,
					Namespace: Namespace,
				},
				Data: map[string]string{
					DefaultConfigKey: `mode: shared-pod`,
				},
			},
			wantMode: MountModeSharedPod,
		},
		{
			name:             "configmap with specific storage class",
			storageClassName: "test-sc",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      MountConfigMapName,
					Namespace: Namespace,
				},
				Data: map[string]string{
					DefaultConfigKey: `mode: per-pvc`,
					"test-sc": `
mode: daemonset
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: test
        operator: Exists`,
				},
			},
			wantMode:        MountModeDaemonSet,
			wantHasAffinity: true,
		},
		{
			name:             "configmap with invalid config falls back to default",
			storageClassName: "test-sc",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      MountConfigMapName,
					Namespace: Namespace,
				},
				Data: map[string]string{
					"test-sc": `mode: invalid`,
				},
			},
			globalShareMount: true,
			wantMode:         MountModeSharedPod,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global variables
			StorageClassShareMount = tt.globalShareMount
			
			// Create fake k8s client
			var objects []runtime.Object
			if tt.configMap != nil {
				objects = append(objects, tt.configMap)
			}
			fakeClient := fake.NewSimpleClientset(objects...)
			k8sClient := &k8sclient.K8sClient{}
			k8sClient.Interface = fakeClient
			
			// Get mount config
			config, err := GetMountConfig(ctx, k8sClient, tt.storageClassName)
			if err != nil {
				t.Errorf("GetMountConfig() error = %v", err)
				return
			}
			
			if config.Mode != tt.wantMode {
				t.Errorf("GetMountConfig() mode = %v, want %v", config.Mode, tt.wantMode)
			}
			
			if tt.wantHasAffinity && config.NodeAffinity == nil {
				t.Errorf("GetMountConfig() expected NodeAffinity but got nil")
			}
		})
	}
}

func TestMountModeHelpers(t *testing.T) {
	tests := []struct {
		name          string
		mountMode     string
		wantDaemonSet bool
		wantSharedPod bool
		wantPVCPod    bool
	}{
		{
			name:          "daemonset mode",
			mountMode:     string(MountModeDaemonSet),
			wantDaemonSet: true,
			wantSharedPod: false,
			wantPVCPod:    false,
		},
		{
			name:          "shared-pod mode",
			mountMode:     string(MountModeSharedPod),
			wantDaemonSet: false,
			wantSharedPod: true,
			wantPVCPod:    false,
		},
		{
			name:          "per-pvc mode",
			mountMode:     string(MountModePVC),
			wantDaemonSet: false,
			wantSharedPod: false,
			wantPVCPod:    true,
		},
		{
			name:          "empty mode defaults to per-pvc",
			mountMode:     "",
			wantDaemonSet: false,
			wantSharedPod: false,
			wantPVCPod:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := &JfsSetting{
				MountMode: tt.mountMode,
			}
			
			if got := ShouldUseDaemonSet(setting); got != tt.wantDaemonSet {
				t.Errorf("ShouldUseDaemonSet() = %v, want %v", got, tt.wantDaemonSet)
			}
			if got := ShouldUseSharedPod(setting); got != tt.wantSharedPod {
				t.Errorf("ShouldUseSharedPod() = %v, want %v", got, tt.wantSharedPod)
			}
			if got := ShouldUsePVCPod(setting); got != tt.wantPVCPod {
				t.Errorf("ShouldUsePVCPod() = %v, want %v", got, tt.wantPVCPod)
			}
		})
	}
}

func TestLoadMountConfig(t *testing.T) {
	ctx := context.Background()
	
	// Create test PV
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pv",
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: "test-sc",
		},
	}
	
	// Create configmap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MountConfigMapName,
			Namespace: Namespace,
		},
		Data: map[string]string{
			"test-sc": `
mode: daemonset
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: test-key
        operator: In
        values:
        - test-value`,
		},
	}
	
	// Create fake k8s client
	fakeClient := fake.NewSimpleClientset(configMap)
	k8sClient := &k8sclient.K8sClient{}
	k8sClient.Interface = fakeClient
	
	// Test loading config
	setting := &JfsSetting{
		PV:       pv,
		UniqueId: "test-unique-id",
	}
	
	err := LoadMountConfig(ctx, k8sClient, setting)
	if err != nil {
		t.Errorf("LoadMountConfig() error = %v", err)
		return
	}
	
	if setting.MountMode != string(MountModeDaemonSet) {
		t.Errorf("LoadMountConfig() mount mode = %v, want %v", setting.MountMode, MountModeDaemonSet)
	}
	
	if setting.StorageClassNodeAffinity == nil {
		t.Errorf("LoadMountConfig() expected NodeAffinity but got nil")
	}
}