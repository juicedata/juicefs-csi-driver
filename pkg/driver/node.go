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
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

const fsType = "juicefs"
const jfsCmd = "/usr/bin/juicefs"

var (
	nodeCaps = []csi.NodeServiceCapability_RPC_Type{}
)

func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// TODO(yujunz): hide NodePublishSecrets from log
	// klog.V(4).Infof("NodePublishVolume: called with args %+v", req)

	source := req.GetVolumeId()

	klog.V(4).Infof("NodePublishVolume: volume_id is %s", source)

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}
	klog.V(4).Infof("NodePublishVolume: volume_capability is %s", volCap)

	if !d.isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not supported")
	}

	klog.V(5).Infof("NodePublishVolume: creating dir %s", target)
	if err := d.mounter.MakeDir(target); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not create dir %q: %v", target, err)
	}

	secrets := req.NodePublishSecrets
	if secrets == nil || secrets["token"] == "" {
		return nil, status.Error(codes.InvalidArgument, "No secrets provided")
	}

	token := secrets["token"]
	args := []string{"auth", source, "--token", token}
	keys := []string{"accesskey", "secretkey", "accesskey2", "secretkey2"}
	for _, k := range keys {
		v := secrets[k]
		args = append(args, "--"+k)
		if v != "" {
			args = append(args, v)
		} else {
			args = append(args, "''")
		}
	}
	stdoutStderr, err := d.exec.Run(jfsCmd, args...)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
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
	for k, v := range req.GetVolumeAttributes() {
		options[k] = v
	}
	var mountOptions []string
	for k, v := range options {
		if v != "" {
			k = fmt.Sprintf("%s=%s", k, v)
		}
		mountOptions = append(mountOptions, k)
	}
	klog.V(5).Infof("NodePublishVolume: mount options: %s", strings.Join(mountOptions, ","))

	klog.V(5).Infof("NodePublishVolume: mounting %s at %s with options %v", source, target, mountOptions)
	if err := d.mounter.Mount(source, target, fsType, mountOptions); err != nil {
		os.Remove(target)
		return nil, status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, target, err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

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

func (d *Driver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(4).Infof("NodeGetInfo: called with args %+v", req)

	return &csi.NodeGetInfoResponse{
		NodeId: d.nodeID,
	}, nil
}

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
