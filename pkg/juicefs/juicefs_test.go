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

package juicefs

import (
	"errors"
	. "github.com/agiledragon/gomonkey"
	"github.com/golang/mock/gomock"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver/mocks"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func Test_jfs_CreateVol(t *testing.T) {
	Convey("Test CreateVol", t, func() {
		Convey("test normal", func() {
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, nil
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
				return nil
			})
			defer patch2.Reset()
			patch3 := ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
				return mocks.FakeFileInfoIno1{}, nil
			})
			defer patch3.Reset()

			j := jfs{
				MountPath: "/mountPath",
			}
			got, err := j.CreateVol("", "subPath")
			So(err, ShouldBeNil)
			So(got, ShouldEqual, "/mountPath/subPath")
		})
		Convey("test exist err", func() {
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, errors.New("test")
			})
			defer patch1.Reset()

			j := jfs{
				MountPath: "/mountPath",
			}
			got, err := j.CreateVol("", "subPath")
			So(err, ShouldNotBeNil)
			So(got, ShouldEqual, "")
			srvErr, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from error: %v", srvErr)
			}
			if srvErr.Code() != codes.Internal {
				t.Fatalf("error status code is not invalid: %v", srvErr.Code())
			}
		})
		Convey("test mkdirAll err", func() {
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, nil
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
				return errors.New("test")
			})
			defer patch2.Reset()

			j := jfs{
				MountPath: "/mountPath",
			}
			got, err := j.CreateVol("", "subPath")
			So(err, ShouldNotBeNil)
			So(got, ShouldEqual, "")
			srvErr, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from error: %v", srvErr)
			}
			if srvErr.Code() != codes.Internal {
				t.Fatalf("error status code is not invalid: %v", srvErr.Code())
			}
		})
		Convey("test stat err", func() {
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, nil
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
				return nil
			})
			defer patch2.Reset()
			patch3 := ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
				return mocks.FakeFileInfoIno1{}, errors.New("test")
			})
			defer patch3.Reset()

			j := jfs{
				MountPath: "/mountPath",
			}
			got, err := j.CreateVol("", "subPath")
			So(err, ShouldNotBeNil)
			So(got, ShouldEqual, "")
			srvErr, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from error: %v", srvErr)
			}
			if srvErr.Code() != codes.Internal {
				t.Fatalf("error status code is not invalid: %v", srvErr.Code())
			}
		})
	})
}

func Test_jfs_DeleteVol(t *testing.T) {
	Convey("Test DeleteVol", t, func() {
		Convey("test normal", func() {
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, nil
			})
			defer patch1.Reset()
			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "RmrDir", func(_ *juicefs, directory string, isCeMount bool) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch2.Reset()

			j := jfs{
				MountPath: "/mountPath",
				Provider:  &juicefs{},
			}
			err := j.DeleteVol("", map[string]string{})
			So(err, ShouldBeNil)
		})
		Convey("exist error", func() {
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, errors.New("test")
			})
			defer patch1.Reset()
			j := jfs{
				MountPath: "/mountPath",
				Provider:  &juicefs{},
			}
			err := j.DeleteVol("", map[string]string{})
			So(err, ShouldNotBeNil)
		})
		Convey("not exist", func() {
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, nil
			})
			defer patch1.Reset()
			j := jfs{
				MountPath: "/mountPath",
				Provider:  &juicefs{},
			}
			err := j.DeleteVol("", map[string]string{})
			So(err, ShouldBeNil)
		})
		Convey("rmr error", func() {
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, nil
			})
			defer patch1.Reset()
			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "RmrDir", func(_ *juicefs, directory string, isCeMount bool) ([]byte, error) {
				return []byte(""), errors.New("test")
			})
			defer patch2.Reset()

			j := jfs{
				MountPath: "/mountPath",
				Provider:  &juicefs{},
			}
			err := j.DeleteVol("", map[string]string{})
			So(err, ShouldNotBeNil)
		})
	})
}

