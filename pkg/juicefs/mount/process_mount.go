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

	_ "github.com/golang/mock/mockgen/model"
	"k8s.io/klog/v2"
	k8sMount "k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

const defaultCheckTimeout = 2 * time.Second

type ProcessMount struct {
	log klog.Logger
	k8sMount.SafeFormatAndMount
}

var _ MntInterface = &ProcessMount{}

func NewProcessMount(mounter k8sMount.SafeFormatAndMount) MntInterface {
	return &ProcessMount{klog.NewKlogr().WithName("process-mount"), mounter}
}

func (p *ProcessMount) JCreateVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	log := util.GenLog(ctx, p.log, "JCreateVolume")
	// 1. mount juicefs
	options := util.StripReadonlyOption(jfsSetting.Options)
	err := p.jmount(ctx, jfsSetting.Source, jfsSetting.MountPath, jfsSetting.Storage, options, jfsSetting.Envs)
	if err != nil {
		return fmt.Errorf("could not mount juicefs: %v", err)
	}

	// 2. create subPath volume
	volPath := filepath.Join(jfsSetting.MountPath, jfsSetting.SubPath)

	log.V(1).Info("checking exists", "volPath", volPath, "mountPath", jfsSetting.MountPath)
	var exists bool
	if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
		exists, err = k8sMount.PathExists(volPath)
		return
	}); err != nil {
		return fmt.Errorf("could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		log.Info("volume not existed, create it", "mountPath", jfsSetting.MountPath)
		if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
			return os.MkdirAll(volPath, os.FileMode(0777))
		}); err != nil {
			return fmt.Errorf("could not make directory for meta %q: %v", volPath, err)
		}

		var fi os.FileInfo
		if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
			fi, err = os.Stat(volPath)
			return err
		}); err != nil {
			return fmt.Errorf("could not stat directory %s: %q", volPath, err)
		}

		if fi.Mode().Perm() != 0777 { // The perm of `volPath` may not be 0777 when the umask applied
			if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
				return os.Chmod(volPath, os.FileMode(0777))
			}); err != nil {
				return fmt.Errorf("could not chmod directory %s: %q", volPath, err)
			}
		}
	}

	// 3. umount
	if err = p.Unmount(jfsSetting.MountPath); err != nil {
		return fmt.Errorf("could not unmount %q: %v", jfsSetting.MountPath, err)
	}
	return nil
}

func (p *ProcessMount) JDeleteVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	log := util.GenLog(ctx, p.log, "JDeleteVolume")
	// 1. mount juicefs
	err := p.jmount(ctx, jfsSetting.Source, jfsSetting.MountPath, jfsSetting.Storage, jfsSetting.Options, jfsSetting.Envs)
	if err != nil {
		return fmt.Errorf("could not mount juicefs: %v", err)
	}

	// 2. delete subPath volume
	volPath := filepath.Join(jfsSetting.MountPath, jfsSetting.SubPath)

	var existed bool

	if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
		existed, err = k8sMount.PathExists(volPath)
		return err
	}); err != nil {
		return fmt.Errorf("could not check volume path %q exists: %v", volPath, err)
	} else if existed {
		stdoutStderr, err := p.RmrDir(ctx, volPath, jfsSetting.IsCe)
		log.Info("rmr output", "output", stdoutStderr)
		if err != nil {
			return fmt.Errorf("could not delete volume path %q: %v", volPath, err)
		}
	}

	// 3. umount
	if err = p.Unmount(jfsSetting.MountPath); err != nil {
		return fmt.Errorf("could not unmount volume %q: %v", jfsSetting.SubPath, err)
	}
	return nil
}

func (p *ProcessMount) JMount(ctx context.Context, _ *jfsConfig.AppInfo, jfsSetting *jfsConfig.JfsSetting) error {
	// create subpath if readonly mount
	if jfsSetting.SubPath != "" {
		if util.ContainsString(jfsSetting.Options, "read-only") || util.ContainsString(jfsSetting.Options, "ro") {
			// generate mount command
			if err := p.JCreateVolume(ctx, jfsSetting); err != nil {
				return err
			}
		}
	}

	return p.jmount(ctx, jfsSetting.Source, jfsSetting.MountPath, jfsSetting.Storage, jfsSetting.Options, jfsSetting.Envs)
}

