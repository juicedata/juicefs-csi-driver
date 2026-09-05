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
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

func TestNewBatchConfig(t *testing.T) {
	type args struct {
		pods        []corev1.Pod
		parallel    int
		ignoreError bool
		recreate    bool
		nodeName    string
		uniqueId    string
		csiNodes    []corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want *BatchConfig
	}{
		{
			name: "normal test",
			args: args{
				recreate: true,
				pods: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test1",
							Namespace: "default",
							UID:       "uid-1",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test2",
							Namespace: "default",
							UID:       "uid-2",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node2"},
					},
				},
				parallel: 2,
			},
			want: &BatchConfig{
				Parallel: 2,
				Kind:     UpgradeKindMountPod,
				Batches: [][]UpgradeTarget{
					{
						{
							Namespace: "default",
							Name:      "test1",
							Node:      "node1",
						},
						{
							Namespace: "default",
							Name:      "test2",
							Node:      "node2",
						},
					},
				},
			},
		},
		{
			name: "different nodes",
			args: args{
				recreate: true,
				pods: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test11",
							Namespace: "default",
							UID:       "uid-11",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test21",
							Namespace: "default",
							UID:       "uid-21",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node2"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test12",
							Namespace: "default",
							UID:       "uid-12",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test13",
							Namespace: "default",
							UID:       "uid-13",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
				},
				parallel: 2,
			},
			want: &BatchConfig{
				Parallel: 2,
				Kind:     UpgradeKindMountPod,
				Batches: [][]UpgradeTarget{
					{
						{
							Namespace: "default",
							Name:      "test11",
							Node:      "node1",
						},
						{
							Namespace: "default",
							Name:      "test12",
							Node:      "node1",
						},
					},
					{
						{
							Namespace: "default",
							Name:      "test13",
							Node:      "node1",
						},
						{
							Namespace: "default",
							Name:      "test21",
							Node:      "node2",
						},
					},
				},
			},
		},
		{
			name: "different uniqueIds",
			args: args{
				recreate: true,
				pods: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test11",
							Namespace: "default",
							UID:       "uid-11",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test21",
							Namespace: "default",
							UID:       "uid-21",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId2",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test12",
							Namespace: "default",
							UID:       "uid-12",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test13",
							Namespace: "default",
							UID:       "uid-13",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
				},
				parallel: 2,
			},
			want: &BatchConfig{
				Parallel: 2,
				Kind:     UpgradeKindMountPod,
				Batches: [][]UpgradeTarget{
					{
						{
							Namespace: "default",
							Name:      "test11",
							Node:      "node1",
						},
						{
							Namespace: "default",
							Name:      "test12",
							Node:      "node1",
						},
					},
					{
						{
							Namespace: "default",
							Name:      "test13",
							Node:      "node1",
						},
						{
							Namespace: "default",
							Name:      "test21",
							Node:      "node1",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBatchConfig(tt.args.pods, tt.args.parallel, tt.args.ignoreError, tt.args.recreate, tt.args.nodeName, tt.args.uniqueId, tt.args.csiNodes)
			assert.Equalf(t, tt.want, got, "NewBatchConfig(%v, %v, %v, %v, %v, %v, %v)", tt.args.pods, tt.args.parallel, tt.args.ignoreError, tt.args.recreate, tt.args.nodeName, tt.args.uniqueId, tt.args.csiNodes)
		})
	}
}

func TestGetDiffWithNodeRespectsNodeSelector(t *testing.T) {
	saved := *GlobalConfig
	defer func() {
		*GlobalConfig = saved
	}()

	GlobalConfig.MountPodPatch = []MountPodPatch{
		{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"topology.kubernetes.io/zone": "us-west-1"},
			},
			Labels: map[string]string{"patch": "matched-node"},
		},
	}

	mountPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mount-pod",
			Namespace: "default",
			Labels: map[string]string{
				common.PodUniqueIdLabelKey:  "unique-id",
				common.PodJuiceHashLabelKey: "old-hash",
			},
			Annotations: map[string]string{
				common.UniqueId:    "unique-id",
				common.JuiceFSUUID: "test-fs",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "node-a",
			Containers: []corev1.Container{
				{
					Name:    "jfs-mount",
					Image:   "juicedata/mount:ee-nightly",
					Command: []string{"sh", "-c", "exec /sbin/mount.juicefs test /jfs/unique-id -o foreground,no-update"},
				},
			},
		},
	}

	t.Run("matched node", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-a",
				Labels: map[string]string{"topology.kubernetes.io/zone": "us-west-1"},
			},
		}

		oldSetting, newSetting, err := GetDiffWithNode(mountPod, nil, nil, nil, nil, node)
		assert.NoError(t, err)
		assert.Empty(t, oldSetting.Attr.Labels)
		assert.Equal(t, "matched-node", newSetting.Attr.Labels["patch"])
	})

	t.Run("unmatched node", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-a",
				Labels: map[string]string{"topology.kubernetes.io/zone": "us-east-1"},
			},
		}

		_, newSetting, err := GetDiffWithNode(mountPod, nil, nil, nil, nil, node)
		assert.NoError(t, err)
		assert.Empty(t, newSetting.Attr.Labels)
	})
}

