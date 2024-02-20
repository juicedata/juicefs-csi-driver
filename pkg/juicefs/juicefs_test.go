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
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver/mocks"
	podmount "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount"
	mntmock "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/mocks"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
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
			got, err := j.CreateVol(context.TODO(), "", "subPath")
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
			got, err := j.CreateVol(context.TODO(), "", "subPath")
			So(err, ShouldNotBeNil)
			So(got, ShouldEqual, "")
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
			got, err := j.CreateVol(context.TODO(), "", "subPath")
			So(err, ShouldNotBeNil)
			So(got, ShouldEqual, "")
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
			got, err := j.CreateVol(context.TODO(), "", "subPath")
			So(err, ShouldNotBeNil)
			So(got, ShouldEqual, "")
		})
	})
}

func Test_jfs_BindTarget(t *testing.T) {
	Convey("Test BindTarget", t, func() {
		Convey("test normal", func() {
			mountPath := "/var/lib/juicefs/volume/ce-static-vsvhgz"
			target := "/var/lib/kubelet/pods/8687ae00-ce35-4715-a117-f2d21e24ae4f/volumes/kubernetes.io~csi/ce-static/mount"
			mockMits := []mount.MountInfo{{
				ID:         3280,
				ParentID:   31,
				Major:      0,
				Minor:      231,
				Root:       "/",
				Source:     "JuiceFS:minio",
				MountPoint: mountPath,
				FsType:     "fuse.juicefs",
			}}
			patch1 := ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
				return mockMits, nil
			})
			defer patch1.Reset()

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()
			mockMount := mocks.NewMockInterface(mockCtl)
			mockMount.EXPECT().Mount(mountPath, target, fsTypeNone, []string{"bind"}).Return(nil)

			j := jfs{
				MountPath: mountPath,
				Provider: &juicefs{
					SafeFormatAndMount: mount.SafeFormatAndMount{
						Interface: mockMount,
						Exec:      k8sexec.New(),
					},
				},
			}
			err := j.BindTarget(context.TODO(), mountPath, target)
			So(err, ShouldBeNil)
		})
		Convey("test already bind", func() {
			mountPath := "/var/lib/juicefs/volume/ce-static-vsvhgz"
			target := "/var/lib/kubelet/pods/8687ae00-ce35-4715-a117-f2d21e24ae4f/volumes/kubernetes.io~csi/ce-static/mount"
			mockMits := []mount.MountInfo{{
				ID:         3280,
				ParentID:   31,
				Major:      0,
				Minor:      231,
				Root:       "/",
				Source:     "JuiceFS:minio",
				MountPoint: mountPath,
				FsType:     "fuse.juicefs",
			}, {
				ID:         3299,
				ParentID:   497,
				Major:      0,
				Minor:      231,
				Root:       "/",
				Source:     "JuiceFS:minio",
				MountPoint: target,
				FsType:     "fuse.juicefs",
			}}
			patch1 := ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
				return mockMits, nil
			})
			defer patch1.Reset()

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()
			mockMount := mocks.NewMockInterface(mockCtl)

			j := jfs{
				MountPath: mountPath,
				Provider: &juicefs{
					SafeFormatAndMount: mount.SafeFormatAndMount{
						Interface: mockMount,
						Exec:      k8sexec.New(),
					},
				},
			}
			err := j.BindTarget(context.TODO(), mountPath, target)
			So(err, ShouldBeNil)
		})
		Convey("test bind other path", func() {
			mountPath := "/var/lib/juicefs/volume/ce-static-vsvhgz"
			target := "/var/lib/kubelet/pods/8687ae00-ce35-4715-a117-f2d21e24ae4f/volumes/kubernetes.io~csi/ce-static/mount"
			mockMits := []mount.MountInfo{{
				ID:         3280,
				ParentID:   31,
				Major:      0,
				Minor:      231,
				Root:       "/",
				Source:     "JuiceFS:minio",
				MountPoint: mountPath,
				FsType:     "fuse.juicefs",
			}, {
				ID:         3299,
				ParentID:   497,
				Major:      0,
				Minor:      232,
				Root:       "/",
				Source:     "JuiceFS:minio",
				MountPoint: target,
				FsType:     "fuse.juicefs",
			}}
			patch1 := ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
				return mockMits, nil
			})
			defer patch1.Reset()
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte{}, nil
			})
			defer patch3.Reset()
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()
			mockMount := mocks.NewMockInterface(mockCtl)
			mockMount.EXPECT().Mount(mountPath, target, fsTypeNone, []string{"bind"}).Return(nil)

			j := jfs{
				MountPath: mountPath,
				Provider: &juicefs{
					SafeFormatAndMount: mount.SafeFormatAndMount{
						Interface: mockMount,
						Exec:      k8sexec.New(),
					},
				},
			}
			err := j.BindTarget(context.TODO(), mountPath, target)
			So(err, ShouldBeNil)
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

