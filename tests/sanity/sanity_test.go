/*
Copyright 2018 The Kubernetes Authors.

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

package sanity

import (
	"os"
	"testing"

	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
)

const (
	mountPath = "/tmp/csi-mount"
	stagePath = "/tmp/csi-stage"
	socket    = "/tmp/csi.sock"
	endpoint  = "unix://" + socket
)

var jfsDriver *driver.Driver

func TestSanity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sanity Tests Suite")
}

var _ = BeforeSuite(func() {
	jfsDriver = driver.NewFakeDriver(endpoint, newFakeJfsProvider())
	go func() {
		Expect(jfsDriver.Run()).NotTo(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	jfsDriver.Stop()
	Expect(os.RemoveAll(socket)).NotTo(HaveOccurred())
})

var _ = Describe("JuiceFS CSI Driver", func() {
	config := sanity.NewTestConfig()
	config.Address = endpoint
	sanity.GinkgoTest(&config)
})
