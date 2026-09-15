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

package driver

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

const (
	testVolumeID = "volume-handle-1"
	testPodUID   = "3f2b1c4d-0000-1111-2222-333344445555"
	testPVName   = "pv-1"
	testNodeID   = "node-1"
	testPVCNs    = "default"
	// For a static PV the volumeHandle and the PV name are unrelated, and the
	// path carries the latter.
	testVolumePath = "/var/lib/kubelet/pods/" + testPodUID + "/volumes/kubernetes.io~csi/" + testPVName + "/mount"
)

func testPV() *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: testPVName},
		Spec: corev1.PersistentVolumeSpec{
			ClaimRef: &corev1.ObjectReference{Namespace: testPVCNs, Name: "pvc-1"},
		},
	}
}

func testPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-1",
			Namespace: testPVCNs,
			UID:       testPodUID,
		},
	}
}

func newVolumeInfoTestService(objs ...runtime.Object) (*nodeService, *fake.Clientset) {
	cs := fake.NewSimpleClientset(objs...)
	registerer, _ := util.NewPrometheus(config.NodeName)
	return &nodeService{
		nodeID:              testNodeID,
		k8sClient:           &k8s.K8sClient{Interface: cs},
		metrics:             newNodeMetrics(registerer),
		publishedVolumeInfo: &sync.Map{},
	}, cs
}

func countActions(cs *fake.Clientset, verb, resource string) int {
	n := 0
	for _, a := range cs.Actions() {
		if a.GetVerb() == verb && a.GetResource().Resource == resource {
			n++
		}
	}
	return n
}

func TestEnsureVolumeInfo_AlreadyPopulated(t *testing.T) {
	d, cs := newVolumeInfoTestService(testPV(), testPod())
	d.publishedVolumeInfo.Store(volumeInfoKey{testVolumeID, testPodUID}, struct{}{})

	d.ensureVolumeInfo(context.Background(), testVolumeID, testVolumePath, testPodUID)

	if got := len(cs.Actions()); got != 0 {
		t.Errorf("expected no API calls for an already populated mount, got %d: %v", got, cs.Actions())
	}
	if got := testutil.CollectAndCount(d.metrics.volumeInfo); got != 0 {
		t.Errorf("expected no series to be created, got %d", got)
	}
}

func TestEnsureVolumeInfo_PopulatesOnMiss(t *testing.T) {
	d, cs := newVolumeInfoTestService(testPV(), testPod())

	d.ensureVolumeInfo(context.Background(), testVolumeID, testVolumePath, testPodUID)

	// util.NewPrometheus adds the juicefs_ prefix and node_name at registration.
	expected := `
# HELP volume_info Identifying labels for a JuiceFS volume mount on this node (value is always 1)
# TYPE volume_info gauge
volume_info{pod_name="app-1",pod_namespace="default",pod_uid="` + testPodUID + `",pvc_name="pvc-1",volume_id="` + testVolumeID + `"} 1
`
	if err := testutil.CollectAndCompare(d.metrics.volumeInfo, strings.NewReader(expected)); err != nil {
		t.Error(err)
	}
	if got := countActions(cs, "get", "persistentvolumes"); got != 1 {
		t.Errorf("expected 1 PV get, got %d", got)
	}
	if got := countActions(cs, "list", "pods"); got != 1 {
		t.Errorf("expected 1 pod list, got %d", got)
	}
	if got := countActions(cs, "get", "persistentvolumeclaims"); got != 0 {
		t.Errorf("expected no PVC get, got %d", got)
	}
	if _, exists := d.publishedVolumeInfo.Load(volumeInfoKey{testVolumeID, testPodUID}); !exists {
		t.Error("expected the mount to be marked done")
	}
}

// config.NodeName is left empty for --by-process=true deployments.
func TestEnsureVolumeInfo_ScopesListToNamespaceAndNodeID(t *testing.T) {
	d, cs := newVolumeInfoTestService(testPV(), testPod())

	d.ensureVolumeInfo(context.Background(), testVolumeID, testVolumePath, testPodUID)

	for _, a := range cs.Actions() {
		la, ok := a.(k8stesting.ListAction)
		if !ok {
			continue
		}
		if got := la.GetNamespace(); got != testPVCNs {
			t.Errorf("expected the list to be scoped to the claimRef namespace, got %q", got)
		}
		if got := la.GetListRestrictions().Fields.String(); got != "spec.nodeName="+testNodeID {
			t.Errorf("expected the list to be scoped to the node id, got %q", got)
		}
		return
	}
	t.Fatal("no list action was recorded")
}

func TestEnsureVolumeInfo_Failures(t *testing.T) {
	tests := []struct {
		name       string
		objs       []runtime.Object
		volumePath string
	}{
		{
			name:       "path carries no PV name",
			objs:       []runtime.Object{testPV(), testPod()},
			volumePath: "/var/lib/kubelet/pods/" + testPodUID + "/volumes/kubernetes.io~empty-dir/foo",
		},
		{
			name:       "PV is gone",
			objs:       []runtime.Object{testPod()},
			volumePath: testVolumePath,
		},
		{
			name: "PV has no claimRef",
			objs: []runtime.Object{
				&corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: testPVName}},
				testPod(),
			},
			volumePath: testVolumePath,
		},
		{
			name: "claimRef carries no PVC name",
			objs: []runtime.Object{
				&corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{Name: testPVName},
					Spec: corev1.PersistentVolumeSpec{
						ClaimRef: &corev1.ObjectReference{Namespace: testPVCNs},
					},
				},
				testPod(),
			},
			volumePath: testVolumePath,
		},
		{
			name:       "pod is no longer on the node",
			objs:       []runtime.Object{testPV()},
			volumePath: testVolumePath,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _ := newVolumeInfoTestService(tt.objs...)

			d.ensureVolumeInfo(context.Background(), testVolumeID, tt.volumePath, testPodUID)

			if got := testutil.CollectAndCount(d.metrics.volumeInfo); got != 0 {
				t.Errorf("expected no series on failure, got %d", got)
			}
			if _, exists := d.publishedVolumeInfo.Load(volumeInfoKey{testVolumeID, testPodUID}); exists {
				t.Error("expected the mount to stay unmarked so a later call retries")
			}
		})
	}
}

// --by-process=true builds the driver without a Kubernetes client.
func TestEnsureVolumeInfo_NoKubernetesClient(t *testing.T) {
	registerer, _ := util.NewPrometheus(config.NodeName)
	d := &nodeService{
		nodeID:              testNodeID,
		metrics:             newNodeMetrics(registerer),
		publishedVolumeInfo: &sync.Map{},
	}

	d.ensureVolumeInfo(context.Background(), testVolumeID, testVolumePath, testPodUID)

	if got := testutil.CollectAndCount(d.metrics.volumeInfo); got != 0 {
		t.Errorf("expected no series without a client, got %d", got)
	}
}
