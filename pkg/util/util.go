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
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/io"
)

const (
	maxListTries                         = 3
	expectedAtLeastNumFieldsPerMountInfo = 10
	procMountInfoPath                    = "/proc/self/mountinfo"
)

var (
	utilLog = klog.NewKlogr().WithName("util")
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
	content, err := io.ConsistentRead(filename, maxListTries)
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

func ContainSubString(slice []string, s string) bool {
	for _, item := range slice {
		if strings.Contains(item, s) {
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range b {
		b[i] = letterRunes[r.Intn(len(letterRunes))]
	}
	return string(b)
}

func DoWithContext(ctx context.Context, f func() error) error {
	doneCh := make(chan error, 1)
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
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- f()
	}()

	select {
	case <-parent.Done():
		return parent.Err()
	case <-timer.C:
		return errors.New("function timeout")
	case err := <-doneCh:
		return err
	}
}

func CheckDynamicPV(name string) (bool, error) {
	return regexp.Match("pvc-\\w{8}(-\\w{4}){3}-\\w{12}", []byte(name))
}

func UmountPath(ctx context.Context, sourcePath string) {
	log := GenLog(ctx, utilLog, "Umount")
	out, err := exec.CommandContext(ctx, "umount", "-l", sourcePath).CombinedOutput()
	if err != nil &&
		!strings.Contains(string(out), "not mounted") &&
		!strings.Contains(string(out), "mountpoint not found") &&
		!strings.Contains(string(out), "no mount point specified") {
		log.Error(err, "Could not lazy unmount", "path", sourcePath, "out", string(out))
	}
}

func GetMountPathOfPod(pod corev1.Pod) (string, string, error) {
	if len(pod.Spec.Containers) == 0 {
		return "", "", fmt.Errorf("pod %v has no container", pod.Name)
	}
	cmd := pod.Spec.Containers[0].Command
	if cmd == nil || len(cmd) < 3 {
		return "", "", fmt.Errorf("get error pod command:%v", cmd)
	}
	sourcePath, volumeId, err := parseMntPath(cmd[2])
	if err != nil {
		return "", "", err
	}
	return sourcePath, volumeId, nil
}

// parseMntPath return mntPath, volumeId (/jfs/volumeId, volumeId err)
func parseMntPath(cmd string) (string, string, error) {
	cmds := strings.Split(cmd, "\n")
	mountCmd := cmds[len(cmds)-1]
	args := strings.Fields(mountCmd)
	if args[0] == "exec" {
		args = args[1:]
	}
	if len(args) < 3 || (!strings.HasPrefix(args[2], "/jfs") && !strings.HasPrefix(args[2], "/mnt/jfs")) {
		return "", "", fmt.Errorf("err cmd:%s", cmd)
	}
	argSlice := strings.Split(args[2], "/")
	if len(argSlice) < 3 {
		return "", "", fmt.Errorf("err mntPath:%s", args[2])
	}
	return args[2], argSlice[2], nil
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
		utilLog.Error(err, "GetDiskUsage: syscall.Statfs failed")
		return 1, 1, 1, 1
	}
}

func NewPrometheus(nodeName string) (prometheus.Registerer, *prometheus.Registry) {
	registry := prometheus.NewRegistry() // replace default so only JuiceFS metrics are exposed
	registerer := prometheus.WrapRegistererWithPrefix("juicefs_", prometheus.WrapRegistererWith(prometheus.Labels{"node_name": nodeName}, registry))
	return registerer, registry
}

// ParseToBytes parses a string with a unit suffix (e.g. "1M", "2G") to bytes.
// default unit is M
func ParseToBytes(value string) (uint64, error) {
	if len(value) == 0 {
		return 0, nil
	}
	s := value
	unit := byte('M')
	if c := s[len(s)-1]; c < '0' || c > '9' {
		unit = c
		s = s[:len(s)-1]
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %s to bytes", value)
	}
	var shift int
	switch unit {
	case 'k', 'K':
		shift = 10
	case 'm', 'M':
		shift = 20
	case 'g', 'G':
		shift = 30
	case 't', 'T':
		shift = 40
	case 'p', 'P':
		shift = 50
	case 'e', 'E':
		shift = 60
	default:
		return 0, fmt.Errorf("cannot parse %s to bytes, invalid unit", value)
	}
	val *= float64(uint64(1) << shift)

	return uint64(val), nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err) //skip mutate
}

type ClientVersion struct {
	IsCe  bool
	Dev   bool
	Major int
	Minor int
	Patch int
}

const ceImageRegex = `ce-v(\d+)\.(\d+)\.(\d+)`
const eeImageRegex = `ee-(\d+)\.(\d+)\.(\d+)`
const ceVersionRegex = `version (\d+)\.(\d+)\.(\d+)+`
const ceDevVersionRegex = `version (\d+)\.(\d+)\.(\d+)-dev`
const eeVersionRegex = `version (\d+)\.(\d+)\.(\d+) `

