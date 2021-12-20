/*
Copyright 2021 Juicedata Inc

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

package mocks

import (
	"io/fs"
	"syscall"
	"time"
)

type FakeFileInfoIno1 struct {
}

func (f FakeFileInfoIno1) Name() string {
	return "fake"
}

func (f FakeFileInfoIno1) Size() int64 {
	return 1
}

func (f FakeFileInfoIno1) Mode() fs.FileMode {
	return fs.ModePerm
}

func (f FakeFileInfoIno1) ModTime() time.Time {
	return time.Time{}
}

func (f FakeFileInfoIno1) IsDir() bool {
	return false
}

func (f FakeFileInfoIno1) Sys() interface{} {
	return &syscall.Stat_t{Ino: 1}
}

type FakeFileInfoIno2 struct {
}

func (f FakeFileInfoIno2) Name() string {
	return "test"
}

func (f FakeFileInfoIno2) Size() int64 {
	return 1
}

func (f FakeFileInfoIno2) Mode() fs.FileMode {
	return fs.ModeDevice
}

func (f FakeFileInfoIno2) ModTime() time.Time {
	return time.Time{}
}

func (f FakeFileInfoIno2) IsDir() bool {
	return false
}

func (f FakeFileInfoIno2) Sys() interface{} {
	return &syscall.Stat_t{Ino: 2}
}
