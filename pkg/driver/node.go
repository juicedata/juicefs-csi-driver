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
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/dispatch"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
	k8sMount "k8s.io/utils/mount"
)

var (
	nodeCaps = []csi.NodeServiceCapability_RPC_Type{csi.NodeServiceCapability_RPC_GET_VOLUME_STATS}
)

const (
	defaultCheckTimeout = 2 * time.Second
	defaultQuotaPoolNum = 4
)

type nodeService struct {
	quotaPool *dispatch.Pool
	csi.UnimplementedNodeServer
	mount.SafeFormatAndMount
	juicefs   juicefs.Interface
	nodeID    string
	k8sClient *k8sclient.K8sClient
	metrics   *nodeMetrics
}

type nodeMetrics struct {
	volumeErrors    prometheus.Counter
	volumeDelErrors prometheus.Counter
}

func newNodeMetrics(reg prometheus.Registerer) *nodeMetrics {
	metrics := &nodeMetrics{}
	metrics.volumeErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "volume_errors",
		Help: "number of volume errors",
	})
	reg.MustRegister(metrics.volumeErrors)
	metrics.volumeDelErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "volume_del_errors",
		Help: "number of volume delete errors",
	})
	reg.MustRegister(metrics.volumeDelErrors)
	return metrics
}

func newNodeService(nodeID string, k8sClient *k8sclient.K8sClient, reg prometheus.Registerer) (*nodeService, error) {
	mounter := &mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      k8sexec.New(),
	}
	metrics := newNodeMetrics(reg)
	jfsProvider := juicefs.NewJfsProvider(mounter, k8sClient)
	return &nodeService{
		quotaPool:          dispatch.NewPool(defaultQuotaPoolNum),
		SafeFormatAndMount: *mounter,
		juicefs:            jfsProvider,
		nodeID:             nodeID,
		k8sClient:          k8sClient,
		metrics:            metrics,
	}, nil
}

