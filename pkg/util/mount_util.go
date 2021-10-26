package util

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/mount"
	"os"
)

type MountInter interface {
	mount.Interface
}

func IsMounted(target string, mounter mount.Interface) (bool, error) {
	exists, err := mount.PathExists(target)
	notMnt, corruptedMnt := true, false
	if exists && err == nil {
		notMnt, err = mount.IsNotMountPoint(mounter, target)
		if err != nil {
			return false, status.Errorf(codes.Internal, "Check target path is mountpoint failed: %q", err)
		}
		return !notMnt, nil
	} else if err != nil {
		if corruptedMnt = mount.IsCorruptedMnt(err); !corruptedMnt {
			return true, nil
		}
	}
	return false, nil // path not exists
}

func IsMntPathConnectedErr(target string) (bool, error) {
	_, err := os.Stat(target)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}
