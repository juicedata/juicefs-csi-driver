/*
Copyright 2018 The Kubernetes Authors.

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

package driver

import (
	"k8s.io/client-go/kubernetes/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

// NewFakeDriver creates a new mock driver used for testing
func NewFakeDriver(endpoint string, fakeProvider juicefs.Interface) *Driver {
	registerer, _ := util.NewPrometheus(config.NodeName)
	metrics := newNodeMetrics(registerer)
	return &Driver{
		endpoint: endpoint,
		controllerService: controllerService{
			juicefs: fakeProvider,
			vols:    make(map[string]int64),
		},
		nodeService: nodeService{
			juicefs:   fakeProvider,
			nodeID:    "fake-node-id",
			k8sClient: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
			metrics:   metrics,
		},
	}
}
