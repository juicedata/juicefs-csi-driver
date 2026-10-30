/**
 * Copyright 2024 Juicedata Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	juicefsiov1 "github.com/juicedata/juicefs-operator/api/v1"
	operatorcommon "github.com/juicedata/juicefs-operator/pkg/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFilterSortAndPaginateCacheGroupWorkers(t *testing.T) {
	worker := func(name, node string, createdAt int64) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				CreationTimestamp: metav1.NewTime(time.Unix(createdAt, 0)),
			},
			Spec: corev1.PodSpec{NodeName: node},
		}
	}
	withConditions := func(pod corev1.Pod, ready, containersReady corev1.ConditionStatus) corev1.Pod {
		pod.Status.Conditions = []corev1.PodCondition{
			{Type: corev1.PodReady, Status: ready},
			{Type: corev1.ContainersReady, Status: containersReady},
		}
		return pod
	}
	readyWorkers := []corev1.Pod{
		withConditions(worker("worker-ready", "node-a", 10), corev1.ConditionTrue, corev1.ConditionTrue),
		withConditions(worker("worker-containers-not-ready", "node-b", 20), corev1.ConditionTrue, corev1.ConditionFalse),
		withConditions(worker("worker-pending", "node-c", 30), corev1.ConditionFalse, corev1.ConditionFalse),
		worker("worker-no-conditions", "node-d", 40),
	}
	workers := []corev1.Pod{
		worker("worker-c", "node-a", 30),
		worker("worker-a", "node-c", 10),
		worker("worker-b", "node-b", 20),
		worker("worker-d", "node-b", 20),
	}

	tests := []struct {
		name                   string
		workers                []corev1.Pod
		nameFilter, nodeFilter string
		sortBy, order          string
		unready                bool
		current, pageSize      uint64
		wantNames              []string
		wantTotal              int
	}{
		{name: "default time ascending", workers: workers, current: 1, pageSize: 10, wantNames: []string{"worker-a", "worker-b", "worker-d", "worker-c"}, wantTotal: 4},
		{name: "name ascending", workers: workers, sortBy: "name", order: "ascend", current: 1, pageSize: 10, wantNames: []string{"worker-a", "worker-b", "worker-c", "worker-d"}, wantTotal: 4},
		{name: "name descending", workers: workers, sortBy: "name", order: "descend", current: 1, pageSize: 10, wantNames: []string{"worker-d", "worker-c", "worker-b", "worker-a"}, wantTotal: 4},
		{name: "node ascending", workers: workers, sortBy: "node", order: "ascend", current: 1, pageSize: 10, wantNames: []string{"worker-c", "worker-b", "worker-d", "worker-a"}, wantTotal: 4},
		{name: "node descending", workers: workers, sortBy: "node", order: "descend", current: 1, pageSize: 10, wantNames: []string{"worker-a", "worker-b", "worker-d", "worker-c"}, wantTotal: 4},
		{name: "unknown sort falls back to time", workers: workers, sortBy: "unknown", order: "ascend", current: 1, pageSize: 10, wantNames: []string{"worker-a", "worker-b", "worker-d", "worker-c"}, wantTotal: 4},
		{name: "filters intersect", workers: workers, nameFilter: "worker-d", nodeFilter: "node-b", current: 1, pageSize: 10, wantNames: []string{"worker-d"}, wantTotal: 1},
		{name: "filters before pagination", workers: workers, nameFilter: "worker-c", current: 1, pageSize: 1, wantNames: []string{"worker-c"}, wantTotal: 1},
		{name: "filtered total", workers: workers, nodeFilter: "node-b", current: 1, pageSize: 1, wantNames: []string{"worker-b"}, wantTotal: 2},
		{name: "page beyond end preserves total", workers: workers, nodeFilter: "node-b", current: 3, pageSize: 1, wantNames: []string{}, wantTotal: 2},
		{name: "equal primary keys use name ascending tie breaker", workers: workers, sortBy: "time", order: "descend", current: 1, pageSize: 10, wantNames: []string{"worker-c", "worker-b", "worker-d", "worker-a"}, wantTotal: 4},
		{name: "unready filter keeps only unready pods", workers: readyWorkers, unready: true, current: 1, pageSize: 10, wantNames: []string{"worker-containers-not-ready", "worker-pending", "worker-no-conditions"}, wantTotal: 3},
		{name: "unready filter combines with node filter before pagination", workers: readyWorkers, unready: true, nodeFilter: "node-b", current: 1, pageSize: 10, wantNames: []string{"worker-containers-not-ready"}, wantTotal: 1},
		{name: "unready filter disabled keeps ready pods", workers: readyWorkers, current: 1, pageSize: 10, wantNames: []string{"worker-ready", "worker-containers-not-ready", "worker-pending", "worker-no-conditions"}, wantTotal: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, total := filterSortAndPaginateCacheGroupWorkers(tt.workers, tt.nameFilter, tt.nodeFilter, tt.unready, tt.sortBy, tt.order, tt.current, tt.pageSize)
			if total != tt.wantTotal {
				t.Fatalf("total = %d, want %d", total, tt.wantTotal)
			}
			if len(items) != len(tt.wantNames) {
				t.Fatalf("len(items) = %d, want %d", len(items), len(tt.wantNames))
			}
			for i, want := range tt.wantNames {
				if items[i].Name != want {
					t.Errorf("items[%d].Name = %q, want %q", i, items[i].Name, want)
				}
			}
		})
	}
}

func TestEnsureNodeSelectorLabels(t *testing.T) {
	tests := []struct {
		name       string
		labels     map[string]string
		selector   map[string]string
		wantLabels map[string]string
		wantUpdate bool
	}{
		{name: "initializes missing labels", selector: map[string]string{"role": "cache"}, wantLabels: map[string]string{"role": "cache"}, wantUpdate: true},
		{name: "keeps matching labels", labels: map[string]string{"role": "cache"}, selector: map[string]string{"role": "cache"}, wantLabels: map[string]string{"role": "cache"}},
		{name: "corrects mismatched values", labels: map[string]string{"role": "compute", "other": "kept"}, selector: map[string]string{"role": "cache"}, wantLabels: map[string]string{"role": "cache", "other": "kept"}, wantUpdate: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels, updated := ensureNodeSelectorLabels(tt.labels, tt.selector)
			if updated != tt.wantUpdate {
				t.Fatalf("updated = %v, want %v", updated, tt.wantUpdate)
			}
			if len(labels) != len(tt.wantLabels) {
				t.Fatalf("labels = %#v, want %#v", labels, tt.wantLabels)
			}
			for key, value := range tt.wantLabels {
				if labels[key] != value {
					t.Errorf("labels[%q] = %q, want %q", key, labels[key], value)
				}
			}
		})
	}
}

func TestListCacheGroupWorkersOnlyListsRequestedNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := juicefsiov1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	labels := map[string]string{
		operatorcommon.LabelCacheGroup: "shared",
		operatorcommon.LabelAppType:    operatorcommon.LabelWorkerValue,
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&juicefsiov1.CacheGroup{ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "wanted"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "wanted-worker", Namespace: "wanted", Labels: labels}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foreign-worker", Namespace: "foreign", Labels: labels}},
	).Build()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?current=1&pageSize=10", nil)
	c.Params = gin.Params{
		{Key: "namespace", Value: "wanted"},
		{Key: "name", Value: "shared"},
	}
	(&API{cachedReader: reader}).listCacheGroupWorkers()(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var response struct {
		Items []corev1.Pod `json:"items"`
		Total int          `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Total != 1 || len(response.Items) != 1 || response.Items[0].Name != "wanted-worker" {
		t.Fatalf("response = %#v, want only wanted-worker", response)
	}
}
