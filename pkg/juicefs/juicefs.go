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
	cmd    = "/usr/bin/juicefs"
	mnt    = "/jfs"
	fsType = "juicefs"
	// DefaultCapacityBytes is 10 Pi
	DefaultCapacityBytes = 10 * 1024 * 1024 * 1024 * 1024 * 1024
)

// JuiceFS abstracts the SaaS
type JuiceFS interface {
	Auth(source string, secrets map[string]string) ([]byte, error)
	Mount(source string, basePath string, options []string) (string, error)
	CreateVolume(pathname string) error
}

type juicefs struct {
	mounter *mount.SafeFormatAndMount
}

var _ JuiceFS = &juicefs{}

// NewJuiceFS returns a new instance of JuiceFS cli
func NewJuiceFS() (JuiceFS, error) {
	return &juicefs{
		mounter: newSafeMounter(),
	}, nil
}

func (j *juicefs) Auth(source string, secrets map[string]string) ([]byte, error) {
	if secrets == nil || secrets["token"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Nil secrets or empty token")
	}

	token := secrets["token"]
	args := []string{"auth", source, "--token", token}
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
	return j.mounter.Exec.Run(cmd, args...)
}

func (j *juicefs) Mount(source string, basePath string, options []string) (string, error) {
	targetPath := path.Join(basePath, source)
	exists, err := j.mounter.ExistsPath(targetPath)

	if err != nil {
		return targetPath, status.Errorf(codes.Internal, "Could not check mount point %q exists: %v", targetPath, err)
	}

	if !exists {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", source, targetPath, options)
		if err := j.mounter.Mount(source, targetPath, fsType, []string{}); err != nil {
			os.Remove(targetPath)
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, targetPath, err)
		}
		return targetPath, nil
	}

	// path exists
	notMnt, err := j.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		return targetPath, status.Errorf(codes.Internal, "Could not check %q IsLikelyNotMountPoint: %v", targetPath, err)
	}

	if notMnt {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", source, targetPath, options)
		if err := j.mounter.Mount(source, targetPath, fsType, []string{}); err != nil {
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, targetPath, err)
		}
		return targetPath, nil
	}

	klog.V(5).Infof("Mount: skip mounting for existing mount point %q", targetPath)
	return targetPath, nil
}

func (j *juicefs) CreateVolume(pathname string) error {
	return j.mounter.MakeDir(pathname)
}

func newSafeMounter() *mount.SafeFormatAndMount {
	return &mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      mount.NewOsExec(),
	}
}
