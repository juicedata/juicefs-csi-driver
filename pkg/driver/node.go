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
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

const (
	fsTypeNone = "none"
)

var (
	nodeCaps = []csi.NodeServiceCapability_RPC_Type{}
)

type nodeService struct {
	juicefs juicefs.Interface
	nodeID  string
}

func newNodeService(nodeID string) nodeService {
	jfsProvider, err := juicefs.NewJfsProvider(nil)
	if err != nil {
		panic(err)
	}

	stdoutStderr, err := jfsProvider.Version()
	if err != nil {
		panic(err)
	}
	klog.V(4).Infof("Node: %s", stdoutStderr)

	go func() {
		metricsPort := 9567
		if v, ok := os.LookupEnv("JFS_METRICS_PORT"); ok {
			if i, err := strconv.Atoi(v); err != nil || i <= 0 || i >= 65536 {
				klog.V(4).Infof("Skip invalid JuiceFS metrics port %s", v)
			} else {
				metricsPort = i
			}
		}
		klog.V(4).Infof("Serve metrics on :%d", metricsPort)
		jfsProvider.ServeMetrics(metricsPort)
	}()

	return nodeService{
		juicefs: jfsProvider,
		nodeID:  nodeID,
	}
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
	// WARNING: debug only, secrets included
	// klog.V(5).Infof("NodePublishVolume: called with args %+v", req)

	volumeID := req.GetVolumeId()

	klog.V(5).Infof("NodePublishVolume: volume_id is %s", volumeID)

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}
	klog.V(5).Infof("NodePublishVolume: volume_capability is %s", volCap)

	if !isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not supported")
	}

	klog.V(5).Infof("NodePublishVolume: creating dir %s", target)
	if err := os.MkdirAll(target, os.FileMode(0755)); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not create dir %q: %v", target, err)
	}

	options := make(map[string]string)
	if req.GetReadonly() {
		options["ro"] = ""
	}
	if m := volCap.GetMount(); m != nil {
		for _, f := range m.MountFlags {
			options[f] = ""
		}
	}

	volCtx := req.GetVolumeContext()

	secrets := req.Secrets
	mountOptions := []string{}
	if opts, ok := volCtx["mountOptions"]; ok {
		mountOptions = strings.Split(opts, ",")
	}
	for k, v := range options {
		if v != "" {
			k = fmt.Sprintf("%s=%s", k, v)
		}
		mountOptions = append(mountOptions, k)
	}

	klog.V(5).Infof("NodePublishVolume: mounting juicefs with secret %+v, options %v", reflect.ValueOf(secrets).MapKeys(), mountOptions)
	jfs, err := d.juicefs.JfsMount(volumeID, target, secrets, mountOptions)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
	}

	bindSource, err := jfs.CreateVol(volumeID, volCtx["subPath"])
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not create volume: %s, %v", volumeID, err)
	}

	klog.V(5).Infof("NodePublishVolume: binding %s at %s with options %v", bindSource, target, mountOptions)
	if err := d.juicefs.Mount(bindSource, target, fsTypeNone, []string{"bind"}); err != nil {
		os.Remove(target)
		return nil, status.Errorf(codes.Internal, "Could not bind %q at %q: %v", bindSource, target, err)
	}

	klog.V(5).Infof("NodePublishVolume: mounted %s at %s with options %v", volumeID, target, mountOptions)
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume is a reverse operation of NodePublishVolume. This RPC is typically called by the CO when the workload using the volume is being moved to a different node, or all the workload using the volume on a node has finished.
func (d *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.V(4).Infof("NodeUnpublishVolume: called with args %+v", req)

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	volumeId := req.GetVolumeId()
	klog.V(5).Infof("NodePublishVolume: volume_id is %s", volumeId)

	// check mount pod is need to delete
	klog.V(5).Infof("Check mount pod is need to delete or not.")
	k8sClient, err := juicefs.NewClient()
	if err != nil {
		klog.V(5).Infof("Can't get k8s client: %v", err)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	pod, err := juicefs.GetPod(k8sClient, volumeId)
	if err != nil {
		klog.V(5).Infof("Can't get pod of volumeId %s: %v", volumeId, err)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	cm, err := juicefs.GetConfigMap(k8sClient, volumeId)
	if err != nil {
		klog.V(5).Infof("Can't get configMap of volumeId %s: %v", volumeId, err)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	cmRefs := cm.Data
	key := base64.StdEncoding.EncodeToString([]byte(target))
	if _, ok := cmRefs[key]; ok {
		var lock sync.RWMutex
		lock.RLock()
		delete(cmRefs, key)
		lock.Unlock()
		if len(cmRefs) == 0 {
			// delete pod & cm of volumeId when refs is last one.
			if err := juicefs.DeleteConfigMap(k8sClient, cm); err != nil {
				klog.V(5).Infof("Delete configMap of volumeId %s error: %v", volumeId, err)
				return &csi.NodeUnpublishVolumeResponse{}, nil
			}
			if err := juicefs.DeletePod(k8sClient, pod); err != nil {
				klog.V(5).Infof("Delete pod of volumeId %s error: %v", volumeId, err)
				return &csi.NodeUnpublishVolumeResponse{}, nil
			}
		} else {
			// delete ref of volumeId in cm.
			cm.Data = cmRefs
			if err := juicefs.UpdateConfigMap(k8sClient, cm); err != nil {
				klog.V(5).Infof("Update configMap of volumeId %s error: %v", volumeId, err)
				return &csi.NodeUnpublishVolumeResponse{}, nil
			}
		}
	}

	var corruptedMnt bool
	exists, err := mount.PathExists(target)
	if err == nil {
		if !exists {
			klog.V(5).Infof("NodeUnpublishVolume: %s target not exists", target)
			return &csi.NodeUnpublishVolumeResponse{}, nil
		}
		var notMnt bool
		notMnt, err = mount.IsNotMountPoint(d.juicefs, target)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Check target path is mountpoint failed: %q", err)
		}
		if notMnt { // target exists but not a mountpoint
			klog.V(5).Infof("NodeUnpublishVolume: %s target not mounted", target)
			return &csi.NodeUnpublishVolumeResponse{}, nil
		}
	} else if corruptedMnt = mount.IsCorruptedMnt(err); !corruptedMnt {
		return nil, status.Errorf(codes.Internal, "Check path %s failed: %q", target, err)
	}

	var refs []string
	refs, err = getMountDeviceRefs(target, corruptedMnt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Fail to get mount device refs: %q", err)
	}

	klog.V(5).Infof("NodeUnpublishVolume: unmounting %s", target)
	if err := d.juicefs.Unmount(target); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not unmount %q: %v", target, err)
	}

	klog.V(5).Infof("NodeUnpublishVolume: unmounting ref for target %s", target)
	// we can only unmount this when only one is left
	// since the PVC might be used by more than one container
	if err == nil && len(refs) == 1 {
		klog.V(5).Infof("NodeUnpublishVolume: unmounting ref %s", refs[0])
		if err := d.juicefs.JfsUnmount(refs[0]); err != nil {
			klog.V(5).Infof("NodeUnpublishVolume: error unmounting mount ref %s, %v", refs[0], err)
		}
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetCapabilities response node capabilities to CO
func (d *nodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(4).Infof("NodeGetCapabilities: called with args %+v", req)
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
	klog.V(4).Infof("NodeGetInfo: called with args %+v", req)

	return &csi.NodeGetInfoResponse{
		NodeId: d.nodeID,
	}, nil
}

// NodeExpandVolume unimplemented
func (d *nodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *nodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeGetVolumeStats is not implemented yet")
}
