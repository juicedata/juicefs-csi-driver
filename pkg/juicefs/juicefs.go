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

package juicefs

import (
	"context"
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	podmount "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
)

// Interface of juicefs provider
type Interface interface {
	mount.Interface
	JfsMount(volumeID string, target string, secrets, volCtx map[string]string, options []string, usePod bool) (Jfs, error)
	JfsUnmount(mountPath string) error
	AuthFs(secrets map[string]string) ([]byte, error)
	MountFs(volumeID string, target string, options []string, jfsSetting *config.JfsSetting) (string, error)
	Version() ([]byte, error)
}

type juicefs struct {
	mount.SafeFormatAndMount
	k8sclient.K8sClient
}

var _ Interface = &juicefs{}

type jfs struct {
	Provider  *juicefs
	Name      string
	MountPath string
	Options   []string
}

// Jfs is the interface of a mounted file system
type Jfs interface {
	GetBasePath() string
	CreateVol(volumeID, subPath string) (string, error)
	DeleteVol(volumeID string, secrets map[string]string) error
}

var _ Jfs = &jfs{}

func (fs *jfs) GetBasePath() string {
	return fs.MountPath
}

// CreateVol creates the directory needed
func (fs *jfs) CreateVol(volumeID, subPath string) (string, error) {
	volPath := filepath.Join(fs.MountPath, subPath)

	klog.V(5).Infof("CreateVol: checking %q exists in %v", volPath, fs)
	exists, err := mount.PathExists(volPath)
	if err != nil {
		return "", status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		klog.V(5).Infof("CreateVol: volume not existed")
		err := os.MkdirAll(volPath, os.FileMode(0777))
		if err != nil {
			return "", status.Errorf(codes.Internal, "Could not make directory for meta %q: %v", volPath, err)
		}
	}
	if fi, err := os.Stat(volPath); err != nil {
		return "", status.Errorf(codes.Internal, "Could not stat directory %s: %q", volPath, err)
	} else if fi.Mode().Perm() != 0777 { // The perm of `volPath` may not be 0777 when the umask applied
		err = os.Chmod(volPath, os.FileMode(0777))
		if err != nil {
			return "", status.Errorf(codes.Internal, "Could not chmod directory %s: %q", volPath, err)
		}
	}

	return volPath, nil
}

func (fs *jfs) DeleteVol(volumeID string, secrets map[string]string) error {
	volPath := filepath.Join(fs.MountPath, volumeID)
	if existed, err := mount.PathExists(volPath); err != nil {
		return status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	} else if existed {
		_, isCeMount := secrets["metaurl"]
		stdoutStderr, err := fs.Provider.RmrDir(volPath, isCeMount)
		klog.V(5).Infof("DeleteVol: rmr output is '%s'", stdoutStderr)
		if err != nil {
			return status.Errorf(codes.Internal, "Could not delete volume path %q: %v", volPath, err)
		}
	}
	return nil
}

// NewJfsProvider creates a provider for JuiceFS file system
func NewJfsProvider(mounter *mount.SafeFormatAndMount) (Interface, error) {
	if mounter == nil {
		mounter = &mount.SafeFormatAndMount{
			Interface: mount.New(""),
			Exec:      k8sexec.New(),
		}
	}
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		klog.V(5).Infof("Can't get k8s client: %v", err)
		return nil, err
	}

	return &juicefs{*mounter, k8sClient}, nil
}

func (j *juicefs) IsNotMountPoint(dir string) (bool, error) {
	return mount.IsNotMountPoint(j, dir)
}

