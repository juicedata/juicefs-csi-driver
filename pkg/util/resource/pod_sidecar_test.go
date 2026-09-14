/*
 Copyright 2025 Juicedata Inc

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

package resource

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func TestParseSidecarBinaryUpgradeAnnotation_ValidJSON(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"juicefs.com/sidecar-binary-upgrade": `{
					"jfs-mount": {
						"image": "juicedata/mount:ce-v1.3.0",
						"upgradedAt": "2026-08-31T10:00:00Z"
					},
					"jfs-mount-1": {
						"image": "registry.example.com/juicefs:5.3.2",
						"upgradedAt": "2026-08-31T10:01:00Z"
					}
				}`,
			},
		},
	}

	result := config.ParseSidecarBinaryUpgradeAnnotation(pod)

	if len(result) != 2 {
		t.Errorf("expected 2 sidecars, got %d", len(result))
	}

	if info, ok := result["jfs-mount"]; ok {
		if info.Image != "juicedata/mount:ce-v1.3.0" {
			t.Errorf("expected image juicedata/mount:ce-v1.3.0, got %s", info.Image)
		}
	} else {
		t.Error("jfs-mount not found in result")
	}

	if info, ok := result["jfs-mount-1"]; ok {
		if info.Image != "registry.example.com/juicefs:5.3.2" {
			t.Errorf("expected image registry.example.com/juicefs:5.3.2, got %s", info.Image)
		}
	} else {
		t.Error("jfs-mount-1 not found in result")
	}
}

func TestParseSidecarBinaryUpgradeAnnotation_MissingAnnotation(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}

	result := config.ParseSidecarBinaryUpgradeAnnotation(pod)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestParseSidecarBinaryUpgradeAnnotation_CorruptJSON(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"juicefs.com/sidecar-binary-upgrade": "invalid json {",
			},
		},
	}

	result := config.ParseSidecarBinaryUpgradeAnnotation(pod)

	// Should return empty map for corrupt JSON (graceful degradation)
	if len(result) != 0 {
		t.Errorf("expected empty result for corrupt JSON, got %d items", len(result))
	}
}

func TestParseSidecarBinaryUpgradeAnnotation_EmptyImage(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"juicefs.com/sidecar-binary-upgrade": `{
					"jfs-mount": {
						"image": "",
						"upgradedAt": "2026-08-31T10:00:00Z"
					}
				}`,
			},
		},
	}

	result := config.ParseSidecarBinaryUpgradeAnnotation(pod)

	if len(result) != 1 {
		t.Errorf("expected 1 sidecar, got %d", len(result))
	}

	if info, ok := result["jfs-mount"]; ok {
		if info.Image != "" {
			t.Errorf("expected empty image, got %s", info.Image)
		}
	} else {
		t.Error("jfs-mount not found in result")
	}
}

func TestEffectiveSidecarImage_AnnotationPriority(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"juicefs.com/sidecar-binary-upgrade": `{
					"jfs-mount": {
						"image": "annotation-image:latest",
						"upgradedAt": "2026-08-31T10:00:00Z"
					}
				}`,
			},
		},
	}

	container := corev1.Container{
		Name:  "jfs-mount",
		Image: "spec-image:v1.0",
	}

	result := config.EffectiveSidecarImage(pod, container)

	if result != "annotation-image:latest" {
		t.Errorf("expected annotation-image:latest (from annotation), got %s", result)
	}
}

func TestEffectiveSidecarImage_FallbackToSpec(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}

	container := corev1.Container{
		Name:  "jfs-mount",
		Image: "spec-image:v1.0",
	}

	result := config.EffectiveSidecarImage(pod, container)

	if result != "spec-image:v1.0" {
		t.Errorf("expected spec-image:v1.0 (from spec), got %s", result)
	}
}

func TestEffectiveSidecarImage_EmptyAnnotationImage(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"juicefs.com/sidecar-binary-upgrade": `{
					"jfs-mount": {
						"image": "",
						"upgradedAt": "2026-08-31T10:00:00Z"
					}
				}`,
			},
		},
	}

	container := corev1.Container{
		Name:  "jfs-mount",
		Image: "spec-image:v1.0",
	}

	result := config.EffectiveSidecarImage(pod, container)

	// Empty annotation image should fall back to spec
	if result != "spec-image:v1.0" {
		t.Errorf("expected spec-image:v1.0 (fallback when annotation empty), got %s", result)
	}
}

func TestEffectiveSidecarImage_ContainerNotInAnnotation(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"juicefs.com/sidecar-binary-upgrade": `{
					"jfs-mount": {
						"image": "other:v1.0",
						"upgradedAt": "2026-08-31T10:00:00Z"
					}
				}`,
			},
		},
	}

	container := corev1.Container{
		Name:  "jfs-mount-1",
		Image: "spec-image:v1.0",
	}

	result := config.EffectiveSidecarImage(pod, container)

	// Container not in annotation should use spec
	if result != "spec-image:v1.0" {
		t.Errorf("expected spec-image:v1.0 (fallback when container not in annotation), got %s", result)
	}
}

func TestIsSidecarContainer_Traditional(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "traditional sidecar with done.sidecar.juicefs.com/inject=true",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"done.sidecar.juicefs.com/inject": "true",
					},
				},
			},
			expected: true,
		},
		{
			name: "pod without sidecar label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name: "pod with sidecar label = false",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"done.sidecar.juicefs.com/inject": "false",
					},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsPodWithSidecar(test.pod)
			if result != test.expected {
				t.Errorf("expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestIsSidecarContainerName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "jfs-mount", input: "jfs-mount", expected: true},
		{name: "jfs-mount-1", input: "jfs-mount-1", expected: true},
		{name: "jfs-mount-999", input: "jfs-mount-999", expected: true},
		{name: "other", input: "other", expected: false},
		{name: "jfs-mount-", input: "jfs-mount-", expected: false},
		{name: "jfs-mount-abc", input: "jfs-mount-abc", expected: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsSidecarContainerName(test.input)
			if result != test.expected {
				t.Errorf("expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestGetSidecarContainers_Traditional(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"done.sidecar.juicefs.com/inject": "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "jfs-mount", Image: "image1:v1"},
				{Name: "jfs-mount-1", Image: "image2:v1"},
				{Name: "app", Image: "app:v1"},
			},
		},
	}

	result := GetSidecarContainers(pod)

	if len(result) != 2 {
		t.Errorf("expected 2 sidecars, got %d", len(result))
	}

	names := make(map[string]bool)
	for _, c := range result {
		names[c.Name] = true
	}

	if !names["jfs-mount"] || !names["jfs-mount-1"] {
		t.Error("expected jfs-mount and jfs-mount-1")
	}
}

func TestGetSidecarContainers_Native(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"done.sidecar.juicefs.com/inject": "true",
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "jfs-mount", Image: "image1:v1"},
				{Name: "other-init", Image: "other:v1"},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}

	result := GetNativeSidecarContainers(pod)

	if len(result) != 1 {
		t.Errorf("expected 1 native sidecar, got %d", len(result))
	}

	if result[0].Name != "jfs-mount" {
		t.Errorf("expected jfs-mount, got %s", result[0].Name)
	}
}

func TestGetNativeSidecarContainers_NoRestartPolicyAlways(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"done.sidecar.juicefs.com/inject": "true",
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "jfs-mount", Image: "image1:v1"},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}

	result := GetNativeSidecarContainers(pod)

	// Should not return native sidecars if RestartPolicy is not Always
	if len(result) != 0 {
		t.Errorf("expected 0 native sidecars (no RestartPolicy=Always), got %d", len(result))
	}
}

func TestParseUpgradedAtTime(t *testing.T) {
	timeStr := "2026-08-31T10:00:00Z"
	info := &config.SidecarBinaryUpgradeInfo{
		Image: "image:v1",
		UpgradedAt: metav1.Time{
			Time: time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC),
		},
	}

	if info.UpgradedAt.Format(time.RFC3339) != timeStr {
		t.Errorf("expected %s, got %s", timeStr, info.UpgradedAt.Format(time.RFC3339))
	}
}
