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

package mount

import (
	"errors"
	. "github.com/agiledragon/gomonkey"
	"github.com/golang/mock/gomock"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver/mocks"
	. "github.com/smartystreets/goconvey/convey"
	k8sexec "k8s.io/utils/exec"
	k8sMount "k8s.io/utils/mount"
	"os"
	"os/exec"
	"reflect"
	"syscall"
	"testing"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func TestNewProcessMount(t *testing.T) {
	type args struct {
		setting *jfsConfig.JfsSetting
	}
	tests := []struct {
		name string
		args args
		want MntInterface
	}{
		{
			name: "test",
			args: args{
				setting: nil,
			},
			want: &ProcessMount{
				k8sMount.SafeFormatAndMount{
					Interface: k8sMount.New(""),
					Exec:      k8sexec.New(),
				}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewProcessMount(
				k8sMount.SafeFormatAndMount{
					Interface: k8sMount.New(""),
					Exec:      k8sexec.New(),
				}); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewProcessMount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessMount_JUmount(t *testing.T) {
	targetPath := "/test"
	type args struct {
		volumeId string
		target   string
	}
	tests := []struct {
		name       string
		expectMock func(mockMounter mocks.MockInterface)
		args       args
		wantErr    bool
	}{
		{
			name: "",
			expectMock: func(mockMounter mocks.MockInterface) {
				mockMounter.EXPECT().Unmount(targetPath).Return(nil)
			},
			args: args{
				volumeId: "ttt",
				target:   targetPath,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMounter := mocks.NewMockInterface(mockCtl)
			if tt.expectMock != nil {
				tt.expectMock(*mockMounter)
			}
			mounter := &k8sMount.SafeFormatAndMount{
				Interface: mockMounter,
				Exec:      k8sexec.New(),
			}
			p := &ProcessMount{
				SafeFormatAndMount: *mounter,
			}
			if err := p.JUmount(tt.args.volumeId, tt.args.target); (err != nil) != tt.wantErr {
				t.Errorf("JUmount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProcessMount_JMount(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "test-ee",
			testFunc: func(t *testing.T) {
				eeSource := "test"
				targetPath := "/test"
				volumeId := "test"

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMounter := mocks.NewMockInterface(mockCtl)
				mounter := &k8sMount.SafeFormatAndMount{
					Interface: mockMounter,
					Exec:      k8sexec.New(),
				}
				mockMounter.EXPECT().Mount(eeSource, targetPath, jfsConfig.FsType, nil).Return(nil)
				p := &ProcessMount{
					SafeFormatAndMount: *mounter,
				}
				if err := p.JMount(&jfsConfig.JfsSetting{Source: eeSource}, volumeId, targetPath, "", nil); err != nil {
					t.Errorf("JMount() error = %v", err)
				}
			},
		},
		{
			name: "test-ee-error",
			testFunc: func(t *testing.T) {
				eeSource := "test"
				targetPath := "/test"
				volumeId := "test"

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMounter := mocks.NewMockInterface(mockCtl)
				mounter := &k8sMount.SafeFormatAndMount{
					Interface: mockMounter,
					Exec:      k8sexec.New(),
				}
				mockMounter.EXPECT().Mount(eeSource, targetPath, jfsConfig.FsType, nil).Return(errors.New("test"))
				p := &ProcessMount{
					SafeFormatAndMount: *mounter,
				}
				if err := p.JMount(&jfsConfig.JfsSetting{Source: eeSource}, volumeId, targetPath, "", nil); err == nil {
					t.Errorf("JMount() error = %v", err)
				}
			},
		},
		{
			name: "test-ce",
			testFunc: func(t *testing.T) {
				ceSource := "redis://127.0.0.1:6379/0"
				targetPath := "/test"
				volumeId := "test"
				options := []string{"debug"}
				Convey("Test JMount", t, func() {
					Convey("test", func() {
						patch1 := ApplyFunc(k8sMount.PathExists, func(path string) (bool, error) {
							return false, nil
						})
						defer patch1.Reset()
						patch2 := ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
							return nil
						})
						defer patch2.Reset()
						patch3 := ApplyFunc(syscall.Environ, func() []string {
							return []string{}
						})
						defer patch3.Reset()
						cmd := exec.Command("mount")
						patch4 := ApplyMethod(reflect.TypeOf(cmd), "Run", func(_ *exec.Cmd) error {
							return nil
						})
						defer patch4.Reset()
						patch5 := ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
							return mocks.FakeFileInfoIno1{}, nil
						})
						defer patch5.Reset()

						mockCtl := gomock.NewController(t)
						defer mockCtl.Finish()

						mockMounter := mocks.NewMockInterface(mockCtl)
						mounter := &k8sMount.SafeFormatAndMount{
							Interface: mockMounter,
							Exec:      k8sexec.New(),
						}
						mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, nil)
						mockMounter.EXPECT().Unmount(targetPath).Return(nil)
						p := &ProcessMount{
							SafeFormatAndMount: *mounter,
						}
						if err := p.JMount(&jfsConfig.JfsSetting{Source: ceSource, Storage: "ceph"}, volumeId, targetPath, "", options); err != nil {
							t.Errorf("JMount() error = %v", err)
						}
					})
					Convey("stat error", func() {
						patch1 := ApplyFunc(k8sMount.PathExists, func(path string) (bool, error) {
							return true, nil
						})
						defer patch1.Reset()
						patch2 := ApplyFunc(syscall.Environ, func() []string {
							return []string{}
						})
						defer patch2.Reset()
						cmd := exec.Command("mount")
						patch3 := ApplyMethod(reflect.TypeOf(cmd), "Run", func(_ *exec.Cmd) error {
							return nil
						})
						defer patch3.Reset()
						patch4 := ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
							return nil, errors.New("test")
						})
						defer patch4.Reset()

						mockCtl := gomock.NewController(t)
						defer mockCtl.Finish()

						mockMounter := mocks.NewMockInterface(mockCtl)
						mounter := &k8sMount.SafeFormatAndMount{
							Interface: mockMounter,
							Exec:      k8sexec.New(),
						}
						mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, nil)
						p := &ProcessMount{
							SafeFormatAndMount: *mounter,
						}
						if err := p.JMount(&jfsConfig.JfsSetting{Source: ceSource, Storage: "ceph"}, volumeId, targetPath, "", options); err == nil {
							t.Errorf("JMount() error = %v", err)
						}
					})
					Convey("finfo inode=2", func() {
						patch1 := ApplyFunc(k8sMount.PathExists, func(path string) (bool, error) {
							return true, nil
						})
						defer patch1.Reset()
						patch2 := ApplyFunc(syscall.Environ, func() []string {
							return []string{}
						})
						defer patch2.Reset()
						cmd := exec.Command("mount")
						patch3 := ApplyMethod(reflect.TypeOf(cmd), "Run", func(_ *exec.Cmd) error {
							return nil
						})
						defer patch3.Reset()
						patch4 := ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
							return mocks.FakeFileInfoIno2{}, nil
						})
						defer patch4.Reset()

						mockCtl := gomock.NewController(t)
						defer mockCtl.Finish()

						mockMounter := mocks.NewMockInterface(mockCtl)
						mounter := &k8sMount.SafeFormatAndMount{
							Interface: mockMounter,
							Exec:      k8sexec.New(),
						}
						mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, nil)
						p := &ProcessMount{
							SafeFormatAndMount: *mounter,
						}
						if err := p.JMount(&jfsConfig.JfsSetting{Source: ceSource, Storage: "ceph"}, volumeId, targetPath, "", options); err == nil {
							t.Errorf("JMount() error = %v", err)
						}
					})
					Convey("pathExist error", func() {
						patch1 := ApplyFunc(k8sMount.PathExists, func(path string) (bool, error) {
							return true, errors.New("test")
						})
						defer patch1.Reset()
						p := &ProcessMount{
							SafeFormatAndMount: k8sMount.SafeFormatAndMount{},
						}
						if err := p.JMount(&jfsConfig.JfsSetting{Source: ceSource}, volumeId, targetPath, "", nil); err == nil {
							t.Errorf("JMount() error = %v", err)
						}
					})
					Convey("MkdirAll error", func() {
						patch1 := ApplyFunc(k8sMount.PathExists, func(path string) (bool, error) {
							return false, nil
						})
						defer patch1.Reset()
						patch2 := ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
							return errors.New("test")
						})
						defer patch2.Reset()
						p := &ProcessMount{
							SafeFormatAndMount: k8sMount.SafeFormatAndMount{},
						}
						if err := p.JMount(&jfsConfig.JfsSetting{Source: ceSource}, volumeId, targetPath, "", nil); err == nil {
							t.Errorf("JMount() error = %v", err)
						}
					})
					Convey("IsLikelyNotMountPoint error", func() {
						patch1 := ApplyFunc(k8sMount.PathExists, func(path string) (bool, error) {
							return true, nil
						})
						defer patch1.Reset()
						mockCtl := gomock.NewController(t)
						defer mockCtl.Finish()

						mockMounter := mocks.NewMockInterface(mockCtl)
						mounter := &k8sMount.SafeFormatAndMount{
							Interface: mockMounter,
							Exec:      k8sexec.New(),
						}
						mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, errors.New("test"))
						p := &ProcessMount{
							SafeFormatAndMount: *mounter,
						}
						if err := p.JMount(&jfsConfig.JfsSetting{Source: ceSource}, volumeId, targetPath, "", nil); err == nil {
							t.Errorf("JMount() error = %v", err)
						}
					})
					Convey("Unmount error", func() {
						patch1 := ApplyFunc(k8sMount.PathExists, func(path string) (bool, error) {
							return true, nil
						})
						defer patch1.Reset()
						mockCtl := gomock.NewController(t)
						defer mockCtl.Finish()

						mockMounter := mocks.NewMockInterface(mockCtl)
						mounter := &k8sMount.SafeFormatAndMount{
							Interface: mockMounter,
							Exec:      k8sexec.New(),
						}
						mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, nil)
						mockMounter.EXPECT().Unmount(targetPath).Return(errors.New("test"))
						p := &ProcessMount{
							SafeFormatAndMount: *mounter,
						}
						if err := p.JMount(&jfsConfig.JfsSetting{Source: ceSource}, volumeId, targetPath, "", nil); err == nil {
							t.Errorf("JMount() error = %v", err)
						}
					})
				})
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}
