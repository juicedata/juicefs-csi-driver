/*
 Copyright 2026 Juicedata Inc

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

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

const testNamespace = "kube-system"

var templateAffinity = &corev1.Affinity{
	NodeAffinity: &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{{
				MatchExpressions: []corev1.NodeSelectorRequirement{{
					Key: "pool", Operator: corev1.NodeSelectorOpIn, Values: []string{"gpu"},
				}},
			}},
		},
	},
}

// pinnedAffinity is what the DaemonSet controller puts on a pod scheduled to node-a.
func pinnedAffinity(node string) *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key: "pool", Operator: corev1.NodeSelectorOpIn, Values: []string{"gpu"},
					}},
					MatchFields: []corev1.NodeSelectorRequirement{{
						Key: metav1.ObjectNameField, Operator: corev1.NodeSelectorOpIn, Values: []string{node},
					}},
				}},
			},
		},
	}
}

func csiNodePod(node string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "juicefs-csi-node-" + node, Namespace: testNamespace,
			Labels: map[string]string{"app": "juicefs-csi-node"},
		},
		Spec: corev1.PodSpec{NodeName: node, Affinity: pinnedAffinity(node)},
	}
}

func TestGetCSINodePodTemplate(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "juicefs-csi-node", Namespace: testNamespace},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Affinity: templateAffinity}},
		},
	}
	tests := []struct {
		name         string
		objects      []runtime.Object
		wantAffinity *corev1.Affinity
		wantErr      bool
	}{
		{
			name:         "prefers DaemonSet template over pinned pod",
			objects:      []runtime.Object{ds, csiNodePod("node-a")},
			wantAffinity: templateAffinity,
		},
		{
			name:         "falls back to pod with node pin removed",
			objects:      []runtime.Object{csiNodePod("node-a")},
			wantAffinity: templateAffinity,
		},
		{
			name:    "fails without DaemonSet or pods",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &k8s.K8sClient{Interface: fake.NewSimpleClientset(tt.objects...)}
			got, err := getCSINodePodTemplate(context.TODO(), client, testNamespace)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Empty(t, got.Spec.NodeName)
			assert.Equal(t, tt.wantAffinity, got.Spec.Affinity)
		})
	}
}

func TestRemoveNodeNamePin(t *testing.T) {
	onlyPin := pinnedAffinity("node-a")
	onlyPin.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions = nil
	tests := []struct {
		name     string
		affinity *corev1.Affinity
		want     *corev1.Affinity
	}{
		{name: "nil", affinity: nil, want: nil},
		{name: "keeps other requirements", affinity: pinnedAffinity("node-a"), want: templateAffinity},
		{name: "drops required selector left empty", affinity: onlyPin, want: &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, removeNodeNamePin(tt.affinity))
		})
	}
}
