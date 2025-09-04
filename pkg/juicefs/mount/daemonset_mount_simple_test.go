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
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sMount "k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func TestDaemonSetMount_CreateOrUpdate(t *testing.T) {
	ctx := context.Background()
	jfsConfig.Namespace = "test-ns"
	
	tests := []struct {
		name       string
		jfsSetting *jfsConfig.JfsSetting
		existingDS *appsv1.DaemonSet
		expectHash string
		expectRefs int
	}{
		{
			name: "create new daemonset",
			jfsSetting: &jfsConfig.JfsSetting{
				UniqueId:   "test-unique-id",
				TargetPath: "/var/lib/kubelet/pods/test-pod/volumes/test-volume",
				VolumeId:   "test-volume",
				Name:       "test-name",
				MetaUrl:    "redis://localhost:6379/1",
				Source:     "test-source",
				PV: &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pv",
					},
				},
				MountMode: string(jfsConfig.MountModeDaemonSet),
				Attr: &jfsConfig.PodAttr{
					Image: "juicedata/mount:latest",
				},
			},
			existingDS: nil,
			expectRefs: 1,
		},
		{
			name: "add reference to existing daemonset",
			jfsSetting: &jfsConfig.JfsSetting{
				UniqueId:   "test-unique-id",
				TargetPath: "/var/lib/kubelet/pods/test-pod/volumes/test-volume2",
				VolumeId:   "test-volume",
				Name:       "test-name",
				MetaUrl:    "redis://localhost:6379/1",
				Source:     "test-source",
				HashVal:    "test-hash",
				PV: &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pv",
					},
				},
				MountMode: string(jfsConfig.MountModeDaemonSet),
				Attr: &jfsConfig.PodAttr{
					Image: "juicedata/mount:latest",
				},
			},
			existingDS: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "juicefs-test-unique-id-mount-ds",
					Namespace: "test-ns",
					Labels: map[string]string{
						common.PodJuiceHashLabelKey: "test-hash",
					},
					Annotations: map[string]string{
						util.GetReferenceKey("/existing/path"): "/existing/path",
					},
				},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.PodUniqueIdLabelKey: "test-unique-id",
						},
					},
				},
			},
			expectHash: "test-hash",
			expectRefs: 2,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake k8s client
			var objects []runtime.Object
			if tt.existingDS != nil {
				objects = append(objects, tt.existingDS)
			}
			
			fakeClient := fake.NewSimpleClientset(objects...)
			k8sClient := &k8sclient.K8sClient{}
			k8sClient.Interface = fakeClient
			
			// Create DaemonSetMount
			mounter := &k8sMount.FakeMounter{}
			d := NewDaemonSetMount(k8sClient, k8sMount.SafeFormatAndMount{
				Interface: mounter,
				Exec:      nil,
			}).(*DaemonSetMount)
			
			// Generate hash if not set
			if tt.jfsSetting.HashVal == "" {
				hashVal := jfsConfig.GenHashOfSetting(d.log, *tt.jfsSetting)
				tt.jfsSetting.HashVal = hashVal
			}
			
			// Call createOrUpdateDaemonSet
			dsName := d.genDaemonSetName(tt.jfsSetting)
			err := d.createOrUpdateDaemonSet(ctx, dsName, tt.jfsSetting)
			if err != nil {
				t.Errorf("createOrUpdateDaemonSet() error = %v", err)
				return
			}
			
			// Check DaemonSet was created/updated
			ds, err := k8sClient.GetDaemonSet(ctx, dsName, jfsConfig.Namespace)
			if err != nil {
				t.Errorf("GetDaemonSet() error = %v", err)
				return
			}
			
			// Check hash
			if tt.expectHash != "" && ds.Labels[common.PodJuiceHashLabelKey] != tt.expectHash {
				t.Errorf("DaemonSet hash = %v, want %v", ds.Labels[common.PodJuiceHashLabelKey], tt.expectHash)
			}
			
			// Debug: print annotations
			t.Logf("DaemonSet annotations: %v", ds.Annotations)
			
			// Count references
			refCount := 0
			referencePrefix := "juicefs-"
			t.Logf("Looking for prefix: %v", referencePrefix)
			for k := range ds.Annotations {
				if strings.HasPrefix(k, referencePrefix) {
					refCount++
					t.Logf("Found reference: %v", k)
				}
			}
			
			if refCount != tt.expectRefs {
				t.Errorf("Reference count = %v, want %v", refCount, tt.expectRefs)
			}
		})
	}
}

