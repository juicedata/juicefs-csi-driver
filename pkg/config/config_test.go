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

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func toPtr[T comparable](s T) *T {
	return &s
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configPath := "/tmp/test-config.yaml"
	defer os.Remove(configPath)

	// Write test data to the config file
	testData := []byte(`
CEMountImage: "juicedata/mount:ce-test"
EEMountImage: "juicedata/mount:ee-test"
MountPodPatch:
  - Labels:
      app: juicefs-mount
    Annotations:
      juicefs.com/finalizer: juicefs.com/finalizer
  - pvcSelector:
      matchLabels:
          app: juicefs-mount
    Labels:
      app: juicefs-mount
    Annotations:
      juicefs.com/finalizer: juicefs.com/finalizer
`)
	err := os.WriteFile(configPath, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Call the LoadConfig function
	err = LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
}

func TestGenMountPodPatch(t *testing.T) {
	testCases := []struct {
		name          string
		baseConfig    *Config
		expectedPatch MountPodPatch
		setting       JfsSetting
	}{
		{
			name:       "nil selector",
			baseConfig: &Config{},
			setting: JfsSetting{
				MountPath: "/var/lib/juicefs/volume",
			},
			expectedPatch: MountPodPatch{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "pvc with matched selector",
			setting: JfsSetting{
				MountPath: "/var/lib/juicefs/volume",
				PVC:       &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "juicefs-mount"}}},
			},
			baseConfig: &Config{
				MountPodPatch: []MountPodPatch{
					{
						PVCSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "juicefs-mount"},
						},
						Labels:      map[string]string{"app": "juicefs-labels"},
						Annotations: map[string]string{"app": "juicefs-annos"},
					},
				},
			},
			expectedPatch: MountPodPatch{
				Labels:      map[string]string{"app": "juicefs-labels"},
				Annotations: map[string]string{"app": "juicefs-annos"},
			},
		},
		{
			name: "pvc with unmatched selector",
			setting: JfsSetting{
				MountPath: "/var/lib/juicefs/volume",
				PVC:       &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "juicefs-mount-not-match-any-config"}}},
			},
			baseConfig: &Config{
				MountPodPatch: []MountPodPatch{
					{
						PVCSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "juicefs-mount"},
						},
						Labels:      map[string]string{"app": "juicefs-labels"},
						Annotations: map[string]string{"app": "juicefs-annos"},
					},
				},
			},
			expectedPatch: MountPodPatch{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "multi mount pod config",
			setting: JfsSetting{
				MountPath: "/var/lib/juicefs/volume",
				PVC:       &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "juicefs-mount"}}},
			},
			baseConfig: &Config{
				MountPodPatch: []MountPodPatch{
					{
						// apply base config
						Labels:      map[string]string{"app": "apply-base-labels"},
						Annotations: map[string]string{"app": "apply-base-annos"},
					},
					{
						// apply config with matched selector
						PVCSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "juicefs-mount"},
						},
						HostNetwork: toPtr(false),
					},
					{
						// overwrite annos with matched selector
						PVCSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "juicefs-mount"},
						},
						Annotations: map[string]string{"app": "overwrite-base-config"},
					},
					{
						// overwrite labels with un matched selector
						PVCSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "juicefs-mount-x"},
						},
						Labels: map[string]string{"app": "overwrite-base-config"},
					},
				},
			},
			expectedPatch: MountPodPatch{
				HostNetwork: toPtr(false),
				Labels:      map[string]string{"app": "apply-base-labels"},
				Annotations: map[string]string{"app": "overwrite-base-config"},
			},
		},
		{
			name: "parse template",
			setting: JfsSetting{
				MountPath: "/jfs/parse_test",
				SubPath:   "sub_path",
				VolumeId:  "dd",
				PVC:       &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "juicefs-mount"}}},
			},
			baseConfig: &Config{
				MountPodPatch: []MountPodPatch{
					{
						Lifecycle: &corev1.Lifecycle{
							PreStop: &corev1.Handler{
								Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "+e", "umount -l ${MOUNT_POINT}; rmdir ${MOUNT_POINT}; exit 0"}},
							},
						},
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "stat ${MOUNT_POINT}/${SUB_PATH}"}},
							},
						},
					},
					{
						PVCSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "juicefs-mount"},
						},
						HostNetwork: toPtr(false),
					},
				},
			},
			expectedPatch: MountPodPatch{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
				Lifecycle: &corev1.Lifecycle{
					PreStop: &corev1.Handler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "+e", "umount -l /jfs/parse_test; rmdir /jfs/parse_test; exit 0"}},
					},
				},
				LivenessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "stat /jfs/parse_test/sub_path"}},
					},
				},
				HostNetwork: toPtr(false),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualPatch := tc.baseConfig.GenMountPodPatch(tc.setting)
			assert.Equal(t, tc.expectedPatch, actualPatch)
		})
	}
}

func TestGenMountPodPatchParseTwice(t *testing.T) {
	baseConfig := &Config{
		MountPodPatch: []MountPodPatch{
			{
				Lifecycle: &corev1.Lifecycle{
					PreStop: &corev1.Handler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "+e", "umount -l ${MOUNT_POINT}; rmdir ${MOUNT_POINT}; exit 0"}},
					},
				},
			},
		},
	}

	setting := JfsSetting{
		MountPath: "",
	}

	expectedPatch1 := MountPodPatch{
		Labels:      map[string]string{},
		Annotations: map[string]string{},
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "+e", "umount -l ; rmdir ; exit 0"}},
			},
		},
	}

	actualPatch := baseConfig.GenMountPodPatch(setting)
	assert.Equal(t, expectedPatch1, actualPatch)

	expectedPatch2 := MountPodPatch{
		Labels:      map[string]string{},
		Annotations: map[string]string{},
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "+e", "umount -l /var/lib/juicefs/volume; rmdir /var/lib/juicefs/volume; exit 0"}},
			},
		},
	}
	setting.MountPath = "/var/lib/juicefs/volume"
	// Call the GenMountPodPatch function again
	actualPatch = baseConfig.GenMountPodPatch(setting)
	assert.Equal(t, expectedPatch2, actualPatch)
}
