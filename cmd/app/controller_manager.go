/*
 Copyright 2025 Juicedata Inc

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
	"os"
	"time"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	mountctrl "github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook/handler"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme = runtime.NewScheme()
	log    = klog.NewKlogr().WithName("controller-manager")
)

type ControllerManager struct {
	mgr                ctrl.Manager
	enableMountManager bool
	enableWebhook      bool
	client             *k8sclient.K8sClient
}

func NewControllerManager(
	enableMountManager bool,
	enableWebhook bool,
	leaderElection bool,
	leaderElectionNamespace string,
	leaderElectionLeaseDuration time.Duration,
	certDir string,
	webhookPort int,
) (*ControllerManager, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		log.Error(err, "can not get kube config")
		return nil, err
	}

	opts := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0.0.0.0:8084",
		},
		LeaderElection:             leaderElection,
		LeaderElectionID:           "csi-controller.juicefs.com",
		LeaderElectionResourceLock: "leases",
		LeaderElectionNamespace:    leaderElectionNamespace,
		LeaseDuration:              &leaderElectionLeaseDuration,
		Cache: cache.Options{
			Scheme: scheme,
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}: {},
				&batchv1.Job{}: {
					Label: labels.SelectorFromSet(labels.Set{common.PodTypeKey: common.JobTypeValue}),
				},
			},
		},
	}
	if config.Webhook {
		opts.WebhookServer = webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: certDir,
		})
	}

	mgr, err := ctrl.NewManager(cfg, opts)
	if err != nil {
		log.Error(err, "initialize controller manager failed")
		os.Exit(1)
	}

	// gen k8s client
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		log.Error(err, "Could not create k8s client")
		return nil, err
	}

	return &ControllerManager{
		mgr:                mgr,
		enableMountManager: enableMountManager,
		enableWebhook:      enableWebhook,
		client:             k8sClient,
	}, nil
}

func (m *ControllerManager) Start(ctx context.Context) error {
	// If webhook is set, we are in sidecar mode
	if m.enableWebhook {
		log.Info("Register webhook handler")
		handler.Register(m.mgr, m.client)
		if err := (mountctrl.NewAppController(m.client)).SetupWithManager(m.mgr); err != nil {
			log.Error(err, "Register app controller error")
			return err
		}
	}

	if m.enableMountManager {
		if err := (mountctrl.NewMountController(m.client)).SetupWithManager(m.mgr); err != nil {
			log.Error(err, "Register mount controller error")
			return err
		}
		if err := (mountctrl.NewJobController(m.client)).SetupWithManager(m.mgr); err != nil {
			log.Error(err, "Register job controller error")
			return err
		}
	}

	if config.CacheClientConf {
		if m.enableMountManager {
			if err := (mountctrl.NewPVController(m.client)).SetupWithManager(m.mgr); err != nil {
				log.Error(err, "Register pv controller error")
				return err
			}
		}
		if err := (mountctrl.NewSecretController(m.client)).SetupWithManager(m.mgr); err != nil {
			log.Error(err, "Register secret controller error")
			return err
		}
	}

	if err := m.mgr.Start(ctx); err != nil {
		log.Error(err, "fail to start controller manager")
		return err
	}
	return nil
}
