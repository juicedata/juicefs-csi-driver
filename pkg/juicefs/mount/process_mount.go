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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/juicedata/juicefs-csi-driver/pkg/util"

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

var _ MntInterface = &ProcessMount{}

func NewProcessMount(mounter k8sMount.SafeFormatAndMount) MntInterface {
	return &ProcessMount{mounter}
}

func (p *ProcessMount) JCreateVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	// 1. mount juicefs
	err := p.JMount(ctx, jfsSetting)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
	}

	// 2. create subPath volume
	volPath := filepath.Join(jfsSetting.MountPath, jfsSetting.SubPath)

	klog.V(6).Infof("JCreateVolume: checking %q exists in %v", volPath, jfsSetting.MountPath)
	var exists bool
	if err := util.DoWithContext(ctx, func() (err error) {
		exists, err = k8sMount.PathExists(volPath)
		return
	}); err != nil {
		return status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		klog.V(5).Infof("JCreateVolume: volume not existed, create %s", jfsSetting.MountPath)
		if err := util.DoWithContext(ctx, func() (err error) {
			return os.MkdirAll(volPath, os.FileMode(0777))
		}); err != nil {
			return status.Errorf(codes.Internal, "Could not make directory for meta %q: %v", volPath, err)
		}

		var fi os.FileInfo
		if err := util.DoWithContext(ctx, func() (err error) {
			fi, err = os.Stat(volPath)
			return err
		}); err != nil {
			return status.Errorf(codes.Internal, "Could not stat directory %s: %q", volPath, err)
		}

		if fi.Mode().Perm() != 0777 { // The perm of `volPath` may not be 0777 when the umask applied
			if err := util.DoWithContext(ctx, func() (err error) {
				return os.Chmod(volPath, os.FileMode(0777))
			}); err != nil {
				return status.Errorf(codes.Internal, "Could not chmod directory %s: %q", volPath, err)
			}
		}
	}

	// 3. umount
	if err = p.Unmount(jfsSetting.MountPath); err != nil {
		return status.Errorf(codes.Internal, "Could not unmount %q: %v", jfsSetting.MountPath, err)
	}
	return nil
}

func (p *ProcessMount) JDeleteVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	// 1. mount juicefs
	err := p.JMount(ctx, jfsSetting)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
	}

	// 2. delete subPath volume
	volPath := filepath.Join(jfsSetting.MountPath, jfsSetting.VolumeId)

	var existed bool

	if err := util.DoWithContext(ctx, func() (err error) {
		existed, err = k8sMount.PathExists(volPath)
		return err
	}); err != nil {
		return status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	} else if existed {
		stdoutStderr, err := p.RmrDir(ctx, volPath, jfsSetting.IsCe)
		klog.V(5).Infof("DeleteVol: rmr output is '%s'", stdoutStderr)
		if err != nil {
			return status.Errorf(codes.Internal, "Could not delete volume path %q: %v", volPath, err)
		}
	}

	// 3. umount
	if err = p.Unmount(jfsSetting.MountPath); err != nil {
		return status.Errorf(codes.Internal, "Could not unmount volume %q: %v", jfsSetting.SubPath, err)
	}
	return nil
}

func (p *ProcessMount) JMount(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	if !strings.Contains(jfsSetting.Source, "://") {
		klog.V(5).Infof("eeMount: mount %v at %v", jfsSetting.Source, jfsSetting.MountPath)
		err := p.Mount(jfsSetting.Source, jfsSetting.MountPath, jfsConfig.FsType, jfsSetting.Options)
		if err != nil {
			return status.Errorf(codes.Internal, "Could not mount %q at %q: %v", jfsSetting.Source, jfsSetting.MountPath, err)
		}
		klog.V(5).Infof("eeMount mount success.")
		return nil
	}
	klog.V(5).Infof("ceMount: mount %v at %v", util.StripPasswd(jfsSetting.Source), jfsSetting.MountPath)
	mountArgs := []string{jfsSetting.Source, jfsSetting.MountPath}

	if len(jfsSetting.Options) > 0 {
		mountArgs = append(mountArgs, "-o", strings.Join(jfsSetting.Options, ","))
	}

	var exist bool

	if err := util.DoWithContext(ctx, func() (err error) {
		exist, err = k8sMount.PathExists(jfsSetting.MountPath)
		return
	}); err != nil {
		return status.Errorf(codes.Internal, "Could not check existence of dir %q: %v", jfsSetting.MountPath, err)
	} else if !exist {
		klog.V(5).Infof("JCreateVolume: volume not existed, create %s", jfsSetting.MountPath)
		if err := util.DoWithContext(ctx, func() (err error) {
			return os.MkdirAll(jfsSetting.MountPath, os.FileMode(0755))
		}); err != nil {
			return status.Errorf(codes.Internal, "Could not create dir %q: %v", jfsSetting.MountPath, err)
		}
	}

	var notMounted bool
	if err := util.DoWithContext(ctx, func() (err error) {
		notMounted, err = p.IsLikelyNotMountPoint(jfsSetting.MountPath)
		return
	}); err != nil {
		return status.Errorf(codes.Internal, "Could not check existence of dir %q: %v", jfsSetting.MountPath, err)
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
	for key, val := range jfsSetting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", key, val))
	}
	mntCmd := exec.Command(jfsConfig.CeMountPath, mountArgs...)
	mntCmd.Env = envs
	mntCmd.Stderr = os.Stderr
	mntCmd.Stdout = os.Stdout
	go func() { _ = mntCmd.Run() }()
	// Wait until the mount point is ready

	for {
		var finfo os.FileInfo
		if err := util.DoWithContext(ctx, func() (err error) {
			finfo, err = os.Stat(jfsSetting.MountPath)
			return err
		}); err != nil {
			if err == context.DeadlineExceeded {
				break
			}
			klog.V(5).Infof("Stat mount path %v failed: %v", jfsSetting.MountPath, err)
			time.Sleep(time.Millisecond * 500)
			continue
		}
		if st, ok := finfo.Sys().(*syscall.Stat_t); ok {
			if st.Ino == 1 {
				return nil
			}
			klog.V(5).Infof("Mount point %v is not ready", jfsSetting.MountPath)
		} else {
			klog.V(5).Info("Cannot reach here")
		}
		time.Sleep(time.Millisecond * 500)
	}
	return status.Errorf(codes.Internal, "Mount %v at %v failed: mount isn't ready in 30 seconds", util.StripPasswd(jfsSetting.Source), jfsSetting.MountPath)
}

