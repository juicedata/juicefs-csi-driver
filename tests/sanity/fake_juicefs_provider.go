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
	"path"

	"k8s.io/kubernetes/pkg/util/mount"
)

type fakeJfsProvider struct {
	mount.FakeMounter
	volumes map[string]bool
}

func newFakeJfsProvider() *fakeJfsProvider {
	return &fakeJfsProvider{
		volumes: map[string]bool{},
	}
}

func (j *fakeJfsProvider) CmdAuth(name string, secrets map[string]string) ([]byte, error) {
	return []byte{}, nil
}

func (j *fakeJfsProvider) SafeMount(name string, options []string) (string, error) {
	target := path.Join("/tmp", name)
	j.Mount(name, target, "juicefs", []string{})

	return target, nil
}

func (j *fakeJfsProvider) MakeDir(pathname string) error {
	j.volumes[pathname] = true
	return nil
}

func (j *fakeJfsProvider) ExistsPath(pathname string) (bool, error) {
	_, ok := j.volumes[pathname]
	return ok, nil
}
