package driver

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog/v2"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/dispatch"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
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
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
	}
)

type controllerService struct {
	csi.UnimplementedControllerServer
	juicefs      juicefs.Interface
	vols         map[string]int64
	volLocks     *resource.VolumeLocks
	quotaPool    *dispatch.Pool
	snapshots    map[string]*csi.Snapshot
	snapshotLock sync.Mutex
}

func newControllerService(k8sClient *k8sclient.K8sClient) (controllerService, error) {
	jfs := juicefs.NewJfsProvider(nil, k8sClient)

	return controllerService{
		juicefs:   jfs,
		vols:      make(map[string]int64),
		volLocks:  resource.NewVolumeLocks(),
		quotaPool: dispatch.NewPool(defaultQuotaPoolNum),
		snapshots: make(map[string]*csi.Snapshot),
	}, nil
}

func (d *controllerService) setQuotaInController(
	ctx context.Context,
	volumeId string,
	capacityRange *csi.CapacityRange,
	mountOptions []string,
	subPath string,
	secrets map[string]string,
	volCtx map[string]string) error {

	log := klog.NewKlogr().WithName("setQuotaInController")
	subdir := util.ParseSubdirFromMountOptions(mountOptions)
	quotaPath := path.Join("/", subdir, subPath)
	if capacityRange != nil && capacityRange.RequiredBytes > 0 {
		capacity := capacityRange.RequiredBytes
		log.V(1).Info("setting quota in controller", "volumeId", volumeId, "name", secrets["name"], "path", quotaPath, "capacity", capacity)

		settings, err := d.juicefs.Settings(ctx, volumeId, volumeId, secrets["name"], secrets, volCtx, mountOptions)
		if err != nil {
			log.Error(err, "failed to get settings for quota")
			return status.Errorf(codes.Internal, "Could not get settings for quota: %v", err)
		}

		if err := d.juicefs.SetQuota(ctx, secrets, settings, quotaPath, capacity); err != nil {
			log.Error(err, "failed to set quota in controller", "quotaPath", quotaPath, "capacity", capacity)
			return status.Errorf(codes.Internal, "Could not set quota: %v", err)
		}
	}
	return nil
}

