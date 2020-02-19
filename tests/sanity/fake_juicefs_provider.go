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
	"path/filepath"

	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
)

type fakeJfs struct {
	basePath string
	volumes  map[string]string
}

type fakeJfsProvider struct {
	mount.FakeMounter
	fs map[string]fakeJfs
}

func (j *fakeJfsProvider) JfsMount(volumeID string, secrets map[string]string, options []string) (juicefs.Jfs, error) {
	jfsName := "fake"
	fs, ok := j.fs[jfsName]

	if ok {
		return &fs, nil
	}

	fs = fakeJfs{
		basePath: "/jfs/fake",
		volumes:  map[string]string{},
	}

	j.fs[jfsName] = fs
	return &fs, nil
}

func (j *fakeJfsProvider) JfsUnmount(volumeID string) {}

func (j *fakeJfsProvider) AuthFs(secrets map[string]string) ([]byte, error) {
	return []byte{}, nil
}

func (j *fakeJfsProvider) MountFs(volumeID, name string, options []string) (string, error) {
	return "/jfs/fake", nil
}

func newFakeJfsProvider() *fakeJfsProvider {
	return &fakeJfsProvider{
		fs: map[string]fakeJfs{},
	}
}

func (fs *fakeJfs) CreateVol(name, subPath string) (string, error) {
	_, ok := fs.volumes[name]

	if !ok {
		vol := filepath.Join(fs.basePath, name)
		fs.volumes[name] = vol
		return vol, nil
	}

	return "", status.Error(codes.AlreadyExists, "Volume already exists")
}

func (fs *fakeJfs) DeleteVol(name string) error {
	delete(fs.volumes, name)
	return nil
}

func (fs *fakeJfs) GetBasePath() string {
	return fs.basePath
}
