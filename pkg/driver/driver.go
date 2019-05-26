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
	"context"
	"net"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"google.golang.org/grpc"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/mount"
)

const (
	driverName = "csi.juicefs.com"
)

var (
	vendorVersion = "0.1.0"
)

var (
	volumeCaps = []csi.VolumeCapability_AccessMode{
		{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
		{
			Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
		},
		// // TODO(yujunz): all modes should be supported, but needs validation
		// // Can only be published once as read/write on a single node, at
		// // any given time.
		// VolumeCapability_AccessMode_SINGLE_NODE_WRITER VolumeCapability_AccessMode_Mode = 1
		// // Can only be published once as readonly on a single node, at
		// // any given time.
		// VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY VolumeCapability_AccessMode_Mode = 2
		// // Can be published as readonly at multiple nodes simultaneously.
		// VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY VolumeCapability_AccessMode_Mode = 3
		// // Can be published at multiple nodes simultaneously. Only one of
		// // the node can be used as read/write. The rest will be readonly.
		// VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER VolumeCapability_AccessMode_Mode = 4
		// // Can be published as read/write at multiple nodes
		// // simultaneously.
		// VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER VolumeCapability_AccessMode_Mode = 5
	}
)

// Driver struct
type Driver struct {
	endpoint string
	nodeID   string

	srv *grpc.Server

	mounter mount.Interface
	exec    mount.Exec
}

// NewDriver creates a new driver
func NewDriver(endpoint string, nodeID string) *Driver {
	return &Driver{
		endpoint: endpoint,
		nodeID:   nodeID,
		mounter:  mount.New(""),
		exec:     mount.NewOsExec(),
	}
}

// Run the driver
func (d *Driver) Run() error {
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
	csi.RegisterNodeServer(d.srv, d)

	klog.Infof("Listening for connections on address: %#v", listener.Addr())
	return d.srv.Serve(listener)
}
