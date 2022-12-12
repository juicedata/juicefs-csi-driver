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
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

const HandlerPath = "/juicefs/mutate"

// Register registers the handlers to the manager
func Register(mgr manager.Manager, client *k8sclient.K8sClient) {
	server := mgr.GetWebhookServer()
	server.Register(HandlerPath, &webhook.Admission{Handler: &SidecarHandler{
		Client: client,
	}})
	klog.Infof("Registered webhook handler path %s", HandlerPath)
}
