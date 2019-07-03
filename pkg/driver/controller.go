package driver

import (
	"context"
	"path"
	"reflect"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

var (
	volumeCaps = []csi.VolumeCapability_AccessMode{
		{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
		{
			Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
		},
	}

	controllerCaps = []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	}
)

type controllerService struct {
	juicefs juicefs.Interface
}

func newControllerService() controllerService {
	juicefs, err := juicefs.NewJfsProvider(nil)
	if err != nil {
		panic(err)
	}

	return controllerService{
		juicefs: juicefs,
	}
}

// CreateVolume create directory in an existing JuiceFS filesystem
func (d *controllerService) CreateVolume(
	ctx context.Context,
	req *csi.CreateVolumeRequest) (
	*csi.CreateVolumeResponse, error) {
	// DEBUG only, secrets exposed in args
	// klog.V(5).Infof("CreateVolume: called with args: %#v", req)

	if len(req.Name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume Name cannot be empty")
	}
	if req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities cannot be empty")
	}

	secrets := req.Secrets
	klog.V(5).Infof("CreateVolume: ControllerCreateSecrets contains keys %+v", reflect.ValueOf(secrets).MapKeys())

	jfsName := req.Parameters["jfsName"]
	volName := req.Name

	stdoutStderr, err := d.juicefs.CmdAuth(jfsName, secrets)
	klog.V(5).Infof("CreateVolume: authentication output is '%s'\n", stdoutStderr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not auth juicefs %s: %v", jfsName, err)
	}

	mountPath, err := d.juicefs.SafeMount(jfsName, []string{})
	if err != nil {
		klog.Errorf("CreateVolume: failed to mount %q", jfsName)
		return nil, err
	}
	volPath := path.Join(mountPath, volName)
	exists, err := d.juicefs.ExistsPath(volPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}

	if !exists {
		if err := d.juicefs.MakeDir(volPath); err != nil {
			return nil, status.Errorf(codes.Internal, "Could not make directory %q", volPath)
		}
	}

	volume := csi.Volume{
		VolumeId: volPath,
		VolumeContext: map[string]string{
			jfsName: jfsName,
		},
		CapacityBytes: juicefs.DefaultCapacityBytes,
	}
	return &csi.CreateVolumeResponse{Volume: &volume}, nil
}

// DeleteVolume moves directory for the volume to trash (TODO)
func (d *controllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.V(4).Infof("DeleteVolume: called with args: %#v", req)
	volumeID := req.GetVolumeId()

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerGetCapabilities gets capabilities
func (d *controllerService) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).Infof("ControllerGetCapabilities: called with args %#v", req)
	var caps []*csi.ControllerServiceCapability
	for _, cap := range controllerCaps {
		c := &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}

// GetCapacity unimplemented
func (d *controllerService) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(4).Infof("GetCapacity: called with args %#v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ListVolumes unimplemented
func (d *controllerService) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(4).Infof("ListVolumes: called with args %#v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities validates volume capabilities
func (d *controllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).Infof("ValidateVolumeCapabilities: called with args %#v", req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	secrets := req.Secrets
	klog.V(5).Infof("CreateVolume: ControllerCreateSecrets contains keys %+v", reflect.ValueOf(secrets).MapKeys())

	jfsName := req.VolumeContext["jfsName"]
	volPath := req.VolumeId

	stdoutStderr, err := d.juicefs.CmdAuth(jfsName, secrets)
	klog.V(5).Infof("CreateVolume: authentication output is '%s'\n", stdoutStderr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not auth juicefs %s: %v", jfsName, err)
	}

	if _, err := d.juicefs.SafeMount(jfsName, []string{}); err != nil {
		klog.Errorf("CreateVolume: failed to mount %q", jfsName)
		return nil, err
	}
	exists, err := d.juicefs.ExistsPath(volPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}

	if !exists {
		return nil, status.Errorf(codes.NotFound, "Volume %q not foundd in %q", req.VolumeId, jfsName)
	}

	var confirmed *csi.ValidateVolumeCapabilitiesResponse_Confirmed
	if isValidVolumeCapabilities(volCaps) {
		confirmed = &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volCaps}
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil
}

func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
	hasSupport := func(cap *csi.VolumeCapability) bool {
		for _, c := range volumeCaps {
			if c.GetMode() == cap.AccessMode.GetMode() {
				return true
			}
		}
		return false
	}

	foundAll := true
	for _, c := range volCaps {
		if !hasSupport(c) {
			foundAll = false
		}
	}
	return foundAll
}

// CreateSnapshot unimplemented
func (d *controllerService) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// DeleteSnapshot unimplemented
func (d *controllerService) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ListSnapshots unimplemented
func (d *controllerService) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerExpandVolume unimplemented
func (d *controllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerPublishVolume unimplemented
func (d *controllerService) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerUnpublishVolume unimplemented
func (d *controllerService) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
