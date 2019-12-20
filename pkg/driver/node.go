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
	"path"
	"reflect"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

const (
	jfsCmd     = "/usr/bin/juicefs"
	jfsMntRoot = "/jfs"

	fsTypeJuiceFS = "juicefs"
	fsTypeNone    = "none"
)

var (
	nodeCaps = []csi.NodeServiceCapability_RPC_Type{}
)

// NodeStageVolume is called by the CO prior to the volume being consumed by any workloads on the node by `NodePublishVolume`
func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeUnstageVolume is a reverse operation of `NodeStageVolume`
func (d *Driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodePublishVolume is called by the CO when a workload that wants to use the specified volume is placed (scheduled) on a node
func (d *Driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// TODO(yujunz): hide NodePublishSecrets from log
	// klog.V(5).Infof("NodePublishVolume: called with args %+v", req)

	source := req.GetVolumeId()

	klog.V(5).Infof("NodePublishVolume: volume_id is %s", source)

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}
	klog.V(5).Infof("NodePublishVolume: volume_capability is %s", volCap)

	if !d.isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not supported")
	}

	klog.V(5).Infof("NodePublishVolume: creating dir %s", target)
	if err := d.mounter.MakeDir(target); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not create dir %q: %v", target, err)
	}

	secrets := req.NodePublishSecrets
	klog.V(5).Infof("NodePublishVolume: NodePublishSecret contains keys %+v", reflect.ValueOf(secrets).MapKeys())

	stdoutStderr, err := d.juicefsAuth(source, secrets)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not auth juicefs: %v", stdoutStderr)
	}

	klog.V(5).Infof("NodePublishVolume: authentication output is %s\n", stdoutStderr)

	options := make(map[string]string)
	if req.GetReadonly() {
		options["ro"] = ""
	}
	if m := volCap.GetMount(); m != nil {
		for _, f := range m.MountFlags {
			options[f] = ""
		}
	}

	var mountOptions = []string{}

	if opts, ok := req.GetVolumeAttributes()["mountOptions"]; ok {
		mountOptions = strings.Split(opts, ",")
	}

	for k, v := range options {
		if v != "" {
			k = fmt.Sprintf("%s=%s", k, v)
		}
		mountOptions = append(mountOptions, k)
	}

	jfsMnt := path.Join(jfsMntRoot, source)

	notMntPoint, err := d.mounter.IsLikelyNotMountPoint(jfsMnt)

	if os.IsNotExist(err) || notMntPoint {
		klog.V(5).Infof("NodePublishVolume: mounting %q at %q with options %v", source, jfsMnt, mountOptions)
		if err := d.mounter.Mount(source, jfsMnt, fsTypeJuiceFS, mountOptions); err != nil {
			os.Remove(jfsMnt)
			return nil, status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, target, err)
		}
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not check mount point %q: %v", jfsMnt, err)
	} else {
		klog.V(5).Infof("NodePublishVolume: skip mounting for mount point %q", jfsMnt)
	}

	bindSource := jfsMnt
	if bindDir, ok := req.GetVolumeAttributes()["bindDir"]; ok {
		bindSource = path.Join(jfsMnt, bindDir)
	}
	klog.V(5).Infof("NodePublishVolume: binding %s at %s with options %v", source, jfsMnt, mountOptions)
	if err := d.mounter.Mount(bindSource, target, fsTypeNone, []string{"bind"}); err != nil {
		os.Remove(target)
		return nil, status.Errorf(codes.Internal, "Could not bind %q at %q: %v", bindSource, target, err)
	}

	klog.V(5).Infof("NodePublishVolume: mounted %s at %s with options %v", source, target, mountOptions)
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume is a reverse operation of NodePublishVolume. This RPC is typically called by the CO when the workload using the volume is being moved to a different node, or all the workload using the volume on a node has finished.
func (d *Driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.V(4).Infof("NodeUnpublishVolume: called with args %+v", req)

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	klog.V(5).Infof("NodeUnpublishVolume: unmounting %s", target)
	err := d.mounter.Unmount(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not unmount %q: %v", target, err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetCapabilities response node capabilities to CO
func (d *Driver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
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
func (d *Driver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(4).Infof("NodeGetInfo: called with args %+v", req)

	return &csi.NodeGetInfoResponse{
		NodeId: d.nodeID,
	}, nil
}

// NodeGetId is called by the CO SHOULD call this RPC for the node at which it wants to place the workload. The result of this call will be used by CO in ControllerPublishVolume.
func (d *Driver) NodeGetId(ctx context.Context, req *csi.NodeGetIdRequest) (*csi.NodeGetIdResponse, error) {
	klog.V(4).Infof("NodeGetId: called with args %+v", req)
	return &csi.NodeGetIdResponse{
		NodeId: d.nodeID,
	}, nil
}

func (d *Driver) isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
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
