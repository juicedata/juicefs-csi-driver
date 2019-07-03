package juicefs

import (
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
)

// Interface of juicefs provider
type Interface interface {
	mount.Interface
	CmdAuth(name string, secrets map[string]string) ([]byte, error)
	SafeMount(name string, options []string) (string, error)
}

type juicefs struct {
	mount.SafeFormatAndMount
}

var _ Interface = &juicefs{}

// NewJfsProvider creates a provider for juicefs volumes
func NewJfsProvider(mounter *mount.SafeFormatAndMount) (Interface, error) {
	if mounter == nil {
		mounter = &mount.SafeFormatAndMount{
			Interface: mount.New(""),
			Exec:      mount.NewOsExec(),
		}
	}

	return &juicefs{}, nil
}

func (j *juicefs) CmdAuth(name string, secrets map[string]string) ([]byte, error) {
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
	mounPath := path.Join(mountBase, name)
	exists, err := j.ExistsPath(mounPath)

	if err != nil {
		return mounPath, status.Errorf(codes.Internal, "Could not check mount point %q exists: %v", mounPath, err)
	}

	if !exists {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", name, mounPath, options)
		if err := j.Mount(name, mounPath, fsType, []string{}); err != nil {
			os.Remove(mounPath)
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", name, mounPath, err)
		}
		return mounPath, nil
	}

	// path exists
	notMnt, err := j.IsLikelyNotMountPoint(mounPath)
	if err != nil {
		return mounPath, status.Errorf(codes.Internal, "Could not check %q IsLikelyNotMountPoint: %v", mounPath, err)
	}

	if notMnt {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", name, mounPath, options)
		if err := j.Mount(name, mounPath, fsType, []string{}); err != nil {
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", name, mounPath, err)
		}
		return mounPath, nil
	}

	klog.V(5).Infof("Mount: skip mounting for existing mount point %q", mounPath)
	return mounPath, nil
}