func (p *ProcessMount) GetMountRef(ctx context.Context, target, podName string) (int, error) {
	var refs []string

	var corruptedMnt bool
	var exists bool

	err := util.DoWithContext(ctx, func() (err error) {
		exists, err = k8sMount.PathExists(target)
		return
	})
	if err == nil {
		if !exists {
			klog.V(5).Infof("ProcessUmount: %s target not exists", target)
			return 0, nil
		}
		var notMnt bool
		err := util.DoWithContext(ctx, func() (err error) {
			notMnt, err = k8sMount.IsNotMountPoint(p, target)
			return err
		})
		if err != nil {
			return 0, status.Errorf(codes.Internal, "Check target path is mountpoint failed: %q", err)
		}
		if notMnt { // target exists but not a mountpoint
			klog.V(5).Infof("ProcessUmount: %s target not mounted", target)
			return 0, nil
		}
	} else if corruptedMnt = k8sMount.IsCorruptedMnt(err); !corruptedMnt {
		return 0, status.Errorf(codes.Internal, "Check path %s failed: %q", target, err)
	}

	refs, err = util.GetMountDeviceRefs(target, corruptedMnt)
	if err != nil {
		return 0, status.Errorf(codes.Internal, "Fail to get mount device refs: %q", err)
	}
	return len(refs), err
}

func (p *ProcessMount) UmountTarget(ctx context.Context, target, podName string) error {
	// process mnt need target to get ref
	// so, umount target in JUmount
	return nil
}

// JUmount umount targetPath
func (p *ProcessMount) JUmount(ctx context.Context, target, podName string) error {
	var refs []string

	var corruptedMnt bool
	var exists bool

	err := util.DoWithContext(ctx, func() (err error) {
		exists, err = k8sMount.PathExists(target)
		return
	})
	if err == nil {
		if !exists {
			klog.V(5).Infof("ProcessUmount: %s target not exists", target)
			return nil
		}
		var notMnt bool
		err := util.DoWithContext(ctx, func() (err error) {
			notMnt, err = k8sMount.IsNotMountPoint(p, target)
			return err
		})
		if err != nil {
			return status.Errorf(codes.Internal, "Check target path is mountpoint failed: %q", err)
		}
		if notMnt { // target exists but not a mountpoint
			klog.V(5).Infof("ProcessUmount: %s target not mounted", target)
			return nil
		}
	} else if corruptedMnt = k8sMount.IsCorruptedMnt(err); !corruptedMnt {
		return status.Errorf(codes.Internal, "Check path %s failed: %q", target, err)
	}

	refs, err = util.GetMountDeviceRefs(target, corruptedMnt)
	if err != nil {
		return status.Errorf(codes.Internal, "Fail to get mount device refs: %q", err)
	}

	klog.V(5).Infof("ProcessUmount: unmounting target %s", target)
	if err := p.Unmount(target); err != nil {
		return status.Errorf(codes.Internal, "Could not unmount %q: %v", target, err)
	}

	// we can only unmount this when only one is left
	// since the PVC might be used by more than one container
	if err == nil && len(refs) == 1 {
		klog.V(5).Infof("ProcessUmount: unmounting ref %s for target %s", refs[0], target)
		if err = p.Unmount(refs[0]); err != nil {
			klog.V(5).Infof("ProcessUmount: error unmounting mount ref %s, %v", refs[0], err)
		}
	}
	return err
}

func (p *ProcessMount) AddRefOfMount(ctx context.Context, target string, podName string) error {
	panic("implement me")
}

func (p *ProcessMount) CleanCache(ctx context.Context, id string, volumeId string, cacheDirs []string) error {
	for _, cacheDir := range cacheDirs {
		// clean up raw dir under cache dir
		rawPath := filepath.Join(cacheDir, id, "raw", "chunks")
		var existed bool
		if err := util.DoWithContext(ctx, func() (err error) {
			existed, err = k8sMount.PathExists(rawPath)
			return
		}); err != nil {
			klog.Errorf("Could not check raw path %q exists: %v", rawPath, err)
			return err
		} else if existed {
			err = os.RemoveAll(rawPath)
			if err != nil {
				klog.Errorf("Could not cleanup cache raw path %q: %v", rawPath, err)
				return err
			}
		}
	}
	return nil
}

func (p *ProcessMount) RmrDir(ctx context.Context, directory string, isCeMount bool) ([]byte, error) {
	klog.V(5).Infof("RmrDir: removing directory recursively: %q", directory)
	if isCeMount {
		return p.Exec.CommandContext(ctx, jfsConfig.CeCliPath, "rmr", directory).CombinedOutput()
	}
	return p.Exec.CommandContext(ctx, jfsConfig.CliPath, "rmr", directory).CombinedOutput()
}