// JfsMount auths and mounts JuiceFS
func (j *juicefs) JfsMount(volumeID string, target string, secrets, volCtx map[string]string, options []string, usePod bool) (Jfs, error) {
	jfsSecret, err := config.ParseSetting(secrets, volCtx, usePod)
	if err != nil {
		klog.V(5).Infof("Parse config error: %v", err)
		return nil, err
	}
	source, isCe := secrets["metaurl"]
	var mountPath string
	if !isCe {
		j.Upgrade()
		stdoutStderr, err := j.AuthFs(secrets)
		klog.V(5).Infof("JfsMount: authentication output is '%s'\n", stdoutStderr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not auth juicefs: %v", err)
		}
		jfsSecret.Source = secrets["name"]
	} else {
		stdoutStderr, err := j.ceFormat(secrets)
		klog.V(5).Infof("JfsMount: format output is '%s'\n", stdoutStderr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not format juicefs: %v", err)
		}
		// Default use redis:// scheme
		if !strings.Contains(source, "://") {
			source = "redis://" + source
		}
		jfsSecret.Source = source
	}
	mountPath, err = j.MountFs(volumeID, target, options, jfsSecret)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
	}

	return &jfs{
		Provider:  j,
		Name:      secrets["name"],
		MountPath: mountPath,
		Options:   options,
	}, nil
}

func (j *juicefs) JfsUnmount(mountPath string) (err error) {
	klog.V(5).Infof("JfsUnmount: umount %s", mountPath)
	if err = j.Unmount(mountPath); err != nil {
		klog.V(5).Infof("JfsUnmount: error umount %s, %v", mountPath, err)
	}
	return
}

func (j *juicefs) RmrDir(directory string, isCeMount bool) ([]byte, error) {
	klog.V(5).Infof("RmrDir: removing directory recursively: %q", directory)
	if isCeMount {
		return j.Exec.Command(config.CeCliPath, "rmr", directory).CombinedOutput()
	}
	return j.Exec.Command("rm", "-rf", directory).CombinedOutput()
}