func TestIsPodUpgradeOngoing(t *testing.T) {
	tests := []struct {
		status   UpgradeStatus
		expected bool
	}{
		{Pending, true},
		{Running, true},
		{Pause, true},
		{Success, false},
		{Fail, false},
		{Stop, false},
		{Skip, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := IsPodUpgradeOngoing(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterTargetsNotInOngoingUpgrade(t *testing.T) {
	originalNamespace := Namespace
	Namespace = "kube-system"
	t.Cleanup(func() {
		Namespace = originalNamespace
	})

	runningConfig := &BatchConfig{
		Parallel:  1,
		Kind:      UpgradeKindSidecar,
		Namespace: "default",
		Batches: [][]UpgradeTarget{
			{
				{
					Namespace:     "default",
					Name:          "app-pod-1",
					ContainerName: "jfs-mount",
				},
			},
		},
	}
	data, err := json.Marshal(runningConfig)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sidecar-job-config",
			Namespace: Namespace,
			Labels: map[string]string{
				common.PodTypeKey: common.ConfigTypeValue,
			},
		},
		Data: map[string]string{"upgrade": string(data)},
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sidecar-job",
			Namespace: Namespace,
			Labels: map[string]string{
				common.JfsJobKind:       common.KindOfUpgrade,
				common.JfsUpgradeConfig: cm.Name,
			},
		},
		Status: batchv1.JobStatus{},
	}

	client := &k8sclient.K8sClient{
		Interface: fake.NewSimpleClientset(cm, job),
	}

	targets := []UpgradeTarget{
		{
			Namespace:     "default",
			Name:          "app-pod-1",
			ContainerName: "jfs-mount",
		},
		{
			Namespace:     "default",
			Name:          "app-pod-2",
			ContainerName: "jfs-mount",
		},
	}

	filtered, skipped, err := FilterTargetsNotInOngoingUpgrade(context.Background(), client, targets)
	assert.NoError(t, err)
	if assert.Len(t, filtered, 1) {
		assert.Equal(t, "app-pod-2/jfs-mount", filtered[0].Key())
	}
	if assert.Len(t, skipped, 1) {
		assert.Equal(t, "app-pod-1/jfs-mount", skipped[0].Key())
	}
}

func TestUpgradeTargetKey(t *testing.T) {
	tests := []struct {
		name     string
		target   *UpgradeTarget
		expected string
	}{
		{
			name: "mount pod without container name",
			target: &UpgradeTarget{
				Name:          "pod-12345",
				ContainerName: "",
			},
			expected: "pod-12345",
		},
		{
			name: "sidecar with container name",
			target: &UpgradeTarget{
				Name:          "pod-67890",
				ContainerName: "juicefs-sidecar",
			},
			expected: "pod-67890/juicefs-sidecar",
		},
		{
			name: "mount pod with explicit container name",
			target: &UpgradeTarget{
				Name:          "pod-abcde",
				ContainerName: "jfs-mount",
			},
			expected: "pod-abcde/jfs-mount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.target.Key()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSelectSidecarUpgradeTargets(t *testing.T) {
	oldConfig := GlobalConfig
	GlobalConfig = &Config{MountPodPatch: []MountPodPatch{{
		CEMountImage: "juicedata/mount:v2",
		EEMountImage: "juicedata/mount:v2",
	}}}
	t.Cleanup(func() { GlobalConfig = oldConfig })

	now := metav1.Now()
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-pod-ready",
				Namespace: "default",
				Labels:    map[string]string{common.InjectSidecarDone: common.True},
			},
			Spec: corev1.PodSpec{
				NodeName: "node-a",
				Containers: []corev1.Container{
					{Name: "jfs-mount", Image: "juicedata/mount:v1", VolumeMounts: []corev1.VolumeMount{{Name: "jfs-scripts", MountPath: "/jfs-scripts"}}},
					{Name: "jfs-mount-1", Image: "juicedata/mount:v2", VolumeMounts: []corev1.VolumeMount{{Name: "jfs-scripts", MountPath: "/jfs-scripts"}}},
					{Name: "jfs-mount-2", Image: "juicedata/mount:v1", VolumeMounts: []corev1.VolumeMount{{Name: "jfs-scripts", MountPath: "/jfs-scripts"}}},
				},
			},
			Status: corev1.PodStatus{Conditions: []corev1.PodCondition{
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-pod-not-ready",
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				NodeName:   "node-b",
				Containers: []corev1.Container{{Name: "jfs-mount"}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "app-pod-terminating",
				Namespace:         "default",
				DeletionTimestamp: &now,
			},
			Spec: corev1.PodSpec{
				NodeName:   "node-c",
				Containers: []corev1.Container{{Name: "jfs-mount"}},
			},
		},
	}

	secretMap := map[types.NamespacedName]corev1.Secret{
		{Name: "jfs-secret", Namespace: "default"}: {
			ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{Kind: "PersistentVolumeClaim", Name: "pvc-1"}}},
			Data:       map[string][]byte{"jfsSettings": []byte(`{"IsCe":true}`)},
		},
	}
	for i := range pods[0].Spec.Containers {
		pods[0].Spec.Containers[i].VolumeMounts = []corev1.VolumeMount{{Name: "jfs-scripts", MountPath: "/jfs-scripts"}}
	}
	pods[0].Spec.Volumes = []corev1.Volume{{Name: "jfs-scripts", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "jfs-secret"}}}}
	eligible, skipped, err := SelectSidecarUpgradeTargets(pods, map[string]corev1.PersistentVolumeClaim{"pvc-1": {}}, secretMap)
	assert.NoError(t, err)
	if assert.Len(t, eligible, 2) {
		assert.Equal(t, "app-pod-ready/jfs-mount", eligible[0].Key())
	}
	if assert.Len(t, skipped, 1) {
		assert.Equal(t, "app-pod-ready/jfs-mount-1", skipped[0].Key())
	}
}

