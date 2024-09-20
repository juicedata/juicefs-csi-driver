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

package handler

import (
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

const (
	SidecarPath    = "/juicefs/inject-v1-pod"
	ServerlessPath = "/juicefs/serverless/inject-v1-pod"
	SecretPath     = "/juicefs/validate-secret"
	PVPath         = "/juicefs/validate-pv"
)

var (
	webhookLog = klog.NewKlogr().WithName("webhook")
)

// Register registers the handlers to the manager
func Register(mgr manager.Manager, client *k8sclient.K8sClient) {
	server := mgr.GetWebhookServer()
	server.Register(SidecarPath, &webhook.Admission{Handler: NewSidecarHandler(client, false)})
	webhookLog.Info("Registered webhook handler for sidecar", "path", SidecarPath)
	server.Register(ServerlessPath, &webhook.Admission{Handler: NewSidecarHandler(client, true)})
	webhookLog.Info("Registered webhook handler path %s for serverless", "path", ServerlessPath)
	if config.ValidatingWebhook {
		server.Register(SecretPath, &webhook.Admission{Handler: NewSecretHandler(client)})
		server.Register(PVPath, &webhook.Admission{Handler: NewPVHandler(client)})
	}
}