func (v ClientVersion) LessThan(o ClientVersion) bool {
	if o.Dev {
		// dev version is always greater
		return true
	}
	if v.Dev {
		return false
	}
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

func parseClientVersionFromImage(image string) ClientVersion {
	if image == "" {
		return ClientVersion{}
	}
	imageSplits := strings.SplitN(image, ":", 2)
	if len(imageSplits) < 2 {
		// latest
		return ClientVersion{IsCe: true, Major: math.MaxInt32}
	}
	_, tag := imageSplits[0], imageSplits[1]
	version := ClientVersion{Dev: true}
	var re *regexp.Regexp

	if strings.HasPrefix(tag, "ce-") {
		version.IsCe = true
		re = regexp.MustCompile(ceImageRegex)
	} else if strings.HasPrefix(tag, "ee-") {
		version.IsCe = false
		re = regexp.MustCompile(eeImageRegex)
	}

	if re != nil {
		matches := re.FindStringSubmatch(tag)
		if len(matches) == 4 {
			version.Major, _ = strconv.Atoi(matches[1])
			version.Minor, _ = strconv.Atoi(matches[2])
			version.Patch, _ = strconv.Atoi(matches[3])
			version.Dev = false
		}
	}

	return version
}

func parseClientVersion(ce bool, version string) ClientVersion {
	v := ClientVersion{IsCe: ce}
	var re *regexp.Regexp
	if !ce {
		re = regexp.MustCompile(eeVersionRegex)
	} else {
		if strings.Contains(version, "dev") {
			re = regexp.MustCompile(ceDevVersionRegex)
			v.Dev = true
		} else {
			re = regexp.MustCompile(ceVersionRegex)
		}
	}

	matches := re.FindStringSubmatch(version)
	if len(matches) == 4 {
		v.Major, _ = strconv.Atoi(matches[1])
		v.Minor, _ = strconv.Atoi(matches[2])
		v.Patch, _ = strconv.Atoi(matches[3])
	}
	return v
}

func SupportUpgradeRecreate(ce bool, version string) bool {
	v := parseClientVersion(ce, version)
	return supportFusePass(v)
}

func SupportUpgradeBinary(ce bool, version string) bool {
	v := parseClientVersion(ce, version)
	return supportUpgradeBinary(v)
}

func SupportFusePass(image string) bool {
	v := parseClientVersionFromImage(image)
	if v.Dev {
		return false
	}
	return supportFusePass(v)
}

func ImageSupportBinary(image string) bool {
	v := parseClientVersionFromImage(image)
	if v.Dev {
		return false
	}
	return supportUpgradeBinary(v)
}

func supportFusePass(v ClientVersion) bool {
	ceFuseVersion := ClientVersion{
		IsCe:  true,
		Dev:   false,
		Major: 1,
		Minor: 2,
		Patch: 1,
	}
	eeFuseVersion := ClientVersion{
		IsCe:  false,
		Dev:   false,
		Major: 5,
		Minor: 1,
		Patch: 0,
	}
	if v.IsCe {
		return !v.LessThan(ceFuseVersion)
	}
	return !v.LessThan(eeFuseVersion)
}

func supportUpgradeBinary(v ClientVersion) bool {
	ceFuseVersion := ClientVersion{
		IsCe:  true,
		Dev:   false,
		Major: 1,
		Minor: 2,
		Patch: 0,
	}
	eeFuseVersion := ClientVersion{
		IsCe:  false,
		Dev:   false,
		Major: 5,
		Minor: 0,
		Patch: 0,
	}
	if v.IsCe {
		return !v.LessThan(ceFuseVersion)
	}
	return !v.LessThan(eeFuseVersion)
}

type JuiceConf struct {
	Meta struct {
		Sid uint64
	}
	Pid  int
	PPid int
}

func ParseConfig(conf []byte) (*JuiceConf, error) {
	var juiceConf JuiceConf
	err := json.Unmarshal(conf, &juiceConf)
	if err != nil {
		klog.Errorf("ParseConfig: %v", err)
		return nil, err
	}
	return &juiceConf, nil
}

func ContainsEnv(envs []corev1.EnvVar, key string) bool {
	for _, env := range envs {
		if env.Name == key {
			return true
		}
	}
	return false
}

func ContainsVolumes(volumes []corev1.Volume, name string) bool {
	for _, volume := range volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}
func ContainsVolumeDevices(vds []corev1.VolumeDevice, name string) bool {
	for _, vd := range vds {
		if vd.Name == name {
			return true
		}
	}
	return false
}
func ContainsVolumeMounts(vms []corev1.VolumeMount, name string) bool {
	for _, vm := range vms {
		if vm.Name == name {
			return true
		}
	}
	return false
}

func GetMountOptionsOfPod(pod *corev1.Pod) []string {
	if len(pod.Spec.Containers) == 0 {
		return nil
	}
	cmd := pod.Spec.Containers[0].Command
	if cmd == nil || len(cmd) < 3 {
		return nil
	}
	mountCmds := strings.Fields(cmd[2])
	// not valid cmd
	if len(mountCmds) < 3 {
		return nil
	}
	// not valid cmd
	if mountCmds[len(mountCmds)-2] != "-o" {
		return nil
	}
	return strings.Split(mountCmds[len(mountCmds)-1], ",")
}

func ToPtr[T any](v T) *T {
	return &v
}

func CpNotNil[T any](s, d *T) *T {
	if s != nil {
		return s
	}
	return d
}

func CpNotEmpty(s, d string) string {
	if s != "" {
		return s
	}
	return d
}

func SortBy[T any](slice []T, less func(i, j int) bool) {
	if slice == nil {
		return
	}
	sort.Slice(slice, less)
}
