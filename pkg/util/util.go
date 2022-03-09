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

package util

import (
	"crypto/sha256"
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	utilio "k8s.io/kubernetes/pkg/util/io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

func ParseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("could not parse endpoint: %v", err)
	}

	addr := path.Join(u.Host, filepath.FromSlash(u.Path))

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "tcp":
	case "unix":
		addr = path.Join("/", addr)
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("could not remove unix domain socket %q: %v", addr, err)
		}
	default:
		return "", "", fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return scheme, addr, nil
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

// GetMountDeviceRefs Get all mountpoints whose source is the device of `pathname` mountpoint,
// the `pathname` will be excluded.
// The `pathname` must be a mountpoint, and if the `corrupted` is true,
// the `pathname` is a corrupted mountpoint.
func GetMountDeviceRefs(pathname string, corrupted bool) ([]string, error) {
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

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func GetReferenceKey(target string) string {
	h := sha256.New()
	h.Write([]byte(target))
	return fmt.Sprintf("juicefs-%x", h.Sum(nil))[:63]
}

// ParseMntPath return mntPath, volumeId (/jfs/volumeId, volumeId err)
func ParseMntPath(cmd string) (string, string, error) {
	args := strings.Fields(cmd)
	if len(args) < 3 || !strings.HasPrefix(args[2], config.PodMountBase) {
		return "", "", fmt.Errorf("err cmd:%s", cmd)
	}
	argSlice := strings.Split(args[2], "/")
	if len(argSlice) < 3 {
		return "", "", fmt.Errorf("err mntPath:%s", args[2])
	}
	return args[2], argSlice[2], nil
}

// GetTimeAfterDelay get time which after delay
func GetTimeAfterDelay(delayStr string) (string, error) {
	delay, err := time.ParseDuration(delayStr)
	if err != nil {
		return "", err
	}
	delayAt := time.Now().Add(delay)
	return delayAt.Format("2006-01-02 15:04:05"), nil
}

func GetTime(str string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", str)
}

func ShouldDelay(pod *corev1.Pod, Client *k8s.K8sClient) (shouldDelay bool, err error) {
	delayStr, delayExist := pod.Annotations[config.DeleteDelayTimeKey]
	if !delayExist {
		// not set delete delay
		return false, nil
	}
	delayAtStr, delayAtExist := pod.Annotations[config.DeleteDelayAtKey]
	if !delayAtExist {
		// need to add delayAt annotation
		d, err := GetTimeAfterDelay(delayStr)
		if err != nil {
			klog.Errorf("delayDelete: can't parse delay time %s: %v", d, err)
			return false, nil
		}
		pod.Annotations[config.DeleteDelayAtKey] = d
		if err := Client.UpdatePod(pod); err != nil {
			klog.Errorf("delayDelete: Update pod %s error: %v", pod.Name, err)
			return true, err
		}
		return true, nil
	}
	delayAt, err := GetTime(delayAtStr)
	if err != nil {
		klog.Errorf("delayDelete: can't parse delayAt %s: %v", delayAtStr, err)
		return false, nil
	}
	return time.Now().Before(delayAt), nil
}

func QuoteForShell(cmd string) string {
	if strings.Contains(cmd, "(") {
		cmd = strings.ReplaceAll(cmd, "(", "\\(")
	}
	if strings.Contains(cmd, ")") {
		cmd = strings.ReplaceAll(cmd, ")", "\\)")
	}
	return cmd
}

func StripPasswd(uri string) string {
	p := strings.Index(uri, "@")
	if p < 0 {
		return uri
	}
	sp := strings.Index(uri, "://")
	cp := strings.Index(uri[sp+3:], ":")
	if cp < 0 || sp+3+cp > p {
		return uri
	}
	return uri[:sp+3+cp] + ":****" + uri[p:]
}
