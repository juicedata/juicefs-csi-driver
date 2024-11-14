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
	"k8s.io/apimachinery/pkg/api/resource"
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
MountPodPatch:
  - ceMountImage: "juicedata/mount:ce-test"
    eeMountImage: "juicedata/mount:ee-test"
  - labels:
      app: juicefs-mount
    annotations:
      app: juicefs-mount
  - pvcSelector:
      matchLabels:
          app: juicefs-mount
      matchName: "test"
      matchStorageClassName: "juicefs-sc"
    labels:
      app: juicefs-mount
    annotations:
      juicefs.com/finalizer: juicefs.com/finalizer
  - resources:
      limits:
        cpu: 64
        memory: 128Gi
      requests:
        cpu: 32
        memory: 64Gi
  - livenessProbe:
      exec:
        command:
        - stat
        - ${MOUNT_POINT}/${SUB_PATH}
      failureThreshold: 3
      initialDelaySeconds: 10
      periodSeconds: 5
      successThreshold: 1
  - terminationGracePeriodSeconds: 60
  - env:
    - name: TEST_ENV
      value: "1"
  - volumeMounts:
    - name: test-volume
      mountPath: /test
    volumes:
    - name: test-volume
      hostPath:
        path: /tmp
  - volumeDevices:
    - name: block-devices
      devicePath: /dev/sda
    volumes:
    - name: block-devices
      persistentVolumeClaim:
        claimName: block-pvc
  - cacheDirs:
    - type: PVC
      name: cache-pvc
    - type: HostPath
      Path: /tmp
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
	defer GlobalConfig.Reset()
	// Check the loaded config
	assert.Equal(t, len(GlobalConfig.MountPodPatch), 10)
	assert.Equal(t, GlobalConfig.MountPodPatch[0], MountPodPatch{
		CEMountImage: "juicedata/mount:ce-test",
		EEMountImage: "juicedata/mount:ee-test",
	})
	assert.Equal(t, GlobalConfig.MountPodPatch[1], MountPodPatch{
		Labels: map[string]string{
			"app": "juicefs-mount",
		},
		Annotations: map[string]string{
			"app": "juicefs-mount",
		},
	})
	assert.Equal(t, GlobalConfig.MountPodPatch[2], MountPodPatch{
		PVCSelector: &PVCSelector{
			LabelSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "juicefs-mount"},
			},
			MatchStorageClassName: "juicefs-sc",
			MatchName:             "test",
		},
		Labels: map[string]string{
			"app": "juicefs-mount",
		},
		Annotations: map[string]string{
			"juicefs.com/finalizer": "juicefs.com/finalizer",
		},
	})
	assert.Equal(t, GlobalConfig.MountPodPatch[3], MountPodPatch{
		Resources: &corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("64"),
				corev1.ResourceMemory: resource.MustParse("128Gi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("32"),
				corev1.ResourceMemory: resource.MustParse("64Gi"),
			},
		},
	})
	assert.Equal(t, GlobalConfig.MountPodPatch[4], MountPodPatch{
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"stat", "${MOUNT_POINT}/${SUB_PATH}"},
				},
			},
			FailureThreshold:    3,
			InitialDelaySeconds: 10,
			PeriodSeconds:       5,
			SuccessThreshold:    1,
		},
	})
	assert.Equal(t, GlobalConfig.MountPodPatch[5], MountPodPatch{
		TerminationGracePeriodSeconds: toPtr(int64(60)),
	})
	assert.Equal(t, GlobalConfig.MountPodPatch[6], MountPodPatch{
		Env: []corev1.EnvVar{
			{
				Name:  "TEST_ENV",
				Value: "1",
			},
		},
	})
	assert.Equal(t, GlobalConfig.MountPodPatch[7], MountPodPatch{
		Volumes: []corev1.Volume{
			{
				Name: "test-volume",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/tmp",
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "test-volume",
				MountPath: "/test",
			},
		},
	})
	assert.Equal(t, GlobalConfig.MountPodPatch[8], MountPodPatch{
		Volumes: []corev1.Volume{
			{
				Name: "block-devices",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "block-pvc",
					},
				},
			},
		},
		VolumeDevices: []corev1.VolumeDevice{
			{
				Name:       "block-devices",
				DevicePath: "/dev/sda",
			},
		},
	})
	assert.Equal(t, GlobalConfig.MountPodPatch[9], MountPodPatch{
		CacheDirs: []MountPatchCacheDir{
			{
				Type: "PVC",
				Name: "cache-pvc",
			},
			{
				Type: "HostPath",
				Path: "/tmp",
			},
		},
	})
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
						PVCSelector: &PVCSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "juicefs-mount"},
							},
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
						PVCSelector: &PVCSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "juicefs-mount"},
							},
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
						PVCSelector: &PVCSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "juicefs-mount"},
							},
						},
						HostNetwork: toPtr(false),
					},
					{
						// overwrite annos with matched selector
						PVCSelector: &PVCSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "juicefs-mount"},
							},
						},
						Annotations: map[string]string{"app": "overwrite-base-config"},
					},
					{
						// overwrite labels with un matched selector
						PVCSelector: &PVCSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "juicefs-mount-x"},
							},
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
							PreStop: &corev1.LifecycleHandler{
								Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "+e", "umount -l ${MOUNT_POINT}; rmdir ${MOUNT_POINT}; exit 0"}},
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "stat ${MOUNT_POINT}/${SUB_PATH}"}},
							},
						},
					},
					{
						PVCSelector: &PVCSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "juicefs-mount"},
							},
						},
						HostNetwork: toPtr(false),
					},
				},
			},
			expectedPatch: MountPodPatch{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
				Lifecycle: &corev1.Lifecycle{
					PreStop: &corev1.LifecycleHandler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "+e", "umount -l /jfs/parse_test; rmdir /jfs/parse_test; exit 0"}},
					},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "stat /jfs/parse_test/sub_path"}},
					},
				},
				HostNetwork: toPtr(false),
			},
		},
		{
			name: "ignore some volumes",
			baseConfig: &Config{
				MountPodPatch: []MountPodPatch{
					{
						Volumes: []corev1.Volume{
							{
								Name: "volume-1",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/tmp",
									},
								},
							},
							{
								Name: "volume-1",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/tmp",
									},
								},
							},
							{
								Name: "cachedir-1",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/cache",
									},
								},
							},
						},
					},
				},
			},
			expectedPatch: MountPodPatch{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
				Volumes: []corev1.Volume{
					{
						Name: "volume-1",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/tmp",
							},
						},
					},
				},
			},
		},
		{
			name: "test mount options",
			baseConfig: &Config{
				MountPodPatch: []MountPodPatch{
					{
						MountOptions: []string{"rw", "nolock"},
					},
				},
			},
			expectedPatch: MountPodPatch{
				Labels:       map[string]string{},
				Annotations:  map[string]string{},
				MountOptions: []string{"rw", "nolock"},
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
					PreStop: &corev1.LifecycleHandler{
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
			PreStop: &corev1.LifecycleHandler{
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
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{Command: []string{"sh", "-c", "+e", "umount -l /var/lib/juicefs/volume; rmdir /var/lib/juicefs/volume; exit 0"}},
			},
		},
	}
	setting.MountPath = "/var/lib/juicefs/volume"
	// Call the GenMountPodPatch function again
	actualPatch = baseConfig.GenMountPodPatch(setting)
	assert.Equal(t, expectedPatch2, actualPatch)
}

