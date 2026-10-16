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

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func nodeNameTerm(nodes ...string) corev1.NodeSelectorTerm {
	return corev1.NodeSelectorTerm{MatchFields: []corev1.NodeSelectorRequirement{{
		Key: metav1.ObjectNameField, Operator: corev1.NodeSelectorOpIn, Values: nodes,
	}}}
}

func requiredAffinity(terms ...corev1.NodeSelectorTerm) *corev1.Affinity {
	return &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: terms},
	}}
}

func TestPinnedNodeName(t *testing.T) {
	poolTerm := corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{{
		Key: "pool", Operator: corev1.NodeSelectorOpIn, Values: []string{"gpu"},
	}}}
	tests := []struct {
		name string
		spec corev1.PodSpec
		want string
	}{
		{name: "unpinned", spec: corev1.PodSpec{}, want: ""},
		{name: "nodeName", spec: corev1.PodSpec{NodeName: "node-a"}, want: "node-a"},
		// The shape jobs copied from a CSI node pod before the controller refreshed it.
		{name: "DaemonSet pin", spec: corev1.PodSpec{Affinity: requiredAffinity(nodeNameTerm("node-a"))}, want: "node-a"},
		{name: "expression only", spec: corev1.PodSpec{Affinity: requiredAffinity(poolTerm)}, want: ""},
		{name: "ORed with an unpinned term", spec: corev1.PodSpec{Affinity: requiredAffinity(nodeNameTerm("node-a"), poolTerm)}, want: ""},
		{name: "several nodes allowed", spec: corev1.PodSpec{Affinity: requiredAffinity(nodeNameTerm("node-a", "node-b"))}, want: ""},
		{name: "terms pin different nodes", spec: corev1.PodSpec{Affinity: requiredAffinity(nodeNameTerm("node-a"), nodeNameTerm("node-b"))}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pinnedNodeName(&tt.spec))
		})
	}
}