func Test_jfs_GetBasePath(t *testing.T) {
	type fields struct {
		MountPath string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test",
			fields: fields{
				MountPath: "/test",
			},
			want: "/test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &jfs{
				MountPath: tt.fields.MountPath,
			}
			if got := fs.GetBasePath(); got != tt.want {
				t.Errorf("GetBasePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewJfsProvider(t *testing.T) {
	Convey("Test NewJfsProvider", t, func() {
		Convey("normal", func() {
			patch1 := ApplyFunc(k8s.NewClient, func() (*k8s.K8sClient, error) {
				return nil, nil
			})
			defer patch1.Reset()

			_, err := NewJfsProvider(nil)
			So(err, ShouldBeNil)
		})
		Convey("err", func() {
			patch1 := ApplyFunc(k8s.NewClient, func() (*k8s.K8sClient, error) {
				return nil, errors.New("test")
			})
			defer patch1.Reset()

			_, err := NewJfsProvider(nil)
			So(err, ShouldNotBeNil)
		})
	})
}

func Test_juicefs_JfsMount(t *testing.T) {
	Convey("Test JfsMount", t, func() {
		Convey("ee normal", func() {
			volumeId := "test-volume-id"
			targetPath := "/target"
			secret := map[string]string{
				"name":  "test",
				"token": "123",
			}

			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {
				return
			})
			defer patch2.Reset()
			patch3 := ApplyMethod(reflect.TypeOf(jf), "AuthFs", func(_ *juicefs, secrets map[string]string, extraEnvs map[string]string) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch3.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, volumeID, target string, options []string, jfsSetting *config.JfsSetting) (string, error) {
				return "", nil
			})
			defer patch4.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          nil,
			}
			_, err := jfs.JfsMount(volumeId, targetPath, secret, map[string]string{}, []string{}, true)
			So(err, ShouldBeNil)
		})
		Convey("ce normal", func() {
			volumeId := "test-volume-id"
			targetPath := "/target"
			secret := map[string]string{
				"name":    "test",
				"metaurl": "127.0.0.1:6379/1",
				"bucket":  "123",
			}

			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {
				return
			})
			defer patch2.Reset()
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch3.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, volumeID, target string, options []string, jfsSetting *config.JfsSetting) (string, error) {
				return "", nil
			})
			defer patch4.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.JfsMount(volumeId, targetPath, secret, map[string]string{}, []string{}, true)
			So(err, ShouldBeNil)
		})
		Convey("parse err", func() {
			volumeId := "test-volume-id"
			targetPath := "/target"
			secret := map[string]string{
				"token": "123",
			}
			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          nil,
			}
			_, err := jfs.JfsMount(volumeId, targetPath, secret, map[string]string{}, []string{}, true)
			So(err, ShouldNotBeNil)
		})
		Convey("ee no token", func() {
			volumeId := "test-volume-id"
			targetPath := "/target"
			secret := map[string]string{
				"name": "test",
			}

			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {
				return
			})
			defer patch2.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, volumeID, target string, options []string, jfsSetting *config.JfsSetting) (string, error) {
				return "", nil
			})
			defer patch4.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          nil,
			}
			_, err := jfs.JfsMount(volumeId, targetPath, secret, map[string]string{}, []string{}, true)
			So(err, ShouldBeNil)
		})
		Convey("mountFs err", func() {
			volumeId := "test-volume-id"
			targetPath := "/target"
			secret := map[string]string{
				"name": "test",
			}

			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {
				return
			})
			defer patch2.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, volumeID, target string, options []string, jfsSetting *config.JfsSetting) (string, error) {
				return "", errors.New("test")
			})
			defer patch4.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          nil,
			}
			_, err := jfs.JfsMount(volumeId, targetPath, secret, map[string]string{}, []string{}, true)
			So(err, ShouldNotBeNil)
		})
		Convey("ce no bucket", func() {
			volumeId := "test-volume-id"
			targetPath := "/target"
			secret := map[string]string{
				"name":    "test",
				"metaurl": "redis://127.0.0.1:6379/1",
			}

			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {
				return
			})
			defer patch2.Reset()
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch3.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, volumeID, target string, options []string, jfsSetting *config.JfsSetting) (string, error) {
				return "", nil
			})
			defer patch4.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.JfsMount(volumeId, targetPath, secret, map[string]string{}, []string{}, true)
			So(err, ShouldBeNil)
		})
		Convey("ce format error", func() {
			volumeId := "test-volume-id"
			targetPath := "/target"
			secret := map[string]string{
				"name":    "test",
				"metaurl": "redis://127.0.0.1:6379/1",
			}

			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {
				return
			})
			defer patch2.Reset()
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), errors.New("test")
			})
			defer patch3.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, volumeID, target string, options []string, jfsSetting *config.JfsSetting) (string, error) {
				return "", nil
			})
			defer patch4.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.JfsMount(volumeId, targetPath, secret, map[string]string{}, []string{}, true)
			So(err, ShouldNotBeNil)
		})
	})
}