func TestMountPodPatch_isMatch(t *testing.T) {
	testCases := []struct {
		name     string
		patch    MountPodPatch
		pvc      *corev1.PersistentVolumeClaim
		expected bool
	}{
		{
			name:     "No PVC Selector",
			patch:    MountPodPatch{},
			expected: true,
		},
		{
			name: "Match PVC Labels",
			patch: MountPodPatch{
				PVCSelector: &PVCSelector{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "juicefs-mount"},
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "juicefs-mount"},
				},
			},
			expected: true,
		},
		{
			name: "Mismatch PVC Labels",
			patch: MountPodPatch{
				PVCSelector: &PVCSelector{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "juicefs-mount"},
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "wrong-label"},
				},
			},
			expected: false,
		},
		{
			name: "Match PVC Name",
			patch: MountPodPatch{
				PVCSelector: &PVCSelector{
					MatchName: "pvc-name",
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-name",
				},
			},
			expected: true,
		},
		{
			name: "Mismatch PVC Name",
			patch: MountPodPatch{
				PVCSelector: &PVCSelector{
					MatchName: "wrong-pvc",
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-name",
				},
			},
			expected: false,
		},
		{
			name: "Match Storage Class Name",
			patch: MountPodPatch{
				PVCSelector: &PVCSelector{
					MatchStorageClassName: "juicefs-sc",
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: toPtr("juicefs-sc"),
				},
			},
			expected: true,
		},
		{
			name: "Mismatch Storage Class Name",
			patch: MountPodPatch{
				PVCSelector: &PVCSelector{
					MatchStorageClassName: "wrong-sc",
				},
			},
			expected: false,
		},
		{
			name: "Mismatch Storage Class Name with pvc nil sc",
			patch: MountPodPatch{
				PVCSelector: &PVCSelector{
					MatchStorageClassName: "wrong-sc",
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"what": "ever"},
				},
			},
			expected: false,
		},
		{
			name: "Mismatch Storage Class Name with pvc empty sc",
			patch: MountPodPatch{
				PVCSelector: &PVCSelector{
					MatchStorageClassName: "wrong-sc",
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"what": "ever"},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: toPtr(""),
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.patch.isMatch(tc.pvc)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
