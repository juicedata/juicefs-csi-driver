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

package pods

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	runtimefake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

// TestListSidecarResourcesReadsSecretsUncached checks that the sidecar-upgrade
// resource gather finds the settings Secret and the node-publish (customer)
// Secret through the uncached client, so it no longer depends on those Secrets
// being in the label-scoped manager cache.
func TestListSidecarResourcesReadsSecretsUncached(t *testing.T) {
	const workloadNS = "apps"

	// A PV with a node-publish (customer) Secret in a different namespace.
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver: "csi.juicefs.com",
					NodePublishSecretRef: &corev1.SecretReference{
						Name:      "customer",
						Namespace: "juicefs",
					},
				},
			},
		},
	}

	// The Secrets exist in the cluster but are NOT labelled juicefs/secret, so
	// they would not be in the label-scoped cache. They live only in the
	// uncached clientset here, never in the cached controller-runtime client.
	settingsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "juicefs-settings", Namespace: workloadNS},
	}
	customerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "customer", Namespace: "juicefs"},
	}

	s := &podService{
		client:    runtimefake.NewClientBuilder().WithObjects(pv).Build(),
		k8sClient: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset(settingsSecret, customerSecret)},
	}

	_, resources, err := s.listSidecarResources(context.Background(), workloadNS)
	assert.NoError(t, err)

	_, hasSettings := resources.Secrets[types.NamespacedName{Namespace: workloadNS, Name: "juicefs-settings"}]
	assert.True(t, hasSettings, "settings secret in the workload namespace is read uncached")

	_, hasCustomer := resources.Secrets[types.NamespacedName{Namespace: "juicefs", Name: "customer"}]
	assert.True(t, hasCustomer, "node-publish customer secret is fetched on demand")
}

// TestListSidecarResourcesMissingCustomerSecretIsSkipped checks that a PV naming
// a node-publish Secret that does not exist does not fail the gather.
func TestListSidecarResourcesMissingCustomerSecretIsSkipped(t *testing.T) {
	const workloadNS = "apps"
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver: "csi.juicefs.com",
					NodePublishSecretRef: &corev1.SecretReference{
						Name:      "missing",
						Namespace: "juicefs",
					},
				},
			},
		},
	}
	s := &podService{
		client:    runtimefake.NewClientBuilder().WithObjects(pv).Build(),
		k8sClient: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
	}

	_, resources, err := s.listSidecarResources(context.Background(), workloadNS)
	assert.NoError(t, err, "a missing node-publish secret is skipped, not an error")
	_, has := resources.Secrets[types.NamespacedName{Namespace: "juicefs", Name: "missing"}]
	assert.False(t, has)
}
