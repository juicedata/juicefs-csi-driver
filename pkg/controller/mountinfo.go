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
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8sMount "k8s.io/utils/mount"
)

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

func (mit *mountInfoTable) setPodsStatus(podList *corev1.PodList) {
	mit.deletedPods = make(map[string]bool)
	if podList == nil {
		return
	}
	for _, pod := range podList.Items {
		deleted := false
		if pod.DeletionTimestamp != nil {
			deleted = true
		}
		mit.deletedPods[string(pod.UID)] = deleted
	}
}

const (
	// place for subpath mounts
	containerSubPathDirectory = "volume-subpaths"
	// place for csi mounts
	containerCsiDirectory = "volumes/kubernetes.io~csi"
)

// return nil if not a valid csi target path
func (mit *mountInfoTable) resolveTarget(target string) *mountItem {
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

	iterms := mit.resolveTargetItem(target, false)
	// must be 1 or 0
	if len(iterms) == 1 {
		mi.baseTarget = iterms[0]
	} else {
		mi.baseTarget = &targetItem{
			target: target,
		}
		mi.baseTarget.check(false)
	}
	subpathTargetPrefix := strings.Join([]string{
		podDir,
		containerSubPathDirectory,
		pvName,
	}, "/")
	mi.subPathTarget = mit.resolveTargetItem(subpathTargetPrefix, true)

	return mi
}

func (mit *mountInfoTable) resolveTargetItem(path string, isPrefix bool) []*targetItem {
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
		record.check(true)
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

type targetItem struct {
	target       string
	subpath      string
	count        int
	inconsistent bool
	status       int
	err          error
}

func (ti *targetItem) check(mounted bool) {
	_, err := os.Stat(ti.target)
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
	baseTarget    *targetItem
	subPathTarget []*targetItem
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