func TestLoadLegacyMountPodBatchConfig(t *testing.T) {
	cm := &corev1.ConfigMap{
		Data: map[string]string{
			"upgrade": `{
				"parallel": 2,
				"ignoreError": false,
				"norecreate": false,
				"node": "",
				"uniqueId": "",
				"batches": [
					[
						{
							"name": "mount-pod-1",
							"node": "node-1",
							"csiNodePod": "csi-node-1",
							"status": "pending"
						},
						{
							"name": "mount-pod-2",
							"node": "node-2",
							"csiNodePod": "csi-node-2",
							"status": "pending"
						}
					]
				],
				"status": "pending"
			}`,
		},
	}

	cfg, err := LoadBatchConfig(cm)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 2, cfg.Parallel)
	assert.Equal(t, 1, len(cfg.Batches))
	assert.Equal(t, 2, len(cfg.Batches[0]))
	assert.Equal(t, "mount-pod-1", cfg.Batches[0][0].Name)
	assert.Equal(t, "mount-pod-2", cfg.Batches[0][1].Name)
	assert.Equal(t, Pending, cfg.Batches[0][0].Status)
	assert.Equal(t, Pending, cfg.Batches[0][1].Status)
}

func TestUpgradeTarget_Unmarshal(t *testing.T) {
	var target UpgradeTarget
	err := json.Unmarshal([]byte(`{
		"name": "mount-pod-1",
		"node": "node-1",
		"csiNodePod": "csi-node-1",
		"status": "pending"
	}`), &target)
	assert.NoError(t, err)
	assert.Equal(t, "mount-pod-1", target.Name)
}

