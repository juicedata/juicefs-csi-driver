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
	"errors"
	"os/exec"
	"reflect"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func TestNewDriver(t *testing.T) {
	Convey("Test NewDriver", t, func() {
		Convey("normal", func() {
			endpoint := "127.0.0.1"
			nodeId := "test-node"
			fakeClientSet := fake.NewSimpleClientset()
			fakeClient := &k8s.K8sClient{Interface: fakeClientSet}
			patch1 := ApplyFunc(k8s.NewClient, func() (*k8s.K8sClient, error) {
				return fakeClient, nil
			})
			defer patch1.Reset()
			patch3 := ApplyFunc(newNodeService, func(nodeID string, k8sClient *k8s.K8sClient) (*nodeService, error) {
				return &nodeService{}, nil
			})
			defer patch3.Reset()

			patch4 := ApplyFunc(newProvisionerService, func(k8sClient *k8s.K8sClient) (provisionerService, error) {
				return provisionerService{}, nil
			})
			defer patch4.Reset()
			var tmpCmd = &exec.Cmd{}
			patch2 := ApplyFunc(exec.Command, func(name string, args ...string) *exec.Cmd {
				return tmpCmd
			})
			defer patch2.Reset()
			patch5 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch5.Reset()

			registerer, _ := util.NewPrometheus(config.NodeName)
			driver, err := NewDriver(endpoint, nodeId, false, "", time.Second, registerer)
			So(err, ShouldBeNil)
			if driver.endpoint != endpoint {
				t.Fatalf("expected driver endpoint: %s, got: %s", endpoint, driver.endpoint)
			}
		})
		Convey("err", func() {
			endpoint := "127.0.0.1"
			nodeId := "test-node"
			fakeClientSet := fake.NewSimpleClientset()
			fakeClient := &k8s.K8sClient{Interface: fakeClientSet}
			patch1 := ApplyFunc(k8s.NewClient, func() (*k8s.K8sClient, error) {
				return fakeClient, nil
			})
			defer patch1.Reset()
			patch3 := ApplyFunc(newNodeService, func(nodeID string, k8sClient *k8s.K8sClient) (*nodeService, error) {
				return &nodeService{}, errors.New("test")
			})
			defer patch3.Reset()
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			var tmpCmd = &exec.Cmd{}
			patch2 := ApplyFunc(exec.Command, func(name string, args ...string) *exec.Cmd {
				return tmpCmd
			})
			defer patch2.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch4.Reset()

			registerer, _ := util.NewPrometheus(config.NodeName)
			_, err := NewDriver(endpoint, nodeId, false, "", time.Second, registerer)
			So(err, ShouldNotBeNil)
		})
		Convey("by process", func() {
			config.ByProcess = true
			endpoint := "127.0.0.1"
			nodeId := "test-node"
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			var tmpCmd = &exec.Cmd{}
			patch2 := ApplyFunc(exec.Command, func(name string, args ...string) *exec.Cmd {
				return tmpCmd
			})
			defer patch2.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch4.Reset()

			registerer, _ := util.NewPrometheus(config.NodeName)
			_, err := NewDriver(endpoint, nodeId, false, "", time.Second, registerer)
			So(err, ShouldBeNil)
		})
	})
}
