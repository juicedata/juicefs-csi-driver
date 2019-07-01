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
)

type fakeJuiceFS struct {}

func newFakeJuiceFS() *fakeJuiceFS {
	return &fakeJuiceFS{}
}

func (j *fakeJuiceFS) Auth(source string, secrets map[string]string) ([]byte, error) {
	return []byte{}, nil
}

func (j *fakeJuiceFS) Mount(source string, basePath string, options []string) (string, error) {
	targetPath := path.Join(basePath, source)

	return targetPath, nil
}


func (j *fakeJuiceFS) CreateVolume(pathname string) error {
	return nil
}
