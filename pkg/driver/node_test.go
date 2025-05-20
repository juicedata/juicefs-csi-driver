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

package driver

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mocks"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var _ = Describe("nodeService", func() {
	stdVolCap := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	registerer, _ := util.NewPrometheus(config.NodeName)
	metrics := newNodeMetrics(registerer)
	var juicefsDriver *nodeService
	BeforeEach(func() {
		juicefsDriver = &nodeService{
			nodeID:    "fake_node_id",
			k8sClient: &k8s.K8sClient{Interface: fake.NewSimpleClientset()},
			metrics:   metrics,
		}
	})

	Describe("Publish", func() {
		Context("test normal", func() {
			volumeId := "vol-test"
			subPath := "/subPath"
			targetPath := "/test/path"
			bindSource := "/test/path"
			volumeCtx := map[string]string{"subPath": subPath}
			secret := map[string]string{"a": "b"}

			var patch *Patches
			BeforeEach(func() {
				patch = ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
					return nil
				})
			})
			AfterEach(func() {
				patch.Reset()
			})
			It("should succeed", func() {
				ctx := util.WithLog(context.TODO(), klog.NewKlogr().WithName("NodePublishVolume").WithValues("volumeId", volumeId))
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().CreateVol(ctx, volumeId, subPath).Return(bindSource, nil)
				mockJfs.EXPECT().BindTarget(ctx, bindSource, targetPath).Return(nil)
				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(ctx, volumeId, targetPath, secret, volumeCtx, []string{"ro"}).Return(mockJfs, nil)
				mockJuicefs.EXPECT().CreateTarget(ctx, targetPath).Return(nil)
				juicefsDriver.juicefs = mockJuicefs
				req := &csi.NodePublishVolumeRequest{
					VolumeId:         volumeId,
					TargetPath:       targetPath,
					VolumeCapability: stdVolCap,
					Readonly:         true,
					Secrets:          secret,
					VolumeContext:    volumeCtx,
				}

				_, err := juicefsDriver.NodePublishVolume(context.TODO(), req)
				Expect(err).Should(BeNil())
			})
		})
		Context("test mountOptions in volumeAttributes", func() {
			volumeId := "vol-test"
			subPath := "/subPath"
			targetPath := "/test/path"
			bindSource := "/test/path"
			mountOptions := []string{"cache-dir=/cache"}
			volumeCtx := map[string]string{
				"subPath":      subPath,
				"mountOptions": "cache-dir=/cache",
			}
			secret := map[string]string{"a": "b"}
			ctx := util.WithLog(context.TODO(), klog.NewKlogr().WithName("NodePublishVolume").WithValues("volumeId", volumeId))
			var patch *Patches
			BeforeEach(func() {
				patch = ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
					return nil
				})
			})
			AfterEach(func() {
				patch.Reset()
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().CreateVol(ctx, volumeId, subPath).Return(bindSource, nil)
				mockJfs.EXPECT().BindTarget(ctx, bindSource, targetPath).Return(nil)
				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(ctx, volumeId, targetPath, secret, volumeCtx, mountOptions).Return(mockJfs, nil)
				mockJuicefs.EXPECT().CreateTarget(ctx, targetPath).Return(nil)
				juicefsDriver.juicefs = mockJuicefs
				req := &csi.NodePublishVolumeRequest{
					VolumeId:         volumeId,
					TargetPath:       targetPath,
					VolumeCapability: stdVolCap,
					Readonly:         false,
					Secrets:          secret,
					VolumeContext:    volumeCtx,
				}

				_, err := juicefsDriver.NodePublishVolume(context.TODO(), req)
				Expect(err).Should(BeNil())
			})
		})
		Context("test mountOptions in spec", func() {
			volumeId := "vol-test"
			subPath := "/subPath"
			targetPath := "/test/path"
			bindSource := "/test/path"
			mountOptions := []string{"cache-dir=/cache"}
			volumeCtx := map[string]string{
				"subPath": subPath,
			}
			secret := map[string]string{"a": "b"}

			var patch *Patches
			ctx := util.WithLog(context.TODO(), klog.NewKlogr().WithName("NodePublishVolume").WithValues("volumeId", volumeId))
			BeforeEach(func() {
				patch = ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
					return nil
				})
			})
			AfterEach(func() {
				patch.Reset()
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().CreateVol(ctx, volumeId, subPath).Return(bindSource, nil)
				mockJfs.EXPECT().BindTarget(ctx, bindSource, targetPath).Return(nil)
				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(ctx, volumeId, targetPath, secret, volumeCtx, mountOptions).Return(mockJfs, nil)
				mockJuicefs.EXPECT().CreateTarget(ctx, targetPath).Return(nil)
				juicefsDriver.juicefs = mockJuicefs
				stdVolCapWithMount := &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							MountFlags: mountOptions,
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				}
				req := &csi.NodePublishVolumeRequest{
					VolumeId:         volumeId,
					TargetPath:       targetPath,
					VolumeCapability: stdVolCapWithMount,
					Readonly:         false,
					Secrets:          secret,
					VolumeContext:    volumeCtx,
				}

				_, err := juicefsDriver.NodePublishVolume(context.TODO(), req)
				Expect(err).Should(BeNil())
			})
		})
		Context("test JfsMount err", func() {
			volumeId := "vol-test"
			subPath := "/subPath"
			targetPath := "/test/path"
			volumeCtx := map[string]string{"subPath": subPath}
			secret := map[string]string{"a": "b"}

			var patch *Patches
			ctx := util.WithLog(context.TODO(), klog.NewKlogr().WithName("NodePublishVolume").WithValues("volumeId", volumeId))
			BeforeEach(func() {
				patch = ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
					return nil
				})
			})
			AfterEach(func() {
				patch.Reset()
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(ctx, volumeId, targetPath, secret, volumeCtx, []string{"ro"}).Return(mockJfs, errors.New("test"))
				mockJuicefs.EXPECT().CreateTarget(ctx, targetPath).Return(nil)
				juicefsDriver.juicefs = mockJuicefs
				req := &csi.NodePublishVolumeRequest{
					VolumeId:         volumeId,
					TargetPath:       targetPath,
					VolumeCapability: stdVolCap,
					Readonly:         true,
					Secrets:          secret,
					VolumeContext:    volumeCtx,
				}

				_, err := juicefsDriver.NodePublishVolume(context.TODO(), req)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("test CreateVol err", func() {
			volumeId := "vol-test"
			subPath := "/subPath"
			targetPath := "/test/path"
			bindSource := "/test/path"
			volumeCtx := map[string]string{"subPath": subPath}
			secret := map[string]string{"a": "b"}

			var patch *Patches
			ctx := util.WithLog(context.TODO(), klog.NewKlogr().WithName("NodePublishVolume").WithValues("volumeId", volumeId))
			BeforeEach(func() {
				patch = ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
					return nil
				})
			})
			AfterEach(func() {
				patch.Reset()
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().CreateVol(ctx, volumeId, subPath).Return(bindSource, errors.New("test"))
				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(ctx, volumeId, targetPath, secret, volumeCtx, []string{"ro"}).Return(mockJfs, nil)
				mockJuicefs.EXPECT().CreateTarget(ctx, targetPath).Return(nil)
				juicefsDriver.juicefs = mockJuicefs
				req := &csi.NodePublishVolumeRequest{
					VolumeId:         volumeId,
					TargetPath:       targetPath,
					VolumeCapability: stdVolCap,
					Readonly:         true,
					Secrets:          secret,
					VolumeContext:    volumeCtx,
				}

				_, err := juicefsDriver.NodePublishVolume(context.TODO(), req)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("test Mount err", func() {
			volumeId := "vol-test"
			subPath := "/subPath"
			targetPath := "/test/path"
			bindSource := "/test/path"
			volumeCtx := map[string]string{"subPath": subPath}
			secret := map[string]string{"a": "b"}

			var patch *Patches
			ctx := util.WithLog(context.TODO(), klog.NewKlogr().WithName("NodePublishVolume").WithValues("volumeId", volumeId))
			BeforeEach(func() {
				patch = ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
					return nil
				})
			})
			AfterEach(func() {
				patch.Reset()
			})
			It("should succeed", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().CreateVol(ctx, volumeId, subPath).Return(bindSource, nil)
				mockJfs.EXPECT().BindTarget(ctx, bindSource, targetPath).Return(errors.New("test"))
				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(ctx, volumeId, targetPath, secret, volumeCtx, []string{"ro"}).Return(mockJfs, nil)
				mockJuicefs.EXPECT().CreateTarget(ctx, targetPath).Return(nil)
				juicefsDriver.juicefs = mockJuicefs
				req := &csi.NodePublishVolumeRequest{
					VolumeId:         volumeId,
					TargetPath:       targetPath,
					VolumeCapability: stdVolCap,
					Readonly:         true,
					Secrets:          secret,
					VolumeContext:    volumeCtx,
				}

				_, err := juicefsDriver.NodePublishVolume(context.TODO(), req)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("test MkdirAll err", func() {
			volumeId := "vol-test"
			subPath := "/subPath"
			targetPath := "/test/path"
			volumeCtx := map[string]string{"subPath": subPath}
			secret := map[string]string{"a": "b"}

			var (
				patches []*Patches
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
						return errors.New("test")
					}),
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return false, nil
					}),
				)
				mounter := &mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				}
				client := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
				jfs := juicefs.NewJfsProvider(mounter, client)
				juicefsDriver.juicefs = jfs
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				req := &csi.NodePublishVolumeRequest{
					VolumeId:         volumeId,
					TargetPath:       targetPath,
					VolumeCapability: stdVolCap,
					Readonly:         true,
					Secrets:          secret,
					VolumeContext:    volumeCtx,
				}

				_, err := juicefsDriver.NodePublishVolume(context.TODO(), req)
				Expect(err).ShouldNot(BeNil())
			})
		})
	})
	Describe("Publish invalid", func() {
		Context("no target", func() {
			It("should fail", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockJuicefs := mocks.NewMockInterface(mockCtl)
				juicefsDriver.juicefs = mockJuicefs
				targetPath := ""

				req := &csi.NodePublishVolumeRequest{
					TargetPath:       targetPath,
					VolumeCapability: stdVolCap,
				}

				_, err := juicefsDriver.NodePublishVolume(context.TODO(), req)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("no capability", func() {
			It("should fail", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockJuicefs := mocks.NewMockInterface(mockCtl)
				juicefsDriver.juicefs = mockJuicefs
				targetPath := "/test"

				req := &csi.NodePublishVolumeRequest{
					TargetPath:       targetPath,
					VolumeCapability: nil,
				}

				_, err := juicefsDriver.NodePublishVolume(context.TODO(), req)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("invalid capability", func() {
			It("should fail", func() {
				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				mockJuicefs := mocks.NewMockInterface(mockCtl)
				juicefsDriver.juicefs = mockJuicefs
				targetPath := "/test"

				invalidVolumeCaps := &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
					},
				}
				req := &csi.NodePublishVolumeRequest{
					TargetPath:       targetPath,
					VolumeCapability: invalidVolumeCaps,
				}

				_, err := juicefsDriver.NodePublishVolume(context.TODO(), req)
				Expect(err).ShouldNot(BeNil())
			})
		})
	})
	Describe("Unpublish", func() {
		Context("test normal", func() {
			var (
				patches []*Patches
			)
			BeforeEach(func() {
				patches = append(patches,
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
				targetPath := "/test/path"
				volumeId := "vol-test"

				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				log := klog.NewKlogr().WithName("NodeUnpublishVolume")
				ctxWithLog := util.WithLog(context.TODO(), log)

				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsUnmount(ctxWithLog, volumeId, targetPath).Return(nil)

				juicefsDriver.juicefs = mockJuicefs

				req := &csi.NodeUnpublishVolumeRequest{
					TargetPath: targetPath,
					VolumeId:   volumeId,
				}

				_, err := juicefsDriver.NodeUnpublishVolume(context.TODO(), req)
				Expect(err).Should(BeNil())
			})
		})
		Context("JfsUnmount err", func() {
			var (
				patches []*Patches
			)
			BeforeEach(func() {
				patches = append(patches,
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
				targetPath := "/test/path"
				volumeId := "vol-test"

				mockCtl := gomock.NewController(GinkgoT())
				defer mockCtl.Finish()
				log := klog.NewKlogr().WithName("NodeUnpublishVolume")
				ctxWithLog := util.WithLog(context.TODO(), log)

				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsUnmount(ctxWithLog, volumeId, targetPath).Return(errors.New("test"))

				juicefsDriver.juicefs = mockJuicefs

				req := &csi.NodeUnpublishVolumeRequest{
					TargetPath: targetPath,
					VolumeId:   volumeId,
				}
				_, err := juicefsDriver.NodeUnpublishVolume(context.TODO(), req)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("nil target", func() {
			It("should succeed", func() {
				req := &csi.NodeUnpublishVolumeRequest{
					TargetPath: "",
					VolumeId:   "vol-test",
				}
				_, err := juicefsDriver.NodeUnpublishVolume(context.TODO(), req)
				Expect(err).ShouldNot(BeNil())
			})
		})
	})
})

func Test_nodeService_NodeGetCapabilities(t *testing.T) {
	type fields struct {
		juicefs   juicefs.Interface
		nodeID    string
		k8sClient *k8s.K8sClient
	}
	type args struct {
		ctx context.Context
		req *csi.NodeGetCapabilitiesRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.NodeGetCapabilitiesResponse
		wantErr bool
	}{
		{
			name:   "test",
			fields: fields{},
			args:   args{},
			want: &csi.NodeGetCapabilitiesResponse{
				Capabilities: []*csi.NodeServiceCapability{
					{
						Type: &csi.NodeServiceCapability_Rpc{
							Rpc: &csi.NodeServiceCapability_RPC{
								Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	registerer, _ := util.NewPrometheus(config.NodeName)
	metrics := newNodeMetrics(registerer)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &nodeService{
				juicefs:   tt.fields.juicefs,
				nodeID:    tt.fields.nodeID,
				k8sClient: tt.fields.k8sClient,
				metrics:   metrics,
			}
			got, err := d.NodeGetCapabilities(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeGetCapabilities() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeGetCapabilities() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeService_NodeGetInfo(t *testing.T) {
	type fields struct {
		juicefs   juicefs.Interface
		nodeID    string
		k8sClient *k8s.K8sClient
	}
	type args struct {
		ctx context.Context
		req *csi.NodeGetInfoRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.NodeGetInfoResponse
		wantErr bool
	}{
		{
			name: "test",
			fields: fields{
				nodeID: "test",
			},
			args:    args{},
			want:    &csi.NodeGetInfoResponse{NodeId: "test"},
			wantErr: false,
		},
	}
	registerer, _ := util.NewPrometheus(config.NodeName)
	metrics := newNodeMetrics(registerer)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &nodeService{
				juicefs:   tt.fields.juicefs,
				nodeID:    tt.fields.nodeID,
				k8sClient: tt.fields.k8sClient,
				metrics:   metrics,
			}
			got, err := d.NodeGetInfo(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeGetInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeGetInfo() got = %v, want %v", got, tt.want)
			}
		})
	}
}

var _ = Describe("Test newNodeService", func() {
	Describe("normal", func() {
		var (
			patches []*Patches
			tmpCmd  = &exec.Cmd{}
		)
		BeforeEach(func() {
			patches = append(patches,
				ApplyFunc(exec.Command, func(name string, args ...string) *exec.Cmd {
					return tmpCmd
				}),
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
			registerer, _ := util.NewPrometheus(config.NodeName)
			_, err := newNodeService("test", nil, registerer)
			Expect(err).Should(BeNil())
		})
	})
})

func Test_nodeService_NodeExpandVolume(t *testing.T) {
	type fields struct {
		juicefs   juicefs.Interface
		nodeID    string
		k8sClient *k8s.K8sClient
	}
	type args struct {
		ctx context.Context
		req *csi.NodeExpandVolumeRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.NodeExpandVolumeResponse
		wantErr bool
	}{
		{
			name:    "test",
			fields:  fields{},
			args:    args{},
			want:    nil,
			wantErr: true,
		},
	}
	registerer, _ := util.NewPrometheus(config.NodeName)
	metrics := newNodeMetrics(registerer)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &nodeService{
				juicefs:   tt.fields.juicefs,
				nodeID:    tt.fields.nodeID,
				k8sClient: tt.fields.k8sClient,
				metrics:   metrics,
			}
			got, err := d.NodeExpandVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeExpandVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeExpandVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeService_NodeGetVolumeStats(t *testing.T) {
	type fields struct {
		juicefs   juicefs.Interface
		nodeID    string
		k8sClient *k8s.K8sClient
	}
	type args struct {
		ctx context.Context
		req *csi.NodeGetVolumeStatsRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.NodeGetVolumeStatsResponse
		wantErr bool
	}{
		{
			name:    "test",
			fields:  fields{},
			args:    args{},
			want:    nil,
			wantErr: true,
		},
	}
	registerer, _ := util.NewPrometheus(config.NodeName)
	metrics := newNodeMetrics(registerer)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &nodeService{
				juicefs:   tt.fields.juicefs,
				nodeID:    tt.fields.nodeID,
				k8sClient: tt.fields.k8sClient,
				metrics:   metrics,
			}
			got, err := d.NodeGetVolumeStats(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeGetVolumeStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeGetVolumeStats() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeService_NodeStageVolume(t *testing.T) {
	type fields struct {
		juicefs   juicefs.Interface
		nodeID    string
		k8sClient *k8s.K8sClient
	}
	type args struct {
		ctx context.Context
		req *csi.NodeStageVolumeRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.NodeStageVolumeResponse
		wantErr bool
	}{
		{
			name:    "test",
			fields:  fields{},
			args:    args{},
			want:    nil,
			wantErr: true,
		},
	}
	registerer, _ := util.NewPrometheus(config.NodeName)
	metrics := newNodeMetrics(registerer)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &nodeService{
				juicefs:   tt.fields.juicefs,
				nodeID:    tt.fields.nodeID,
				k8sClient: tt.fields.k8sClient,
				metrics:   metrics,
			}
			got, err := d.NodeStageVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeStageVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeStageVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeService_NodeUnstageVolume(t *testing.T) {
	type fields struct {
		juicefs   juicefs.Interface
		nodeID    string
		k8sClient *k8s.K8sClient
	}
	type args struct {
		ctx context.Context
		req *csi.NodeUnstageVolumeRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.NodeUnstageVolumeResponse
		wantErr bool
	}{
		{
			name:    "test",
			fields:  fields{},
			args:    args{},
			want:    nil,
			wantErr: true,
		},
	}
	registerer, _ := util.NewPrometheus(config.NodeName)
	metrics := newNodeMetrics(registerer)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &nodeService{
				juicefs:   tt.fields.juicefs,
				nodeID:    tt.fields.nodeID,
				k8sClient: tt.fields.k8sClient,
				metrics:   metrics,
			}
			got, err := d.NodeUnstageVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeUnstageVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeUnstageVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}