// AuthFs authenticates JuiceFS, enterprise edition only
func (j *juicefs) AuthFs(secrets map[string]string) ([]byte, error) {
	if secrets == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Nil secrets")
	}

	if secrets["name"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty name")
	}

	if secrets["token"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty token")
	}

	args := []string{"auth", secrets["name"]}
	argsStripped := []string{"auth", secrets["name"]}
	keys := []string{
		"accesskey",
		"accesskey2",
		"bucket",
		"bucket2",
	}
	keysStripped := []string{
		"token",
		"secretkey",
		"secretkey2",
		"passphrase"}
	isOptional := map[string]bool{
		"accesskey2": true,
		"secretkey2": true,
		"bucket":     true,
		"bucket2":    true,
		"passphrase": true,
	}
	for _, k := range keys {
		if !isOptional[k] || secrets[k] != "" {
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
			argsStripped = append(argsStripped, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	for _, k := range keysStripped {
		if !isOptional[k] || secrets[k] != "" {
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
			argsStripped = append(argsStripped, fmt.Sprintf("--%s=[secret]", k))
		}
	}
	if v, ok := os.LookupEnv("JFS_NO_UPDATE_CONFIG"); ok && v == "enabled" {
		args = append(args, "--no-update")
		argsStripped = append(argsStripped, "--no-update")

		if secrets["bucket"] == "" {
			return nil, status.Errorf(codes.InvalidArgument,
				"bucket argument is required when --no-update option is provided")
		}
		if secrets["initconfig"] != "" {
			conf := secrets["name"] + ".conf"
			confPath := filepath.Join("/root/.juicefs", conf)
			if _, err := os.Stat(confPath); os.IsNotExist(err) {
				err = ioutil.WriteFile(confPath, []byte(secrets["initconfig"]), 0644)
				if err != nil {
					return nil, status.Errorf(codes.Internal,
						"Create config file %q failed: %v", confPath, err)
				}
				klog.V(5).Infof("Create config file: %q success", confPath)
			}
		}
	}
	klog.V(5).Infof("AuthFs: cmd %q, args %#v", config.CliPath, argsStripped)
	return j.Exec.Command(config.CliPath, args...).CombinedOutput()
}

// MountFs mounts JuiceFS with idempotency
func (j *juicefs) MountFs(volumeID, target string, options []string, jfsSetting *config.JfsSetting) (string, error) {
	var mountPath string
	var mnt podmount.Interface
	if jfsSetting.UsePod {
		mountPath = filepath.Join(config.PodMountBase, volumeID)
		mnt = podmount.NewPodMount(jfsSetting, j.K8sClient)
	} else {
		mountPath = filepath.Join(config.MountBase, volumeID)
		mnt = podmount.NewProcessMount(jfsSetting)
	}

	exists, err := mount.PathExists(mountPath)
	if err != nil && mount.IsCorruptedMnt(err) {
		klog.V(5).Infof("MountFs: %s is a corrupted mountpoint, unmounting", mountPath)
		if err = j.Unmount(mountPath); err != nil {
			klog.V(5).Infof("Unmount corrupted mount point %s failed: %v", mountPath, err)
			return mountPath, err
		}
	} else if err != nil {
		return mountPath, status.Errorf(codes.Internal, "Could not check mount point %q exists: %v", mountPath, err)
	}

	if !exists {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", jfsSetting.Source, mountPath, options)
		err = mnt.JMount(jfsSetting.Storage, volumeID, mountPath, target, options)
		if err != nil {
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", jfsSetting.Source, mountPath, err)
		}
		return mountPath, nil
	}

	// path exists
	notMnt, err := j.IsLikelyNotMountPoint(mountPath)
	if err != nil {
		return mountPath, status.Errorf(codes.Internal, "Could not check %q IsLikelyNotMountPoint: %v", mountPath, err)
	}

	if notMnt {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", jfsSetting.Source, mountPath, options)
		err = mnt.JMount(jfsSetting.Storage, volumeID, mountPath, target, options)
		if err != nil {
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", jfsSetting.Source, mountPath, err)
		}
		return mountPath, nil
	}

	klog.V(5).Infof("Mount: skip mounting for existing mount point %q", mountPath)

	if jfsSetting.UsePod {
		klog.V(5).Infof("Mount: add mount ref of configMap of volumeId %q", volumeID)
		err = mnt.AddRefOfMount(target, podmount.GeneratePodNameByVolumeId(volumeID))
	}
	return mountPath, err
}

// Upgrade upgrades binary file in `cliPath` to newest version
func (j *juicefs) Upgrade() {
	if v, ok := os.LookupEnv("JFS_AUTO_UPGRADE"); !ok || v != "enabled" {
		return
	}

	timeout := 10
	if t, ok := os.LookupEnv("JFS_AUTO_UPGRADE_TIMEOUT"); ok {
		if v, err := strconv.Atoi(t); err == nil {
			timeout = v
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	err := exec.CommandContext(ctx, config.CliPath, "version", "-u").Run()
	if ctx.Err() == context.DeadlineExceeded {
		klog.V(5).Infof("Upgrade: did not finish in %v", timeout)
		return
	}

	if err != nil {
		klog.V(5).Infof("Upgrade: err %v", err)
		return
	}

	klog.V(5).Infof("Upgrade: successfully upgraded to newest version")
}

func (j *juicefs) Version() ([]byte, error) {
	return j.Exec.Command(config.CliPath, "version").CombinedOutput()
}

func (j *juicefs) ceFormat(secrets map[string]string) ([]byte, error) {
	if secrets == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Nil secrets")
	}

	if secrets["name"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty name")
	}

	if secrets["metaurl"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty metaurl")
	}

	args := []string{"format", "--no-update"}
	if secrets["storage"] == "ceph" {
		os.Setenv("JFS_NO_CHECK_OBJECT_STORAGE", "1")
	}
	argsStripped := []string{"format"}
	keys := []string{
		"storage",
		"bucket",
		"access-key",
		"block-size",
		"compress",
	}
	keysStripped := []string{"secret-key"}
	isOptional := map[string]bool{
		"block-size": true,
		"compress":   true,
	}
	for _, k := range keys {
		if !isOptional[k] || secrets[k] != "" {
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
			argsStripped = append(argsStripped, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	for _, k := range keysStripped {
		if !isOptional[k] || secrets[k] != "" {
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
			argsStripped = append(argsStripped, fmt.Sprintf("--%s=[secret]", k))
		}
	}
	args = append(args, secrets["metaurl"], secrets["name"])
	argsStripped = append(argsStripped, "[metaurl]", secrets["name"])
	klog.V(5).Infof("ceFormat: cmd %q, args %#v", config.CeCliPath, argsStripped)
	return j.Exec.Command(config.CeCliPath, args...).CombinedOutput()
}
