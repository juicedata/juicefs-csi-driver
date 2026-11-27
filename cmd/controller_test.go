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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

const testNamespace = "kube-system"

func templateSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key: "pool", Operator: corev1.NodeSelectorOpIn, Values: []string{"gpu"},
						}},
					}},
				},
			},
		},
		Tolerations: []corev1.Toleration{{Key: "pool", Operator: corev1.TolerationOpExists}},
	}
}

func csiNodeDaemonSet(name string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec:       appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: templateSpec()}},
	}
}

// csiNodePod mirrors a pod the DaemonSet controller creates on node: its required node affinity is
// replaced by a single `metadata.name In [node]` term, and ServiceAccount admission has added the
// service account's image pull secrets.
func csiNodePod(node string, ready bool) *corev1.Pod {
	spec := templateSpec()
	spec.NodeName = node
	spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{{
		MatchFields: []corev1.NodeSelectorRequirement{{
			Key: metav1.ObjectNameField, Operator: corev1.NodeSelectorOpIn, Values: []string{node},
		}},
	}}
	spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "sa-regcred"}}
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "juicefs-csi-node-" + node, Namespace: testNamespace,
			Labels: map[string]string{"app": "juicefs-csi-node"},
		},
		Spec: spec,
		Status: corev1.PodStatus{Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: status},
			{Type: corev1.ContainersReady, Status: status},
		}},
	}
}

func TestGetCSINodePod(t *testing.T) {
	deleting := csiNodePod("node-a", true)
	deleting.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	deleting.Finalizers = []string{"test"}
	tests := []struct {
		name     string
		dsName   string
		objects  []runtime.Object
		dsGetErr error
		want     *corev1.Pod
		wantErr  bool
	}{
		{
			name:    "prefers Ready pod over DaemonSet template",
			objects: []runtime.Object{csiNodeDaemonSet("juicefs-csi-node"), csiNodePod("node-a", false), csiNodePod("node-b", true)},
			want:    csiNodePod("node-b", true),
		},
		{
			name:    "skips pod being deleted",
			objects: []runtime.Object{csiNodeDaemonSet("juicefs-csi-node"), deleting},
			want:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "juicefs-csi-node", Namespace: testNamespace}, Spec: templateSpec()},
		},
		{
			name:    "uses DaemonSet template without Ready pods",
			objects: []runtime.Object{csiNodeDaemonSet("juicefs-csi-node"), csiNodePod("node-a", false)},
			want:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "juicefs-csi-node", Namespace: testNamespace}, Spec: templateSpec()},
		},
		{
			name:    "reads DaemonSet named by JUICEFS_CSI_NODE_DS_NAME",
			dsName:  "custom-csi-node",
			objects: []runtime.Object{csiNodeDaemonSet("custom-csi-node")},
			want:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "custom-csi-node", Namespace: testNamespace}, Spec: templateSpec()},
		},
		{
			name:    "uses unready pod when DaemonSet is not apps/v1",
			objects: []runtime.Object{csiNodePod("node-a", false)},
			want:    csiNodePod("node-a", false),
		},
		{
			name:     "fails on DaemonSet errors other than NotFound",
			objects:  []runtime.Object{csiNodePod("node-a", false)},
			dsGetErr: k8serrors.NewInternalError(assert.AnError),
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JUICEFS_CSI_NODE_DS_NAME", tt.dsName)
			clientset := fake.NewSimpleClientset(tt.objects...)
			if tt.dsGetErr != nil {
				clientset.PrependReactor("get", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, tt.dsGetErr
				})
			}
			got, err := getCSINodePod(t.Context(), &k8s.K8sClient{Interface: clientset}, testNamespace)
			if tt.wantErr {
				require.Error(t, err)
				assert.NotErrorIs(t, err, errNoCSINode)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("reports missing CSI node", func(t *testing.T) {
		t.Setenv("JUICEFS_CSI_NODE_DS_NAME", "")
		clientset := fake.NewSimpleClientset()
		clientset.PrependReactor("get", "daemonsets", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, k8serrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "daemonsets"}, "juicefs-csi-node")
		})
		_, err := getCSINodePod(t.Context(), &k8s.K8sClient{Interface: clientset}, testNamespace)
		assert.ErrorIs(t, err, errNoCSINode)
	})
}

func TestInheritCSINodePod(t *testing.T) {
	t.Setenv("JUICEFS_CSI_NODE_DS_NAME", "")
	config.Namespace = testNamespace

	t.Run("refreshes after the pod's node is removed", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(csiNodePod("node-a", true))
		client := &k8s.K8sClient{Interface: clientset}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		require.NoError(t, inheritCSINodePod(ctx, client, 10*time.Millisecond))
		assert.Equal(t, "juicefs-csi-node-node-a", config.GetCSIPod().Name)

		require.NoError(t, clientset.CoreV1().Pods(testNamespace).Delete(t.Context(), "juicefs-csi-node-node-a", metav1.DeleteOptions{}))
		_, err := clientset.CoreV1().Pods(testNamespace).Create(t.Context(), csiNodePod("node-b", true), metav1.CreateOptions{})
		require.NoError(t, err)
		assert.Eventually(t, func() bool {
			return config.GetCSIPod().Name == "juicefs-csi-node-node-b"
		}, 5*time.Second, 10*time.Millisecond)
	})

	t.Run("tolerates missing CSI node only in webhook mode", func(t *testing.T) {
		client := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		config.Webhook = false
		assert.ErrorIs(t, inheritCSINodePod(ctx, client, time.Hour), errNoCSINode)
		config.Webhook = true
		defer func() { config.Webhook = false }()
		assert.NoError(t, inheritCSINodePod(ctx, client, time.Hour))
	})
}
