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
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"os"
	"os/exec"
	"path/filepath"
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

var _ MntInterface = &ProcessMount{}

func NewProcessMount(mounter k8sMount.SafeFormatAndMount) MntInterface {
	return &ProcessMount{mounter}
}

func (p *ProcessMount) JCreateVolume(jfsSetting *jfsConfig.JfsSetting) error {
	// 1. mount juicefs
	err := p.JMount(jfsSetting)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
	}

	// 2. create subPath volume
	volPath := filepath.Join(jfsSetting.MountPath, jfsSetting.SubPath)

	klog.V(6).Infof("JCreateVolume: checking %q exists in %v", volPath, jfsSetting.MountPath)
	exists, err := k8sMount.PathExists(volPath)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		klog.V(5).Infof("JCreateVolume: volume not existed, create %s", jfsSetting.MountPath)
		err := os.MkdirAll(volPath, os.FileMode(0777))
		if err != nil {
			return status.Errorf(codes.Internal, "Could not make directory for meta %q: %v", volPath, err)
		}
		if fi, err := os.Stat(volPath); err != nil {
			return status.Errorf(codes.Internal, "Could not stat directory %s: %q", volPath, err)
		} else if fi.Mode().Perm() != 0777 { // The perm of `volPath` may not be 0777 when the umask applied
			err = os.Chmod(volPath, os.FileMode(0777))
			if err != nil {
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

func (p *ProcessMount) JDeleteVolume(jfsSetting *jfsConfig.JfsSetting) error {
	// 1. mount juicefs
	err := p.JMount(jfsSetting)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
	}

	// 2. delete subPath volume
	volPath := filepath.Join(jfsSetting.MountPath, jfsSetting.VolumeId)
	if existed, err := k8sMount.PathExists(volPath); err != nil {
		return status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	} else if existed {
		stdoutStderr, err := p.RmrDir(volPath, jfsSetting.IsCe)
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

func (p *ProcessMount) GetMountRef(uniqueId, target string) (int, error) {
	var refs []string

	var corruptedMnt bool
	exists, err := k8sMount.PathExists(target)
	if err == nil {
		if !exists {
			klog.V(5).Infof("ProcessUmount: %s target not exists", target)
			return 0, nil
		}
		var notMnt bool
		notMnt, err = k8sMount.IsNotMountPoint(p, target)
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

func (p *ProcessMount) UmountTarget(uniqueId, target string) error {
	// process mnt need target to get ref
	// so, umount target in JUmount
	return nil
}

//JUmount umount targetPath
func (p *ProcessMount) JUmount(uniqueId, target string) error {
	var refs []string

	var corruptedMnt bool
	exists, err := k8sMount.PathExists(target)
	if err == nil {
		if !exists {
			klog.V(5).Infof("ProcessUmount: %s target not exists", target)
			return nil
		}
		var notMnt bool
		notMnt, err = k8sMount.IsNotMountPoint(p, target)
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

func (p *ProcessMount) AddRefOfMount(target string, podName string) error {
	panic("implement me")
}

func (p *ProcessMount) CleanCache(id string, volumeId string, cacheDirs []string) error {
	for _, cacheDir := range cacheDirs {
		// clean up raw dir under cache dir
		rawPath := filepath.Join(cacheDir, id, "raw", "chunks")
		if existed, err := k8sMount.PathExists(rawPath); err != nil {
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

func (p *ProcessMount) RmrDir(directory string, isCeMount bool) ([]byte, error) {
	klog.V(5).Infof("RmrDir: removing directory recursively: %q", directory)
	if isCeMount {
		return p.Exec.Command(jfsConfig.CeCliPath, "rmr", directory).CombinedOutput()
	}
	return p.Exec.Command(jfsConfig.CliPath, "rmr", directory).CombinedOutput()
}
