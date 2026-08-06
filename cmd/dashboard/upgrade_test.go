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

package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

func readyCSINodePod(name, namespace, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       corev1.PodSpec{NodeName: nodeName},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
			},
		},
	}
}

func TestPrecheckNode(t *testing.T) {
	const ns = "kube-system"

	tests := []struct {
		name    string
		objects []runtime.Object
		csiNode  string
		wantOK  bool
	}{
		{
			name: "schedulable node with ready csi node pod",
			objects: []runtime.Object{
				&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				readyCSINodePod("csi-node-1", ns, "node-1"),
			},
			csiNode: "csi-node-1",
			wantOK: true,
		},
		{
			name: "node marked SchedulingDisabled",
			objects: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
					Spec:       corev1.NodeSpec{Unschedulable: true},
				},
				readyCSINodePod("csi-node-1", ns, "node-1"),
			},
			csiNode: "csi-node-1",
			wantOK: false,
		},
		{
			name: "csi node pod not ready",
			objects: []runtime.Object{
				&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "csi-node-1", Namespace: ns},
					Spec:       corev1.PodSpec{NodeName: "node-1"},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionFalse},
						},
					},
				},
			},
			csiNode: "csi-node-1",
			wantOK: false,
		},
		{
			name: "node not found (api error) -> skip",
			objects: []runtime.Object{
				readyCSINodePod("csi-node-1", ns, "node-1"),
			},
			csiNode: "csi-node-1",
			wantOK: false,
		},
		{
			name: "csi node pod not found (api error) -> skip",
			objects: []runtime.Object{
				&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
			},
			csiNode: "csi-node-1",
			wantOK: false,
		},
		{
			name: "empty csi node pod name -> skip",
			objects: []runtime.Object{
				&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
			},
			csiNode: "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &k8sclient.K8sClient{Interface: fake.NewSimpleClientset(tt.objects...)}
			u := &BatchUpgrade{sysNamespace: ns, k8sClient: client}
			ok, reason := u.precheckNode(context.Background(), tt.csiNode)
			assert.Equal(t, tt.wantOK, ok)
			if !ok {
				assert.NotEmpty(t, reason)
			}
		})
	}
}

func TestGetBatchUpgradeTimeout(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv(batchUpgradeTimeoutEnv, "")
		timeout, err := getBatchUpgradeTimeout()
		assert.NoError(t, err)
		assert.Equal(t, defaultPodUpgradeTimeout, timeout)
	})

	t.Run("integer seconds", func(t *testing.T) {
		t.Setenv(batchUpgradeTimeoutEnv, "45")
		timeout, err := getBatchUpgradeTimeout()
		assert.NoError(t, err)
		assert.Equal(t, 45*time.Second, timeout)
	})

	t.Run("below minimum", func(t *testing.T) {
		t.Setenv(batchUpgradeTimeoutEnv, "4")
		_, err := getBatchUpgradeTimeout()
		assert.Error(t, err)
	})

	t.Run("at minimum", func(t *testing.T) {
		t.Setenv(batchUpgradeTimeoutEnv, "5")
		timeout, err := getBatchUpgradeTimeout()
		assert.NoError(t, err)
		assert.Equal(t, 5*time.Second, timeout)
	})

	t.Run("invalid integer", func(t *testing.T) {
		t.Setenv(batchUpgradeTimeoutEnv, "nope")
		_, err := getBatchUpgradeTimeout()
		assert.Error(t, err)
	})

	t.Run("duration string rejected", func(t *testing.T) {
		t.Setenv(batchUpgradeTimeoutEnv, "45s")
		_, err := getBatchUpgradeTimeout()
		assert.Error(t, err)
	})
}