func Test_juicefs_JfsMount(t *testing.T) {
	k8sClient := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}

	Convey("Test JfsMount", t, func() {
		Convey("ee normal", func() {
			volumeId := "test-volume-id"
			targetPath := "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
			secret := map[string]string{
				"name":  "test",
				"token": "123",
			}

			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {
			})
			defer patch2.Reset()
			patch3 := ApplyMethod(reflect.TypeOf(jf), "AuthFs", func(_ *juicefs, _ context.Context, secrets map[string]string, setting *config.JfsSetting) (string, error) {
				return "", nil
			})
			defer patch3.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, _ context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
				return "", nil
			})
			defer patch4.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          k8sClient,
			}
			_, err := jfs.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
			So(err, ShouldBeNil)
		})
		Convey("ce normal", func() {
			volumeId := "test-volume-id"
			targetPath := "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
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
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, _ context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
				return "", nil
			})
			defer patch4.Reset()
			patch5 := ApplyMethod(reflect.TypeOf(jf), "GetJfsVolUUID", func(_ *juicefs, _ context.Context, name string) (string, error) {
				return "test", nil
			})
			defer patch5.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: k8sClient,
			}
			_, err := jfs.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
			So(err, ShouldBeNil)
		})
		Convey("parse err", func() {
			volumeId := "test-volume-id"
			targetPath := "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
			secret := map[string]string{
				"token": "123",
			}
			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          k8sClient,
			}
			_, err := jfs.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
			So(err, ShouldNotBeNil)
		})
		Convey("ee no token", func() {
			volumeId := "test-volume-id"
			targetPath := "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
			secret := map[string]string{
				"name": "test",
			}

			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {})
			defer patch2.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, _ context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
				return "", nil
			})
			defer patch4.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          k8sClient,
			}
			_, err := jfs.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
			So(err, ShouldBeNil)
		})
		Convey("mountFs err", func() {
			volumeId := "test-volume-id"
			targetPath := "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
			secret := map[string]string{
				"name": "test",
			}

			jf := &juicefs{}
			patch2 := ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {})
			defer patch2.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, _ context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
				return "", errors.New("test")
			})
			defer patch4.Reset()
			patch5 := ApplyMethod(reflect.TypeOf(jf), "GetJfsVolUUID", func(_ *juicefs, _ context.Context, name string) (string, error) {
				return "test", nil
			})
			defer patch5.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          k8sClient,
			}
			_, err := jfs.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
			So(err, ShouldNotBeNil)
		})
		Convey("ce no bucket", func() {
			volumeId := "test-volume-id"
			targetPath := "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
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
			patch4 := ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, _ context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
				return "", nil
			})
			defer patch4.Reset()
			patch5 := ApplyMethod(reflect.TypeOf(jf), "GetJfsVolUUID", func(_ *juicefs, _ context.Context, name string) (string, error) {
				return "test", nil
			})
			defer patch5.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: k8sClient,
			}
			_, err := jfs.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
			So(err, ShouldBeNil)
		})
		Convey("ce format error", func() {
			volumeId := "test-volume-id"
			targetPath := "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
			secret := map[string]string{
				"metaurl": "redis://127.0.0.1:6379/1",
			}
			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: k8sClient,
			}
			_, err := jfs.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
			So(err, ShouldNotBeNil)
		})
	})
}

