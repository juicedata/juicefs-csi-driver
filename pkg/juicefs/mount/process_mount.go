/*
Copyright 2021 Juicedata Inc

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

package mount

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	k8sMount "k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
)

type ProcessMount struct {
	k8sMount.SafeFormatAndMount
	jfsSetting *jfsConfig.JfsSetting
}

func NewProcessMount(setting *jfsConfig.JfsSetting) Interface {
	mounter := &k8sMount.SafeFormatAndMount{
		Interface: k8sMount.New(""),
		Exec:      k8sexec.New(),
	}
	return &ProcessMount{*mounter, setting}
}

func (p *ProcessMount) JMount(storage, volumeId, mountPath string, target string, options []string) error {
	if !strings.Contains(p.jfsSetting.Source, "://") {
		klog.V(5).Infof("eeMount: mount %v at %v", p.jfsSetting.Source, mountPath)
		err := p.Mount(p.jfsSetting.Source, mountPath, jfsConfig.FsType, options)
		if err != nil {
			return status.Errorf(codes.Internal, "Could not mount %q at %q: %v", p.jfsSetting.Source, mountPath, err)
		}
		klog.V(5).Infof("eeMount mount success.")
		return nil
	}
	klog.V(5).Infof("ceMount: mount %v at %v", p.jfsSetting.Source, mountPath)
	mountArgs := []string{p.jfsSetting.Source, mountPath}

	if len(options) > 0 {
		mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
	}

	if exist, err := k8sMount.PathExists(mountPath); err != nil {
		return status.Errorf(codes.Internal, "Could not check existence of dir %q: %v", mountPath, err)
	} else if !exist {
		if err = os.MkdirAll(mountPath, os.FileMode(0755)); err != nil {
			return status.Errorf(codes.Internal, "Could not create dir %q: %v", mountPath, err)
		}
	}

	if notMounted, err := p.IsLikelyNotMountPoint(mountPath); err != nil {
		return err
	} else if !notMounted {
		err = p.Unmount(mountPath)
		if err != nil {
			klog.V(5).Infof("Unmount before mount failed: %v", err)
			return err
		}
		klog.V(5).Infof("Unmount %v", mountPath)
	}

	envs := append(syscall.Environ(), "JFS_FOREGROUND=1")
	if storage == "ceph" {
		envs = append(envs, "JFS_NO_CHECK_OBJECT_STORAGE=1")
	}
	mntCmd := exec.Command(jfsConfig.CeMountPath, mountArgs...)
	mntCmd.Env = envs
	mntCmd.Stderr = os.Stderr
	mntCmd.Stdout = os.Stdout
	go func() { _ = mntCmd.Run() }()
	// Wait until the mount point is ready
	for i := 0; i < 30; i++ {
		finfo, err := os.Stat(mountPath)
		if err != nil {
			return status.Errorf(codes.Internal, "Stat mount path %v failed: %v", mountPath, err)
		}
		if st, ok := finfo.Sys().(*syscall.Stat_t); ok {
			if st.Ino == 1 {
				return nil
			}
			klog.V(5).Infof("Mount point %v is not ready", mountPath)
		} else {
			klog.V(5).Info("Cannot reach here")
		}
		time.Sleep(time.Second)
	}
	return status.Errorf(codes.Internal, "Mount %v at %v failed: mount isn't ready in 30 seconds", p.jfsSetting.Source, mountPath)
}

func (p *ProcessMount) JUmount(volumeId, target string) error {
	return p.Unmount(target)
}

func (p *ProcessMount) AddRefOfMount(target string, podName string) error {
	panic("implement me")
}
