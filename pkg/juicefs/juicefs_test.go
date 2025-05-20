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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

var _ = Describe("jfs", func() {
	var (
		j jfs
	)
	Describe("test create volume", func() {
		BeforeEach(func() {
			j = jfs{
				MountPath: "/mountPath",
			}
		})
		Context("test normal", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return false, nil
					}),
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return nil
					}),
					ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
						return mocks.FakeFileInfoIno1{}, nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				got, err := j.CreateVol(context.TODO(), "", "subPath")
				Expect(err).Should(BeNil())
				Expect(got).Should(Equal("/mountPath"))
			})
		})
		Context("test exist err", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return false, errors.New("test")
					}),
					ApplyGlobalVar(&config.StorageClassShareMount, true),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				got, err := j.CreateVol(context.TODO(), "", "subPath")
				Expect(err).ShouldNot(BeNil())
				Expect(got).Should(Equal(""))
			})
		})
		Context("test mkdirAll err", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return false, nil
					}),
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return errors.New("test")
					}),
					ApplyGlobalVar(&config.StorageClassShareMount, true),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				got, err := j.CreateVol(context.TODO(), "", "subPath")
				Expect(err).ShouldNot(BeNil())
				Expect(got).Should(Equal(""))
			})
		})
		Context("test stat err", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return false, nil
					}),
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return nil
					}),
					ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
						return mocks.FakeFileInfoIno1{}, errors.New("test")
					}),
					ApplyGlobalVar(&config.StorageClassShareMount, true),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				got, err := j.CreateVol(context.TODO(), "", "subPath")
				Expect(err).ShouldNot(BeNil())
				Expect(got).Should(Equal(""))
			})
		})
	})

	Describe("test bind target", func() {
		BeforeEach(func() {
			j = jfs{
				Provider: &juicefs{
					SafeFormatAndMount: mount.SafeFormatAndMount{
						Exec: k8sexec.New(),
					},
				},
			}
		})

		Context("test normal", func() {
			var (
				patches   []*Patches
				mountPath = "/var/lib/juicefs/volume/ce-static-vsvhgz"
				target    = "/var/lib/kubelet/pods/8687ae00-ce35-4715-a117-f2d21e24ae4f/volumes/kubernetes.io~csi/ce-static/mount"
				mockMits  = []mount.MountInfo{{
					ID:         3280,
					ParentID:   31,
					Major:      0,
					Minor:      231,
					Root:       "/",
					Source:     "JuiceFS:minio",
					MountPoint: mountPath,
					FsType:     "fuse.juicefs",
				}}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
						return mockMits, nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockMount := mocks.NewMockInterface(mockCtl)
				mockMount.EXPECT().Mount(mountPath, target, fsTypeNone, []string{"bind"}).Return(nil)
				j.MountPath = mountPath
				j.Provider.SafeFormatAndMount.Interface = mockMount
				err := j.BindTarget(context.TODO(), mountPath, target)
				Expect(err).Should(BeNil())
			})
		})

		Context("test already bind", func() {
			var (
				patches   []*Patches
				mountPath = "/var/lib/juicefs/volume/ce-static-vsvhgz"
				target    = "/var/lib/kubelet/pods/8687ae00-ce35-4715-a117-f2d21e24ae4f/volumes/kubernetes.io~csi/ce-static/mount"
				mockMits  = []mount.MountInfo{{
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
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
						return mockMits, nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				mockMount := mocks.NewMockInterface(mockCtl)
				defer mockCtl.Finish()
				j.MountPath = mountPath
				j.Provider.SafeFormatAndMount.Interface = mockMount
				err := j.BindTarget(context.TODO(), mountPath, target)
				Expect(err).Should(BeNil())
			})
		})

		Context("test bind other path", func() {
			var (
				patches   []*Patches
				mountPath = "/var/lib/juicefs/volume/ce-static-vsvhgz"
				target    = "/var/lib/kubelet/pods/8687ae00-ce35-4715-a117-f2d21e24ae4f/volumes/kubernetes.io~csi/ce-static/mount"
				mockMits  = []mount.MountInfo{{
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
				tmpCmd = &exec.Cmd{}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
						return mockMits, nil
					}),
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte{}, nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockMount := mocks.NewMockInterface(mockCtl)
				mockMount.EXPECT().Mount(mountPath, target, fsTypeNone, []string{"bind"}).Return(nil)
				j.MountPath = mountPath
				j.Provider.SafeFormatAndMount.Interface = mockMount
				err := j.BindTarget(context.TODO(), mountPath, target)
				Expect(err).Should(BeNil())
			})
		})
	})
})

