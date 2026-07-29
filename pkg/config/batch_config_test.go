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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
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
							Name: "test1",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test2",
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
				Batches: [][]MountPodUpgrade{
					{
						{
							Name:   "test1",
							Node:   "node1",
							Status: Pending,
						},
						{
							Name:   "test2",
							Node:   "node2",
							Status: Pending,
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
							Name: "test11",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test21",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node2"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test12",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test13",
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
				Batches: [][]MountPodUpgrade{
					{
						{
							Name:   "test11",
							Node:   "node1",
							Status: Pending,
						},
						{
							Name:   "test12",
							Node:   "node1",
							Status: Pending,
						},
					},
					{
						{
							Name:   "test13",
							Node:   "node1",
							Status: Pending,
						},
						{
							Name:   "test21",
							Node:   "node2",
							Status: Pending,
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
							Name: "test11",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test21",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId2",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test12",
							Annotations: map[string]string{
								common.UniqueId: "uniqueId1",
							},
						},
						Spec: corev1.PodSpec{NodeName: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test13",
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
				Batches: [][]MountPodUpgrade{
					{
						{
							Name:   "test11",
							Node:   "node1",
							Status: Pending,
						},
						{
							Name:   "test12",
							Node:   "node1",
							Status: Pending,
						},
					},
					{
						{
							Name:   "test13",
							Node:   "node1",
							Status: Pending,
						},
						{
							Name:   "test21",
							Node:   "node1",
							Status: Pending,
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
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := IsPodUpgradeOngoing(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
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
					Batches: [][]MountPodUpgrade{
						{
							{Name: "pod1", Status: Running},
							{Name: "pod2", Status: Running},
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
					Batches: [][]MountPodUpgrade{
						{
							{Name: "pod1", Status: Pending},
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
					Batches: [][]MountPodUpgrade{
						{
							{Name: "pod1", Status: Running},
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
					Batches: [][]MountPodUpgrade{
						{
							{Name: "pod1", Status: Success},
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
					Status: Running,
					Batches: [][]MountPodUpgrade{
						{
							{Name: "pod1", Status: Success},
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
					Status: Running,
					Batches: [][]MountPodUpgrade{
						{
							{Name: "", Status: Running},
							{Name: "pod1", Status: Running},
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
