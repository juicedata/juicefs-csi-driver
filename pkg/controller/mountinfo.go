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
	"context"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	k8sMount "k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var miLog = klog.NewKlogr().WithName("mountinfo")

type mountInfoTable struct {
	mis []k8sMount.MountInfo
	// key is pod UID
	deletedPods map[string]bool
}

func newMountInfoTable() *mountInfoTable {
	return &mountInfoTable{
		deletedPods: make(map[string]bool),
	}
}

func (mit *mountInfoTable) parse() (err error) {
	mit.mis, err = k8sMount.ParseMountInfo("/proc/self/mountinfo")
	return
}

func (mit *mountInfoTable) setPodStatus(pod *corev1.Pod) {
	if pod.DeletionTimestamp != nil {
		mit.deletedPods[string(pod.UID)] = true
	} else {
		mit.deletedPods[string(pod.UID)] = false
	}
}

const (
	// place for subpath mounts
	containerSubPathDirectory = "volume-subpaths"
	// place for csi mounts
	containerCsiDirectory = "volumes/kubernetes.io~csi"
)

// resolve target path with subPath(volumeMount.subPath) in container
// return nil if not a valid csi target path
func (mit *mountInfoTable) resolveTarget(ctx context.Context, target string) *mountItem {
	pair := strings.Split(target, containerCsiDirectory)
	if len(pair) != 2 {
		return nil
	}
	podDir := strings.TrimSuffix(pair[0], "/")
	podUID := getPodUid(target)
	if podUID == "" {
		return nil
	}
	pvName := getPVName(target)
	if pvName == "" {
		return nil
	}

	mi := &mountItem{}
	mi.podDeleted, mi.podExist = mit.deletedPods[podUID]

	iterms := mit.resolveTargetItem(ctx, target, false)
	// must be 1 or 0
	if len(iterms) == 1 {
		mi.baseTarget = iterms[0]
	} else {
		mi.baseTarget = &targetItem{
			target: target,
		}
		mi.baseTarget.check(ctx, false)
	}
	subpathTargetPrefix := strings.Join([]string{
		podDir,
		containerSubPathDirectory,
		pvName,
	}, "/")
	mi.subPathTarget = mit.resolveTargetItem(ctx, subpathTargetPrefix, true)

	return mi
}

func (mit *mountInfoTable) resolveTargetItem(ctx context.Context, path string, isPrefix bool) []*targetItem {
	records := make(map[string]*targetItem)
	for _, mi := range mit.mis {
		match := false
		if isPrefix {
			if strings.HasPrefix(mi.MountPoint, path) {
				match = true
			}
		} else {
			if path == mi.MountPoint {
				match = true
			}
		}

		if !match {
			continue
		}

		subpath := strings.TrimSuffix(mi.Root, "//deleted")
		subpath = strings.Trim(subpath, "/")
		record, ok := records[mi.MountPoint]
		if !ok {
			records[mi.MountPoint] = &targetItem{
				target:  mi.MountPoint,
				subpath: subpath,
				count:   1,
			}
		} else {
			if record.subpath != subpath {
				record.inconsistent = true
			}
			record.count++
		}
	}
	var res []*targetItem
	for _, record := range records {
		record.check(ctx, true)
		res = append(res, record)
	}
	return res
}

const (
	targetStatusNotExist = iota
	targetStatusMounted
	targetStatusNotMount
	targetStatusCorrupt
	targetStatusUnexpect
)

/*
when target is not subPath: /var/lib/kubelet/pods/<pod-id>/volumes/kubernetes.io~csi/<pv-volumeHandle>/mount

	item in mountinfo: 3952 3905 0:864 / /var/lib/kubelet/pods/<pod-id>/volumes/kubernetes.io~csi/<pv-volumeHandle>/mount rw,nosuid,nodev,relatime shared:449 - fuse.juicefs JuiceFS:xxx rw
	targetItem: {
		target: /var/lib/kubelet/pods/<pod-id>/volumes/kubernetes.io~csi/<pv-volumeHandle>/mount
		subpath: /
	}
	mis: {
		MountPoint: /var/lib/kubelet/pods/<pod-id>/volumes/kubernetes.io~csi/<pv-volumeHandle>/mount
		Root: /
	}

when target is subPath: /var/lib/kubelet/pods/<pod-id>/volume-subpaths/<pv-volumeHandle>/tools/0

	item in mountinfo: 3955 3905 0:864 /abc /var/lib/kubelet/pods/<pod-id>/volume-subpaths/<pv-volumeHandle>/tools/0 rw,relatime shared:449 - fuse.juicefs JuiceFS:test-fluid-2 rw,user_id=0,group_id=0,default_permissions,allow_other
	targetItem: {
		target: /var/lib/kubelet/pods/<pod-id>/volume-subpaths/<pv-volumeHandle>/tools/0
		subpath: abc
	}
	mis: {
		MountPoint: /var/lib/kubelet/pods/<pod-id>/volume-subpaths/<pv-volumeHandle>/tools/0
		Root: /abc
	}
*/
type targetItem struct {
	target       string
	subpath      string
	count        int
	inconsistent bool
	status       int
	err          error
}

func (ti *targetItem) check(ctx context.Context, mounted bool) {
	err := util.DoWithTimeout(ctx, defaultCheckoutTimeout, func(ctx context.Context) error {
		_, err := os.Stat(ti.target)
		return err
	})
	if err == nil {
		if mounted {
			// target exist and is mounted
			// most likely happen
			ti.status = targetStatusMounted
		} else {
			// target exist but is a normal directory, not mounted
			ti.status = targetStatusNotMount
		}
	} else if os.IsNotExist(err) {
		ti.status = targetStatusNotExist
	} else {
		if !mounted {
			ti.status = targetStatusUnexpect
			ti.err = err
		}

		if err.Error() == "function timeout" {
			ti.status = targetStatusCorrupt
			return
		}
		corrupted := k8sMount.IsCorruptedMnt(err)
		if corrupted {
			ti.status = targetStatusCorrupt
		} else {
			ti.status = targetStatusUnexpect
			ti.err = err
		}
	}
}

type mountItem struct {
	podExist      bool
	podDeleted    bool
	baseTarget    *targetItem   // target path in container which is bind to mount point
	subPathTarget []*targetItem // subPath of target path in container which is .spec.container[].volumeMount.subPath
}

func getPodUid(target string) string {
	pair := strings.Split(target, containerCsiDirectory)
	if len(pair) != 2 {
		return ""
	}

	podDir := strings.TrimSuffix(pair[0], "/")
	index := strings.LastIndex(podDir, "/")
	if index <= 0 {
		return ""
	}
	return podDir[index+1:]
}

func getPVName(target string) string {
	pair := strings.Split(target, containerCsiDirectory)
	if len(pair) != 2 {
		return ""
	}

	pvName := strings.TrimPrefix(pair[1], "/")
	index := strings.Index(pvName, "/")
	if index <= 0 {
		return ""
	}
	return pvName[:index]
}
