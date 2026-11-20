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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestParseSettingWithNodeExpandsTemplates verifies that ${.node.*} templates
// in mount options are resolved against the node running the CSI node plugin
// before options are validated and hashed, so node-dependent options like
// cache-dir (hostPath cache volumes are generated from the literal value in
// GenCacheDirs) adapt to the actual machine.
func TestParseSettingWithNodeExpandsTemplates(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-node-1",
			Annotations: map[string]string{
				"storage.example.com/cache-dirs": "/data0:/data1",
			},
		},
	}

	setting, err := ParseSettingWithNode(nil,
		map[string]string{"name": "test"},
		nil,
		[]string{
			"cache-dir=${.node.annotations.storage.example.com/cache-dirs}",
			"buffer-size=1024",
		},
		"pv-name", "unique-id", "test-uuid",
		nil, nil, node,
	)
	assert.NoError(t, err)
	assert.Contains(t, setting.Options, "cache-dir=/data0:/data1")
	// CacheDirs must reflect the resolved value, not the raw template.
	assert.Equal(t, []string{"/data0", "/data1"}, setting.CacheDirs)
}

// TestParseSettingWithNodeNilNodeKeepsTemplate verifies that without a node
// object (e.g. controller-side parsing) templates are left untouched instead
// of being resolved against wrong data.
func TestParseSettingWithNodeNilNodeKeepsTemplate(t *testing.T) {
	setting, err := ParseSettingWithNode(nil,
		map[string]string{"name": "test"},
		nil,
		[]string{"cache-dir=${.node.annotations.storage.example.com/cache-dirs}"},
		"pv-name", "unique-id", "test-uuid",
		nil, nil, nil,
	)
	assert.NoError(t, err)
	assert.Contains(t, setting.Options, "cache-dir=${.node.annotations.storage.example.com/cache-dirs}")
}
