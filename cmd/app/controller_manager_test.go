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

package app

import (
	"testing"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"k8s.io/apimachinery/pkg/labels"
)

func TestBuildPodCacheByObject_MountOnly(t *testing.T) {
	config.Namespace = "test-namespace"
	_, val, ok := buildPodCacheByObject(true, false)
	if !ok {
		t.Fatal("expected pod entry to be included for mount-only mode")
	}
	if val.Label != nil {
		t.Errorf("expected cluster-wide Label to be nil, got %v", val.Label)
	}
	if val.Namespaces == nil {
		t.Fatal("expected Namespaces to be set for mount-only mode")
	}
	nsCfg, exists := val.Namespaces[config.Namespace]
	if !exists {
		t.Fatalf("expected Namespaces to contain %q", config.Namespace)
	}
	want := labels.SelectorFromSet(labels.Set{common.PodTypeKey: common.PodTypeValue})
	if nsCfg.LabelSelector.String() != want.String() {
		t.Errorf("label selector = %q, want %q", nsCfg.LabelSelector.String(), want.String())
	}
}

func TestBuildPodCacheByObject_WebhookOnly(t *testing.T) {
	_, val, ok := buildPodCacheByObject(false, true)
	if !ok {
		t.Fatal("expected pod entry to be included for webhook-only mode")
	}
	if val.Namespaces != nil {
		t.Errorf("expected Namespaces to be nil, got %v", val.Namespaces)
	}
	want := labels.SelectorFromSet(labels.Set{common.InjectSidecarDone: common.True})
	if val.Label == nil {
		t.Fatal("expected Label to be set for webhook-only mode")
	}
	if val.Label.String() != want.String() {
		t.Errorf("label selector = %q, want %q", val.Label.String(), want.String())
	}
}

func TestBuildPodCacheByObject_Both(t *testing.T) {
	_, val, ok := buildPodCacheByObject(true, true)
	if !ok {
		t.Fatal("expected pod entry to be included when both modes active")
	}
	if val.Label != nil {
		t.Errorf("expected Label to be nil (unconstrained), got %v", val.Label)
	}
	if val.Namespaces != nil {
		t.Errorf("expected Namespaces to be nil (unconstrained), got %v", val.Namespaces)
	}
}

func TestBuildPodCacheByObject_Neither(t *testing.T) {
	key, _, ok := buildPodCacheByObject(false, false)
	if ok {
		t.Error("expected pod entry to be omitted when neither mode is active")
	}
	if key != nil {
		t.Errorf("expected key to be nil, got %v", key)
	}
}
