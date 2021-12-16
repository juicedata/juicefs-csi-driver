package driver

import (
	"context"
	"errors"
	. "github.com/agiledragon/gomonkey"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mocks"
	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/mount"
	"reflect"
	"testing"
)

func TestNewControllerService(t *testing.T) {
	Convey("Test newNodeService", t, func() {
		Convey("normal", func() {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockJuicefs := mocks.NewMockInterface(mockCtl)
			mockJuicefs.EXPECT().Version().Return([]byte(""), nil)
			patch1 := ApplyFunc(juicefs.NewJfsProvider, func(mounter *mount.SafeFormatAndMount) (juicefs.Interface, error) {
				return mockJuicefs, nil
			})
			defer patch1.Reset()

			controllerSvc := newControllerService()

			if controllerSvc.juicefs != mockJuicefs {
				t.Fatalf("expected juicefs attribute to be equal to instantiated juicefs")
			}
		})
		Convey("new error", func() {
			patch1 := ApplyFunc(juicefs.NewJfsProvider, func(mounter *mount.SafeFormatAndMount) (juicefs.Interface, error) {
				return nil, errors.New("test")
			})
			defer patch1.Reset()
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("The code did not panic")
				}
			}()

			_ = newControllerService()
		})
		Convey("version error", func() {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockJuicefs := mocks.NewMockInterface(mockCtl)
			mockJuicefs.EXPECT().Version().Return([]byte(""), errors.New("test"))
			patch1 := ApplyFunc(juicefs.NewJfsProvider, func(mounter *mount.SafeFormatAndMount) (juicefs.Interface, error) {
				return mockJuicefs, nil
			})
			defer patch1.Reset()

			defer func() {
				if r := recover(); r == nil {
					t.Errorf("The code did not panic")
				}
			}()

			_ = newControllerService()
		})
	})
}

