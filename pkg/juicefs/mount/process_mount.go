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
	k8sMount "k8s.io/utils/mount"

	_ "github.com/golang/mock/mockgen/model"
	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
)

type ProcessMount struct {
	k8sMount.SafeFormatAndMount
}

func NewProcessMount(mounter k8sMount.SafeFormatAndMount) MntInterface {
	return &ProcessMount{mounter}
}

func (p *ProcessMount) JMount(jfsSetting *jfsConfig.JfsSetting) error {
	if !strings.Contains(jfsSetting.Source, "://") {
		klog.V(5).Infof("eeMount: mount %v at %v", jfsSetting.Source, jfsSetting.MountPath)
		err := p.Mount(jfsSetting.Source, jfsSetting.MountPath, jfsConfig.FsType, jfsSetting.Options)
		if err != nil {
			return status.Errorf(codes.Internal, "Could not mount %q at %q: %v", jfsSetting.Source, jfsSetting.MountPath, err)
		}
		klog.V(5).Infof("eeMount mount success.")
		return nil
	}
	klog.V(5).Infof("ceMount: mount %v at %v", jfsSetting.Source, jfsSetting.MountPath)
	mountArgs := []string{jfsSetting.Source, jfsSetting.MountPath}

	if len(jfsSetting.Options) > 0 {
		mountArgs = append(mountArgs, "-o", strings.Join(jfsSetting.Options, ","))
	}

	if exist, err := k8sMount.PathExists(jfsSetting.MountPath); err != nil {
		return status.Errorf(codes.Internal, "Could not check existence of dir %q: %v", jfsSetting.MountPath, err)
	} else if !exist {
		if err = os.MkdirAll(jfsSetting.MountPath, os.FileMode(0755)); err != nil {
			return status.Errorf(codes.Internal, "Could not create dir %q: %v", jfsSetting.MountPath, err)
		}
	}

	if notMounted, err := p.IsLikelyNotMountPoint(jfsSetting.MountPath); err != nil {
		return err
	} else if !notMounted {
		err = p.Unmount(jfsSetting.MountPath)
		if err != nil {
			klog.V(5).Infof("Unmount before mount failed: %v", err)
			return err
		}
		klog.V(5).Infof("Unmount %v", jfsSetting.MountPath)
	}

	envs := append(syscall.Environ(), "JFS_FOREGROUND=1")
	if jfsSetting.Storage == "ceph" || jfsSetting.Storage == "gs" {
		envs = append(envs, "JFS_NO_CHECK_OBJECT_STORAGE=1")
	}
	mntCmd := exec.Command(jfsConfig.CeMountPath, mountArgs...)
	mntCmd.Env = envs
	mntCmd.Stderr = os.Stderr
	mntCmd.Stdout = os.Stdout
	go func() { _ = mntCmd.Run() }()
	// Wait until the mount point is ready
	for i := 0; i < 30; i++ {
		finfo, err := os.Stat(jfsSetting.MountPath)
		if err != nil {
			return status.Errorf(codes.Internal, "Stat mount path %v failed: %v", jfsSetting.MountPath, err)
		}
		if st, ok := finfo.Sys().(*syscall.Stat_t); ok {
			if st.Ino == 1 {
				return nil
			}
			klog.V(5).Infof("Mount point %v is not ready", jfsSetting.MountPath)
		} else {
			klog.V(5).Info("Cannot reach here")
		}
		time.Sleep(time.Second)
	}
	return status.Errorf(codes.Internal, "Mount %v at %v failed: mount isn't ready in 30 seconds", jfsSetting.Source, jfsSetting.MountPath)
}

func (p *ProcessMount) JUmount(volumeId, target string) error {
	return p.Unmount(target)
}

func (p *ProcessMount) AddRefOfMount(target string, podName string) error {
	panic("implement me")
}