func Test_juicefs_JfsUnmount(t *testing.T) {
	Convey("Test JfsUnmount", t, func() {
		Convey("normal", func() {
			targetPath := "/target"

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMounter := mocks.NewMockInterface(mockCtl)
			mounter := &mount.SafeFormatAndMount{
				Interface: mockMounter,
				Exec:      k8sexec.New(),
			}
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, nil)

			jfs := juicefs{
				SafeFormatAndMount: *mounter,
				K8sClient:          nil,
			}
			err := jfs.JfsUnmount(targetPath)
			So(err, ShouldBeNil)
		})
		Convey("broken", func() {
			targetPath := "/target"

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMounter := mocks.NewMockInterface(mockCtl)
			mounter := &mount.SafeFormatAndMount{
				Interface: mockMounter,
				Exec:      k8sexec.New(),
			}
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(true, errors.New("test"))

			jfs := juicefs{
				SafeFormatAndMount: *mounter,
				K8sClient:          nil,
			}
			err := jfs.JfsUnmount(targetPath)
			So(err, ShouldNotBeNil)
		})
		Convey("unmount error", func() {
			targetPath := "/target"

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMounter := mocks.NewMockInterface(mockCtl)
			mounter := &mount.SafeFormatAndMount{
				Interface: mockMounter,
				Exec:      k8sexec.New(),
			}
			mockMounter.EXPECT().IsLikelyNotMountPoint(targetPath).Return(false, nil)

			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), errors.New("not mounted")
			})
			defer patch3.Reset()

			jfs := juicefs{
				SafeFormatAndMount: *mounter,
				K8sClient:          nil,
			}
			err := jfs.JfsUnmount(targetPath)
			So(err, ShouldNotBeNil)
		})
	})
}

func Test_juicefs_RmrDir(t *testing.T) {
	Convey("Test RmrDir", t, func() {
		Convey("ce normal", func() {
			targetPath := "/target"
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch3.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.RmrDir(targetPath, true)
			So(err, ShouldBeNil)
		})
		Convey("ee normal", func() {
			targetPath := "/target"
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch3.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.RmrDir(targetPath, false)
			So(err, ShouldBeNil)
		})
		Convey("error", func() {
			targetPath := "/target"
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), errors.New("test")
			})
			defer patch3.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.RmrDir(targetPath, true)
			So(err, ShouldNotBeNil)
		})
	})
}

func Test_juicefs_CleanupMountPoint(t *testing.T) {
	Convey("Test CleanupMountPoint", t, func() {
		Convey("normal", func() {
			targetPath := "/target"
			patch3 := ApplyFunc(mount.CleanupMountPoint, func(mountPath string, mounter mount.Interface, extensiveMountPointCheck bool) error {
				return nil
			})
			defer patch3.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			err := jfs.JfsCleanupMountPoint(targetPath)
			So(err, ShouldBeNil)
		})
	})
}

func Test_juicefs_AuthFs(t *testing.T) {
	Convey("Test AuthFs", t, func() {
		Convey("normal", func() {
			os.Setenv("JFS_NO_UPDATE_CONFIG", "enable")
			secrets := map[string]string{
				"name":       "test",
				"bucket":     "test",
				"initconfig": "",
			}
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch3.Reset()
			patch1 := ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
				return nil, nil
			})
			defer patch1.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.AuthFs(secrets, map[string]string{})
			So(err, ShouldBeNil)
		})
		Convey("secret nil", func() {
			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.AuthFs(nil, map[string]string{})
			So(err, ShouldNotBeNil)
		})
		Convey("secret no name", func() {
			secret := map[string]string{}
			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.AuthFs(secret, map[string]string{})
			So(err, ShouldNotBeNil)
		})
	})
}

func Test_juicefs_Upgrade(t *testing.T) {
	Convey("Test Upgrade", t, func() {
		Convey("normal", func() {
			os.Setenv("JFS_AUTO_UPGRADE", "enabled")
			os.Setenv("JFS_AUTO_UPGRADE_TIMEOUT", "10")

			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "Run", func(_ *exec.Cmd) error {
				return nil
			})
			defer patch3.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			jfs.Upgrade()
		})
	})
}
