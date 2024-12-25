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
	"os"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	mountctrl "github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var (
	scheme = runtime.NewScheme()
	log    = klog.NewKlogr().WithName("manager")
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

type MountManager struct {
	mgr    ctrl.Manager
	client *k8sclient.K8sClient
}

func NewMountManager(
	leaderElection bool,
	leaderElectionNamespace string,
	leaderElectionLeaseDuration time.Duration) (*MountManager, error) {
	conf, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	mgr, err := ctrl.NewManager(conf, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0.0.0.0:8083",
		},
		LeaderElection:             leaderElection,
		LeaderElectionNamespace:    leaderElectionNamespace,
		LeaderElectionResourceLock: "leases",
		LeaderElectionID:           "mount.juicefs.com",
		LeaseDuration:              &leaderElectionLeaseDuration,
		Cache: cache.Options{
			Scheme: scheme,
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Label: labels.SelectorFromSet(labels.Set{common.PodTypeKey: common.PodTypeValue}),
				},
				&batchv1.Job{}: {
					Label: labels.SelectorFromSet(labels.Set{common.PodTypeKey: common.JobTypeValue}),
				},
			},
		},
	})
	if err != nil {
		log.Error(err, "New mount controller error")
		return nil, err
	}

	// gen k8s client
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		log.Error(err, "Could not create k8s client")
		return nil, err
	}

	return &MountManager{
		mgr:    mgr,
		client: k8sClient,
	}, err
}

func (m *MountManager) Start(ctx context.Context) {
	// init Reconciler（Controller）
	if err := (mountctrl.NewMountController(m.client)).SetupWithManager(m.mgr); err != nil {
		log.Error(err, "Register mount controller error")
		return
	}
	if err := (mountctrl.NewJobController(m.client)).SetupWithManager(m.mgr); err != nil {
		log.Error(err, "Register job controller error")
		return
	}
	if config.CacheClientConf {
		if err := (mountctrl.NewPVController(m.client)).SetupWithManager(m.mgr); err != nil {
			log.Error(err, "Register pv controller error")
			return
		}
		if err := (mountctrl.NewSecretController(m.client)).SetupWithManager(m.mgr); err != nil {
			log.Error(err, "Register secret controller error")
			return
		}
	}
	log.Info("Mount manager started.")
	if err := m.mgr.Start(ctx); err != nil {
		log.Error(err, "Mount manager start error")
		os.Exit(1)
	}
}