func (p *ProcessMount) jmount(ctx context.Context, source, mountPath, storage string, options []string, extraEnvs map[string]string) error {
	log := util.GenLog(ctx, p.log, "jmount")
	if !strings.Contains(source, "://") {
		log.Info("eeMount", "source", source, "mountPath", mountPath)
		err := p.Mount(source, mountPath, jfsConfig.FsType, options)
		if err != nil {
			return fmt.Errorf("could not mount %q at %q: %v", source, mountPath, err)
		}
		log.Info("eeMount mount success.")
		return nil
	}
	log.Info("ceMount", "source", util.StripPasswd(source), "mountPath", mountPath)
	mountArgs := []string{source, mountPath}

	if len(options) > 0 {
		mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
	}

	var exist bool

	if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
		exist, err = k8sMount.PathExists(mountPath)
		return
	}); err != nil {
		return fmt.Errorf("could not check existence of dir %q: %v", mountPath, err)
	} else if !exist {
		log.Info("volume not existed, create it", "mountPath", mountPath)
		if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
			return os.MkdirAll(mountPath, os.FileMode(0755))
		}); err != nil {
			return fmt.Errorf("could not create dir %q: %v", mountPath, err)
		}
	}

	var notMounted bool
	if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
		notMounted, err = p.IsLikelyNotMountPoint(mountPath)
		return
	}); err != nil {
		return fmt.Errorf("could not check existence of dir %q: %v", mountPath, err)
	} else if !notMounted {
		err = p.Unmount(mountPath)
		if err != nil {
			log.Info("Unmount before mount failed", "error", err)
			return err
		}
		log.Info("Unmount", "mountPath", mountPath)
	}

	envs := append(syscall.Environ(), "JFS_FOREGROUND=1")
	for key, val := range extraEnvs {
		envs = append(envs, fmt.Sprintf("%s=%s", key, val))
	}
	mntCmd := exec.Command(jfsConfig.CeMountPath, mountArgs...)
	mntCmd.Env = envs
	mntCmd.Stderr = os.Stderr
	mntCmd.Stdout = os.Stdout
	go func() { _ = mntCmd.Run() }()
	// Wait until the mount point is ready

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		var finfo os.FileInfo
		if err := util.DoWithTimeout(waitCtx, defaultCheckTimeout, func(ctx context.Context) (err error) {
			finfo, err = os.Stat(mountPath)
			return err
		}); err != nil {
			if err == context.DeadlineExceeded || err == context.Canceled {
				break
			}
			log.Info("Stat mount path failed", "mountPath", mountPath, "error", err)
			time.Sleep(time.Millisecond * 500)
			continue
		}
		if st, ok := finfo.Sys().(*syscall.Stat_t); ok {
			if st.Ino == 1 {
				return nil
			}
			log.Info("Mount point is not ready", "mountPath", mountPath)
		} else {
			log.Info("Cannot reach here")
		}
		time.Sleep(time.Millisecond * 500)
	}
	return fmt.Errorf("mount %v at %v failed: mount isn't ready in 30 seconds", util.StripPasswd(source), mountPath)
}

func (p *ProcessMount) GetMountRef(ctx context.Context, target, podName string) (int, error) {
	log := util.GenLog(ctx, p.log, "GetMountRef")
	var refs []string

	var corruptedMnt bool
	var exists bool

	err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
		exists, err = k8sMount.PathExists(target)
		return
	})
	if err == nil {
		if !exists {
			log.Info("target not exists", "target", target)
			return 0, nil
		}
		var notMnt bool
		err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
			notMnt, err = k8sMount.IsNotMountPoint(p, target)
			return err
		})
		if err != nil {
			return 0, fmt.Errorf("check target path is mountpoint failed: %q", err)
		}
		if notMnt { // target exists but not a mountpoint
			log.Info("target not mounted", "target", target)
			return 0, nil
		}
	} else if corruptedMnt = k8sMount.IsCorruptedMnt(err); !corruptedMnt {
		return 0, fmt.Errorf("check path %s failed: %q", target, err)
	}

	refs, err = util.GetMountDeviceRefs(target, corruptedMnt)
	if err != nil {
		return 0, fmt.Errorf("fail to get mount device refs: %q", err)
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
	log := util.GenLog(ctx, p.log, "JUmount")
	var refs []string

	var corruptedMnt bool
	var exists bool

	err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
		exists, err = k8sMount.PathExists(target)
		return
	})
	if err == nil {
		if !exists {
			log.Info("target not exists", "target", target)
			return nil
		}
		var notMnt bool
		err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
			notMnt, err = k8sMount.IsNotMountPoint(p, target)
			return err
		})
		if err != nil {
			return fmt.Errorf("check target path is mountpoint failed: %q", err)
		}
		if notMnt { // target exists but not a mountpoint
			log.Info("target not mounted", "target", target)
			return nil
		}
	} else if corruptedMnt = k8sMount.IsCorruptedMnt(err); !corruptedMnt {
		return fmt.Errorf("check path %s failed: %q", target, err)
	}

	refs, err = util.GetMountDeviceRefs(target, corruptedMnt)
	if err != nil {
		return fmt.Errorf("fail to get mount device refs: %q", err)
	}

	log.Info("unmounting target", "target", target)
	if err := p.Unmount(target); err != nil {
		return fmt.Errorf("could not unmount %q: %v", target, err)
	}

	// we can only unmount this when only one is left
	// since the PVC might be used by more than one container
	if err == nil && len(refs) == 1 {
		log.Info("unmounting ref for target", "ref", refs[0], "target", target)
		if err = p.Unmount(refs[0]); err != nil {
			log.Info("error unmounting mount ref", "ref", refs[0], "error", err)
		}
	}
	return err
}

func (p *ProcessMount) AddRefOfMount(ctx context.Context, target string, podName string) error {
	panic("implement me")
}

func (p *ProcessMount) CleanCache(ctx context.Context, _ string, id string, _ string, cacheDirs []string) error {
	log := util.GenLog(ctx, p.log, "CleanCache")
	for _, cacheDir := range cacheDirs {
		// clean up raw dir under cache dir
		rawPath := filepath.Join(cacheDir, id, "raw", "chunks")
		var existed bool
		if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
			existed, err = k8sMount.PathExists(rawPath)
			return
		}); err != nil {
			log.Error(err, "Could not check raw path exists", "rawPath", rawPath)
			return err
		} else if existed {
			err = os.RemoveAll(rawPath)
			if err != nil {
				log.Error(err, "Could not cleanup cache raw path", "rawPath", rawPath, "error", err)
				return err
			}
		}
	}
	return nil
}

func (p *ProcessMount) RmrDir(ctx context.Context, directory string, isCeMount bool) ([]byte, error) {
	log := util.GenLog(ctx, p.log, "RmrDir")
	log.Info("removing directory recursively", "directory", directory)
	if isCeMount {
		return p.Exec.CommandContext(ctx, jfsConfig.CeCliPath, "rmr", directory).CombinedOutput()
	}
	return p.Exec.CommandContext(ctx, jfsConfig.CliPath, "rmr", directory).CombinedOutput()
}
