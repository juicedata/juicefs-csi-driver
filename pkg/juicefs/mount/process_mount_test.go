package mount

import (
	"github.com/golang/mock/gomock"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver/mocks"
	"reflect"
	"testing"

	k8sexec "k8s.io/utils/exec"
	k8sMount "k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
)

func TestNewProcessMount(t *testing.T) {
	type args struct {
		setting *jfsConfig.JfsSetting
	}
	tests := []struct {
		name string
		args args
		want Interface
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
				}, nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewProcessMount(tt.args.setting); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewProcessMount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessMount_JUmount(t *testing.T) {
	targetPath := "/test"
	type fields struct {
		SafeFormatAndMount k8sMount.SafeFormatAndMount
		jfsSetting         *jfsConfig.JfsSetting
	}
	type args struct {
		volumeId string
		target   string
	}
	tests := []struct {
		name       string
		fields     fields
		expectMock func(mockMounter mocks.MockInterface)
		args       args
		wantErr    bool
	}{
		{
			name: "",
			fields: fields{
				jfsSetting: nil,
			},
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
				jfsSetting:         tt.fields.jfsSetting,
			}
			if err := p.JUmount(tt.args.volumeId, tt.args.target); (err != nil) != tt.wantErr {
				t.Errorf("JUmount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