func TestDaemonSetMount_References(t *testing.T) {
	ctx := context.Background()
	jfsConfig.Namespace = "test-ns"
	
	// Create fake k8s client with a DaemonSet
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ds",
			Namespace: "test-ns",
			Annotations: map[string]string{
				util.GetReferenceKey("/path1"): "/path1",
			},
		},
	}
	
	fakeClient := fake.NewSimpleClientset(ds)
	k8sClient := &k8sclient.K8sClient{}
	k8sClient.Interface = fakeClient
	
	// Create DaemonSetMount
	mounter := &k8sMount.FakeMounter{}
	d := NewDaemonSetMount(k8sClient, k8sMount.SafeFormatAndMount{
		Interface: mounter,
		Exec:      nil,
	}).(*DaemonSetMount)
	
	// Test adding reference
	err := d.addReference(ctx, "test-ds", util.GetReferenceKey("/path2"), "/path2")
	if err != nil {
		t.Errorf("addReference() error = %v", err)
	}
	
	// Check reference was added
	refCount, err := d.GetMountRef(ctx, "/any", "test-ds")
	if err != nil {
		t.Errorf("GetMountRef() error = %v", err)
	}
	if refCount != 2 {
		t.Errorf("Reference count after add = %v, want 2", refCount)
	}
	
	// Test removing reference
	err = d.removeReference(ctx, "test-ds", util.GetReferenceKey("/path1"))
	if err != nil {
		t.Errorf("removeReference() error = %v", err)
	}
	
	// Check reference was removed
	refCount, err = d.GetMountRef(ctx, "/any", "test-ds")
	if err != nil {
		t.Errorf("GetMountRef() error = %v", err)
	}
	if refCount != 1 {
		t.Errorf("Reference count after remove = %v, want 1", refCount)
	}
}

func TestMountSelector_ConfigFallback(t *testing.T) {
	ctx := context.Background()
	jfsConfig.Namespace = "test-ns"
	
	tests := []struct {
		name             string
		configMap        *corev1.ConfigMap
		storageClassName string
		globalShareMount bool
		wantMode         jfsConfig.MountMode
	}{
		{
			name:             "no configmap, fallback to global per-pvc",
			configMap:        nil,
			storageClassName: "test-sc",
			globalShareMount: false,
			wantMode:         jfsConfig.MountModePVC,
		},
		{
			name:             "no configmap, fallback to global shared-pod",
			configMap:        nil,
			storageClassName: "test-sc",
			globalShareMount: true,
			wantMode:         jfsConfig.MountModeSharedPod,
		},
		{
			name: "invalid config in configmap, fallback to global",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jfsConfig.MountConfigMapName,
					Namespace: jfsConfig.Namespace,
				},
				Data: map[string]string{
					"test-sc": `mode: invalid-mode`,
				},
			},
			storageClassName: "test-sc",
			globalShareMount: true,
			wantMode:         jfsConfig.MountModeSharedPod,
		},
		{
			name: "configmap overrides global",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jfsConfig.MountConfigMapName,
					Namespace: jfsConfig.Namespace,
				},
				Data: map[string]string{
					"test-sc": `mode: per-pvc`,
				},
			},
			storageClassName: "test-sc",
			globalShareMount: true,
			wantMode:         jfsConfig.MountModePVC,
		},
		{
			name: "use default key when storage class not in configmap",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jfsConfig.MountConfigMapName,
					Namespace: jfsConfig.Namespace,
				},
				Data: map[string]string{
					jfsConfig.DefaultConfigKey: `mode: daemonset`,
				},
			},
			storageClassName: "unknown-sc",
			globalShareMount: false,
			wantMode:         jfsConfig.MountModeDaemonSet,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global variables
			jfsConfig.StorageClassShareMount = tt.globalShareMount
			
			// Create fake k8s client
			var objects []runtime.Object
			if tt.configMap != nil {
				objects = append(objects, tt.configMap)
			}
			
			fakeClient := fake.NewSimpleClientset(objects...)
			k8sClient := &k8sclient.K8sClient{}
			k8sClient.Interface = fakeClient
			
			// Create JfsSetting
			jfsSetting := &jfsConfig.JfsSetting{
				UniqueId: "test-id",
				VolumeId: "test-volume",
				PV: &corev1.PersistentVolume{
					Spec: corev1.PersistentVolumeSpec{
						StorageClassName: tt.storageClassName,
					},
				},
			}
			
			// Load mount config
			err := jfsConfig.LoadMountConfig(ctx, k8sClient, jfsSetting)
			if err != nil {
				t.Errorf("LoadMountConfig() error = %v", err)
				return
			}
			
			// Check mount mode
			actualMode := jfsConfig.MountMode(jfsSetting.MountMode)
			if actualMode == "" {
				// Determine from helper functions
				if jfsConfig.ShouldUseDaemonSet(jfsSetting) {
					actualMode = jfsConfig.MountModeDaemonSet
				} else if jfsConfig.ShouldUseSharedPod(jfsSetting) {
					actualMode = jfsConfig.MountModeSharedPod
				} else {
					actualMode = jfsConfig.MountModePVC
				}
			}
			
			if actualMode != tt.wantMode {
				t.Errorf("Mount mode = %v, want %v", actualMode, tt.wantMode)
			}
		})
	}
}