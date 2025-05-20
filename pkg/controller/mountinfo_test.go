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

package controller

import (
	ctx "context"
	"os"
	"strconv"
	"strings"
	"syscall"

	. "github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sMount "k8s.io/utils/mount"
)

type mockMountInfoTable struct {
	mis []k8sMount.MountInfo
}

func (mmit *mockMountInfoTable) addBaseTarget(root, podUID, pv string) {
	mountPoint := strings.Join([]string{
		"/poddir",
		podUID,
		containerCsiDirectory,
		pv,
		"mount",
	}, "/")
	mi := k8sMount.MountInfo{
		MountPoint: mountPoint,
		Root:       root,
	}
	mmit.mis = append(mmit.mis, mi)
}

func (mmit *mockMountInfoTable) addSubpathTarget(root, podUID, pv string, idx int) {
	mountPoint := strings.Join([]string{
		"/poddir",
		podUID,
		containerSubPathDirectory,
		pv,
		strconv.Itoa(idx),
	}, "/")
	mi := k8sMount.MountInfo{
		MountPoint: mountPoint,
		Root:       root,
	}
	mmit.mis = append(mmit.mis, mi)
}

var _ = Describe("MountInfo", func() {
	var (
		podUID = "uid-1"
		pv     = "pvn"
		mmit   = &mockMountInfoTable{}
		mit    = &mountInfoTable{}
	)

	BeforeEach(func() {
		mmit.addBaseTarget("/sub", podUID, pv)
		mmit.addSubpathTarget("/sub/sub_0", podUID, pv, 0)
		mmit.addSubpathTarget("/sub/sub_1", podUID, pv, 1)
		mmit.addSubpathTarget("/sub/sub_1", podUID, pv, 1)
		mmit.addBaseTarget("/sub", "uid-2", pv)
		mmit.addBaseTarget("/sub", "uid-3", pv)
		mmit.addSubpathTarget("/sub/sub_0", "uid-3", pv, 0)
		mit = newMountInfoTable()
		mit.mis = mmit.mis
	})
	AfterEach(func() {
		mmit = &mockMountInfoTable{}
	})

	When("Test MountInfo", func() {
		Context("test normal target", func() {
			var (
				patches []*Patches
				outputs = []OutputCell{
					{Values: Params{nil, nil}, Times: 4},
				}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, outputs),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				mi := mit.resolveTarget(ctx.TODO(), target)
				Expect(mi).ShouldNot(BeNil())
				Expect(mi.baseTarget).ShouldNot(BeNil())
				Expect(mi.baseTarget.target).Should(Equal(target))
				Expect(mi.baseTarget.subpath).Should(Equal("sub"))
				Expect(mi.baseTarget.status).Should(Equal(targetStatusMounted))
				Expect(mi.subPathTarget).Should(HaveLen(2))

				for _, m := range mi.subPathTarget {
					if m.subpath == "sub/sub_0" {
						Expect(m.subpath).Should(Equal("sub/sub_0"))
						Expect(m.target).Should(Equal("/poddir/uid-1/volume-subpaths/pvn/0"))
						Expect(m.count).Should(Equal(1))
						Expect(m.status).Should(Equal(targetStatusMounted))
					} else if m.subpath == "sub/sub_1" {
						Expect(m.subpath).Should(Equal("sub/sub_1"))
						Expect(m.target).Should(Equal("/poddir/uid-1/volume-subpaths/pvn/1"))
						Expect(m.count).Should(Equal(2))
						Expect(m.status).Should(Equal(targetStatusMounted))
					} else {
						Fail("error subPath")
					}
				}
			})
		})
		Context("test invalid base target", func() {
			It("should succeed", func() {
				mi := mit.resolveTarget(ctx.TODO(), "/invalid-target-path")
				Expect(mi).Should(BeNil())
			})
		})
		Context("test stat err", func() {
			var (
				patches []*Patches
				outputs = []OutputCell{
					{Values: Params{nil, volErr}, Times: 3},
				}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, outputs),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})

			It("should succeed", func() {
				mi := mit.resolveTarget(ctx.TODO(), target)

				Expect(mi).ShouldNot(BeNil())
				Expect(mi.baseTarget.status).Should(Equal(targetStatusUnexpect))
				Expect(mi.baseTarget.err).ShouldNot(BeNil())

				Expect(mi.subPathTarget).Should(HaveLen(2))
				Expect(mi.subPathTarget[0].status).Should(Equal(targetStatusUnexpect))
				Expect(mi.subPathTarget[0].err).ShouldNot(BeNil())
				Expect(mi.subPathTarget[1].status).Should(Equal(targetStatusUnexpect))
				Expect(mi.subPathTarget[1].err).ShouldNot(BeNil())
			})
		})
		Context("test target umounted", func() {
			var (
				patches []*Patches
				outputs = []OutputCell{
					{Values: Params{nil, nil}},
				}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, outputs),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})

			It("should succeed", func() {
				mi := mit.resolveTarget(ctx.TODO(), "/poddir/uid-1/volumes/kubernetes.io~csi/pvn-not-exist/mount")

				Expect(mi).ShouldNot(BeNil())
				Expect(mi.baseTarget.count).Should(Equal(0))
				Expect(mi.baseTarget.status).Should(Equal(targetStatusNotMount))
			})
		})
		Context("test target corrupt", func() {
			var (
				patches []*Patches
				outputs = []OutputCell{
					{Values: Params{nil, os.NewSyscallError("", syscall.ENOTCONN)}},
					{Values: Params{nil, nil}, Times: 2},
				}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, outputs),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})

			It("should succeed", func() {
				mit.mis = mmit.mis
				mi := mit.resolveTarget(ctx.TODO(), target)

				Expect(mi).ShouldNot(BeNil())
				Expect(mi.baseTarget.count).Should(Equal(1))
				Expect(mi.baseTarget.status).Should(Equal(targetStatusCorrupt))
			})
		})
		Context("test target not exist", func() {
			var (
				patches []*Patches
				outputs = []OutputCell{
					{Values: Params{nil, notExistsErr}},
					{Values: Params{nil, nil}, Times: 2},
				}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, outputs),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})

			It("should succeed", func() {
				mi := mit.resolveTarget(ctx.TODO(), target)

				Expect(mi).ShouldNot(BeNil())
				Expect(mi.baseTarget.status).Should(Equal(targetStatusNotExist))
			})
		})
		Context("test inconsistent", func() {
			var (
				patches []*Patches
				outputs = []OutputCell{
					{Values: Params{nil, nil}, Times: 3},
				}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, outputs),
				)
				mmit.addBaseTarget("/sub-x", podUID, pv)
				mit.mis = mmit.mis
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})

			It("should succeed", func() {
				mi := mit.resolveTarget(ctx.TODO(), target)

				Expect(mi).ShouldNot(BeNil())
				Expect(mi.baseTarget.inconsistent).Should(BeTrue())
			})
		})
	})
})
