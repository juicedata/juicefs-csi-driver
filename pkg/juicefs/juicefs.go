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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	podmount "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
)

// Interface of juicefs provider
type Interface interface {
	mount.Interface
	JfsMount(volumeID string, target string, secrets, volCtx map[string]string, options []string) (Jfs, error)
	JfsCreateVol(volumeID string, subPath string, secrets map[string]string) error
	JfsDeleteVol(volumeID string, target string, secrets map[string]string) error
	JfsUnmount(volumeID, mountPath string) error
	JfsCleanupMountPoint(mountPath string) error
	Version() ([]byte, error)
}

type juicefs struct {
	mount.SafeFormatAndMount
	*k8sclient.K8sClient
	podMount     podmount.MntInterface
	processMount podmount.MntInterface
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

	klog.V(6).Infof("CreateVol: checking %q exists in %v", volPath, fs)
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
		if fi, err := os.Stat(volPath); err != nil {
			return "", status.Errorf(codes.Internal, "Could not stat directory %s: %q", volPath, err)
		} else if fi.Mode().Perm() != 0777 { // The perm of `volPath` may not be 0777 when the umask applied
			err = os.Chmod(volPath, os.FileMode(0777))
			if err != nil {
				return "", status.Errorf(codes.Internal, "Could not chmod directory %s: %q", volPath, err)
			}
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
	processMnt := podmount.NewProcessMount(*mounter)
	var podMnt podmount.MntInterface
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		klog.V(5).Infof("Can't get k8s client: %v", err)
		return nil, err
	}
	podMnt = podmount.NewPodMount(k8sClient, *mounter)

	return &juicefs{*mounter, k8sClient, podMnt, processMnt}, nil
}

func (j *juicefs) JfsCreateVol(volumeID string, subPath string, secrets map[string]string) error {
	jfsSetting, err := j.getSettings(volumeID, "", secrets, nil, []string{})
	if err != nil {
		return err
	}
	jfsSetting.SubPath = subPath
	jfsSetting.MountPath = filepath.Join(config.PodMountBase, jfsSetting.VolumeId)
	if config.FormatInPod {
		return j.podMount.JCreateVolume(jfsSetting)
	}
	return j.processMount.JCreateVolume(jfsSetting)
}

func (j *juicefs) JfsDeleteVol(volumeID string, subPath string, secrets map[string]string) error {
	jfsSetting, err := j.getSettings(volumeID, "", secrets, nil, []string{})
	if err != nil {
		return err
	}
	jfsSetting.SubPath = subPath
	jfsSetting.MountPath = filepath.Join(config.PodMountBase, jfsSetting.VolumeId)

	mnt := j.processMount
	if config.FormatInPod {
		mnt = j.podMount
	}
	if err := mnt.JDeleteVolume(jfsSetting); err != nil {
		return err
	}
	return j.JfsCleanupMountPoint(jfsSetting.MountPath)
}

func (j *juicefs) JfsMount(volumeID string, target string, secrets, volCtx map[string]string, options []string) (Jfs, error) {
	jfsSetting, err := j.getSettings(volumeID, target, secrets, volCtx, options)
	if err != nil {
		return nil, err
	}
	mountPath, err := j.MountFs(jfsSetting)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	return &jfs{
		Provider:  j,
		Name:      secrets["name"],
		MountPath: mountPath,
		Options:   options,
	}, nil
}

// JfsMount auths and mounts JuiceFS
func (j *juicefs) getSettings(volumeID string, target string, secrets, volCtx map[string]string, options []string) (*config.JfsSetting, error) {
	jfsSetting, err := config.ParseSetting(secrets, volCtx, !config.ByProcess)
	if err != nil {
		klog.V(5).Infof("Parse config error: %v", err)
		return nil, err
	}
	jfsSetting.VolumeId = volumeID
	jfsSetting.TargetPath = target
	jfsSetting.Options = options
	source, isCe := secrets["metaurl"]
	if !isCe {
		if secrets["token"] == "" {
			klog.V(5).Infof("token is empty, skip authfs.")
		} else {
			res, err := j.AuthFs(secrets, jfsSetting)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Could not auth juicefs: %v", err)
			}
			if config.FormatInPod {
				jfsSetting.FormatCmd = res
			}
		}
		jfsSetting.Source = secrets["name"]
	} else {
		noUpdate := false
		if secrets["storage"] == "" || secrets["bucket"] == "" {
			klog.V(5).Infof("JfsMount: storage or bucket is empty, format --no-update.")
			noUpdate = true
		}
		if config.FormatInPod && (secrets["storage"] == "ceph" || secrets["storage"] == "gs") {
			jfsSetting.Envs["JFS_NO_CHECK_OBJECT_STORAGE"] = "1"
		}
		res, err := j.ceFormat(secrets, noUpdate, jfsSetting)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
		// Default use redis:// scheme
		if !strings.Contains(source, "://") {
			source = "redis://" + source
		}
		jfsSetting.Source = source
		if config.FormatInPod {
			jfsSetting.FormatCmd = res
		}
	}
	return jfsSetting, nil
}