func Test_juicefs_JfsUnmount(t *testing.T) {
	Convey("Test JfsUnmount", t, func() {
		Convey("normal", func() {
			targetPath := "/target"

			mounter := &mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			}
			var tmpCmd = &exec.Cmd{}
			patch1 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte("not mounted"), errors.New("not mounted")
			})
			defer patch1.Reset()

			k8sClient := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
			jfs := juicefs{
				SafeFormatAndMount: *mounter,
				processMount:       podmount.NewProcessMount(*mounter),
				K8sClient:          k8sClient,
				podMount: podmount.NewPodMount(k8sClient, mount.SafeFormatAndMount{
					Interface: *mounter,
					Exec:      k8sexec.New(),
				}),
			}
			err := jfs.JfsUnmount(context.TODO(), "test", targetPath)
			So(err, ShouldBeNil)
		})
		Convey("unmount error", func() {
			targetPath := "/target"

			mounter := &mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			}
			var tmpCmd = &exec.Cmd{}
			patch1 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), errors.New("umount has some error")
			})
			defer patch1.Reset()

			k8sClient := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
			jfs := juicefs{
				Mutex:              sync.Mutex{},
				SafeFormatAndMount: *mounter,
				K8sClient:          k8sClient,
				processMount:       podmount.NewProcessMount(*mounter),
				podMount: podmount.NewPodMount(k8sClient, mount.SafeFormatAndMount{
					Interface: *mounter,
					Exec:      k8sexec.New(),
				}),
			}
			err := jfs.JfsUnmount(context.TODO(), "test", targetPath)
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
			err := jfs.JfsCleanupMountPoint(context.TODO(), targetPath)
			So(err, ShouldBeNil)
		})
	})
}

func Test_juicefs_AuthFs(t *testing.T) {
	Convey("Test AuthFs", t, func() {
		Convey("normal", func() {
			os.Setenv("JFS_NO_UPDATE_CONFIG", "enabled")
			secrets := map[string]string{
				"name":       "test",
				"bucket":     "test",
				"initconfig": "abc",
				"access-key": "abc",
				"secret-key": "abc",
			}
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch3.Reset()
			patch1 := ApplyFunc(os.WriteFile, func(filename string, data []byte, perm fs.FileMode) error {
				return nil
			})
			defer patch1.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			setting, err := config.ParseSetting(nil, map[string]string{}, []string{}, true)
			So(err, ShouldBeNil)
			_, err = jfs.AuthFs(context.TODO(), secrets, setting, false)
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
			_, err := jfs.AuthFs(context.TODO(), nil, nil, false)
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
			_, err := jfs.AuthFs(context.TODO(), secret, nil, false)
			So(err, ShouldNotBeNil)
		})
		Convey("secret no bucket", func() {
			os.Setenv("JFS_NO_UPDATE_CONFIG", "enabled")
			secrets := map[string]string{
				"name": "test",
			}
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
			setting, err := config.ParseSetting(nil, map[string]string{}, []string{}, true)
			So(err, ShouldBeNil)
			_, err = jfs.AuthFs(context.TODO(), secrets, setting, false)
			So(err, ShouldNotBeNil)
		})
	})
}

