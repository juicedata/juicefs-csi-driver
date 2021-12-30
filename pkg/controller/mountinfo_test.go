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
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"
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

func TestMountInfo(t *testing.T) {
	podUID := "uid-1"
	pv := "pvn"
	mmit := &mockMountInfoTable{}
	mmit.addBaseTarget("/sub", podUID, pv)
	mmit.addSubpathTarget("/sub/sub_0", podUID, pv, 0)
	mmit.addSubpathTarget("/sub/sub_1", podUID, pv, 1)
	mmit.addSubpathTarget("/sub/sub_1", podUID, pv, 1)
	mmit.addBaseTarget("/sub", "uid-2", pv)
	mmit.addBaseTarget("/sub", "uid-3", pv)
	mmit.addSubpathTarget("/sub/sub_0", "uid-3", pv, 0)

	patch1 := ApplyFunc(k8sMount.ParseMountInfo, func(filename string) ([]k8sMount.MountInfo, error) {
		return mmit.mis, nil
	})
	defer patch1.Reset()

	Convey("Test MountInfo", t, func() {
		Convey("test normal target", func() {
			outputs := []OutputCell{
				{Values: Params{nil, nil}, Times: 4},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()

			mit := newMountInfoTable()
			mit.parse()
			mi := mit.resolveTarget(target)

			So(mi, ShouldNotBeNil)
			So(mi.baseTarget, ShouldNotBeNil)
			So(mi.baseTarget.target, ShouldEqual, target)
			So(mi.baseTarget.subpath, ShouldEqual, "sub")
			So(mi.baseTarget.status, ShouldEqual, targetStatusMounted)

			So(len(mi.subPathTarget), ShouldEqual, 2)

			So(mi.subPathTarget[0].subpath, ShouldEqual, "sub/sub_0")
			So(mi.subPathTarget[0].target, ShouldEqual, "/poddir/uid-1/volume-subpaths/pvn/0")
			So(mi.subPathTarget[0].count, ShouldEqual, 1)
			So(mi.subPathTarget[0].status, ShouldEqual, targetStatusMounted)

			So(mi.subPathTarget[1].subpath, ShouldEqual, "sub/sub_1")
			So(mi.subPathTarget[1].target, ShouldEqual, "/poddir/uid-1/volume-subpaths/pvn/1")
			So(mi.subPathTarget[1].count, ShouldEqual, 2)
			So(mi.subPathTarget[1].status, ShouldEqual, targetStatusMounted)
		})
		Convey("test invalid base target", func() {
			mit := newMountInfoTable()
			mit.parse()
			mi := mit.resolveTarget("/invalid-target-path")
			So(mi, ShouldBeNil)
		})
		Convey("test stat err", func() {
			outputs := []OutputCell{
				{Values: Params{nil, volErr}, Times: 3},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()

			mit := newMountInfoTable()
			mit.parse()
			mi := mit.resolveTarget(target)

			So(mi, ShouldNotBeNil)
			So(mi.baseTarget.status, ShouldEqual, targetStatusUnexpect)
			So(mi.baseTarget.err, ShouldNotBeNil)

			So(len(mi.subPathTarget), ShouldEqual, 2)
			So(mi.subPathTarget[0].status, ShouldEqual, targetStatusUnexpect)
			So(mi.subPathTarget[0].err, ShouldNotBeNil)
			So(mi.subPathTarget[1].status, ShouldEqual, targetStatusUnexpect)
			So(mi.subPathTarget[1].err, ShouldNotBeNil)
		})
		Convey("test target umounted", func() {
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()

			mit := newMountInfoTable()
			mit.parse()
			mi := mit.resolveTarget("/poddir/uid-1/volumes/kubernetes.io~csi/pvn-not-exist/mount")

			So(mi, ShouldNotBeNil)
			So(mi.baseTarget.count, ShouldEqual, 0)
			So(mi.baseTarget.status, ShouldEqual, targetStatusNotMount)
		})
		Convey("test target corrupt", func() {
			outputs := []OutputCell{
				{Values: Params{nil, os.NewSyscallError("", syscall.ENOTCONN)}},
				{Values: Params{nil, nil}, Times: 2},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()

			mit := newMountInfoTable()
			mit.parse()
			mi := mit.resolveTarget(target)

			So(mi, ShouldNotBeNil)
			So(mi.baseTarget.count, ShouldEqual, 1)
			So(mi.baseTarget.status, ShouldEqual, targetStatusCorrupt)
		})
		Convey("test target not exist", func() {
			outputs := []OutputCell{
				{Values: Params{nil, notExistsErr}},
				{Values: Params{nil, nil}, Times: 2},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()

			mit := newMountInfoTable()
			mit.parse()
			mi := mit.resolveTarget(target)

			So(mi, ShouldNotBeNil)
			So(mi.baseTarget.status, ShouldEqual, targetStatusNotExist)
		})
		Convey("test inconsistent", func() {
			outputs := []OutputCell{
				{Values: Params{nil, nil}, Times: 3},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()

			mmit.addBaseTarget("/sub-x", podUID, pv)

			mit := newMountInfoTable()
			mit.parse()
			mi := mit.resolveTarget(target)

			So(mi, ShouldNotBeNil)
			So(mi.baseTarget.inconsistent, ShouldEqual, true)
		})
	})
}