// CreateVolume create directory in an existing JuiceFS filesystem
func (d *controllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	log := klog.NewKlogr().WithName("CreateVolume")
	// DEBUG only, secrets exposed in args
	// klog.Infof("CreateVolume: called with args: %#v", req)

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
	log.Info("Secrets contains keys", "secretKeys", reflect.ValueOf(secrets).MapKeys())

	// Check if restoring from snapshot
	var snapshotID string
	var sourceVolumeID string
	if req.VolumeContentSource != nil {
		if snapshot := req.VolumeContentSource.GetSnapshot(); snapshot != nil {
			snapshotID = snapshot.GetSnapshotId()
			log.Info("Creating volume from snapshot", "volumeId", volumeId, "snapshotID", snapshotID)

			// Try in-memory first
			d.snapshotLock.Lock()
			if existingSnapshot, ok := d.snapshots[snapshotID]; ok {
				sourceVolumeID = existingSnapshot.SourceVolumeId
				log.Info("Found source volume from memory", "snapshotID", snapshotID, "sourceVolumeID", sourceVolumeID)
			}
			d.snapshotLock.Unlock()

			// If not in memory, extract from snapshotID which encodes source volume
			// The snapshotID format is: snapshot-<uuid>|<source-volume-id>
			if sourceVolumeID == "" && strings.Contains(snapshotID, "|") {
				parts := strings.Split(snapshotID, "|")
				if len(parts) == 2 {
					sourceVolumeID = parts[1]
					log.Info("Extracted source volume from snapshot handle", "snapshotID", snapshotID, "sourceVolumeID", sourceVolumeID)
				}
			}
		}
	}

	requiredCap := req.CapacityRange.GetRequiredBytes()
	if capa, ok := d.vols[req.Name]; ok && capa < requiredCap {
		return nil, status.Errorf(codes.AlreadyExists, "Volume: %q, capacity bytes: %d", req.Name, requiredCap)
	}
	d.vols[req.Name] = requiredCap

	// set volume context
	volCtx := make(map[string]string)
	for k, v := range req.Parameters {
		if strings.HasPrefix(v, "$") {
			log.Info("volume parameters uses template pattern, please enable provisioner in CSI Controller, not works in default mode.", "volumeId", volumeId, "parameters", k)
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
		log.Info("volume uses pathPattern, please enable provisioner in CSI Controller, not works in default mode.", "volumeId", volumeId)
	}
	// check if use secretFinalizer
	if req.Parameters["secretFinalizer"] == "true" {
		log.Info("volume uses secretFinalizer, please enable provisioner in CSI Controller, not works in default mode.", "volumeId", volumeId)
	}

	options := []string{}
	for _, vc := range req.VolumeCapabilities {
		if m := vc.GetMount(); m != nil {
			options = append(options, m.MountFlags...)
		}
	}

	if config.GlobalConfig.EnableSetQuota == nil || *config.GlobalConfig.EnableSetQuota {
		if config.GlobalConfig.EnableControllerSetQuota == nil || *config.GlobalConfig.EnableControllerSetQuota {
			if util.SupportQuotaPathCreate(true, config.BuiltinCeVersion) && util.SupportQuotaPathCreate(false, config.BuiltinEeVersion) {
				volCtx[common.ControllerQuotaSetKey] = "true"
				d.quotaPool.Run(context.Background(), func(ctx context.Context) {
					if err := d.setQuotaInController(ctx, volumeId, req.GetCapacityRange(), options, subPath, secrets, volCtx); err != nil {
						log.Error(err, "set quota in controller error")
					}
				})
			}
		}
	}

	volCtx["subPath"] = subPath
	volCtx["capacity"] = strconv.FormatInt(requiredCap, 10)

	// If creating from snapshot, store snapshot info for restoration during mount
	if snapshotID != "" {
		volCtx["restoreFromSnapshot"] = snapshotID
		volCtx["restoreFromSourceVolume"] = sourceVolumeID
		log.Info("Volume will be restored from snapshot during first mount", "volumeId", volumeId, "snapshotID", snapshotID, "sourceVolumeID", sourceVolumeID)
	}

	volume := csi.Volume{
		VolumeId:      volumeId,
		CapacityBytes: requiredCap,
		VolumeContext: volCtx,
		ContentSource: req.VolumeContentSource,
	}
	return &csi.CreateVolumeResponse{Volume: &volume}, nil
}

// DeleteVolume moves directory for the volume to trash (TODO)
func (d *controllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	log := klog.NewKlogr().WithName("DeleteVolume")
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
		log.Info("Volume is not dynamic PV, ignore.", "volumeId", volumeID)
		return &csi.DeleteVolumeResponse{}, nil
	}

	secrets := req.Secrets
	log.Info("Secrets contains keys", "secretKeys", reflect.ValueOf(secrets).MapKeys())
	if len(secrets) == 0 {
		log.Info("Secrets is empty, skip.")
		return &csi.DeleteVolumeResponse{}, nil
	}

	if acquired := d.volLocks.TryAcquire(volumeID); !acquired {
		log.Info("Volume is being used by another operation", "volumeId", volumeID)
		return nil, status.Errorf(codes.Aborted, "DeleteVolume: Volume %q is being used by another operation", volumeID)
	}
	defer d.volLocks.Release(volumeID)

	log.Info("Deleting volume", "volumeId", volumeID)
	err = d.juicefs.JfsDeleteVol(ctx, volumeID, volumeID, secrets, nil, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not delVol in juicefs: %v", err)
	}

	delete(d.vols, volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerGetCapabilities gets capabilities
func (d *controllerService) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	log := klog.NewKlogr().WithName("ControllerGetCapabilities")
	log.V(1).Info("called with args", "args", req)
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
	log := klog.NewKlogr().WithName("GetCapacity")
	log.V(1).Info("called with args", "args", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ListVolumes unimplemented
func (d *controllerService) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	log := klog.NewKlogr().WithName("ListVolumes")
	log.V(1).Info("called with args", "args", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities validates volume capabilities
func (d *controllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	log := klog.NewKlogr().WithName("ValidateVolumeCapabilities")
	secrets := req.Secrets
	req.Secrets = nil
	log.V(1).Info("called with args", "args", req, "secrets", util.StripSecret(secrets))
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
		for i := range volumeCaps {
			if volumeCaps[i].GetMode() == cap.AccessMode.GetMode() {
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

// CreateSnapshot creates a snapshot of a volume
func (d *controllerService) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	log := klog.NewKlogr().WithName("CreateSnapshot")
	log.V(1).Info("called with args", "args", req)

	// Validate input
	sourceVolumeID := req.GetSourceVolumeId()
	if len(sourceVolumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Source volume ID cannot be empty")
	}

	snapshotName := req.GetName()
	if len(snapshotName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Snapshot name cannot be empty")
	}

	secrets := req.GetSecrets()
	log.Info("Secrets contains keys", "secretKeys", reflect.ValueOf(secrets).MapKeys())

	// Check if snapshot already exists (idempotency)
	d.snapshotLock.Lock()
	if existingSnapshot, ok := d.snapshots[snapshotName]; ok {
		d.snapshotLock.Unlock()
		log.Info("snapshot already exists", "snapshotName", snapshotName)
		return &csi.CreateSnapshotResponse{
			Snapshot: existingSnapshot,
		}, nil
	}
	d.snapshotLock.Unlock()

	// Get the source volume's subpath
	subPath, err := d.juicefs.GetSubPath(ctx, sourceVolumeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get subpath for volume %q: %v", sourceVolumeID, err)
	}

	sourcePath := fmt.Sprintf("/%s", subPath)
	if subPath == "" || subPath == sourceVolumeID {
		sourcePath = fmt.Sprintf("/%s", sourceVolumeID)
	}

	// Get volume context - try to retrieve PV if available
	volCtx := make(map[string]string)
	// Volume context could be populated from PV if needed in the future
	log.V(1).Info("creating snapshot", "sourceVolumeID", sourceVolumeID, "sourcePath", sourcePath)

	// Create the snapshot
	err = d.juicefs.CreateSnapshot(ctx, snapshotName, sourceVolumeID, sourcePath, secrets, volCtx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not create snapshot: %v", err)
	}

	// Build snapshot response
	// Encode source volume ID in snapshot handle for stateless restore
	// Format: snapshot-<uuid>|<source-volume-id>
	snapshotHandle := fmt.Sprintf("%s|%s", snapshotName, sourceVolumeID)

	creationTime := timestamppb.Now()
	snapshot := &csi.Snapshot{
		SnapshotId:     snapshotHandle,
		SourceVolumeId: sourceVolumeID,
		CreationTime:   creationTime,
		ReadyToUse:     true,
	}

	// Store snapshot metadata
	d.snapshotLock.Lock()
	d.snapshots[snapshotName] = snapshot
	d.snapshotLock.Unlock()

	log.Info("snapshot created successfully", "snapshotID", snapshotName, "sourceVolumeID", sourceVolumeID)
	return &csi.CreateSnapshotResponse{
		Snapshot: snapshot,
	}, nil
}

// DeleteSnapshot deletes a snapshot
func (d *controllerService) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	log := klog.NewKlogr().WithName("DeleteSnapshot")
	log.V(1).Info("called with args", "args", req)

	// Validate input
	snapshotID := req.GetSnapshotId()
	if len(snapshotID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Snapshot ID cannot be empty")
	}

	secrets := req.GetSecrets()
	log.Info("Secrets contains keys", "secretKeys", reflect.ValueOf(secrets).MapKeys())

	// Check if snapshot exists
	d.snapshotLock.Lock()
	_, exists := d.snapshots[snapshotID]
	d.snapshotLock.Unlock()

	if !exists {
		// Snapshot doesn't exist in our map, but try to delete it anyway for idempotency
		log.Info("snapshot not found in map, attempting deletion anyway", "snapshotID", snapshotID)
	}

	// Delete the snapshot
	snapshotPath := fmt.Sprintf("/.snapshots/%s", snapshotID)
	err := d.juicefs.DeleteSnapshot(ctx, snapshotID, snapshotPath, secrets)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not delete snapshot: %v", err)
	}

	// Remove from our snapshot map
	d.snapshotLock.Lock()
	delete(d.snapshots, snapshotID)
	d.snapshotLock.Unlock()

	log.Info("snapshot deleted successfully", "snapshotID", snapshotID)
	return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots lists snapshots
func (d *controllerService) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	log := klog.NewKlogr().WithName("ListSnapshots")
	log.V(1).Info("called with args", "args", req)

	d.snapshotLock.Lock()
	defer d.snapshotLock.Unlock()

	var entries []*csi.ListSnapshotsResponse_Entry

	// Filter by snapshot ID if specified
	if snapshotID := req.GetSnapshotId(); snapshotID != "" {
		if snapshot, ok := d.snapshots[snapshotID]; ok {
			entries = append(entries, &csi.ListSnapshotsResponse_Entry{
				Snapshot: snapshot,
			})
		}
		return &csi.ListSnapshotsResponse{
			Entries: entries,
		}, nil
	}

	// Filter by source volume ID if specified
	sourceVolumeID := req.GetSourceVolumeId()
	for _, snapshot := range d.snapshots {
		if sourceVolumeID == "" || snapshot.SourceVolumeId == sourceVolumeID {
			entries = append(entries, &csi.ListSnapshotsResponse_Entry{
				Snapshot: snapshot,
			})
		}
	}

	log.Info("listed snapshots", "count", len(entries))
	return &csi.ListSnapshotsResponse{
		Entries: entries,
	}, nil
}

// ControllerExpandVolume adjusts quota according to capacity settings
func (d *controllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if config.GlobalConfig.EnableSetQuota != nil && !*config.GlobalConfig.EnableSetQuota {
		return nil, status.Error(codes.InvalidArgument, "EnableSetQuota is false in config, skipping set quota")
	}
	log := klog.NewKlogr().WithName("ControllerExpandVolume")
	secrets := req.Secrets
	req.Secrets = nil
	log.V(1).Info("called with args", "args", req, "secrets", util.StripSecret(secrets))

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
	options := []string{}
	volCap := req.GetVolumeCapability()
	if volCap != nil {
		log.Info("volume capability", "volCap", volCap)
		if m := volCap.GetMount(); m != nil {
			// get mountOptions from PV.spec.mountOptions or StorageClass.mountOptions
			options = append(options, m.MountFlags...)
		}
	}

	subPath, err := d.juicefs.GetSubPath(ctx, volumeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get subpath error: %v", err)
	}

	if err := d.setQuotaInController(ctx, volumeID, capRange, options, subPath, secrets, nil); err != nil {
		return nil, err
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

func (d *Driver) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