func Test_juicefs_MountFs(t *testing.T) {
	Convey("Test MountFs", t, func() {
		Convey("normal", func() {
			mountPath := "/var/lib/jfs/test-volume-id"
			volumeId := "test-volume-id"
			options := []string{}

			jfsSetting := &config.JfsSetting{
				Source:   mountPath,
				UsePod:   false,
				VolumeId: volumeId,
				UniqueId: volumeId,
				Options:  options,
			}
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, nil
			})
			defer patch1.Reset()

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMount := mocks.NewMockInterface(mockCtl)
			//mockMount.EXPECT().IsLikelyNotMountPoint(mountPath).Return(false, nil)
			mockMount.EXPECT().Mount(mountPath, mountPath, config.FsType, options).Return(nil)

			k8sClient := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mockMount,
					Exec:      k8sexec.New(),
				},
				K8sClient: k8sClient,
				processMount: podmount.NewProcessMount(mount.SafeFormatAndMount{
					Interface: mockMount,
					Exec:      k8sexec.New(),
				}),
			}
			_, e := jfs.MountFs(context.TODO(), nil, jfsSetting)
			So(e, ShouldBeNil)
		})
		Convey("not MountPoint err", func() {
			jfsSetting := &config.JfsSetting{}
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, errors.New("test")
			})
			defer patch1.Reset()

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMount := mocks.NewMockInterface(mockCtl)
			//mockMount.EXPECT().IsLikelyNotMountPoint(mountPath).Return(false, errors.New("test"))

			k8sClient := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mockMount,
					Exec:      k8sexec.New(),
				},
				K8sClient: k8sClient,
				processMount: podmount.NewPodMount(k8sClient, mount.SafeFormatAndMount{
					Interface: mockMount,
					Exec:      k8sexec.New(),
				}),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, e := jfs.MountFs(ctx, nil, jfsSetting)
			So(e, ShouldNotBeNil)
		})
		Convey("add ref err", func() {
			mountPath := "/jfs/test-volume-id"
			volumeId := "test-volume-id"
			target := "/test"
			options := []string{}

			jfsSetting := &config.JfsSetting{
				UsePod:     true,
				MountPath:  mountPath,
				VolumeId:   volumeId,
				TargetPath: target,
				Options:    options,
			}
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, nil
			})
			defer patch1.Reset()

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMount := mocks.NewMockInterface(mockCtl)
			//mockMount.EXPECT().IsLikelyNotMountPoint(mountPath).Return(false, nil)
			mockMnt := mntmock.NewMockMntInterface(mockCtl)
			mockMnt.EXPECT().JMount(context.TODO(), nil, jfsSetting).Return(errors.New("test"))

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mockMount,
					Exec:      k8sexec.New(),
				},
				K8sClient: &k8s.K8sClient{Interface: fake.NewSimpleClientset()},
				podMount:  mockMnt,
			}
			_, e := jfs.MountFs(context.TODO(), nil, jfsSetting)
			So(e, ShouldNotBeNil)
		})
		Convey("jmount err", func() {
			mountPath := "/var/lib/jfs/test-volume-id"
			volumeId := "test-volume-id"
			target := "/test"
			options := []string{}

			jfsSetting := &config.JfsSetting{
				MountPath:  mountPath,
				VolumeId:   volumeId,
				TargetPath: target,
				Options:    options,
			}
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, nil
			})
			defer patch1.Reset()

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMount := mocks.NewMockInterface(mockCtl)
			//mockMount.EXPECT().IsLikelyNotMountPoint(mountPath).Return(true, nil)
			mockMnt := mntmock.NewMockMntInterface(mockCtl)
			mockMnt.EXPECT().JMount(context.TODO(), nil, jfsSetting).Return(errors.New("test"))

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mockMount,
					Exec:      k8sexec.New(),
				},
				K8sClient:    &k8s.K8sClient{Interface: fake.NewSimpleClientset()},
				processMount: mockMnt,
			}
			_, e := jfs.MountFs(context.TODO(), nil, jfsSetting)
			So(e, ShouldNotBeNil)
		})
		Convey("jmount", func() {
			mountPath := "/var/lib/jfs/test-volume-id"
			volumeId := "test-volume-id"
			target := "/test"
			options := []string{}

			jfsSetting := &config.JfsSetting{
				MountPath:  mountPath,
				VolumeId:   volumeId,
				TargetPath: target,
				Options:    options,
			}
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, nil
			})
			defer patch1.Reset()

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMount := mocks.NewMockInterface(mockCtl)
			//mockMount.EXPECT().IsLikelyNotMountPoint(mountPath).Return(true, nil)
			mockMnt := mntmock.NewMockMntInterface(mockCtl)
			mockMnt.EXPECT().JMount(context.TODO(), nil, jfsSetting).Return(nil)

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mockMount,
					Exec:      k8sexec.New(),
				},
				K8sClient:    &k8s.K8sClient{Interface: fake.NewSimpleClientset()},
				processMount: mockMnt,
			}
			_, e := jfs.MountFs(context.TODO(), nil, jfsSetting)
			So(e, ShouldBeNil)
		})
		Convey("not exist jmount err", func() {
			mountPath := "/var/lib/jfs/test-volume-id"
			volumeId := "test-volume-id"
			target := "/test"
			options := []string{}

			jfsSetting := &config.JfsSetting{
				MountPath:  mountPath,
				VolumeId:   volumeId,
				TargetPath: target,
				Options:    options,
			}
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, nil
			})
			defer patch1.Reset()

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMount := mocks.NewMockInterface(mockCtl)
			mockMnt := mntmock.NewMockMntInterface(mockCtl)
			mockMnt.EXPECT().JMount(context.TODO(), nil, jfsSetting).Return(errors.New("test"))

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mockMount,
					Exec:      k8sexec.New(),
				},
				K8sClient:    &k8s.K8sClient{Interface: fake.NewSimpleClientset()},
				processMount: mockMnt,
			}
			_, e := jfs.MountFs(context.TODO(), nil, jfsSetting)
			So(e, ShouldNotBeNil)
		})
		Convey("not exist", func() {
			mountPath := "/var/lib/jfs/test-volume-id"
			volumeId := "test-volume-id"
			target := "/test"
			options := []string{}

			jfsSetting := &config.JfsSetting{
				MountPath:  mountPath,
				VolumeId:   volumeId,
				TargetPath: target,
				Options:    options,
			}
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, nil
			})
			defer patch1.Reset()

			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMount := mocks.NewMockInterface(mockCtl)
			mockMnt := mntmock.NewMockMntInterface(mockCtl)
			mockMnt.EXPECT().JMount(context.TODO(), nil, jfsSetting).Return(nil)

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mockMount,
					Exec:      k8sexec.New(),
				},
				K8sClient:    &k8s.K8sClient{Interface: fake.NewSimpleClientset()},
				processMount: mockMnt,
			}
			_, e := jfs.MountFs(context.TODO(), nil, jfsSetting)
			So(e, ShouldBeNil)
		})
	})
}

