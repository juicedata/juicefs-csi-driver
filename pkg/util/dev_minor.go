/*
 Copyright 2024 Juicedata Inc

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

package util

import (
	"strings"
	"sync"

	k8sMount "k8s.io/utils/mount"
)

var procSelfMountInfoPath = "/proc/self/mountinfo"

var (
	devMinorCache = sync.Map{}
)

// TODO: save in mountpod annotation,
func SaveFuseDevMinor(podName, mntPath string) {
	devMinor, ok := GetFuseDevMinor(mntPath)
	if !ok {
		return
	}
	devMinorCache.Store(podName, devMinor)
}

func GetSavedFuseDevMinor(podName string) (uint32, bool) {
	v, ok := devMinorCache.Load(podName)
	if !ok {
		return 0, false
	}
	return v.(uint32), true
}

func DeleteFuseDevMinor(podName string) {
	devMinorCache.Delete(podName)
}

func GetFuseDevMinor(mntPath string) (uint32, bool) {
	mis, err := k8sMount.ParseMountInfo(procSelfMountInfoPath)
	if err != nil {
		return 0, false
	}
	for _, mi := range mis {
		if mi.MountPoint == mntPath && (mi.FsType == "fuse" || strings.HasPrefix(mi.FsType, "fuse.")) {
			return uint32(mi.Minor), true
		}
	}
	return 0, false
}
