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

package dashboard

import (
	"context"
	"testing"

	juicefsiov1 "github.com/juicedata/juicefs-operator/api/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

func perVolumeSecret(name, namespace string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{common.JuicefsSecretLabelKey: "true"},
		},
	}
}

func pvWithNodePublishSecret(name, secretName, secretNamespace string) corev1.PersistentVolume {
	pv := corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: name}}
	pv.Spec.CSI = &corev1.CSIPersistentVolumeSource{Driver: "csi.juicefs.com"}
	if secretName != "" {
		pv.Spec.CSI.NodePublishSecretRef = &corev1.SecretReference{
			Name:      secretName,
			Namespace: secretNamespace,
		}
	}
	return pv
}

// TestMergeNodePublishSecretsFetchesUnlabelledOnDemand checks that a volume's
// node-publish Secret, which is not labelled juicefs/secret and so is not in the
// (label-scoped) Secret cache ListAllSecrets reads from, is still found by name
// through the uncached client, so the batch-upgrade diff sees the same Secrets as
// before the cache was scoped.
func TestMergeNodePublishSecretsFetchesUnlabelledOnDemand(t *testing.T) {
	// An unlabelled node-publish Secret, present in the cluster but not in the cache.
	nodePublish := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "juicefs-secret", Namespace: "kube-system"},
	}
	api := &API{
		client: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset(nodePublish)},
	}

	// What the cache returns: only the labelled per-volume Secret.
	cached := []corev1.Secret{perVolumeSecret("juicefs-vol-a-secret", "kube-system")}
	pvs := []corev1.PersistentVolume{
		pvWithNodePublishSecret("pv-a", "juicefs-secret", "kube-system"),
		pvWithNodePublishSecret("pv-no-ref", "", ""),
	}

	got, err := api.mergeNodePublishSecrets(context.Background(), pvs, cached)
	assert.NoError(t, err)

	names := map[string]bool{}
	for _, s := range got {
		names[s.Namespace+"/"+s.Name] = true
	}
	assert.True(t, names["kube-system/juicefs-vol-a-secret"], "the labelled per-volume secret is kept")
	assert.True(t, names["kube-system/juicefs-secret"], "the unlabelled node-publish secret is fetched on demand")
}

// TestMergeNodePublishSecretsSkipsAlreadyPresentAndMissing checks the merge does
// not duplicate a Secret already in the list and does not fail when a referenced
// Secret does not exist.
func TestMergeNodePublishSecretsSkipsAlreadyPresentAndMissing(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "already", Namespace: "kube-system"},
	}
	api := &API{
		client: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset(existing)},
	}

	cached := []corev1.Secret{
		perVolumeSecret("already", "kube-system"), // same name/ns as a node-publish ref below
	}
	pvs := []corev1.PersistentVolume{
		pvWithNodePublishSecret("pv-dup", "already", "kube-system"),
		pvWithNodePublishSecret("pv-missing", "does-not-exist", "kube-system"),
	}

	got, err := api.mergeNodePublishSecrets(context.Background(), pvs, cached)
	assert.NoError(t, err, "a missing node-publish secret is skipped, not an error")

	count := 0
	for _, s := range got {
		if s.Namespace == "kube-system" && s.Name == "already" {
			count++
		}
	}
	assert.Equal(t, 1, count, "an already-present secret is not duplicated")
}

// TestValidateCgReadsSecretThroughUncachedClient checks that cache group
// validation finds its Secret through the uncached client, so it does not depend
// on the Secret being in the label-scoped cache (a cache group's Secret is not
// labelled juicefs/secret).
func TestValidateCgReadsSecretThroughUncachedClient(t *testing.T) {
	unlabelledCgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "cg-secret", Namespace: "jfs"},
	}
	client := &k8sclient.K8sClient{Interface: fake.NewSimpleClientset(unlabelledCgSecret)}

	cg := juicefsiov1.CacheGroup{
		ObjectMeta: metav1.ObjectMeta{Namespace: "jfs", Name: "cg"},
	}
	cg.Spec.SecretRef = &corev1.SecretEnvSource{}
	cg.Spec.SecretRef.Name = "cg-secret"
	cg.Spec.Worker.Template.NodeSelector = map[string]string{"juicefs.io/cache": "true"}

	assert.NoError(t, validateCg(context.Background(), client, cg),
		"an existing but unlabelled cache group secret validates")

	cg.Spec.SecretRef.Name = "missing"
	err := validateCg(context.Background(), client, cg)
	assert.Error(t, err, "a missing cache group secret is rejected")
	assert.Contains(t, err.Error(), "not found")
}
