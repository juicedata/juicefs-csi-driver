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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newNode() *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-node-1",
			Labels: map[string]string{
				"topology.kubernetes.io/zone": "cn-north-1a",
			},
			Annotations: map[string]string{
				"storage.example.com/cache-dirs": "/data0:/data1",
			},
		},
		Spec: v1.NodeSpec{
			PodCIDR: "10.244.0.0/24",
		},
	}
}

func TestExpandNodeTemplates(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		expected string
	}{
		{
			name:     "no template stays unchanged",
			in:       "cache-dir=/data0:/data1",
			expected: "cache-dir=/data0:/data1",
		},
		{
			name:     "node name",
			in:       "cache-group=${.node.name}",
			expected: "cache-group=gpu-node-1",
		},
		{
			name:     "podCIDR",
			in:       "something-${.node.podCIDR}",
			expected: "something-10.244.0.0/24",
		},
		{
			name:     "node label",
			in:       "cache-group=${.node.labels.topology.kubernetes.io/zone}",
			expected: "cache-group=cn-north-1a",
		},
		{
			name:     "node annotation",
			in:       "cache-dir=${.node.annotations.storage.example.com/cache-dirs}",
			expected: "cache-dir=/data0:/data1",
		},
		{
			name:     "missing key resolves to empty string",
			in:       "cache-dir=${.node.annotations.no.such/key}",
			expected: "cache-dir=",
		},
		{
			name:     "unknown field left untouched",
			in:       "cache-dir=${.node.spec.foo}",
			expected: "cache-dir=${.node.spec.foo}",
		},
		{
			name:     "multiple templates in one option",
			in:       "cache-dir=${.node.annotations.storage.example.com/cache-dirs},cache-group=${.node.name}",
			expected: "cache-dir=/data0:/data1,cache-group=gpu-node-1",
		},
		{
			name:     "pvc templates are not in scope",
			in:       "cache-dir=${.pvc.annotations.foo}",
			expected: "cache-dir=${.pvc.annotations.foo}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ExpandNodeTemplates(tt.in, newNode()))
		})
	}
}

func TestExpandNodeTemplatesNilNode(t *testing.T) {
	in := "cache-dir=${.node.annotations.storage.example.com/cache-dirs}"
	assert.Equal(t, in, ExpandNodeTemplates(in, nil))
}