var _ = Describe("juicefs", func() {
	var (
		j         juicefs
		k8sClient *k8s.K8sClient
	)
	Describe("jfsMount", func() {
		BeforeEach(func() {
			k8sClient = &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
			config.AccessToKubelet = true
			j = juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          k8sClient,
			}
		})
		Context("ee normal", func() {
			var (
				patches    []*Patches
				volumeId   = "test-volume-id"
				targetPath = "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
				secret     = map[string]string{
					"name":  "test",
					"token": "123",
				}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(&j), "Upgrade", func(_ *juicefs) {
					}),
					ApplyMethod(reflect.TypeOf(&j), "AuthFs", func(_ *juicefs, _ context.Context, secrets map[string]string, setting *config.JfsSetting) (string, error) {
						return "", nil
					}),
					ApplyMethod(reflect.TypeOf(&j), "MountFs", func(_ *juicefs, _ context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
						return "", nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := j.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
				Expect(err).Should(BeNil())
			})
		})
		Context("ce normal", func() {
			var (
				patches    []*Patches
				volumeId   = "test-volume-id"
				targetPath = "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
				secret     = map[string]string{
					"name":    "test",
					"metaurl": "127.0.0.1:6379/1",
					"bucket":  "123",
				}
				tmpCmd = &exec.Cmd{}
			)
			BeforeEach(func() {
				jf := &juicefs{}
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {}),
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte(""), nil
					}),
					ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, _ context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
						return "", nil
					}),
					ApplyFunc(config.GetJfsVolUUID, func(_ context.Context, jfsSetting *config.JfsSetting) (string, error) {
						return "test", nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := j.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
				Expect(err).Should(BeNil())
			})
		})
		Context("parse err", func() {
			var (
				volumeId   = "test-volume-id"
				targetPath = "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
				secret     = map[string]string{
					"token": "123",
				}
			)
			It("should succeed", func() {
				_, err := j.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("ee no token", func() {
			var (
				patches    []*Patches
				volumeId   = "test-volume-id"
				targetPath = "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
				secret     = map[string]string{
					"name": "test",
				}
			)
			BeforeEach(func() {
				jf := &juicefs{}
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {}),
					ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, _ context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
						return "", nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := j.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
				Expect(err).Should(BeNil())
			})
		})
		Context("mountFs err", func() {
			var (
				patches    []*Patches
				volumeId   = "test-volume-id"
				targetPath = "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
				secret     = map[string]string{
					"name": "test",
				}
			)
			BeforeEach(func() {
				jf := &juicefs{}
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {}),
					ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, _ context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
						return "", errors.New("test")
					}),
					ApplyFunc(config.GetJfsVolUUID, func(_ context.Context, jfsSetting *config.JfsSetting) (string, error) {
						return "test", nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := j.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("ce no bucket", func() {
			var (
				patches    []*Patches
				volumeId   = "test-volume-id"
				targetPath = "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
				secret     = map[string]string{
					"name":    "test",
					"metaurl": "redis://127.0.0.1:6379/1",
				}
			)
			BeforeEach(func() {
				jf := &juicefs{}
				var tmpCmd = &exec.Cmd{}
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(jf), "Upgrade", func(_ *juicefs) {}),
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte(""), nil
					}),
					ApplyMethod(reflect.TypeOf(jf), "MountFs", func(_ *juicefs, _ context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
						return "", nil
					}),
					ApplyFunc(config.GetJfsVolUUID, func(_ context.Context, jfsSetting *config.JfsSetting) (string, error) {
						return "test", nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := j.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
				Expect(err).Should(BeNil())
			})
		})
		Context("ce format error", func() {
			volumeId := "test-volume-id"
			targetPath := "/var/lib/kubelet/pods/a019aa39-cfa9-42fd-9b26-1a4fd796212d/volumes/kubernetes.io~csi/pvc-090cf941-0dcd-4ddc-8099-b86dd6caa5eb/mount"
			secret := map[string]string{
				"metaurl": "redis://127.0.0.1:6379/1",
			}
			It("should succeed", func() {
				_, err := j.JfsMount(context.TODO(), volumeId, targetPath, secret, map[string]string{}, []string{})
				Expect(err).ShouldNot(BeNil())
			})
		})
	})

	Describe("jfsUnmount", func() {
		BeforeEach(func() {
			mounter := &mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			}
			j = juicefs{
				Mutex:              sync.Mutex{},
				SafeFormatAndMount: *mounter,
				K8sClient:          k8sClient,
				mnt: podmount.NewPodMount(k8sClient, mount.SafeFormatAndMount{
					Interface: *mounter,
					Exec:      k8sexec.New(),
				}),
			}
		})
		Context("normal", func() {
			var patch *Patches
			BeforeEach(func() {
				var tmpCmd = &exec.Cmd{}
				patch = ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
					return []byte("not mounted"), errors.New("not mounted")
				})
			})
			AfterEach(func() {
				patch.Reset()
			})
			It("should succeed", func() {
				targetPath := "/target"
				err := j.JfsUnmount(context.TODO(), "test", targetPath)
				Expect(err).Should(BeNil())
			})
		})
		Context("unmount error", func() {
			var patch *Patches
			targetPath := "/target"
			BeforeEach(func() {
				var tmpCmd = &exec.Cmd{}
				patch = ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
					return []byte(""), errors.New("umount has some error")
				})
			})
			AfterEach(func() {
				patch.Reset()
			})
			It("should succeed", func() {
				err := j.JfsUnmount(context.TODO(), "test", targetPath)
				Expect(err).ShouldNot(BeNil())
			})
		})
	})

	Describe("cleanupMountPoint", func() {
		BeforeEach(func() {
			j = juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
			}
		})
		Context("normal", func() {
			var patch *Patches
			BeforeEach(func() {
				patch = ApplyFunc(mount.CleanupMountPoint, func(mountPath string, mounter mount.Interface, extensiveMountPointCheck bool) error {
					return nil
				})
			})
			AfterEach(func() {
				patch.Reset()
			})
			It("should succeed", func() {
				targetPath := "/target"
				err := j.JfsCleanupMountPoint(context.TODO(), targetPath)
				Expect(err).Should(BeNil())
			})
		})
	})

	Describe("authFs", func() {
		BeforeEach(func() {
			j = juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
			}
		})
		Context("normal", func() {
			var (
				secrets = map[string]string{
					"name":       "test",
					"bucket":     "test",
					"initconfig": "abc",
					"access-key": "abc",
					"secret-key": "abc",
				}
				patches []*Patches
			)
			BeforeEach(func() {
				var tmpCmd = &exec.Cmd{}
				os.Setenv("JFS_NO_UPDATE_CONFIG", "enabled")
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte(""), nil
					}),
					ApplyFunc(os.WriteFile, func(filename string, data []byte, perm fs.FileMode) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				setting, err := config.ParseSetting(context.TODO(), nil, map[string]string{}, []string{}, "", "", secrets["name"], nil, nil)
				Expect(err).Should(BeNil())
				_, err = j.AuthFs(context.TODO(), secrets, setting, false)
				Expect(err).Should(BeNil())
			})
		})
		Context("secret nil", func() {
			It("should succeed", func() {
				_, err := j.AuthFs(context.TODO(), nil, nil, false)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("secret no name", func() {
			It("should succeed", func() {
				secret := map[string]string{}
				_, err := j.AuthFs(context.TODO(), secret, nil, false)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("secret no bucket", func() {
			var (
				patches []*Patches
				secrets = map[string]string{
					"name": "test",
				}
			)
			BeforeEach(func() {
				var tmpCmd = &exec.Cmd{}
				os.Setenv("JFS_NO_UPDATE_CONFIG", "enabled")
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte(""), nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				setting, err := config.ParseSetting(context.TODO(), nil, map[string]string{}, []string{}, "", "", secrets["name"], nil, nil)
				Expect(err).Should(BeNil())
				_, err = j.AuthFs(context.TODO(), secrets, setting, false)
				Expect(err).ShouldNot(BeNil())
			})
		})
	})

	Describe("MountFs", func() {
		var (
			processMnt *podmount.ProcessMount
		)
		BeforeEach(func() {
			processMnt = &podmount.ProcessMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Exec: k8sexec.New(),
				},
			}
			j = juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Exec: k8sexec.New(),
				},
				K8sClient: k8sClient,
				mnt:       processMnt,
			}
		})
		Context("normal", func() {
			var (
				patches    []*Patches
				mountPath  = "/var/lib/jfs/test-volume-id"
				volumeId   = "test-volume-id"
				options    = []string{}
				jfsSetting = &config.JfsSetting{
					Source:   mountPath,
					UsePod:   false,
					VolumeId: volumeId,
					UniqueId: volumeId,
					Options:  options,
				}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return true, nil
					}),
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockMount := mocks.NewMockInterface(mockCtl)
				//mockMount.EXPECT().IsLikelyNotMountPoint(mountPath).Return(false, nil)
				mockMount.EXPECT().Mount(mountPath, mountPath, config.FsType, options).Return(nil)
				processMnt.SafeFormatAndMount.Interface = mockMount
				j.mnt = processMnt
				j.SafeFormatAndMount.Interface = mockMount
				_, e := j.MountFs(context.TODO(), nil, jfsSetting)
				Expect(e).Should(BeNil())
			})
		})
		Context("not MountPoint err", func() {
			var (
				patches []*Patches
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return true, errors.New("test")
					}),
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockMount := mocks.NewMockInterface(mockCtl)
				//mockMount.EXPECT().IsLikelyNotMountPoint(mountPath).Return(false, errors.New("test"))
				j.SafeFormatAndMount.Interface = mockMount
				j.mnt = &podmount.PodMount{
					SafeFormatAndMount: mount.SafeFormatAndMount{
						Interface: mockMount,
						Exec:      k8sexec.New(),
					},
					K8sClient: k8sClient,
				}
				jfsSetting := &config.JfsSetting{
					Attr: &config.PodAttr{},
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, e := j.MountFs(ctx, nil, jfsSetting)
				Expect(e).ShouldNot(BeNil())
			})
		})
		Context("add ref err", func() {
			var (
				patches    []*Patches
				mountPath  = "/jfs/test-volume-id"
				volumeId   = "test-volume-id"
				target     = "/test"
				options    = []string{}
				jfsSetting = &config.JfsSetting{
					UsePod:     true,
					MountPath:  mountPath,
					VolumeId:   volumeId,
					TargetPath: target,
					Options:    options,
				}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return true, nil
					}),
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockMount := mocks.NewMockInterface(mockCtl)
				//mockMount.EXPECT().IsLikelyNotMountPoint(mountPath).Return(false, nil)
				mockMnt := mntmock.NewMockMntInterface(mockCtl)
				mockMnt.EXPECT().JMount(context.TODO(), nil, jfsSetting).Return(errors.New("test"))
				j.SafeFormatAndMount.Interface = mockMount
				j.mnt = mockMnt
				_, e := j.MountFs(context.TODO(), nil, jfsSetting)
				Expect(e).ShouldNot(BeNil())
			})
		})
		Context("jmount err", func() {
			var (
				patches    []*Patches
				mountPath  = "/var/lib/jfs/test-volume-id"
				volumeId   = "test-volume-id"
				target     = "/test"
				options    = []string{}
				jfsSetting = &config.JfsSetting{
					MountPath:  mountPath,
					VolumeId:   volumeId,
					TargetPath: target,
					Options:    options,
				}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return true, nil
					}),
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockMount := mocks.NewMockInterface(mockCtl)
				//mockMount.EXPECT().IsLikelyNotMountPoint(mountPath).Return(true, nil)
				mockMnt := mntmock.NewMockMntInterface(mockCtl)
				mockMnt.EXPECT().JMount(context.TODO(), nil, jfsSetting).Return(errors.New("test"))
				j.SafeFormatAndMount.Interface = mockMount
				j.mnt = mockMnt
				_, e := j.MountFs(context.TODO(), nil, jfsSetting)
				Expect(e).ShouldNot(BeNil())
			})
		})
		Context("jmount", func() {
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
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return true, nil
					}),
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockMount := mocks.NewMockInterface(mockCtl)
				//mockMount.EXPECT().IsLikelyNotMountPoint(mountPath).Return(true, nil)
				mockMnt := mntmock.NewMockMntInterface(mockCtl)
				mockMnt.EXPECT().JMount(context.TODO(), nil, jfsSetting).Return(nil)
				j.SafeFormatAndMount.Interface = mockMount
				j.mnt = mockMnt
				_, e := j.MountFs(context.TODO(), nil, jfsSetting)
				Expect(e).Should(BeNil())
			})
		})
		Context("not exist jmount err", func() {
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
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return false, nil
					}),
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockMount := mocks.NewMockInterface(mockCtl)
				mockMnt := mntmock.NewMockMntInterface(mockCtl)
				mockMnt.EXPECT().JMount(context.TODO(), nil, jfsSetting).Return(errors.New("test"))
				j.SafeFormatAndMount.Interface = mockMount
				j.mnt = mockMnt
				_, e := j.MountFs(context.TODO(), nil, jfsSetting)
				Expect(e).ShouldNot(BeNil())
			})
		})
		Context("not exist", func() {
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
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return false, nil
					}),
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockMount := mocks.NewMockInterface(mockCtl)
				mockMnt := mntmock.NewMockMntInterface(mockCtl)
				mockMnt.EXPECT().JMount(context.TODO(), nil, jfsSetting).Return(nil)
				j.SafeFormatAndMount.Interface = mockMount
				j.mnt = mockMnt
				_, e := j.MountFs(context.TODO(), nil, jfsSetting)
				Expect(e).Should(BeNil())
			})
		})
	})

	Describe("ceFormat", func() {
		BeforeEach(func() {
			j = juicefs{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: nil,
					Exec:      k8sexec.New(),
				},
				K8sClient: nil,
			}
		})
		Context("normal", func() {
			secret := map[string]string{
				"name":    "test",
				"metaurl": "redis://127.0.0.1:6379/1",
				"storage": "ceph",
			}

			var (
				patches []*Patches
				tmpCmd  = &exec.Cmd{}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte(""), nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				setting, err := config.ParseSetting(context.TODO(), secret, map[string]string{}, []string{}, "", "", secret["name"], nil, nil)
				Expect(err).To(BeNil())
				_, err = j.ceFormat(context.TODO(), secret, true, setting)
				Expect(err).To(BeNil())
			})
		})
		Context("no name", func() {
			secret := map[string]string{
				"metaurl": "redis://127.0.0.1:6379/1",
			}

			var (
				tmpCmd  = &exec.Cmd{}
				patches []*Patches
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte(""), nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := j.ceFormat(context.TODO(), secret, true, nil)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("no metaurl", func() {
			secret := map[string]string{
				"name": "test",
			}

			var (
				patches []*Patches
				tmpCmd  = &exec.Cmd{}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte(""), nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				setting, err := config.ParseSetting(context.TODO(), secret, map[string]string{}, []string{}, "", "", secret["name"], nil, nil)
				Expect(err).To(BeNil())
				_, err = j.ceFormat(context.TODO(), secret, true, setting)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("nil secret", func() {
			It("should succeed", func() {
				_, err := j.ceFormat(context.TODO(), nil, true, nil)
				Expect(err).ShouldNot(BeNil())
			})
		})
	})
})

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

func Test_juicefs_ceFormat_format_in_pod(t *testing.T) {
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