func Test_juicefs_Upgrade(t *testing.T) {
	Convey("Test Upgrade", t, func() {
		Convey("not upgrade", func() {
			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			jfs.Upgrade()
		})
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

func Test_juicefs_ceFormat(t *testing.T) {
	Convey("Test ceFormat", t, func() {
		Convey("normal", func() {
			secret := map[string]string{
				"name":    "test",
				"metaurl": "redis://127.0.0.1:6379/1",
				"storage": "ceph",
			}

			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch3.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			setting, err := config.ParseSetting(secret, map[string]string{}, []string{}, true)
			So(err, ShouldBeNil)
			_, err = jfs.ceFormat(context.TODO(), secret, true, setting)
			So(err, ShouldBeNil)
		})
		Convey("no name", func() {
			secret := map[string]string{
				"metaurl": "redis://127.0.0.1:6379/1",
			}

			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch3.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.ceFormat(context.TODO(), secret, true, nil)
			So(err, ShouldNotBeNil)
		})
		Convey("no metaurl", func() {
			secret := map[string]string{
				"name": "test",
			}

			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch3.Reset()

			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			setting, err := config.ParseSetting(secret, map[string]string{}, []string{}, true)
			So(err, ShouldBeNil)
			_, err = jfs.ceFormat(context.TODO(), secret, true, setting)
			So(err, ShouldNotBeNil)
		})
		Convey("nil secret", func() {
			jfs := juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
			_, err := jfs.ceFormat(context.TODO(), nil, true, nil)
			So(err, ShouldNotBeNil)
		})
	})
}

func Test_juicefs_getVolumeUUID(t *testing.T) {
	Convey("Test juicefs status", t, func() {
		Convey("normal", func() {
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(`
2022/05/05 07:16:30.498501 juicefs[284385] <INFO>: Meta address: redis://127.0.0.1/1
2022/05/05 07:16:30.500868 juicefs[284385] <WARNING>: AOF is not enabled, you may lose data if Redis is not shutdown properly.
2022/05/05 07:16:30.501443 juicefs[284385] <INFO>: Ping redis: 536.562µs
{
  "Setting": {
    "Name": "minio",
    "UUID": "e267db92-051d-4214-b1aa-e97bf61bff1a",
    "Storage": "minio",
    "Bucket": "http://10.98.166.242:9000/minio/test2",
    "AccessKey": "minioadmin",
    "SecretKey": "removed",
    "BlockSize": 4096,
    "Compression": "none",
    "Shards": 0,
    "Partitions": 0,
    "Capacity": 0,
    "Inodes": 0,
    "TrashDays": 2
  },
  "Sessions": []
}
`), nil
			})
			defer patch3.Reset()

			mounter := &mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			}
			jfs := juicefs{
				SafeFormatAndMount: *mounter,
				K8sClient:          nil,
				processMount:       podmount.NewProcessMount(*mounter),
			}
			id, err := jfs.GetJfsVolUUID(context.TODO(), "test")
			So(err, ShouldBeNil)
			So(id, ShouldEqual, "e267db92-051d-4214-b1aa-e97bf61bff1a")
		})
		Convey("status error", func() {
			var tmpCmd = &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), errors.New("test")
			})
			defer patch3.Reset()

			mounter := &mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			}
			jfs := juicefs{
				SafeFormatAndMount: *mounter,
				K8sClient:          nil,
				processMount:       podmount.NewProcessMount(*mounter),
			}
			_, err := jfs.GetJfsVolUUID(context.TODO(), "test")
			So(err, ShouldNotBeNil)
		})
	})
}

