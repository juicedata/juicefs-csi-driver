package driver

import (
	"context"
	"net"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"google.golang.org/grpc"
	"k8s.io/klog"
)

const (
	// DriverName to be registered
	DriverName = "csi.juicefs.com"
)

// Driver struct
type Driver struct {
	controllerService
	nodeService

	srv      *grpc.Server
	endpoint string
}

// NewDriver creates a new driver
func NewDriver(endpoint string, nodeID string) (*Driver, error) {
	klog.Infof("Driver: %v version %v commit %v date %v", DriverName, driverVersion, gitCommit, buildDate)

	return &Driver{
		endpoint:          endpoint,
		controllerService: newControllerService(),
		nodeService:       newNodeService(nodeID),
	}, nil
}

// Run runs the server
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
