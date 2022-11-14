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

package controller

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook/cert"
)

type WebhookReconciler struct {
	CertBuilder *cert.CertificateBuilder
	WebhookName string
	CaCert      []byte
}

func (r *WebhookReconciler) Reconcile(context.Context, ctrl.Request) (ctrl.Result, error) {
	// patch ca of MutatingWebhookConfiguration
	err := r.CertBuilder.PatchCABundle(r.WebhookName, r.CaCert)
	if err != nil {
		return util.RequeueAfterInterval(10 * time.Second)
	}
	return util.NoRequeue()
}
