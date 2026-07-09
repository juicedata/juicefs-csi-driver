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

package ddschedulerextender

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestFilterWithHeadroom(t *testing.T) {
	nodeA := testNode("node-a", "2", "2Gi", 10)
	nodeB := testNode("node-b", "2", "2Gi", 10)
	client := fake.NewSimpleClientset(
		nodeA,
		nodeB,
		testPod("used-a", "node-a", "1500m", "1Gi", nil),
		testPod("used-b", "node-b", "500m", "512Mi", nil),
	)
	server := newSyncedTestServer(t, client)

	incoming := testPod("incoming", "", "400m", "256Mi", map[string]string{
		defaultAnnotationPrefix + "/cpu":    "500m",
		defaultAnnotationPrefix + "/memory": "512Mi",
		defaultAnnotationPrefix + "/pods":   "1",
	})
	args := &ExtenderArgs{
		Pod:   *incoming,
		Nodes: &corev1.NodeList{Items: []corev1.Node{*nodeA, *nodeB}},
	}

	result := server.Filter(args)

	require.Empty(t, result.Error)
	require.NotNil(t, result.Nodes)
	require.Equal(t, []corev1.Node{*nodeB}, result.Nodes.Items)
	require.Contains(t, result.FailedNodes, "node-a")
	require.Contains(t, result.FailedNodes["node-a"], "cpu")
}

func TestFilterWithNodeNames(t *testing.T) {
	node := testNode("node-a", "2", "2Gi", 10)
	client := fake.NewSimpleClientset(node)
	server := newSyncedTestServer(t, client)
	names := []string{"node-a"}
	incoming := testPod("incoming", "", "100m", "128Mi", map[string]string{
		defaultAnnotationPrefix + "/pods": "1",
	})

	result := server.Filter(&ExtenderArgs{Pod: *incoming, NodeNames: &names})

	require.Empty(t, result.Error)
	require.NotNil(t, result.NodeNames)
	require.Equal(t, names, *result.NodeNames)
}

func TestFilterWithoutHeadroomPassesAllCandidates(t *testing.T) {
	node := testNode("node-a", "1", "1Gi", 10)
	client := fake.NewSimpleClientset(node, testPod("used", "node-a", "1", "1Gi", nil))
	server := newSyncedTestServer(t, client)
	incoming := testPod("incoming", "", "1", "1Gi", nil)

	result := server.Filter(&ExtenderArgs{
		Pod:   *incoming,
		Nodes: &corev1.NodeList{Items: []corev1.Node{*node}},
	})

	require.Empty(t, result.Error)
	require.NotNil(t, result.Nodes)
	require.Equal(t, []corev1.Node{*node}, result.Nodes.Items)
	require.Empty(t, result.FailedNodes)
}

func TestHeadroomAnnotationsOverrideDefaults(t *testing.T) {
	defaults := Headroom{MilliCPU: 1000, Memory: int64(1024 * 1024 * 1024), Pods: 1}
	pod := testPod("incoming", "", "100m", "128Mi", map[string]string{
		defaultAnnotationPrefix + "/cpu":   "250m",
		legacyAnnotationPrefix + "/memory": "2Gi",
		defaultAnnotationPrefix + "/pods":  "2",
	})

	headroom, err := headroomFromPod(pod, defaults, nil)

	require.NoError(t, err)
	require.Equal(t, int64(250), headroom.MilliCPU)
	require.Equal(t, int64(2*1024*1024*1024), headroom.Memory)
	require.Equal(t, int64(2), headroom.Pods)
}

func newSyncedTestServer(t *testing.T, client *fake.Clientset) *Server {
	t.Helper()
	server, err := NewServer(client, Options{CacheSyncTimeout: time.Second})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	server.informerFactory.Start(ctx.Done())
	require.True(t, cache.WaitForCacheSync(ctx.Done(), server.podInformer.Informer().HasSynced, server.nodeInformer.Informer().HasSynced))
	return server
}

func testNode(name, cpu, memory string, pods int64) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(memory),
				corev1.ResourcePods:   *resource.NewQuantity(pods, resource.DecimalSI),
			},
		},
	}
}

func testPod(name, nodeName, cpu, memory string, annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "default",
			Name:        name,
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name: "app",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(cpu),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
				},
			}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}