func Test_juicefs_ceFormat_format_in_pod(t *testing.T) {
	config.FormatInPod = true
	type args struct {
		secrets  map[string]string
		noUpdate bool
		setting  *config.JfsSetting
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "name-metaurl",
			args: args{
				secrets: map[string]string{
					"name":    "test",
					"metaurl": "redis://127.0.0.1:6379/0",
				},
				noUpdate: false,
				setting: &config.JfsSetting{
					FormatOptions: "",
				},
			},
			want:    "/usr/local/bin/juicefs format ${metaurl} test",
			wantErr: false,
		},
		{
			name: "all",
			args: args{
				secrets: map[string]string{
					"name":       "test",
					"metaurl":    "redis://127.0.0.1:6379/0",
					"access-key": "minioadmin",
					"secret-key": "minioadmin",
					"storage":    "s3",
					"bucket":     "http://test.127.0.0.1:9000",
				},
				noUpdate: false,
				setting: &config.JfsSetting{
					FormatOptions: "",
				},
			},
			want:    "/usr/local/bin/juicefs format --storage=s3 --bucket=http://test.127.0.0.1:9000 --access-key=minioadmin --secret-key=${secretkey} ${metaurl} test",
			wantErr: false,
		},
		{
			name: "option",
			args: args{
				secrets: map[string]string{
					"name":    "test",
					"metaurl": "redis://127.0.0.1:6379/0",
				},
				noUpdate: false,
				setting: &config.JfsSetting{
					FormatOptions: "block-size=100,trash-days=0,shards=0",
				},
			},
			want:    "/usr/local/bin/juicefs format ${metaurl} test --block-size=100 --trash-days=0 --shards=0",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &juicefs{}
			got, err := j.ceFormat(context.TODO(), tt.args.secrets, tt.args.noUpdate, tt.args.setting)
			if (err != nil) != tt.wantErr {
				t.Errorf("ceFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ceFormat() got = %v, \nwant %v", got, tt.want)
			}
		})
	}
}

func Test_juicefs_validTarget(t *testing.T) {
	config.ByProcess = false
	config.CSIPod = corev1.Pod{
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "kubelet-dir",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/lib/kubelet",
						},
					},
				},
			},
		},
	}
	type args struct {
		target string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test-right",
			args: args{
				target: "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount",
			},
			wantErr: false,
		},
		{
			name: "test-wrong",
			args: args{
				target: "/var/snap/microk8s/common/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount",
			},
			wantErr: true,
		},
		{
			name: "test-invalid1",
			args: args{
				target: "/.abc",
			},
			wantErr: true,
		},
		{
			name: "test-invalid2",
			args: args{
				target: "/.",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &juicefs{}
			if got := j.validTarget(tt.args.target); (got != nil) != tt.wantErr {
				t.Errorf("validTarget() = %v, wantErr %v", got, tt.wantErr)
			}
		})
	}
}

