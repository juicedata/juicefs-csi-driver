package juicefs

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/mount"
)

const (
	cmd       = "/usr/bin/juicefs"
	mountBase = "/jfs"
	fsType    = "juicefs"
	// DefaultCapacityBytes is 10 Pi
	DefaultCapacityBytes = 10 * 1024 * 1024 * 1024 * 1024 * 1024
	// DotFile saves volume attributes
	metaFile = ".juicefs"
)

// Interface of juicefs provider
type Interface interface {
	mount.Interface
	MountFs(name string, secrets map[string]string, options []string) (Jfs, error)
	Auth(name string, secrets map[string]string) ([]byte, error)
	SafeMount(name string, options []string) (string, error)
}

type juicefs struct {
	mount.SafeFormatAndMount
}

var _ Interface = &juicefs{}

// Volume in JuiceFS is a managed directory
type Volume struct {
	// CapacityBytes of the volume
	CapacityBytes int64 `json:"capacity_bytes"`
}

// Meta file
type Meta struct {
	// Volume meta
	Volume Volume `json:"volume"`
}

type jfs struct {
	mount.SafeFormatAndMount

	Name      string
	MountPath string
	Options   []string
	Volumes   []Volume
}

// Jfs is the interface of a mounted file system
type Jfs interface {
	GetBasePath() string
	CreateVol(volName string, capacityBytes int64) (Volume, error)
	DeleteVol(volName string) error
	GetVolByID(volID string) (Volume, error)
}

var _ Jfs = &jfs{}

func (fs *jfs) GetVolByID(volID string) (Volume, error) {
	volPath := path.Join(fs.MountPath, volID)

	exists, err := fs.ExistsPath(volPath)
	if err != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		return Volume{}, status.Errorf(codes.NotFound, "Could not find volume: %q", volID)
	}

	metaPath := path.Join(volPath, metaFile)
	file, err := ioutil.ReadFile(metaPath)
	if err != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not read volume meta from %q", metaPath)
	}

	meta := Meta{}
	if json.Unmarshal([]byte(file), &meta) != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not unmarshal meta %v", file)
	}

	return meta.Volume, nil
}

func (fs *jfs) GetBasePath() string {
	return fs.MountPath
}

func (fs *jfs) CreateVol(volName string, capacityBytes int64) (Volume, error) {
	volPath := path.Join(fs.MountPath, volName)
	metaPath := path.Join(volPath, metaFile)

	exists, err := fs.ExistsPath(volPath)
	if err != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}
	if exists {
		file, err := ioutil.ReadFile(metaPath)
		if err != nil {
			return Volume{}, status.Errorf(codes.Internal, "Could not read volume meta from %q", metaPath)
		}
		meta := Meta{}
		if json.Unmarshal([]byte(file), &meta) != nil {
			return Volume{}, status.Errorf(codes.Internal, "Invalid meta %q", metaPath)
		}
		if meta.Volume.CapacityBytes >= capacityBytes {
			return meta.Volume, nil
		}
		return Volume{}, status.Errorf(codes.AlreadyExists, "Volume: %q, capacity bytes: %d", volName, capacityBytes)
	}

	vol := Volume{
		CapacityBytes: capacityBytes,
	}

	meta, err := json.Marshal(Meta{
		vol,
	})
	if err != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not marshal meta ID=%q capacityBytes=%v", volName, capacityBytes)
	}
	if err := fs.MakeDir(volPath); err != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not make directory %q", volPath)
	}
	if ioutil.WriteFile(metaPath, meta, 0644) != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not write meta to %q", metaPath)
	}

	return vol, nil
}

func (fs *jfs) DeleteVol(volName string) error {
	return status.Errorf(codes.Unimplemented, "Not implemented")
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

// MountFs auths and mount the specified file system
func (j *juicefs) MountFs(name string, secrets map[string]string, options []string) (Jfs, error) {
	stdoutStderr, err := j.Auth(name, secrets)
	klog.V(5).Infof("MountFs: authentication output is '%s'\n", stdoutStderr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not auth juicefs %s: %v", name, err)
	}

	mountPath, err := j.SafeMount(name, options)
	if err != nil {
		klog.Errorf("MountFs: failed to mount %q", name)
		return nil, err
	}

	return &jfs{
		Name:      name,
		MountPath: mountPath,
		Options:   options,
	}, nil
}

func (j *juicefs) Auth(name string, secrets map[string]string) ([]byte, error) {
	if secrets == nil || secrets["token"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Nil secrets or empty token")
	}

	token := secrets["token"]
	args := []string{"auth", name, "--token", token}
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
	// DEBUG only, secrets exposed in args
	// klog.V(5).Infof("Auth: cmd %q, args %#v", cmd, args)
	return j.Exec.Run(cmd, args...)
}

// SafeMount checks mount point for idempotency
func (j *juicefs) SafeMount(name string, options []string) (string, error) {
	mountPath := path.Join(mountBase, name)
	exists, err := j.ExistsPath(mountPath)

	if err != nil {
		return mountPath, status.Errorf(codes.Internal, "Could not check mount point %q exists: %v", mountPath, err)
	}

	if !exists {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", name, mountPath, options)
		if err := j.Mount(name, mountPath, fsType, []string{}); err != nil {
			os.Remove(mountPath)
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", name, mountPath, err)
		}
		return mountPath, nil
	}

	// path exists
	notMnt, err := j.IsLikelyNotMountPoint(mountPath)
	if err != nil {
		return mountPath, status.Errorf(codes.Internal, "Could not check %q IsLikelyNotMountPoint: %v", mountPath, err)
	}

	if notMnt {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", name, mountPath, options)
		if err := j.Mount(name, mountPath, fsType, []string{}); err != nil {
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", name, mountPath, err)
		}
		return mountPath, nil
	}

	klog.V(5).Infof("Mount: skip mounting for existing mount point %q", mountPath)
	return mountPath, nil
}
