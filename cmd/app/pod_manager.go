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

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

var (
	podManagerLog = klog.NewKlogr().WithName("pod-manager")
)

type PodManager struct {
	mgr         ctrl.Manager
	client      *k8sclient.K8sClient
	cacheReader client.Reader
}

func NewPodManager() (*PodManager, error) {
	conf, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	mgr, err := ctrl.NewManager(conf, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0.0.0.0:8082",
		},
		LeaderElectionResourceLock: "leases",
		LeaderElectionID:           "pod.juicefs.com",
		Cache: cache.Options{
			Scheme: scheme,
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Label: labels.SelectorFromSet(map[string]string{
						common.PodTargetNodeLabelKey: config.NodeName,
					}),
				},
			},
		},
	})
	if err != nil {
		podManagerLog.Error(err, "New pod controller error")
		return nil, err
	}

	// gen k8s client
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		podManagerLog.Info("Could not create k8s client")
		return nil, err
	}

	return &PodManager{
		mgr:         mgr,
		cacheReader: mgr.GetAPIReader(),
		client:      k8sClient,
	}, err
}

func (m *PodManager) Start(ctx context.Context) error {
	// init Reconciler（Controller）
	if err := (mountctrl.NewPodController(m.client, m.cacheReader)).SetupWithManager(m.mgr); err != nil {
		podManagerLog.Error(err, "Register pod controller error")
		return err
	}
	podManagerLog.Info("Pod manager started.")
	if err := m.mgr.Start(ctx); err != nil {
		podManagerLog.Error(err, "Pod manager start error")
		return err
	}
	return nil
}
