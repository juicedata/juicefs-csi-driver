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

package app

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	mountctrl "github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook/handler"
)

type WebhookManager struct {
	mgr    ctrl.Manager
	client *k8sclient.K8sClient
}

func NewWebhookManager(certDir string, webhookPort int, leaderElection bool,
	leaderElectionNamespace string,
	leaderElectionLeaseDuration time.Duration) (*WebhookManager, error) {
	_ = clientgoscheme.AddToScheme(scheme)
	cfg, err := ctrl.GetConfig()
	if err != nil {
		log.Error(err, "can not get kube config")
		return nil, err
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                  scheme,
		Port:                    webhookPort,
		CertDir:                 certDir,
		MetricsBindAddress:      "0.0.0.0:8084",
		LeaderElection:          leaderElection,
		LeaderElectionID:        "webhook.juicefs.com",
		LeaderElectionNamespace: leaderElectionNamespace,
		LeaseDuration:           &leaderElectionLeaseDuration,
		NewCache: cache.BuilderWithOptions(cache.Options{
			Scheme: scheme,
			SelectorsByObject: cache.SelectorsByObject{
				&corev1.Pod{}: {
					Label: labels.SelectorFromSet(labels.Set{common.InjectSidecarDone: common.True}),
				},
			},
		}),
	})

	if err != nil {
		log.Error(err, "initialize controller manager failed")
		return nil, err
	}
	// gen k8s client
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		log.Error(err, "Could not create k8s client")
		return nil, err
	}
	if config.CacheClientConf {
		if err := (mountctrl.NewSecretController(k8sClient)).SetupWithManager(mgr); err != nil {
			log.Error(err, "Register secret controller error")
			return nil, err
		}
	}
	return &WebhookManager{
		mgr:    mgr,
		client: k8sClient,
	}, nil
}

func (w *WebhookManager) Start(ctx context.Context) error {
	if err := w.registerWebhook(); err != nil {
		log.Error(err, "Register webhook error")
		return err
	}
	if err := w.registerAppController(); err != nil {
		log.Error(err, "Register app controller error")
		return err
	}
	log.Info("Webhook manager started.")
	if err := w.mgr.Start(ctx); err != nil {
		log.Error(err, "Webhook manager start error")
		return err
	}
	return nil
}

func (w *WebhookManager) registerWebhook() error {
	// register admission handlers
	log.Info("Register webhook handler")
	handler.Register(w.mgr, w.client)
	return nil
}

func (w *WebhookManager) registerAppController() error {
	// init Reconciler（Controller）
	log.Info("Register app controller")
	return (mountctrl.NewAppController(w.client)).SetupWithManager(w.mgr)
}
