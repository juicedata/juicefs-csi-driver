/*
 Copyright 2022 Juicedata Inc

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

package cert

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

const (
	CertSecretName = "juicefs-csi-webhook-certs"
)

type CertificateBuilder struct {
	Client *k8sclient.K8sClient
}

func NewCertificateBuilder(c *k8sclient.K8sClient) *CertificateBuilder {
	ch := &CertificateBuilder{Client: c}
	return ch
}

// BuildOrSyncCABundle use service name and namespace to generate webhook certs
// or sync the certs from the secret
func (c *CertificateBuilder) BuildOrSyncCABundle(svcName, cerPath string) ([]byte, error) {
	klog.Info("start generate certificate", "service", svcName, "namespace", config.Namespace, "cert dir", cerPath)

	certs, err := c.genCA(config.Namespace, svcName, cerPath)
	if err != nil {
		return []byte{}, err
	}

	return certs.CACert, nil
}

// genCA generate the caBundle and store it in secret and local path
func (c *CertificateBuilder) genCA(ns, svc, certPath string) (*Artifacts, error) {
	certWriter, err := NewSecretCertWriter(SecretCertWriterOptions{
		Client: c.Client,
		Secret: &types.NamespacedName{Namespace: ns, Name: CertSecretName},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to new certWriter: %v", err)
	}

	dnsName := ServiceToCommonName(ns, svc)

	certs, _, err := certWriter.EnsureCert(dnsName)
	if err != nil {
		return certs, fmt.Errorf("failed to ensure certs: %v", err)
	}

	if err := WriteCertsToDir(certPath, certs); err != nil {
		return certs, fmt.Errorf("failed to WriteCertsToDir: %v", err)
	}
	return certs, nil
}

// PatchCABundle patch the caBundle to MutatingWebhookConfiguration
func (c *CertificateBuilder) PatchCABundle(webhookName string, ca []byte) (err error) {

	var m *v1.MutatingWebhookConfiguration

	klog.Info("start patch MutatingWebhookConfiguration caBundle", "name", webhookName)

	ctx := context.Background()

	if m, err = c.Client.GetAdmissionWebhookConfig(ctx, webhookName); err != nil {
		klog.Error(err, "fail to get mutatingWebHook", "name", webhookName)
		return err
	}

	current := m.DeepCopy()
	for i := range m.Webhooks {
		m.Webhooks[i].ClientConfig.CABundle = ca
	}

	if reflect.DeepEqual(m.Webhooks, current.Webhooks) {
		klog.Info("no need to patch the MutatingWebhookConfiguration", "name", webhookName)
		return nil
	}

	patchWebhook := map[string]interface{}{
		"webhooks": m.Webhooks,
	}
	patchJson, err := json.Marshal(patchWebhook)
	if err != nil {
		return err
	}
	if err := c.Client.PatchAdmissionWebhookConfig(ctx, m.Name, patchJson); err != nil {
		klog.Error(err, "fail to patch CABundle to mutatingWebHook", "name", webhookName)
		return err
	}

	klog.Infof("finished patch MutatingWebhookConfiguration caBundle %s", webhookName)

	return nil
}