func (j *juicefs) JfsUnmount(volumeId, mountPath string) error {
	if config.ByProcess {
		return j.processMount.JUmount(volumeId, mountPath)
	}
	// targetPath may be mount bind many times when mount point recovered.
	// umount until it's not mounted.
	klog.V(5).Infof("JfsUnmount: umount %s", mountPath)
	for {
		command := exec.Command("umount", mountPath)
		out, err := command.CombinedOutput()
		if err == nil {
			continue
		}
		klog.V(6).Infoln(string(out))
		if !strings.Contains(string(out), "not mounted") &&
			!strings.Contains(string(out), "mountpoint not found") &&
			!strings.Contains(string(out), "no mount point specified") {
			klog.V(5).Infof("Unmount %s failed: %q, try to lazy unmount", mountPath, err)
			output, err := exec.Command("umount", "-l", mountPath).CombinedOutput()
			if err != nil {
				klog.V(5).Infof("Could not lazy unmount %q: %v, output: %s", mountPath, err, string(output))
				return err
			}
		}
		break
	}

	// cleanup target path
	if err := j.JfsCleanupMountPoint(mountPath); err != nil {
		klog.V(5).Infof("Clean mount point error: %v", err)
		return err
	}

	return j.podMount.JUmount(volumeId, mountPath)
}

func (j *juicefs) RmrDir(directory string, isCeMount bool) ([]byte, error) {
	klog.V(5).Infof("RmrDir: removing directory recursively: %q", directory)
	if isCeMount {
		return j.Exec.Command(config.CeCliPath, "rmr", directory).CombinedOutput()
	}
	return j.Exec.Command("rm", "-rf", directory).CombinedOutput()
}

func (j *juicefs) JfsCleanupMountPoint(mountPath string) error {
	klog.V(5).Infof("JfsCleanupMountPoint: clean up mount point: %q", mountPath)
	return mount.CleanupMountPoint(mountPath, j.SafeFormatAndMount.Interface, false)
}

// AuthFs authenticates JuiceFS, enterprise edition only
func (j *juicefs) AuthFs(secrets map[string]string, setting *config.JfsSetting) (string, error) {
	if secrets == nil {
		return "", status.Errorf(codes.InvalidArgument, "Nil secrets")
	}

	if secrets["name"] == "" {
		return "", status.Errorf(codes.InvalidArgument, "Empty name")
	}

	args := []string{"auth", secrets["name"]}
	cmdArgs := []string{config.CliPath, "auth", secrets["name"]}

	keysCompatible := map[string]string{
		"access-key":  "accesskey",
		"access-key2": "accesskey2",
		"secret-key":  "secretkey",
		"secret-key2": "secretkey2",
	}
	// compatible
	for compatibleKey, realKey := range keysCompatible {
		if value, ok := secrets[compatibleKey]; ok {
			klog.Infof("transform key [%s] to [%s]", compatibleKey, realKey)
			secrets[realKey] = value
			delete(secrets, compatibleKey)
		}
	}

	keys := []string{
		"accesskey",
		"accesskey2",
		"bucket",
		"bucket2",
		"subdir",
	}
	keysStripped := []string{
		"token",
		"secretkey",
		"secretkey2",
		"passphrase"}
	isOptional := map[string]bool{
		"accesskey":  true,
		"accesskey2": true,
		"secretkey":  true,
		"secretkey2": true,
		"bucket":     true,
		"bucket2":    true,
		"passphrase": true,
		"subdir":     true,
	}
	for _, k := range keys {
		if !isOptional[k] || secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, secrets[k]))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	for _, k := range keysStripped {
		if !isOptional[k] || secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=${%s}", k, k))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	if v, ok := os.LookupEnv("JFS_NO_UPDATE_CONFIG"); ok && v == "enabled" {
		cmdArgs = append(cmdArgs, "--no-update")
		args = append(args, "--no-update")
		if secrets["bucket"] == "" {
			return "", status.Errorf(codes.InvalidArgument,
				"bucket argument is required when --no-update option is provided")
		}
		if !config.FormatInPod && secrets["initconfig"] != "" {
			conf := secrets["name"] + ".conf"
			confPath := filepath.Join("/root/.juicefs", conf)
			if _, err := os.Stat(confPath); os.IsNotExist(err) {
				err = ioutil.WriteFile(confPath, []byte(secrets["initconfig"]), 0644)
				if err != nil {
					return "", status.Errorf(codes.Internal,
						"Create config file %q failed: %v", confPath, err)
				}
				klog.V(5).Infof("Create config file: %q success", confPath)
			}
		}
	}
	klog.V(5).Infof("AuthFs cmd: %v", cmdArgs)

	if config.FormatInPod {
		cmd := strings.Join(cmdArgs, " ")
		return cmd, nil
	}

	authCmd := j.Exec.Command(config.CliPath, args...)
	envs := syscall.Environ()
	for key, val := range setting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", key, val))
	}
	authCmd.SetEnv(envs)
	res, err := authCmd.CombinedOutput()
	klog.Infof("Auth output is %s", res)
	return string(res), err
}