func TestCreateVolume(t *testing.T) {
	stdVolCap := []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
	stdVolSize := int64(5 * 1024 * 1024 * 1024)
	stdCapRange := &csi.CapacityRange{RequiredBytes: stdVolSize}

	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				volumeId := "vol-test"
				targetPath := "/test/path"
				secret := map[string]string{"a": "b"}
				volCtx := map[string]string{"c": "d"}
				req := &csi.CreateVolumeRequest{
					Name:               volumeId,
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Secrets:            secret,
					Parameters:         volCtx,
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().CreateVol(volumeId, volumeId).Return("", nil)
				mockJfs.EXPECT().GetBasePath().Return(targetPath)

				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(volumeId, "", secret, nil, []string{}, false).Return(mockJfs, nil)
				mockJuicefs.EXPECT().Unmount(targetPath).Return(nil)

				juicefsDriver := controllerService{
					juicefs: mockJuicefs,
					vols:    make(map[string]int64),
				}

				got, err := juicefsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
				if v, ok := got.Volume.VolumeContext["subPath"]; !ok || v != volumeId {
					t.Fatalf("volumeContext is not volumeId: %v", got.Volume.VolumeContext)
				}
				for k, v := range volCtx {
					if value, ok := got.Volume.VolumeContext[k]; !ok || v != value {
						t.Fatalf("volumeContext is not volumeId: %v", got.Volume.VolumeContext)
					}
				}
				if juicefsDriver.vols[volumeId] != stdVolSize {
					t.Fatalf("volume size in driver is not %v: %v", stdVolSize, got.Volume.VolumeContext)
				}
			},
		},
		{
			name: "name nil",
			testFunc: func(t *testing.T) {
				secret := map[string]string{"a": "b"}
				req := &csi.CreateVolumeRequest{
					Name:               "",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Secrets:            secret,
				}

				ctx := context.Background()

				juicefsDriver := controllerService{
					juicefs: nil,
					vols:    make(map[string]int64),
				}

				_, err := juicefsDriver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatalf("error is nil")
				}
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.InvalidArgument {
					t.Fatalf("error status code is not invalid: %v", srvErr.Code())
				}
			},
		},
		{
			name: "nil cap",
			testFunc: func(t *testing.T) {
				volumeId := "vol-test"
				secret := map[string]string{"a": "b"}
				req := &csi.CreateVolumeRequest{
					Name:               volumeId,
					CapacityRange:      stdCapRange,
					VolumeCapabilities: nil,
					Secrets:            secret,
				}

				ctx := context.Background()
				juicefsDriver := controllerService{
					juicefs: nil,
					vols:    make(map[string]int64),
				}

				_, err := juicefsDriver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatalf("error is nil")
				}
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.InvalidArgument {
					t.Fatalf("error status code is not invalid: %v", srvErr.Code())
				}
			},
		},
		{
			name: "invalid cap",
			testFunc: func(t *testing.T) {
				volumeId := "vol-test"
				secret := map[string]string{"a": "b"}
				req := &csi.CreateVolumeRequest{
					Name:               volumeId,
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Secrets:            secret,
				}

				ctx := context.Background()
				juicefsDriver := controllerService{
					juicefs: nil,
					vols:    map[string]int64{volumeId: int64(5)},
				}

				_, err := juicefsDriver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatalf("error is nil")
				}
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.AlreadyExists {
					t.Fatalf("error status code is not invalid: %v", srvErr.Code())
				}
			},
		},
		{
			name: "JfsMount error",
			testFunc: func(t *testing.T) {
				volumeId := "vol-test"
				secret := map[string]string{"a": "b"}
				volCtx := map[string]string{"c": "d"}
				req := &csi.CreateVolumeRequest{
					Name:               volumeId,
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Secrets:            secret,
					Parameters:         volCtx,
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(volumeId, "", secret, nil, []string{}, false).Return(nil, errors.New("test"))

				juicefsDriver := controllerService{
					juicefs: mockJuicefs,
					vols:    make(map[string]int64),
				}

				_, err := juicefsDriver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatalf("error is nil")
				}
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.Internal {
					t.Fatalf("error status code is not invalid: %v", srvErr.Code())
				}
			},
		},
		{
			name: "CreateVol error",
			testFunc: func(t *testing.T) {
				volumeId := "vol-test"
				secret := map[string]string{"a": "b"}
				volCtx := map[string]string{"c": "d"}
				req := &csi.CreateVolumeRequest{
					Name:               volumeId,
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Secrets:            secret,
					Parameters:         volCtx,
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().CreateVol(volumeId, volumeId).Return("", errors.New("test"))

				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(volumeId, "", secret, nil, []string{}, false).Return(mockJfs, nil)

				juicefsDriver := controllerService{
					juicefs: mockJuicefs,
					vols:    make(map[string]int64),
				}

				_, err := juicefsDriver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatalf("error is nil")
				}
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.Internal {
					t.Fatalf("error status code is not invalid: %v", srvErr.Code())
				}
			},
		},
		{
			name: "Umount error",
			testFunc: func(t *testing.T) {
				volumeId := "vol-test"
				targetPath := "/test/path"
				secret := map[string]string{"a": "b"}
				volCtx := map[string]string{"c": "d"}
				req := &csi.CreateVolumeRequest{
					Name:               volumeId,
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Secrets:            secret,
					Parameters:         volCtx,
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().CreateVol(volumeId, volumeId).Return("", nil)
				mockJfs.EXPECT().GetBasePath().Return(targetPath)

				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(volumeId, "", secret, nil, []string{}, false).Return(mockJfs, nil)
				mockJuicefs.EXPECT().Unmount(targetPath).Return(errors.New("test"))

				juicefsDriver := controllerService{
					juicefs: mockJuicefs,
					vols:    make(map[string]int64),
				}

				_, err := juicefsDriver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatalf("error is nil")
				}
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.Internal {
					t.Fatalf("error status code is not invalid: %v", srvErr.Code())
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestDeleteVolume(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				volumeId := "vol-test"
				targetPath := "/test/path"
				secret := map[string]string{"a": "b"}
				req := &csi.DeleteVolumeRequest{
					VolumeId: "vol-test",
					Secrets:  secret,
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().DeleteVol(volumeId, secret).Return(nil)
				mockJfs.EXPECT().GetBasePath().Return(targetPath)

				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(volumeId, "", secret, nil, []string{}, false).Return(mockJfs, nil)
				mockJuicefs.EXPECT().Unmount(targetPath).Return(nil)

				juicefsDriver := controllerService{
					juicefs: mockJuicefs,
					vols: map[string]int64{
						volumeId: int64(1),
					},
				}

				_, err := juicefsDriver.DeleteVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
				if _, ok := juicefsDriver.vols[volumeId]; ok {
					t.Fatalf("volume size in driver is not deleted: %v", juicefsDriver.vols)
				}
			},
		},
		{
			name: "volumeId nil",
			testFunc: func(t *testing.T) {
				secret := map[string]string{"a": "b"}
				req := &csi.DeleteVolumeRequest{
					VolumeId: "",
					Secrets:  secret,
				}

				ctx := context.Background()

				juicefsDriver := controllerService{
					juicefs: nil,
					vols:    make(map[string]int64),
				}

				_, err := juicefsDriver.DeleteVolume(ctx, req)
				if err == nil {
					t.Fatalf("error is nil")
				}
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.InvalidArgument {
					t.Fatalf("error status code is not invalid: %v", srvErr.Code())
				}
			},
		},
		{
			name: "JfsMount error",
			testFunc: func(t *testing.T) {
				volumeId := "vol-test"
				secret := map[string]string{"a": "b"}
				req := &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
					Secrets:  secret,
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(volumeId, "", secret, nil, []string{}, false).Return(nil, errors.New("test"))

				juicefsDriver := controllerService{
					juicefs: mockJuicefs,
					vols:    map[string]int64{volumeId: int64(1)},
				}

				_, err := juicefsDriver.DeleteVolume(ctx, req)
				if err == nil {
					t.Fatalf("error is nil")
				}
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.Internal {
					t.Fatalf("error status code is not invalid: %v", srvErr.Code())
				}
			},
		},
		{
			name: "DeleteVol error",
			testFunc: func(t *testing.T) {
				volumeId := "vol-test"
				secret := map[string]string{"a": "b"}
				req := &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
					Secrets:  secret,
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().DeleteVol(volumeId, secret).Return(errors.New("test"))

				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(volumeId, "", secret, nil, []string{}, false).Return(mockJfs, nil)

				juicefsDriver := controllerService{
					juicefs: mockJuicefs,
					vols:    map[string]int64{volumeId: int64(1)},
				}

				_, err := juicefsDriver.DeleteVolume(ctx, req)
				if err == nil {
					t.Fatalf("error is nil")
				}
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.Internal {
					t.Fatalf("error status code is not invalid: %v", srvErr.Code())
				}
			},
		},
		{
			name: "Unmount error",
			testFunc: func(t *testing.T) {
				volumeId := "vol-test"
				targetPath := "/test"
				secret := map[string]string{"a": "b"}
				req := &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
					Secrets:  secret,
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockJfs := mocks.NewMockJfs(mockCtl)
				mockJfs.EXPECT().DeleteVol(volumeId, secret).Return(nil)
				mockJfs.EXPECT().GetBasePath().Return(targetPath)

				mockJuicefs := mocks.NewMockInterface(mockCtl)
				mockJuicefs.EXPECT().JfsMount(volumeId, "", secret, nil, []string{}, false).Return(mockJfs, nil)
				mockJuicefs.EXPECT().Unmount(targetPath).Return(errors.New("test"))

				juicefsDriver := controllerService{
					juicefs: mockJuicefs,
					vols:    map[string]int64{volumeId: int64(1)},
				}

				_, err := juicefsDriver.DeleteVolume(ctx, req)
				if err == nil {
					t.Fatalf("error is nil")
				}
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.Internal {
					t.Fatalf("error status code is not invalid: %v", srvErr.Code())
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestControllerGetCapabilities(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.ControllerGetCapabilitiesRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ControllerGetCapabilitiesResponse
		wantErr bool
	}{
		{
			name:   "test",
			fields: fields{},
			args:   args{},
			want: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: []*csi.ControllerServiceCapability{{
					Type: &csi.ControllerServiceCapability_Rpc{
						Rpc: &csi.ControllerServiceCapability_RPC{
							Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
						},
					},
				}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &controllerService{
				juicefs: tt.fields.juicefs,
				vols:    tt.fields.vols,
			}
			got, err := d.ControllerGetCapabilities(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ControllerGetCapabilities() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ControllerGetCapabilities() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerService_ValidateVolumeCapabilities(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.ValidateVolumeCapabilitiesRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ValidateVolumeCapabilitiesResponse
		wantErr bool
	}{
		{
			name: "test",
			fields: fields{
				vols: map[string]int64{"test": int64(1)},
			},
			args: args{
				req: &csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: "test",
					VolumeCapabilities: []*csi.VolumeCapability{{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					}},
				},
			},
			want: &csi.ValidateVolumeCapabilitiesResponse{
				Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
					VolumeCapabilities: []*csi.VolumeCapability{{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					}},
				},
			},
			wantErr: false,
		},
		{
			name: "volCap nil",
			fields: fields{
				vols: map[string]int64{"test": int64(1)},
			},
			args: args{
				req: &csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: "test2",
					VolumeCapabilities: []*csi.VolumeCapability{{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					}},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "volumeId nil",
			fields: fields{
				vols: map[string]int64{"test": int64(1)},
			},
			args: args{
				req: &csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: "",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &controllerService{
				juicefs: tt.fields.juicefs,
				vols:    tt.fields.vols,
			}
			got, err := d.ValidateVolumeCapabilities(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVolumeCapabilities() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateVolumeCapabilities() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isValidVolumeCapabilities(t *testing.T) {
	type args struct {
		volCaps []*csi.VolumeCapability
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test",
			args: args{
				volCaps: []*csi.VolumeCapability{{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
					},
				}},
			},
			want: true,
		},
		{
			name: "test-false",
			args: args{
				volCaps: []*csi.VolumeCapability{{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
					},
				}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidVolumeCapabilities(tt.args.volCaps); got != tt.want {
				t.Errorf("isValidVolumeCapabilities() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerService_GetCapacity(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.GetCapacityRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.GetCapacityResponse
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &controllerService{
				juicefs: tt.fields.juicefs,
				vols:    tt.fields.vols,
			}
			got, err := d.GetCapacity(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCapacity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCapacity() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerService_ListVolumes(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.ListVolumesRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ListVolumesResponse
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &controllerService{
				juicefs: tt.fields.juicefs,
				vols:    tt.fields.vols,
			}
			got, err := d.ListVolumes(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListVolumes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListVolumes() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerService_CreateSnapshot(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.CreateSnapshotRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.CreateSnapshotResponse
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &controllerService{
				juicefs: tt.fields.juicefs,
				vols:    tt.fields.vols,
			}
			got, err := d.CreateSnapshot(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSnapshot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateSnapshot() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerService_DeleteSnapshot(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.DeleteSnapshotRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.DeleteSnapshotResponse
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &controllerService{
				juicefs: tt.fields.juicefs,
				vols:    tt.fields.vols,
			}
			got, err := d.DeleteSnapshot(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteSnapshot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteSnapshot() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerService_ListSnapshots(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.ListSnapshotsRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ListSnapshotsResponse
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &controllerService{
				juicefs: tt.fields.juicefs,
				vols:    tt.fields.vols,
			}
			got, err := d.ListSnapshots(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListSnapshots() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListSnapshots() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerService_ControllerExpandVolume(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.ControllerExpandVolumeRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ControllerExpandVolumeResponse
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &controllerService{
				juicefs: tt.fields.juicefs,
				vols:    tt.fields.vols,
			}
			got, err := d.ControllerExpandVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ControllerExpandVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ControllerExpandVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerService_ControllerPublishVolume(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.ControllerPublishVolumeRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ControllerPublishVolumeResponse
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &controllerService{
				juicefs: tt.fields.juicefs,
				vols:    tt.fields.vols,
			}
			got, err := d.ControllerPublishVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ControllerPublishVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ControllerPublishVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerService_ControllerUnpublishVolume(t *testing.T) {
	type fields struct {
		juicefs juicefs.Interface
		vols    map[string]int64
	}
	type args struct {
		ctx context.Context
		req *csi.ControllerUnpublishVolumeRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *csi.ControllerUnpublishVolumeResponse
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &controllerService{
				juicefs: tt.fields.juicefs,
				vols:    tt.fields.vols,
			}
			got, err := d.ControllerUnpublishVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ControllerUnpublishVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ControllerUnpublishVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}
