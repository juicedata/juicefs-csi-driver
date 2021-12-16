package driver

import (
	"errors"
	. "github.com/agiledragon/gomonkey"
	"github.com/golang/mock/gomock"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mocks"
	. "github.com/smartystreets/goconvey/convey"
	"k8s.io/utils/mount"
	"testing"
)

func TestNewDriver(t *testing.T) {
	Convey("Test NewDriver", t, func() {
		Convey("normal", func() {
			endpoint := "127.0.0.1"
			nodeId := "test-node"
			patch1 := ApplyFunc(k8s.NewClient, func() (*k8s.K8sClient, error) {
				return nil, nil
			})
			defer patch1.Reset()
			patch3 := ApplyFunc(newNodeService, func(nodeID string) (*nodeService, error) {
				return &nodeService{}, nil
			})
			defer patch3.Reset()
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockJuicefs := mocks.NewMockInterface(mockCtl)
			mockJuicefs.EXPECT().Version().Return([]byte(""), nil)
			patch2 := ApplyFunc(juicefs.NewJfsProvider, func(mounter *mount.SafeFormatAndMount) (juicefs.Interface, error) {
				return mockJuicefs, nil
			})
			defer patch2.Reset()

			driver, err := NewDriver(endpoint, nodeId)
			So(err, ShouldBeNil)
			if driver.endpoint != endpoint {
				t.Fatalf("expected driver endpoint: %s, got: %s", endpoint, driver.endpoint)
			}
		})
		Convey("err", func() {
			endpoint := "127.0.0.1"
			nodeId := "test-node"
			patch1 := ApplyFunc(k8s.NewClient, func() (*k8s.K8sClient, error) {
				return nil, nil
			})
			defer patch1.Reset()
			patch3 := ApplyFunc(newNodeService, func(nodeID string) (*nodeService, error) {
				return &nodeService{}, errors.New("test")
			})
			defer patch3.Reset()
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockJuicefs := mocks.NewMockInterface(mockCtl)
			mockJuicefs.EXPECT().Version().Return([]byte(""), nil)
			patch2 := ApplyFunc(juicefs.NewJfsProvider, func(mounter *mount.SafeFormatAndMount) (juicefs.Interface, error) {
				return mockJuicefs, nil
			})
			defer patch2.Reset()

			_, err := NewDriver(endpoint, nodeId)
			So(err, ShouldNotBeNil)
		})
	})
}