func Test_juicefs_validOptions(t *testing.T) {
	type args struct {
		volumeId string
		options  []string
		volCtx   map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "test-normal",
			args: args{
				volumeId: "test",
				options:  []string{"cache-dir=xxx"},
			},
			want:    []string{"cache-dir=xxx"},
			wantErr: false,
		},
		{
			name: "test-space1",
			args: args{
				volumeId: "test",
				options:  []string{" cache-dir=xxx "},
			},
			want:    []string{"cache-dir=xxx"},
			wantErr: false,
		},
		{
			name: "test-space2",
			args: args{
				volumeId: "test",
				options:  []string{" cache-dir = xxx "},
			},
			want:    []string{"cache-dir=xxx"},
			wantErr: false,
		},
		{
			name: "test-error",
			args: args{
				volumeId: "test",
				options:  []string{"cache-dir=xxx cache-size=1024"},
			},
			want:    []string{},
			wantErr: true,
		},
		{
			name: "test-buffersize",
			args: args{
				volumeId: "test",
				options:  []string{"buffer-size=1024"},
				volCtx: map[string]string{
					config.MountPodMemLimitKey: "1Mi",
				},
			},
			want:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &juicefs{}
			got, err := j.validOptions(tt.args.volumeId, tt.args.options, tt.args.volCtx)
			if (err != nil) != tt.wantErr {
				t.Errorf("validOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("validOptions() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseRawVersion(t *testing.T) {
	type parseRawVersionCase struct {
		name string
		raw  string
		want clientVersion
		fail bool
	}

	cases := []parseRawVersionCase{
		{
			name: "test-normal",
			raw:  "juicefs version 0.11.0",
			want: clientVersion{
				Major: 0,
				Minor: 11,
				Patch: 0,
			},
		},
		{
			name: "test-normal2",
			raw:  "juicefs version 0.11.0-rc1",
			want: clientVersion{
				Major: 0,
				Minor: 11,
				Patch: 0,
			},
		},
		{
			name: "test-normal3",
			raw:  "juicefs version 1.1.0-dev+2023-04-12.2488ce21",
			want: clientVersion{
				Major: 1,
				Minor: 1,
				Patch: 0,
			},
		},
		{
			name: "test-normal4",
			raw:  "JuiceFS version 4.10.0 (2023-04-10 9e92fe6f)",
			want: clientVersion{
				Major: 4,
				Minor: 10,
				Patch: 0,
			},
		},
		{
			name: "test-normal5",
			raw:  "juicefs version 1.0.4+2023-04-06.f1c475d9",
			want: clientVersion{
				Major: 1,
				Minor: 0,
				Patch: 4,
			},
		},
		{
			name: "test-error",
			raw:  "juicefs version 0.11",
			want: clientVersion{},
			fail: true,
		},
		{
			name: "test-error2",
			raw:  "juicefs version",
			want: clientVersion{},
			fail: true,
		},
		{
			name: "test-error3",
			raw:  "juicefs 0.11",
			want: clientVersion{},
			fail: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseRawVersion(c.raw)
			if err != nil && !c.fail {
				t.Errorf("parseRawVersion() error = %v, wantErr %v", err, c.fail)
				return
			}
			if err == nil && c.fail {
				t.Errorf("parseRawVersion(\"%s\") wantErr %v", c.raw, c.fail)
				return
			}
			if err != nil && c.fail {
				return
			}
			if !got.Approximate(&c.want) {
				t.Errorf("parseRawVersion() got = %v, want %v", got, c.want)
			}
		})
	}
}

func Test_clientVersion_LessThan(t *testing.T) {
	type fields struct {
		Major int
		Minor int
		Patch int
	}
	type args struct {
		other clientVersion
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "test-normal",
			fields: fields{
				Major: 0,
				Minor: 11,
				Patch: 0,
			},
			args: args{
				other: clientVersion{
					Major: 0,
					Minor: 11,
					Patch: 1,
				},
			},
			want: true,
		},
		{
			name: "test-normal2",
			fields: fields{
				Major: 0,
				Minor: 11,
				Patch: 0,
			},
			args: args{
				other: clientVersion{
					Major: 0,
					Minor: 10,
					Patch: 1,
				},
			},
			want: false,
		},
		{
			name: "test-normal3",
			fields: fields{
				Major: 0,
				Minor: 11,
				Patch: 0,
			},
			args: args{
				other: clientVersion{
					Major: 0,
					Minor: 11,
					Patch: 0,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &clientVersion{
				Major: tt.fields.Major,
				Minor: tt.fields.Minor,
				Patch: tt.fields.Patch,
			}
			if got := c.LessThan(&tt.args.other); got != tt.want {
				t.Errorf("clientVersion.LessThan() = %v, want %v", got, tt.want)
			}
		})
	}
}
