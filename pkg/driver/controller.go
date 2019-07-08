package driver

import (
	"context"
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
	klog.V(5).Infof("CreateVolume: Secrets contains keys %+v", reflect.ValueOf(secrets).MapKeys())

	capacityBytes := int64(req.CapacityRange.GetRequiredBytes())

	klog.V(5).Infof("CreateVolume: Mounting juicefs %q", req.Name)
	jfs, err := d.juicefs.JfsMount(secrets, []string{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
	}

	klog.V(5).Infof("CreateVolume: Creating volume %q, capacity bytes: %d", req.Name, capacityBytes)
	vol, err := jfs.CreateVol(req.Name, capacityBytes)
	if status.Convert(err).Code() == codes.AlreadyExists {
		return nil, err
	}

	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Could not create volume %q size %d bytes: %v", req.Name, capacityBytes, err)
	}

	volume := csi.Volume{
		VolumeId:      req.Name,
		CapacityBytes: vol.CapacityBytes,
		VolumeContext: req.Parameters,
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

	secrets := req.Secrets
	klog.V(5).Infof("DeleteVolume: Secrets contains keys %+v", reflect.ValueOf(secrets).MapKeys())

	jfs, err := d.juicefs.JfsMount(secrets, []string{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
	}

	klog.V(5).Infof("DeleteVolume: Deleting volume %q", volumeID)
	err = jfs.DeleteVol(volumeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not delete volume: %q", volumeID)
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
	jfs, err := d.juicefs.JfsMount(secrets, []string{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not mount juicefs with secret %v: %v", reflect.ValueOf(secrets).MapKeys(), err)
	}

	volID := req.VolumeId
	if _, err := jfs.GetVolByID(volID); err != nil {
		return nil, status.Errorf(codes.NotFound, "Could not get volume by ID %q", volID)
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