// NodeStageVolume is called by the CO prior to the volume being consumed by any workloads on the node by `NodePublishVolume`
func (d *nodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeUnstageVolume is a reverse operation of `NodeStageVolume`
func (d *nodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodePublishVolume is called by the CO when a workload that wants to use the specified volume is placed (scheduled) on a node
func (d *nodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volCtx := req.GetVolumeContext()
	log := klog.NewKlogr().WithName("NodePublishVolume")
	if volCtx != nil && volCtx[common.PodInfoName] != "" {
		log = log.WithValues("appName", volCtx[common.PodInfoName])
	}
	volumeID := req.GetVolumeId()
	log = log.WithValues("volumeId", volumeID)

	ctxWithLog := util.WithLog(ctx, log)
	secrets := req.Secrets
	req.Secrets = nil
	log.V(1).Info("called with args", "args", req, "secrets", util.StripSecret(secrets))

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}
	log.Info("get volume_capability", "volCap", volCap.String())

	if !isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not supported")
	}

	log.Info("creating dir", "target", target)
	if err := d.juicefs.CreateTarget(ctxWithLog, target); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not create dir %q: %v", target, err)
	}

	options := []string{}
	if req.GetReadonly() || req.VolumeCapability.AccessMode.GetMode() == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
		options = append(options, "ro")
	}
	if m := volCap.GetMount(); m != nil {
		// get mountOptions from PV.spec.mountOptions or StorageClass.mountOptions
		options = append(options, m.MountFlags...)
	}

	log.Info("get volume context", "volCtx", volCtx)

	mountOptions := []string{}
	// get mountOptions from PV.volumeAttributes or StorageClass.parameters
	if opts, ok := volCtx["mountOptions"]; ok {
		mountOptions = strings.Split(opts, ",")
	}
	mountOptions = append(mountOptions, options...)

	log.Info("mounting juicefs", "secret", fmt.Sprintf("%+v", reflect.ValueOf(secrets).MapKeys()), "options", mountOptions)
	jfs, err := d.juicefs.JfsMount(ctxWithLog, volumeID, target, secrets, volCtx, mountOptions)
	if err != nil {
		d.metrics.volumeErrors.Inc()
		return nil, status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
	}

	bindSource, err := jfs.CreateVol(ctxWithLog, volumeID, volCtx["subPath"])
	if err != nil {
		d.metrics.volumeErrors.Inc()
		return nil, status.Errorf(codes.Internal, "Could not create volume: %s, %v", volumeID, err)
	}

	if err := jfs.BindTarget(ctxWithLog, bindSource, target); err != nil {
		d.metrics.volumeErrors.Inc()
		return nil, status.Errorf(codes.Internal, "Could not bind %q at %q: %v", bindSource, target, err)
	}

	if cap, exist := volCtx["capacity"]; exist {
		capacity, err := strconv.ParseInt(cap, 10, 64)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "invalid capacity %s: %v", cap, err)
		}
		settings := jfs.GetSetting()
		if settings.PV != nil {
			capacity = settings.PV.Spec.Capacity.Storage().Value()
		}
		quotaPath := settings.SubPath
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

		d.quotaPool.Run(context.Background(), func(ctx context.Context) {
			err := retry.OnError(retry.DefaultRetry, func(err error) bool { return true }, func() error {
				return d.juicefs.SetQuota(ctx, secrets, settings, path.Join(subdir, quotaPath), capacity)
			})
			if err != nil {
				log.Error(err, "set quota failed")
			}
		})
	}

	log.Info("juicefs volume mounted", "volumeId", volumeID, "target", target)
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume is a reverse operation of NodePublishVolume. This RPC is typically called by the CO when the workload using the volume is being moved to a different node, or all the workload using the volume on a node has finished.
func (d *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	log := klog.NewKlogr().WithName("NodeUnpublishVolume")
	ctxWithLog := util.WithLog(ctx, log)
	log.V(1).Info("called with args", "args", req)

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	volumeId := req.GetVolumeId()
	log.Info("get volume_id", "volumeId", volumeId)

	err := d.juicefs.JfsUnmount(ctxWithLog, volumeId, target)
	if err != nil {
		d.metrics.volumeDelErrors.Inc()
		return nil, status.Errorf(codes.Internal, "Could not unmount %q: %v", target, err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetCapabilities response node capabilities to CO
func (d *nodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	log := klog.NewKlogr().WithName("NodeGetCapabilities")
	log.V(1).Info("called with args", "args", req)
	var caps []*csi.NodeServiceCapability
	for _, cap := range nodeCaps {
		c := &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

// NodeGetInfo is called by CO for the node at which it wants to place the workload. The result of this call will be used by CO in ControllerPublishVolume.
func (d *nodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	log := klog.NewKlogr().WithName("NodeGetInfo")
	log.V(1).Info("called with args", "args", req)

	return &csi.NodeGetInfoResponse{
		NodeId: d.nodeID,
	}, nil
}

// NodeExpandVolume unimplemented
func (d *nodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *nodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	log := klog.NewKlogr().WithName("NodeGetVolumeStats")
	log.V(1).Info("called with args", "args", req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volumePath := req.GetVolumePath()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume path not provided")
	}

	var exists bool

	err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
		exists, err = mount.PathExists(volumePath)
		return
	})
	if err == nil {
		if !exists {
			log.Info("Volume path not exists", "volumePath", volumePath)
			return nil, status.Error(codes.NotFound, "Volume path not exists")
		}
		if d.SafeFormatAndMount.Interface != nil {
			var notMnt bool
			err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
				notMnt, err = mount.IsNotMountPoint(d.SafeFormatAndMount.Interface, volumePath)
				return err
			})
			if err != nil {
				log.Info("Check volume path is mountpoint failed", "volumePath", volumePath, "error", err)
				return nil, status.Errorf(codes.Internal, "Check volume path is mountpoint failed: %s", err)
			}
			if notMnt { // target exists but not a mountpoint
				log.Info("volume path not mounted", "volumePath", volumePath)
				return nil, status.Error(codes.Internal, "Volume path not mounted")
			}
		}
	} else {
		if k8sMount.IsCorruptedMnt(err) {
			go func() {
				if err := resource.HandleCorruptedMountPath(d.k8sClient, volumeID, volumePath); err != nil {
					log.Error(err, "HandleCorruptedMountPath failed", "volumeID", volumeID, "volumePath", volumePath)
				}
			}()
		}
		log.Error(err, "check volume path", "volumePath", volumePath, "error", err)
		return nil, status.Errorf(codes.Internal, "Check volume path, err: %s", err)
	}

	totalSize, freeSize, totalInodes, freeInodes := util.GetDiskUsage(volumePath)
	usedSize := int64(totalSize) - int64(freeSize)
	usedInodes := int64(totalInodes) - int64(freeInodes)

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Available: int64(freeSize),
				Total:     int64(totalSize),
				Used:      usedSize,
				Unit:      csi.VolumeUsage_BYTES,
			},
			{
				Available: int64(freeInodes),
				Total:     int64(totalInodes),
				Used:      usedInodes,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
	}, nil
}
