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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/mount"
)

const (
	cliPath     = "/usr/bin/juicefs"
	ceMountPath = "/bin/mount.juicefs"
	mountBase   = "/jfs"
	fsType      = "juicefs"
	// DefaultCapacityBytes is 10 Pi
	DefaultCapacityBytes = 10 * 1024 * 1024 * 1024 * 1024 * 1024
)

// Interface of juicefs provider
type Interface interface {
	mount.Interface
	JfsMount(volumeID string, secrets map[string]string, options []string) (Jfs, error)
	JfsUnmount(volumeID string)
	AuthFs(secrets map[string]string) ([]byte, error)
	MountFs(volumeID, source string, options []string) (string, error)
	Version() ([]byte, error)
}

type juicefs struct {
	mount.SafeFormatAndMount
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
	DeleteVol(volumeID string) error
}

var _ Jfs = &jfs{}

func (fs *jfs) GetBasePath() string {
	return fs.MountPath
}

// CreateVol creates the directory needed
func (fs *jfs) CreateVol(volumeID, subPath string) (string, error) {
	volPath := filepath.Join(fs.MountPath, subPath)

	klog.V(5).Infof("CreateVol: checking %q exists in %v", volPath, fs)
	exists, err := fs.Provider.ExistsPath(volPath)
	if err != nil {
		return "", status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		klog.V(5).Infof("CreateVol: volume not existed")
		err := fs.Provider.MakeDir(volPath)
		if err != nil {
			return "", status.Errorf(codes.Internal, "Could not make directory for meta %q", volPath)
		}
	}

	return volPath, nil
}

func (fs *jfs) DeleteVol(volumeID string) error {
	volPath := filepath.Join(fs.MountPath, volumeID)
	if existed, err := fs.Provider.ExistsPath(volPath); err != nil {
		return status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	} else if existed {
		stdoutStderr, err := fs.Provider.RmrDir(volPath)
		klog.V(5).Infof("DeleteVol: rmr output is '%s'", stdoutStderr)
		if err != nil {
			return status.Errorf(codes.Internal, "Could not delete volume path %q: %v", volPath, err)
		}
	}
	return nil
}

// NewJfsProvider creates a provider for juicefs file system
func NewJfsProvider(mounter *mount.SafeFormatAndMount) (Interface, error) {
	if mounter == nil {
		mounter = &mount.SafeFormatAndMount{
			Interface: mount.New(""),
			Exec:      mount.NewOsExec(),
		}
	}

	return &juicefs{*mounter}, nil
}

func (j *juicefs) IsNotMountPoint(dir string) (bool, error) {
	return mount.IsNotMountPoint(j, dir)
}

// JfsMount auths and mounts juicefs
func (j *juicefs) JfsMount(volumeID string, secrets map[string]string, options []string) (Jfs, error) {
	source, ok := secrets["metaurl"]
	if !ok {
		j.Upgrade()
		stdoutStderr, err := j.AuthFs(secrets)
		klog.V(5).Infof("MountFs: authentication output is '%s'\n", stdoutStderr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not auth juicefs: %v", err)
		}
		source = secrets["name"]
	}

	mountPath, err := j.MountFs(volumeID, source, options)
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

func (j *juicefs) JfsUnmount(volumeID string) {
	mountPath := filepath.Join(mountBase, volumeID)
	klog.V(5).Infof("JfsUnmount: umount %s", mountPath)
	if err := j.Unmount(mountPath); err != nil {
		klog.V(5).Infof("JfsUnmount: error umount %s, %v", mountPath, err)
	}
}

func (j *juicefs) RmrDir(directory string) ([]byte, error) {
	args := []string{"rmr", directory}
	klog.V(5).Infof("RmrDir: removing directory recursively: %q", directory)
	return j.Exec.Run(cliPath, args...)
}

// AuthFs authenticates juicefs
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
	klog.V(5).Infof("AuthFs: cmd %q, args %#v", cliPath, argsStripped)
	return j.Exec.Run(cliPath, args...)
}

// MountFs mounts juicefs with idempotency
func (j *juicefs) MountFs(volumeID, source string, options []string) (string, error) {
	var isCeMount bool
	if strings.Contains(source, "://") {
		isCeMount = true
	}

	mountPath := filepath.Join(mountBase, volumeID)

	exists, err := j.ExistsPath(mountPath)
	if err != nil {
		return mountPath, status.Errorf(codes.Internal, "Could not check mount point %q exists: %v", mountPath, err)
	}

	if !exists {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", source, mountPath, options)
		if isCeMount {
			err = j.ceMount(source, mountPath, fsType, options)
		} else {
			err = j.Mount(source, mountPath, fsType, options)
		}
		if err != nil {
			os.Remove(mountPath)
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, mountPath, err)
		}
		return mountPath, nil
	}

	// path exists
	notMnt, err := j.IsLikelyNotMountPoint(mountPath)
	if err != nil {
		return mountPath, status.Errorf(codes.Internal, "Could not check %q IsLikelyNotMountPoint: %v", mountPath, err)
	}

	if notMnt {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", source, mountPath, options)
		if isCeMount {
			err = j.ceMount(source, mountPath, fsType, options)
		} else {
			err = j.Mount(source, mountPath, fsType, options)
		}
		if err != nil {
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, mountPath, err)
		}
		return mountPath, nil
	}

	klog.V(5).Infof("Mount: skip mounting for existing mount point %q", mountPath)
	return mountPath, nil
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

	err := exec.CommandContext(ctx, cliPath, "version", "-u").Run()
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
	return j.Exec.Run(cliPath, "version")
}

func (j *juicefs) ceMount(source string, mountPath string, fsType string, options []string) error {
	klog.V(5).Infof("ceMount: mount %v at %v", source, mountPath)
	mountArgs := []string{source, mountPath}
	if len(options) > 0 {
		mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
	}

	if exist, err := j.ExistsPath(mountPath); err != nil {
		return status.Errorf(codes.Internal, "Could not check existence of dir %q: %v", mountPath, err)
	} else if !exist {
		if err = j.MakeDir(mountPath); err != nil {
			return status.Errorf(codes.Internal, "Could not create dir %q: %v", mountPath, err)
		}
	}

	if notMounted, err := j.IsLikelyNotMountPoint(mountPath); err != nil {
		return err
	} else if !notMounted {
		err = j.Unmount(mountPath)
		if err != nil {
			klog.V(5).Infof("Unmount before mount failed: %v", err)
			return err
		}
		klog.V(5).Infof("Unmount %v", mountPath)
	}

	out, err := j.Exec.Run(ceMountPath, mountArgs...)
	if err != nil {
		klog.V(5).Infof("ceMount failed with output: %v", string(out))
	}

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
	return status.Errorf(codes.Internal, "Mount %v at %v failed: mount isn't ready in 30 seconds", source, mountPath)
}
