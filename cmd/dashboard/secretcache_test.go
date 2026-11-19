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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
)

// TestSecretCacheByObjectUnscopedWithSecretManagement checks that, with Secret
// management on (the default), the manager's Secret cache is not restricted:
// Secret management lists, edits and deletes the Secrets users bring for
// StorageClasses and PVs, which carry no JuiceFS label.
func TestSecretCacheByObjectUnscopedWithSecretManagement(t *testing.T) {
	for obj := range secretCacheByObject(true) {
		if _, ok := obj.(*corev1.Secret); ok {
			t.Fatal("Secret cache is restricted while Secret management is on; user Secrets would disappear from the dashboard")
		}
	}
}

// TestSecretCacheByObjectScopedToJuiceFSSecrets checks that, with Secret
// management off, the manager's Secret cache is restricted to the per-volume
// Secrets the driver labels, instead of every Secret in the cluster.
func TestSecretCacheByObjectScopedToJuiceFSSecrets(t *testing.T) {
	byObject := secretCacheByObject(false)

	var secretConfig *struct{ label labels.Selector }
	found := false
	for obj, cfg := range byObject {
		if _, ok := obj.(*corev1.Secret); ok {
			found = true
			if cfg.Label == nil {
				t.Fatal("Secret cache config has no label selector; the cache would watch every Secret")
			}
			secretConfig = &struct{ label labels.Selector }{label: cfg.Label}
		}
	}
	if !found {
		t.Fatal("secretCacheByObject(false) does not scope *corev1.Secret")
	}

	// A JuiceFS per-volume Secret (labelled by the driver) is cached.
	if !secretConfig.label.Matches(labels.Set{common.JuicefsSecretLabelKey: "true"}) {
		t.Error("selector must match a Secret labelled juicefs/secret")
	}
	// An unrelated Secret (e.g. a ServiceAccount token or app credential) is not.
	if secretConfig.label.Matches(labels.Set{}) {
		t.Error("selector must not match a Secret without the juicefs/secret label")
	}
	if secretConfig.label.Matches(labels.Set{"app": "something-else"}) {
		t.Error("selector must not match an unrelated Secret")
	}
}
