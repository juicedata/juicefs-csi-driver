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

package watch

import (
	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/klog"
	ctrlruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	ctrl "github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook/cert"
)

func SetupWatcherForWebhook(mgr ctrlruntime.Manager, certBuilder *cert.CertificateBuilder, caCert []byte) (err error) {
	klog.Info("SetupWatcherForWebhook start")
	options := controller.Options{}
	webhookName := config.WebhookName
	options.Reconciler = &ctrl.WebhookReconciler{
		CertBuilder: certBuilder,
		WebhookName: webhookName,
		CaCert:      caCert,
	}
	webhookController, err := controller.New("webhook-controller", mgr, options)
	if err != nil {
		return err
	}

	mutatingWebhookConfigurationEventHandler := &mutatingWebhookConfigurationEventHandler{}
	err = webhookController.Watch(&source.Kind{
		Type: &admissionv1.MutatingWebhookConfiguration{},
	}, &handler.EnqueueRequestForObject{},
		predicate.Funcs{
			CreateFunc: mutatingWebhookConfigurationEventHandler.onCreateFunc(webhookName),
			UpdateFunc: mutatingWebhookConfigurationEventHandler.onUpdateFunc(webhookName),
			DeleteFunc: mutatingWebhookConfigurationEventHandler.onDeleteFunc(webhookName),
		})
	if err != nil {
		klog.Error(err, "Failed to watch mutatingWebhookConfiguration")
		return err
	}

	return
}

type mutatingWebhookConfigurationEventHandler struct{}

func (handler *mutatingWebhookConfigurationEventHandler) onCreateFunc(webhookName string) func(e event.CreateEvent) bool {
	return func(e event.CreateEvent) (onCreate bool) {
		klog.Info("receive mutatingWebhookConfigurationEventHandler.onCreateFunc")
		mutatingWebhookConfiguration, ok := e.Object.(*admissionv1.MutatingWebhookConfiguration)
		if !ok {
			klog.Infof("mutatingWebhookConfiguration.onCreateFunc Skip. object: %v", e.Object)
			return false
		}

		if mutatingWebhookConfiguration.GetName() != webhookName {
			klog.V(6).Infof("mutatingWebhookConfiguration.onUpdateFunc Skip. object: %v", e.Object)
			return false
		}

		klog.V(5).Infof("mutatingWebhookConfigurationEventHandler.onCreateFunc name: %s", mutatingWebhookConfiguration.GetName())
		return true
	}
}

func (handler *mutatingWebhookConfigurationEventHandler) onUpdateFunc(webhookName string) func(e event.UpdateEvent) bool {
	return func(e event.UpdateEvent) (needUpdate bool) {
		klog.Info("receive mutatingWebhookConfigurationEventHandler.onUpdateFunc")
		mutatingWebhookConfigurationNew, ok := e.ObjectNew.(*admissionv1.MutatingWebhookConfiguration)
		if !ok {
			klog.Infof("mutatingWebhookConfiguration.onUpdateFunc Skip. object: %v", e.ObjectNew)
			return false
		}

		mutatingWebhookConfigurationOld, ok := e.ObjectOld.(*admissionv1.MutatingWebhookConfiguration)
		if !ok {
			klog.Infof("mutatingWebhookConfiguration.onUpdateFunc Skip. object: %v", e.ObjectNew)
			return false
		}

		if mutatingWebhookConfigurationOld.GetName() != webhookName || mutatingWebhookConfigurationNew.GetName() != webhookName {
			klog.V(6).Infof("mutatingWebhookConfiguration.onUpdateFunc Skip. object: %v", e.ObjectNew)
			return false
		}

		klog.V(6).Infof("mutatingWebhookConfigurationEventHandler.onUpdateFunc name: %s", mutatingWebhookConfigurationNew.GetName())
		return true
	}
}

func (handler *mutatingWebhookConfigurationEventHandler) onDeleteFunc(webhookName string) func(e event.DeleteEvent) bool {
	return func(e event.DeleteEvent) bool {
		return false
	}
}
