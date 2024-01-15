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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	mountctrl "github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var (
	scheme = runtime.NewScheme()
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
		Scheme:                  scheme,
		Port:                    9443,
		MetricsBindAddress:      "0.0.0.0:8083",
		LeaderElection:          leaderElection,
		LeaderElectionNamespace: leaderElectionNamespace,
		LeaderElectionID:        "mount.juicefs.com",
		LeaseDuration:           &leaderElectionLeaseDuration,
		NewCache: cache.BuilderWithOptions(cache.Options{
			Scheme: scheme,
			SelectorsByObject: cache.SelectorsByObject{
				&corev1.Pod{}: {
					Label: labels.SelectorFromSet(labels.Set{config.PodTypeKey: config.PodTypeValue}),
				},
				&batchv1.Job{}: {
					Label: labels.SelectorFromSet(labels.Set{config.PodTypeKey: config.JobTypeValue}),
				},
			},
		}),
	})
	if err != nil {
		klog.Errorf("New mount controller error: %v", err)
		return nil, err
	}

	// gen k8s client
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		klog.V(5).Infof("Could not create k8s client %v", err)
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
		klog.Errorf("Register mount controller error: %v", err)
		return
	}
	if err := (mountctrl.NewJobController(m.client)).SetupWithManager(m.mgr); err != nil {
		klog.Errorf("Register job controller error: %v", err)
		return
	}
	if config.CacheClientConf {
		if err := (mountctrl.NewSecretController(m.client)).SetupWithManager(m.mgr); err != nil {
			klog.Errorf("Register secret controller error: %v", err)
			return
		}
	}
	klog.Info("Mount manager started.")
	if err := m.mgr.Start(ctx); err != nil {
		klog.Errorf("Mount manager start error: %v", err)
		return
	}
}
