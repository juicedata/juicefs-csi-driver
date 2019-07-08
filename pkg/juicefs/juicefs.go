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
	cliPath   = "/usr/bin/juicefs"
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
	JfsMount(secrets map[string]string, options []string) (Jfs, error)
	AuthFs(secrets map[string]string) ([]byte, error)
	MountFs(name string, options []string) (string, error)
}

type juicefs struct {
	mount.SafeFormatAndMount
}

var _ Interface = &juicefs{}

// Volume in JuiceFS is a managed directory
type Volume struct {
	// CapacityBytes of the volume
	CapacityBytes int64 `json:"capacity_bytes"`
	MountPoints   map[string]struct{}
}

// Meta file
type Meta struct {
	// Volume meta
	Volume Volume `json:"volume"`
}

type jfs struct {
	Provider  *juicefs
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

	exists, err := fs.Provider.ExistsPath(volPath)
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

	klog.V(5).Infof("CreateVol: checking %q exists in %v", volPath, fs)
	exists, err := fs.Provider.ExistsPath(volPath)
	if err != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}
	if exists {
		klog.V(5).Infof("CreateVol: reading meta from %q", metaPath)
		file, err := ioutil.ReadFile(metaPath)
		if err != nil {
			return Volume{}, status.Errorf(codes.Internal, "Could not read volume meta from %q", metaPath)
		}
		meta := Meta{}
		if json.Unmarshal([]byte(file), &meta) != nil {
			return Volume{}, status.Errorf(codes.Internal, "Invalid meta %q", metaPath)
		}
		if meta.Volume.CapacityBytes >= capacityBytes {
			klog.V(5).Infof("CreateVol: returning existed volume %v", meta.Volume)
			return meta.Volume, nil
		}
		return Volume{}, status.Errorf(codes.AlreadyExists, "Volume: %q, capacity bytes: %d", volName, capacityBytes)
	}

	klog.V(5).Infof("CreateVol: volume not existed")
	vol := Volume{
		CapacityBytes: capacityBytes,
	}
	meta, err := json.Marshal(Meta{
		vol,
	})
	if err != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not marshal meta ID=%q capacityBytes=%v", volName, capacityBytes)
	}
	if err := fs.Provider.MakeDir(volPath); err != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not make directory %q", volPath)
	}
	if ioutil.WriteFile(metaPath, meta, 0644) != nil {
		return Volume{}, status.Errorf(codes.Internal, "Could not write meta to %q", metaPath)
	}

	return vol, nil
}

func (fs *jfs) DeleteVol(volName string) error {
	jfsProvider := fs.Provider
	_, err := fs.GetVolByID(volName)
	st, ok := status.FromError(err)
	if ok && st.Code() == codes.NotFound {
		return nil
	}
	if err != nil {
		return err
	}
	stdoutStderr, err := jfsProvider.RmrDir(path.Join(fs.MountPath, volName))
	klog.V(5).Infof("DeleteVol: rmr output is '%s'\n", stdoutStderr)
	return err
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

// JfsMount auths and mounts juicefs
func (j *juicefs) JfsMount(secrets map[string]string, options []string) (Jfs, error) {
	stdoutStderr, err := j.AuthFs(secrets)
	klog.V(5).Infof("MountFs: authentication output is '%s'\n", stdoutStderr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not auth juicefs: %v", err)
	}

	mountPath, err := j.MountFs(secrets["name"], options)
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

func (j *juicefs) RmrDir(directory string) ([]byte, error) {
	args := []string{"rmr", directory}
	klog.V(5).Infof("RmrDir: removing directory recursively: %q", directory)
	return j.Exec.Run(cliPath, args...)
}

// AuthFs authenticates juicefs
func (j *juicefs) AuthFs(secrets map[string]string) ([]byte, error) {
	if secrets == nil || secrets["name"] == "" || secrets["token"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Nil secrets, or empty name or token")
	}

	args := []string{"auth", secrets["name"]}
	keys := []string{"token", "accesskey", "secretkey", "accesskey2", "secretkey2"}
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
	// klog.V(5).Infof("AuthFs: cmd %q, args %#v", cmd, args)
	return j.Exec.Run(cliPath, args...)
}

// MountFs mounts juicefs with idempotency
func (j *juicefs) MountFs(name string, options []string) (string, error) {
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
