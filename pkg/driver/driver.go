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

package driver

import (
	"context"
	"net"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"

	"github.com/prometheus/client_golang/prometheus"
)

// Driver struct
type Driver struct {
	controllerService
	nodeService
	provisionerService

	srv      *grpc.Server
	endpoint string
}

// NewDriver creates a new driver
func NewDriver(endpoint string, nodeID string,
	leaderElection bool,
	leaderElectionNamespace string,
	leaderElectionLeaseDuration time.Duration, reg prometheus.Registerer) (*Driver, error) {
	klog.Infof("Driver: %v version %v commit %v date %v", config.DriverName, driverVersion, gitCommit, buildDate)

	var k8sClient *k8sclient.K8sClient
	if !config.ByProcess {
		var err error
		k8sClient, err = k8sclient.NewClient()
		if err != nil {
			klog.V(5).Infof("Can't get k8s client: %v", err)
			return nil, err
		}
	}
	cs, err := newControllerService(k8sClient)
	if err != nil {
		return nil, err
	}

	ns, err := newNodeService(nodeID, k8sClient, reg)
	if err != nil {
		return nil, err
	}

	ps, err := newProvisionerService(k8sClient, leaderElection, leaderElectionNamespace, leaderElectionLeaseDuration, reg)
	if err != nil {
		return nil, err
	}

	return &Driver{
		controllerService:  cs,
		nodeService:        *ns,
		provisionerService: ps,
		endpoint:           endpoint,
	}, nil
}

// Run runs the server
func (d *Driver) Run() error {
	if config.Provisioner {
		go d.provisionerService.Run(context.Background())
	}
	scheme, addr, err := util.ParseEndpoint(d.endpoint)
	if err != nil {
		return err
	}

	listener, err := net.Listen(scheme, addr)
	if err != nil {
		return err
	}

	logErr := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			klog.Errorf("GRPC error: %v", err)
		}
		return resp, err
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logErr),
	}
	d.srv = grpc.NewServer(opts...)

	csi.RegisterIdentityServer(d.srv, d)
	csi.RegisterControllerServer(d.srv, d)
	csi.RegisterNodeServer(d.srv, d)

	klog.Infof("Listening for connection on address: %#v", listener.Addr())
	return d.srv.Serve(listener)
}

// Stop stops server
func (d *Driver) Stop() {
	klog.Infof("Stopped server")
	d.srv.Stop()
}
