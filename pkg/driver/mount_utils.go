/*
Copyright 2014 The Kubernetes Authors.

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

// copied from k8s.io/kubernetes/pkg/util/mount
// and did little changes to meet my needs

package driver

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	utilio "k8s.io/kubernetes/pkg/util/io"
)

const (
	maxListTries                         = 3
	expectedAtLeastNumFieldsPerMountInfo = 10
	procMountInfoPath                    = "/proc/self/mountinfo"
)

type mountInfo struct {
	// Unique ID for the mount (maybe reused after umount).
	id int
	// The ID of the parent mount (or of self for the root of this mount namespace's mount tree).
	parentID int
	// The value of `st_dev` for files on this filesystem.
	majorMinor string
	// The pathname of the directory in the filesystem which forms the root of this mount.
	root string
	// Mount source, filesystem-specific information. e.g. device, tmpfs name.
	source string
	// Mount point, the pathname of the mount point.
	mountPoint string
	// Optional fieds, zero or more fields of the form "tag[:value]".
	optionalFields []string
	// The filesystem type in the form "type[.subtype]".
	fsType string
	// Per-mount options.
	mountOptions []string
	// Per-superblock options.
	superOptions []string
}

func parseMountInfo(filename string) ([]mountInfo, error) {
	content, err := utilio.ConsistentRead(filename, maxListTries)
	if err != nil {
		return []mountInfo{}, err
	}
	contentStr := string(content)
	infos := []mountInfo{}

	for _, line := range strings.Split(contentStr, "\n") {
		if line == "" {
			// the last split() item is empty string following the last \n
			continue
		}
		// See `man proc` for authoritative description of format of the file.
		fields := strings.Fields(line)
		if len(fields) < expectedAtLeastNumFieldsPerMountInfo {
			return nil, fmt.Errorf("wrong number of fields in (expected at least %d, got %d): %s", expectedAtLeastNumFieldsPerMountInfo, len(fields), line)
		}
		id, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, err
		}
		parentID, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, err
		}
		info := mountInfo{
			id:           id,
			parentID:     parentID,
			majorMinor:   fields[2],
			root:         fields[3],
			mountPoint:   fields[4],
			mountOptions: strings.Split(fields[5], ","),
		}
		// All fields until "-" are "optional fields".
		i := 6
		for ; i < len(fields) && fields[i] != "-"; i++ {
			info.optionalFields = append(info.optionalFields, fields[i])
		}
		// Parse the rest 3 fields.
		i += 1
		if len(fields)-i < 3 {
			return nil, fmt.Errorf("expect 3 fields in %s, got %d", line, len(fields)-i)
		}
		info.fsType = fields[i]
		info.source = fields[i+1]
		info.superOptions = strings.Split(fields[i+2], ",")
		infos = append(infos, info)
	}
	return infos, nil
}

func startsWithBackstep(rel string) bool {
	// normalize to / and check for ../
	return rel == ".." || strings.HasPrefix(filepath.ToSlash(rel), "../")
}

func pathWithinBase(fullPath, basePath string) bool {
	rel, err := filepath.Rel(basePath, fullPath)
	if err != nil {
		return false
	}
	return !startsWithBackstep(rel)
}

func searchMountPoints(hostSource, mountInfoPath string) ([]string, error) {
	mis, err := parseMountInfo(mountInfoPath)
	if err != nil {
		return nil, err
	}

	var (
		mountID                      int
		rootPath, majorMinor, fsType string
	)
	// Finding the underlying root path and major:minor if possible.
	// We need search in backward order because it's possible for later mounts
	// to overlap earlier mounts.
	for i := len(mis) - 1; i >= 0; i-- {
		if hostSource == mis[i].mountPoint || pathWithinBase(hostSource, mis[i].mountPoint) {
			// If it's a mount point or path under a mount point.
			mountID = mis[i].id
			rootPath = filepath.Join(mis[i].root, strings.TrimPrefix(hostSource, mis[i].mountPoint))
			majorMinor = mis[i].majorMinor
			fsType = mis[i].fsType
			break
		}
	}

	if rootPath == "" || majorMinor == "" {
		return nil, fmt.Errorf("failed to get root path and major:minor for %s", hostSource)
	}

	var refs []string
	for _, mi := range mis {
		if mi.id == mountID {
			// Ignore mount entry for mount source itself.
			continue
		}
		if mi.majorMinor == majorMinor && mi.fsType == fsType {
			// NOTE: CAN ONLY BE USED HERE!!!
			// add all the same sources
			refs = append(refs, mi.mountPoint)
		}
	}
	return refs, nil
}

// Get all mountpoints whose source is the device of `pathname` mountpoint,
// the `pathname` will be excluded.
// The `pathname` must be a mountpoint, and if the `corrupted` is true,
// the `pathname` is a corrupted mountpoint.
func getMountDeviceRefs(pathname string, corrupted bool) ([]string, error) {
	var realpath string
	var err error

	if corrupted { // Corrupted mountpoint will fail in Lstat which is used by filepath.EvalSymlinks()
		pathname = strings.TrimSuffix(pathname, string(filepath.Separator))
		realpath, err = filepath.EvalSymlinks(filepath.Dir(pathname))
		if err != nil {
			return nil, err
		}
		realpath = filepath.Join(realpath, filepath.Base(pathname))
	} else if realpath, err = filepath.EvalSymlinks(pathname); err != nil {
		return nil, err
	}
	return searchMountPoints(realpath, procMountInfoPath)
}