// MountFs mounts JuiceFS with idempotency
func (j *juicefs) MountFs(jfsSetting *config.JfsSetting) (string, error) {
	var mnt podmount.MntInterface
	if jfsSetting.UsePod {
		jfsSetting.MountPath = filepath.Join(config.PodMountBase, jfsSetting.VolumeId)
		mnt = j.podMount
	} else {
		jfsSetting.MountPath = filepath.Join(config.MountBase, jfsSetting.VolumeId)
		mnt = j.processMount
	}

	klog.V(5).Infof("Mount: mounting %q at %q with options %v", jfsSetting.Source, jfsSetting.MountPath, jfsSetting.Options)
	err := mnt.JMount(jfsSetting)
	if err != nil {
		return "", err
	}
	return jfsSetting.MountPath, nil
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

func (j *juicefs) ceFormat(secrets map[string]string, noUpdate bool, setting *config.JfsSetting) (string, error) {
	if secrets == nil {
		return "", status.Errorf(codes.InvalidArgument, "Nil secrets")
	}

	if secrets["name"] == "" {
		return "", status.Errorf(codes.InvalidArgument, "Empty name")
	}

	if secrets["metaurl"] == "" {
		return "", status.Errorf(codes.InvalidArgument, "Empty metaurl")
	}

	args := []string{"format"}
	cmdArgs := []string{config.CeCliPath, "format"}
	if noUpdate {
		cmdArgs = append(cmdArgs, "--no-update")
		args = append(args, "--no-update")
	}
	keys := []string{
		"storage",
		"bucket",
		"access-key",
		"block-size",
		"compress",
		"trash-days",
		"capacity",
		"inodes",
		"shards",
	}
	keysStripped := map[string]string{"secret-key": "secretkey"}
	isOptional := map[string]bool{
		"block-size": true,
		"compress":   true,
		"trash-days": true,
		"capacity":   true,
		"inodes":     true,
		"shards":     true,
	}
	for _, k := range keys {
		if !isOptional[k] || secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, secrets[k]))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	for k, v := range keysStripped {
		if !isOptional[k] || secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=${%s}", k, v))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	cmdArgs = append(cmdArgs, "${metaurl}", secrets["name"])
	args = append(args, secrets["metaurl"], secrets["name"])

	klog.V(5).Infof("ceFormat cmd: %v", cmdArgs)

	if config.FormatInPod {
		cmd := strings.Join(cmdArgs, " ")
		return cmd, nil
	}

	formatCmd := j.Exec.Command(config.CeCliPath, args...)
	envs := syscall.Environ()
	for key, val := range setting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", key, val))
	}
	if secrets["storage"] == "ceph" || secrets["storage"] == "gs" {
		envs = append(envs, "JFS_NO_CHECK_OBJECT_STORAGE=1")
	}
	formatCmd.SetEnv(envs)
	res, err := formatCmd.CombinedOutput()
	klog.Infof("Format output is %s", res)
	return string(res), err
}
