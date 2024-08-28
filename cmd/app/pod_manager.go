/*
 Copyright 2023 Juicedata Inc

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	mountctrl "github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

type PodManager struct {
	mgr    ctrl.Manager
	client *k8sclient.K8sClient
}

func NewPodManager() (*PodManager, error) {
	conf, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	mgr, err := ctrl.NewManager(conf, ctrl.Options{
		Scheme:             scheme,
		Port:               9442,
		MetricsBindAddress: "0.0.0.0:8082",
		LeaderElectionID:   "pod.juicefs.com",
		NewCache: cache.BuilderWithOptions(cache.Options{
			Scheme: scheme,
			SelectorsByObject: cache.SelectorsByObject{
				&corev1.Pod{}: {
					Label: labels.SelectorFromSet(labels.Set{config.PodTypeKey: config.PodTypeValue}),
				},
			},
		}),
	})
	if err != nil {
		log.Error(err, "New pod controller error")
		return nil, err
	}

	// gen k8s client
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		log.Info("Could not create k8s client")
		return nil, err
	}

	return &PodManager{
		mgr:    mgr,
		client: k8sClient,
	}, err
}

func (m *PodManager) Start(ctx context.Context) error {
	// init Reconciler（Controller）
	if err := (mountctrl.NewPodController(m.client)).SetupWithManager(m.mgr); err != nil {
		log.Error(err, "Register pod controller error")
		return err
	}
	log.Info("Pod manager started.")
	if err := m.mgr.Start(ctx); err != nil {
		log.Error(err, "Pod manager start error")
		return err
	}
	return nil
}
