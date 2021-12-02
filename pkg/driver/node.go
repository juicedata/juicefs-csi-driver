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
	"os"
	"os/exec"
	"reflect"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/utils/mount"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	podmount "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount"
)

const (
	fsTypeNone = "none"
)

var (
	nodeCaps = []csi.NodeServiceCapability_RPC_Type{}
)

type nodeService struct {
	juicefs   juicefs.Interface
	nodeID    string
	k8sClient *k8sclient.K8sClient
}

func newNodeService(nodeID string) (*nodeService, error) {
	jfsProvider, err := juicefs.NewJfsProvider(nil)
	if err != nil {
		panic(err)
	}

	stdoutStderr, err := jfsProvider.Version()
	if err != nil {
		panic(err)
	}
	klog.V(4).Infof("Node: %s", stdoutStderr)

	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		klog.V(5).Infof("Can't get k8s client: %v", err)
		return nil, err
	}

	return &nodeService{
		juicefs:   jfsProvider,
		nodeID:    nodeID,
		k8sClient: k8sClient,
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
	if req.GetReadonly() || req.VolumeCapability.AccessMode.GetMode() == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
		options["ro"] = ""
	}
	if m := volCap.GetMount(); m != nil {
		for _, f := range m.MountFlags {
			options[f] = ""
		}
	}

	volCtx := req.GetVolumeContext()
	klog.V(5).Infof("NodePublishVolume: volume context: %v", volCtx)

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
	jfs, err := d.juicefs.JfsMount(volumeID, target, secrets, volCtx, mountOptions, true)
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
	klog.V(5).Infof("NodeUnpublishVolume: volume_id is %s", volumeId)

	exists, err := mount.PathExists(target)
	notMnt, corruptedMnt := true, false
	if exists && err == nil {
		notMnt, err = mount.IsNotMountPoint(d.juicefs, target)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Check target path is mountpoint failed: %q", err)
		}
		if notMnt { // target exists but not a mountpoint
			klog.V(5).Infof("NodeUnpublishVolume: target %s not mounted", target)
		}
	} else if err != nil {
		if corruptedMnt = mount.IsCorruptedMnt(err); !corruptedMnt {
			return nil, status.Errorf(codes.Internal, "Check target path %s failed: %q", target, err)
		}
		klog.V(5).Infof("NodeUnpublishVolume: target %s is a corrupted mountpoint", target)
	}

	if !notMnt || corruptedMnt {
		klog.V(5).Infof("NodeUnpublishVolume: unmounting %s", target)
		for {
			//err = d.juicefs.Unmount(target)
			out, err := exec.Command("umount", target).CombinedOutput()
			if err == nil {
				continue
			}
			if !strings.Contains(string(out), "not mounted") && !strings.Contains(string(out), "mountpoint not found") {
				klog.V(5).Infof("Unmount %s failed: %q, try to lazy unmount", target, err)
				output, err1 := exec.Command("umount", "-l", target).CombinedOutput()
				if err1 != nil {
					return nil, status.Errorf(codes.Internal, "Could not lazy unmount %q: %v, output: %s", target, err1, string(output))
				}
			}
			klog.V(5).Infof("umount:%s success", target)
			break
		}
	}
	// Related issue: https://github.com/kubernetes/kubernetes/issues/60987
	if exists {
		klog.V(5).Infof("NodeUnpublishVolume: remove target %s", target)
		if err = os.Remove(target); err != nil {
			klog.V(5).Infof("Remove target directory %s failed: %q", target, err)
		}
	}

	mnt := podmount.NewPodMount(nil, d.k8sClient)
	if err := mnt.JUmount(volumeId, target); err != nil {
		return &csi.NodeUnpublishVolumeResponse{}, err
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