func TestNewBatchConfig_MissingCSINode(t *testing.T) {
	// Test that missing CSI node is handled gracefully
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				UID:       "uid-1",
			},
			Spec: corev1.PodSpec{NodeName: "node-that-does-not-exist"},
		},
	}

	csiNodes := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-node-1",
			},
			Spec: corev1.PodSpec{NodeName: "node-1"},
		},
	}

	// Should handle missing CSI node gracefully
	cfg := NewBatchConfig(pods, 1, false, true, "", "", csiNodes)
	assert.NotNil(t, cfg)
	assert.Equal(t, 1, len(cfg.Batches))
	assert.Equal(t, 1, len(cfg.Batches[0]))
	// CSINodePod should be empty or have a default value, not panic
	assert.Equal(t, "", cfg.Batches[0][0].CSINodePod)
}

func TestPodList_SortWithNilAnnotations(t *testing.T) {
	// Test that sorting works even with nil annotations
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod1",
				// No Annotations
			},
			Spec: corev1.PodSpec{NodeName: "node1"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod2",
				Annotations: map[string]string{
					common.UniqueId: "uniqueId2",
				},
			},
			Spec: corev1.PodSpec{NodeName: "node1"},
		},
	}

	// Should not panic when sorting
	sort.Sort(podList(pods))
	assert.Equal(t, 2, len(pods))
}

func TestUnmarshalJSON_MalformedBatch(t *testing.T) {
	// Test that malformed batch data is handled
	cm := &corev1.ConfigMap{
		Data: map[string]string{
			"upgrade": `{
				"parallel": 1,
				"ignoreError": false,
				"batches": [
					"not-an-array"
				],
				"status": "pending"
			}`,
		},
	}

	cfg, err := LoadBatchConfig(cm)
	// Should not crash, but handle gracefully
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestUpgradeTarget_KeyUsesPodName(t *testing.T) {
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				UID:       "uid-123",
			},
			Spec: corev1.PodSpec{NodeName: "node1"},
		},
	}

	csiNodes := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csi-node-1",
			},
			Spec: corev1.PodSpec{NodeName: "node1"},
		},
	}

	cfg := NewBatchConfig(pods, 1, false, true, "", "", csiNodes)
	target := cfg.Batches[0][0]

	// Key should work correctly
	assert.Equal(t, "test-pod", target.Key(), "Key should return Name when ContainerName is empty")
}

func TestFilterPodsFromConfigs(t *testing.T) {
	tests := []struct {
		name         string
		configs      map[string]*BatchConfig
		expectedPods []string
	}{
		{
			name:         "empty configs returns empty",
			configs:      map[string]*BatchConfig{},
			expectedPods: []string{},
		},
		{
			name: "running config filters matching pods",
			configs: map[string]*BatchConfig{
				"config1": {
					Status: Running,
					Batches: [][]UpgradeTarget{
						{
							{Name: "pod1"},
							{Name: "pod2"},
						},
					},
				},
			},
			expectedPods: []string{"pod1", "pod2"},
		},
		{
			name: "pending config filters matching pods",
			configs: map[string]*BatchConfig{
				"config1": {
					Status: Pending,
					Batches: [][]UpgradeTarget{
						{
							{Name: "pod1"},
						},
					},
				},
			},
			expectedPods: []string{"pod1"},
		},
		{
			name: "pause config filters matching pods",
			configs: map[string]*BatchConfig{
				"config1": {
					Status: Pause,
					Batches: [][]UpgradeTarget{
						{
							{Name: "pod1"},
						},
					},
				},
			},
			expectedPods: []string{"pod1"},
		},
		{
			name: "success config does not filter pods",
			configs: map[string]*BatchConfig{
				"config1": {
					Status: Success,
					Batches: [][]UpgradeTarget{
						{
							{Name: "pod1"},
						},
					},
				},
			},
			expectedPods: []string{},
		},
		{
			name: "pod status success in running config does not filter",
			configs: map[string]*BatchConfig{
				"config1": {
					Status: Success,
					Batches: [][]UpgradeTarget{
						{
							{Name: "pod1"},
						},
					},
				},
			},
			expectedPods: []string{},
		},
		{
			name: "empty pod name in config is ignored",
			configs: map[string]*BatchConfig{
				"config1": {
					Batches: [][]UpgradeTarget{
						{
							{Name: ""},
							{Name: "pod1"},
						},
					},
				},
			},
			expectedPods: []string{"pod1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterPodsFromConfigs(tt.configs)
			resultNames := make([]string, 0, len(result))
			for name := range result {
				resultNames = append(resultNames, name)
			}
			sort.Strings(resultNames)
			sort.Strings(tt.expectedPods)
			assert.Equal(t, tt.expectedPods, resultNames)
		})
	}
}
