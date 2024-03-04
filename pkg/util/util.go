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
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	maxListTries                         = 3
	expectedAtLeastNumFieldsPerMountInfo = 10
	procMountInfoPath                    = "/proc/self/mountinfo"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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

// ConsistentRead repeatedly reads a file until it gets the same content twice.
// This is useful when reading files in /proc that are larger than page size
// and kernel may modify them between individual read() syscalls.
func ConsistentRead(filename string, attempts int) ([]byte, error) {
	oldContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	for i := 0; i < attempts; i++ {
		newContent, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		if bytes.Compare(oldContent, newContent) == 0 {
			return newContent, nil
		}
		// Files are different, continue reading
		oldContent = newContent
	}
	return nil, fmt.Errorf("could not get consistent content of %s after %d attempts", filename, attempts)
}

func parseMountInfo(filename string) ([]mountInfo, error) {
	content, err := ConsistentRead(filename, maxListTries)
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

// ContainsString checks if a string is in a string slice.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ContainsPrefix String checks if a string slice contains a string with a given prefix
func ContainsPrefix(slice []string, s string) bool {
	for _, item := range slice {
		if strings.HasPrefix(item, s) {
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
	cmds := strings.Split(cmd, "\n")
	mountCmd := cmds[len(cmds)-1]
	args := strings.Fields(mountCmd)
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

func ShouldDelay(ctx context.Context, pod *corev1.Pod, Client *k8s.K8sClient) (shouldDelay bool, err error) {
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
		addAnnotation := map[string]string{config.DeleteDelayAtKey: d}
		klog.Infof("delayDelete: add annotation %v to pod %s", addAnnotation, pod.Name)
		if err := AddPodAnnotation(ctx, Client, pod, addAnnotation); err != nil {
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

func StripReadonlyOption(options []string) []string {
	news := make([]string, 0)
	for _, option := range options {
		if option != "ro" && option != "read-only" {
			news = append(news, option)
		}
	}
	return news
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

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func DoWithContext(ctx context.Context, f func() error) error {
	doneCh := make(chan error)
	go func() {
		doneCh <- f()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-doneCh:
		return err
	}
}

func DoWithTimeout(parent context.Context, timeout time.Duration, f func() error) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return DoWithContext(ctx, f)
}

func CheckDynamicPV(name string) (bool, error) {
	return regexp.Match("pvc-\\w{8}(-\\w{4}){3}-\\w{12}", []byte(name))
}

func UmountPath(ctx context.Context, sourcePath string) {
	out, err := exec.CommandContext(ctx, "umount", "-l", sourcePath).CombinedOutput()
	if !strings.Contains(string(out), "not mounted") &&
		!strings.Contains(string(out), "mountpoint not found") &&
		!strings.Contains(string(out), "no mount point specified") {
		klog.Errorf("Could not lazy unmount %q: %v, output: %s", sourcePath, err, string(out))
	}
}

// CheckExpectValue Check if the key has the expected value
func CheckExpectValue(m map[string]string, key string, targetValue string) bool {
	if len(m) == 0 {
		return false
	}
	if v, ok := m[key]; ok {
		return v == targetValue
	}
	return false
}

// ImageResol check if image contains CE or EE
// ce image starts with "ce-" (latest image is CE)
// ee image starts with "ee-"
// Compatible with previous images: has both ce and ee
func ImageResol(image string) (hasCE, hasEE bool) {
	images := strings.Split(image, ":")
	if len(images) < 2 {
		// if image has no tag, it is CE
		return true, false
	}
	tag := images[1]
	if tag == "latest" {
		return true, false
	}
	if strings.HasPrefix(tag, "ee-") {
		return false, true
	}
	if strings.HasPrefix(tag, "ce-") {
		return true, false
	}
	return true, true
}

func GetDiskUsage(path string) (uint64, uint64, uint64, uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err == nil {
		// in bytes
		blockSize := uint64(stat.Bsize)
		totalSize := blockSize * stat.Blocks
		freeSize := blockSize * stat.Bfree
		totalFiles := stat.Files
		freeFiles := stat.Ffree
		return totalSize, freeSize, totalFiles, freeFiles
	} else {
		klog.Errorf("GetDiskUsage: syscall.Statfs failed: %v", err)
		return 1, 1, 1, 1
	}
}

func NewPrometheus(nodeName string) (prometheus.Registerer, *prometheus.Registry) {
	registry := prometheus.NewRegistry() // replace default so only JuiceFS metrics are exposed
	registerer := prometheus.WrapRegistererWithPrefix("juicefs_", prometheus.WrapRegistererWith(prometheus.Labels{"node_name": nodeName}, registry))
	return registerer, registry
}
