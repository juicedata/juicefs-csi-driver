package driver

import (
	"context"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var (
	volumeCaps = []csi.VolumeCapability_AccessMode{
		{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
		{
			Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
		},
		{
			Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
		},
	}

	controllerCaps = []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	}
)

type controllerService struct {
	juicefs juicefs.Interface
	vols    map[string]int64
}

func newControllerService(k8sClient *k8sclient.K8sClient) (controllerService, error) {
	jfs := juicefs.NewJfsProvider(nil, k8sClient)

	return controllerService{
		juicefs: jfs,
		vols:    make(map[string]int64),
	}, nil
}

// CreateVolume create directory in an existing JuiceFS filesystem
func (d *controllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// DEBUG only, secrets exposed in args
	// klog.V(5).Infof("CreateVolume: called with args: %#v", req)

	if len(req.Name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume Name cannot be empty")
	}
	if req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities cannot be empty")
	}
	// CSI doesn't provide a capability query for block volumes, so COs will simply pass through
	// requests for block volume creation to CSI plugins, and plugins are allowed to fail with
	// the InvalidArgument GRPC error code if they don't support block volumes.
	if !isValidVolumeCapabilities(req.VolumeCapabilities) {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities not fully supported")
	}

	volumeId := req.Name
	subPath := req.Name
	secrets := req.Secrets
	klog.V(5).Infof("CreateVolume: Secrets contains keys %+v", reflect.ValueOf(secrets).MapKeys())

	requiredCap := req.CapacityRange.GetRequiredBytes()
	if capa, ok := d.vols[req.Name]; ok && capa < requiredCap {
		return nil, status.Errorf(codes.AlreadyExists, "Volume: %q, capacity bytes: %d", req.Name, requiredCap)
	}
	d.vols[req.Name] = requiredCap

	// set volume context
	volCtx := make(map[string]string)
	for k, v := range req.Parameters {
		if strings.HasPrefix(v, "$") {
			klog.Warningf("CreateVolume: volume %s parameters %s uses template pattern, please enable provisioner in CSI Controller, not works in default mode.", volumeId, k)
		}
		volCtx[k] = v
	}
	// return error if set readonly in dynamic provisioner
	for _, vc := range req.VolumeCapabilities {
		if vc.AccessMode.GetMode() == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
			return nil, status.Errorf(codes.InvalidArgument, "Dynamic mounting uses the sub-path named pv name as data isolation, so read-only mode cannot be used.")
		}
	}
	// create volume
	//err := d.juicefs.JfsCreateVol(ctx, volumeId, subPath, secrets, volCtx)
	//if err != nil {
	//	return nil, status.Errorf(codes.Internal, "Could not createVol in juicefs: %v", err)
	//}

	// check if use pathpattern
	if req.Parameters["pathPattern"] != "" {
		klog.Warningf("CreateVolume: volume %s uses pathPattern, please enable provisioner in CSI Controller, not works in default mode.", volumeId)
	}
	// check if use secretFinalizer
	if req.Parameters["secretFinalizer"] == "true" {
		klog.Warningf("CreateVolume: volume %s uses secretFinalizer, please enable provisioner in CSI Controller, not works in default mode.", volumeId)
	}

	volCtx["subPath"] = subPath
	volCtx["capacity"] = strconv.FormatInt(requiredCap, 10)
	volume := csi.Volume{
		VolumeId:      volumeId,
		CapacityBytes: requiredCap,
		VolumeContext: volCtx,
	}
	return &csi.CreateVolumeResponse{Volume: &volume}, nil
}

// DeleteVolume moves directory for the volume to trash (TODO)
func (d *controllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	// check pv if dynamic
	dynamic, err := util.CheckDynamicPV(volumeID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Check Volume ID error: %v", err)
	}
	if !dynamic {
		klog.V(5).Infof("Volume %s not dynamic PV, ignore.", volumeID)
		return &csi.DeleteVolumeResponse{}, nil
	}

	secrets := req.Secrets
	klog.V(5).Infof("DeleteVolume: Secrets contains keys %+v", reflect.ValueOf(secrets).MapKeys())
	if len(secrets) == 0 {
		klog.V(5).Infof("DeleteVolume: Secrets is empty, skip.")
		return &csi.DeleteVolumeResponse{}, nil
	}

	klog.V(5).Infof("DeleteVolume: Deleting volume %q", volumeID)
	err = d.juicefs.JfsDeleteVol(ctx, volumeID, volumeID, secrets, nil, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not delVol in juicefs: %v", err)
	}

	delete(d.vols, volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerGetCapabilities gets capabilities
func (d *controllerService) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(6).Infof("ControllerGetCapabilities: called with args %#v", req)
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
	klog.V(6).Infof("GetCapacity: called with args %#v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ListVolumes unimplemented
func (d *controllerService) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(6).Infof("ListVolumes: called with args %#v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities validates volume capabilities
func (d *controllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(6).Infof("ValidateVolumeCapabilities: called with args %#v", req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	if _, ok := d.vols[volumeID]; !ok {
		return nil, status.Errorf(codes.NotFound, "Could not get volume by ID %q", volumeID)
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
		switch cap.GetAccessType().(type) {
		case *csi.VolumeCapability_Block:
			return false
		case *csi.VolumeCapability_Mount:
			break
		default:
			return false
		}
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

// ControllerExpandVolume adjusts quota according to capacity settings
func (d *controllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(6).Infof("ControllerExpandVolume request: %+v", *req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	// get cap
	capRange := req.GetCapacityRange()
	if capRange == nil {
		return nil, status.Error(codes.InvalidArgument, "Capacity range not provided")
	}

	newSize := capRange.GetRequiredBytes()
	maxVolSize := capRange.GetLimitBytes()
	if maxVolSize > 0 && maxVolSize < newSize {
		return nil, status.Error(codes.InvalidArgument, "After round-up, volume size exceeds the limit specified")
	}

	// get mount options
	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}
	klog.V(5).Infof("NodePublishVolume: volume_capability is %s", volCap)
	options := []string{}
	if m := volCap.GetMount(); m != nil {
		// get mountOptions from PV.spec.mountOptions or StorageClass.mountOptions
		options = append(options, m.MountFlags...)
	}

	capacity, err := strconv.ParseInt(strconv.FormatInt(newSize, 10), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid capacity %d: %v", capacity, err)
	}

	// get quota path
	quotaPath, err := d.juicefs.GetSubPath(ctx, volumeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get quotaPath error: %v", err)
	}
	settings, err := d.juicefs.Settings(ctx, volumeID, req.GetSecrets(), nil, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get settings: %v", err)
	}

	var subdir string
	for _, o := range settings.Options {
		pair := strings.Split(o, "=")
		if len(pair) != 2 {
			continue
		}
		if pair[0] == "subdir" {
			subdir = path.Join("/", pair[1])
		}
	}

	err = d.juicefs.SetQuota(ctx, req.GetSecrets(), settings, path.Join(subdir, quotaPath), capacity)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set quota: %v", err)
	}
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         newSize,
		NodeExpansionRequired: false,
	}, nil
}

// ControllerPublishVolume unimplemented
func (d *controllerService) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerUnpublishVolume unimplemented
func (d *controllerService) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *controllerService) ControllerGetVolume(ctx context.Context, request *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
