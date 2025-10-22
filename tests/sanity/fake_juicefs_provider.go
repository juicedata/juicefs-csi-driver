/*
Copyright 2018 The Kubernetes Authors.

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

package sanity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"

	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
)

type fakeJfs struct {
	basePath string
	volumes  map[string]string
	settings *config.JfsSetting
}

type fakeJfsProvider struct {
	mount.FakeMounter
	fs        map[string]fakeJfs
	snapshots map[string]string
}

// CreateSnapshot implements juicefs.Interface.
func (j *fakeJfsProvider) CreateSnapshot(ctx context.Context, snapshotID string, sourceVolumeID string, secrets map[string]string, volCtx map[string]string) error {
	if volumeId, ok := j.snapshots[snapshotID]; ok {
		if volumeId != sourceVolumeID {
			return os.ErrExist
		}
	}
	j.snapshots[snapshotID] = sourceVolumeID
	return nil
}

// DeleteSnapshot implements juicefs.Interface.
func (j *fakeJfsProvider) DeleteSnapshot(ctx context.Context, snapshotID string, sourceVolumeID string, secrets map[string]string) error {
	delete(j.snapshots, snapshotID)
	return nil
}

// GetMountRefs implements juicefs.Interface.
// Subtle: this method shadows the method (FakeMounter).GetMountRefs of fakeJfsProvider.FakeMounter.
func (j *fakeJfsProvider) GetMountRefs(pathname string) ([]string, error) {
	panic("unimplemented")
}

// IsLikelyNotMountPoint implements juicefs.Interface.
// Subtle: this method shadows the method (FakeMounter).IsLikelyNotMountPoint of fakeJfsProvider.FakeMounter.
func (j *fakeJfsProvider) IsLikelyNotMountPoint(file string) (bool, error) {
	panic("unimplemented")
}

// List implements juicefs.Interface.
// Subtle: this method shadows the method (FakeMounter).List of fakeJfsProvider.FakeMounter.
func (j *fakeJfsProvider) List() ([]mount.MountPoint, error) {
	panic("unimplemented")
}

// Mount implements juicefs.Interface.
// Subtle: this method shadows the method (FakeMounter).Mount of fakeJfsProvider.FakeMounter.
func (j *fakeJfsProvider) Mount(source string, target string, fstype string, options []string) error {
	panic("unimplemented")
}

// MountSensitive implements juicefs.Interface.
// Subtle: this method shadows the method (FakeMounter).MountSensitive of fakeJfsProvider.FakeMounter.
func (j *fakeJfsProvider) MountSensitive(source string, target string, fstype string, options []string, sensitiveOptions []string) error {
	panic("unimplemented")
}

// RestoreSnapshot implements juicefs.Interface.
func (j *fakeJfsProvider) RestoreSnapshot(ctx context.Context, snapshotID string, sourceVolumeID string, targetVolumeID string, targetPath string, secrets map[string]string, volCtx map[string]string) error {
	if _, ok := j.snapshots[snapshotID]; !ok {
		return fmt.Errorf("snapshot %s not found", snapshotID)
	}
	return nil
}

// Unmount implements juicefs.Interface.
// Subtle: this method shadows the method (FakeMounter).Unmount of fakeJfsProvider.FakeMounter.
func (j *fakeJfsProvider) Unmount(target string) error {
	panic("unimplemented")
}

var _ juicefs.Interface = &fakeJfsProvider{}

func (j *fakeJfsProvider) CreateTarget(ctx context.Context, target string) error {
	exist, err := mount.PathExists(target)
	if err != nil {
		return err
	}
	if !exist {
		return os.Mkdir(target, 0750)
	}
	return nil
}

func (j *fakeJfsProvider) Settings(ctx context.Context, volumeID, uniqueId, uuid string, secrets, volCtx map[string]string, options []string) (*config.JfsSetting, error) {
	return new(config.JfsSetting), nil
}

func (j *fakeJfsProvider) JfsCreateVol(ctx context.Context, volumeID string, subPath string, secrets, volCtx map[string]string) error {
	return nil
}

func (j *fakeJfsProvider) JfsDeleteVol(ctx context.Context, volumeID string, target string, secrets, volCtx map[string]string, options []string) error {
	return nil
}

func (j *fakeJfsProvider) JfsMount(ctx context.Context, volumeID string, target string, secrets, volCtx map[string]string, options []string) (juicefs.Jfs, error) {
	jfsName := "fake"
	fs, ok := j.fs[jfsName]

	if ok {
		return &fs, nil
	}

	fs = fakeJfs{
		basePath: "/jfs/fake",
		volumes:  map[string]string{},
		settings: &config.JfsSetting{},
	}

	j.fs[jfsName] = fs
	return &fs, nil
}

func (j *fakeJfsProvider) JfsCleanupMountPoint(ctx context.Context, mountPath string) error {
	return nil
}

func (j *fakeJfsProvider) AuthFs(ctx context.Context, secrets map[string]string, setting *config.JfsSetting, force bool) (string, error) {
	return "", nil
}
func (j *fakeJfsProvider) JfsUnmount(ctx context.Context, volumeId, mountPath string) error {
	exist, err := mount.PathExists(mountPath)
	if err != nil {
		return err
	}
	if exist {
		return os.Remove(mountPath)
	}
	return nil
}

func (j *fakeJfsProvider) SetQuota(ctx context.Context, secrets map[string]string, jfsSetting *config.JfsSetting, quotaPath string, capacity int64) error {
	return nil
}

func (j *fakeJfsProvider) GetSubPath(ctx context.Context, volumeID string) (string, error) {
	return volumeID, nil
}

func newFakeJfsProvider() *fakeJfsProvider {
	return &fakeJfsProvider{
		fs:        map[string]fakeJfs{},
		snapshots: map[string]string{},
	}
}

func (fs *fakeJfs) CreateVol(ctx context.Context, name, subPath string) (string, error) {
	_, ok := fs.volumes[name]

	if !ok {
		vol := filepath.Join(fs.basePath, name)
		fs.volumes[name] = vol
		return vol, nil
	}

	return fs.volumes[name], nil
}

func (fs *fakeJfs) GetBasePath() string {
	return fs.basePath
}

func (fs *fakeJfs) GetSetting() *config.JfsSetting {
	return fs.settings
}

func (fs *fakeJfs) BindTarget(ctx context.Context, bindSource, target string) error {
	return nil
}

func (j *fakeJfsProvider) Status(ctx context.Context, metaUrl string) error {
	return nil
}
