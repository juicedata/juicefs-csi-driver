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

package juicefs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	podmount "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

const (
	defaultCheckTimeout = 2 * time.Second
	fsTypeNone          = "none"
	procMountInfoPath   = "/proc/self/mountinfo"
)

// Interface of juicefs provider
type Interface interface {
	mount.Interface
	JfsMount(ctx context.Context, volumeID string, target string, secrets, volCtx map[string]string, options []string) (Jfs, error)
	JfsCreateVol(ctx context.Context, volumeID string, subPath string, secrets, volCtx map[string]string) error
	JfsDeleteVol(ctx context.Context, volumeID string, target string, secrets, volCtx map[string]string) error
	JfsUnmount(ctx context.Context, volumeID, mountPath string) error
	JfsCleanupMountPoint(ctx context.Context, mountPath string) error
	GetJfsVolUUID(ctx context.Context, name string) (string, error)
}

type juicefs struct {
	sync.Mutex
	mount.SafeFormatAndMount
	*k8sclient.K8sClient

	podMount     podmount.MntInterface
	processMount podmount.MntInterface
	UUIDMaps     map[string]string
	CacheDirMaps map[string][]string
}

var _ Interface = &juicefs{}

type jfs struct {
	Provider  *juicefs
	Name      string
	MountPath string
	Options   []string
}

// Jfs is the interface of a mounted file system
type Jfs interface {
	GetBasePath() string
	CreateVol(ctx context.Context, volumeID, subPath string) (string, error)
	BindTarget(ctx context.Context, bindSource, target string) error
}

var _ Jfs = &jfs{}

func (fs *jfs) GetBasePath() string {
	return fs.MountPath
}

// CreateVol creates the directory needed
func (fs *jfs) CreateVol(ctx context.Context, volumeID, subPath string) (string, error) {
	volPath := filepath.Join(fs.MountPath, subPath)
	klog.V(6).Infof("CreateVol: checking %q exists in %v", volPath, fs)
	var exists bool
	if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func() (err error) {
		exists, err = mount.PathExists(volPath)
		return
	}); err != nil {
		return "", fmt.Errorf("could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		klog.V(5).Infof("CreateVol: volume not existed")
		if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func() (err error) {
			return os.MkdirAll(volPath, os.FileMode(0777))
		}); err != nil {
			return "", fmt.Errorf("could not make directory for meta %q: %v", volPath, err)
		}
		var fi os.FileInfo
		if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func() (err error) {
			fi, err = os.Stat(volPath)
			return err
		}); err != nil {
			return "", fmt.Errorf("could not stat directory %s: %q", volPath, err)
		} else if fi.Mode().Perm() != 0777 { // The perm of `volPath` may not be 0777 when the umask applied
			if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func() (err error) {
				return os.Chmod(volPath, os.FileMode(0777))
			}); err != nil {
				return "", fmt.Errorf("could not chmod directory %s: %q", volPath, err)
			}
		}
	}

	return volPath, nil
}

func (fs *jfs) BindTarget(ctx context.Context, bindSource, target string) error {
	mountInfos, err := mount.ParseMountInfo(procMountInfoPath)
	if err != nil {
		return err
	}
	var mountMinor, targetMinor *int
	for _, mi := range mountInfos {
		if mi.MountPoint == fs.MountPath {
			minor := mi.Minor
			mountMinor = &minor
		}
		if mi.MountPoint == target {
			targetMinor = &mi.Minor
		}
	}
	if mountMinor == nil {
		return fmt.Errorf("BindTarget: mountPath %s not mounted", fs.MountPath)
	}
	if targetMinor != nil {
		if *targetMinor == *mountMinor {
			// target already binded mountpath
			klog.V(6).Infof("BindTarget: target %s already bind mount to %s", target, fs.MountPath)
			return nil
		}
		// target is bind by other path, umount it
		klog.Infof("BindTarget: target %s bind mount to other path, umount it", target)
		util.UmountPath(ctx, target)
	}
	// bind target to mountpath
	klog.Infof("BindTarget: binding %s at %s", bindSource, target)
	if err := fs.Provider.Mount(bindSource, target, fsTypeNone, []string{"bind"}); err != nil {
		os.Remove(target)
		return err
	}
	return nil
}

// NewJfsProvider creates a provider for JuiceFS file system
func NewJfsProvider(mounter *mount.SafeFormatAndMount, k8sClient *k8sclient.K8sClient) Interface {
	if mounter == nil {
		mounter = &mount.SafeFormatAndMount{
			Interface: mount.New(""),
			Exec:      k8sexec.New(),
		}
	}
	processMnt := podmount.NewProcessMount(*mounter)
	podMnt := podmount.NewPodMount(k8sClient, *mounter)

	uuidMaps := make(map[string]string)
	cacheDirMaps := make(map[string][]string)
	return &juicefs{
		Mutex:              sync.Mutex{},
		SafeFormatAndMount: *mounter,
		K8sClient:          k8sClient,
		podMount:           podMnt,
		processMount:       processMnt,
		UUIDMaps:           uuidMaps,
		CacheDirMaps:       cacheDirMaps,
	}
}

func (j *juicefs) JfsCreateVol(ctx context.Context, volumeID string, subPath string, secrets, volCtx map[string]string) error {
	jfsSetting, err := j.getSettings(ctx, volumeID, "", secrets, volCtx, []string{})
	if err != nil {
		return err
	}
	jfsSetting.SubPath = subPath
	jfsSetting.MountPath = filepath.Join(config.TmpPodMountBase, jfsSetting.VolumeId)
	if config.FormatInPod {
		return j.podMount.JCreateVolume(ctx, jfsSetting)
	}
	return j.processMount.JCreateVolume(ctx, jfsSetting)
}

func (j *juicefs) JfsDeleteVol(ctx context.Context, volumeID string, subPath string, secrets, volCtx map[string]string) error {
	jfsSetting, err := j.getSettings(ctx, volumeID, "", secrets, volCtx, []string{})
	if err != nil {
		return err
	}
	jfsSetting.SubPath = subPath
	jfsSetting.MountPath = filepath.Join(config.TmpPodMountBase, jfsSetting.VolumeId)

	mnt := j.processMount
	if config.FormatInPod {
		mnt = j.podMount
	}
	if err := mnt.JDeleteVolume(ctx, jfsSetting); err != nil {
		return err
	}
	return j.JfsCleanupMountPoint(ctx, jfsSetting.MountPath)
}

func (j *juicefs) JfsMount(ctx context.Context, volumeID string, target string, secrets, volCtx map[string]string, options []string) (Jfs, error) {
	if err := j.validTarget(target); err != nil {
		return nil, err
	}
	if err := j.validOptions(volumeID, options); err != nil {
		return nil, err
	}
	jfsSetting, err := j.getSettings(ctx, volumeID, target, secrets, volCtx, options)
	if err != nil {
		return nil, err
	}
	mountPath, err := j.MountFs(ctx, jfsSetting)
	if err != nil {
		return nil, err
	}

	return &jfs{
		Provider:  j,
		Name:      secrets["name"],
		MountPath: mountPath,
		Options:   options,
	}, nil
}

// JfsMount auths and mounts JuiceFS
func (j *juicefs) getSettings(ctx context.Context, volumeID string, target string, secrets, volCtx map[string]string, options []string) (*config.JfsSetting, error) {
	jfsSetting, err := config.ParseSetting(secrets, volCtx, options, !config.ByProcess)
	if err != nil {
		klog.V(5).Infof("Parse config error: %v", err)
		return nil, err
	}
	jfsSetting.VolumeId = volumeID
	jfsSetting.TargetPath = target
	if !jfsSetting.IsCe {
		if secrets["token"] == "" {
			klog.V(5).Infof("token is empty, skip authfs.")
		} else {
			res, err := j.AuthFs(ctx, secrets, jfsSetting)
			if err != nil {
				return nil, fmt.Errorf("juicefs auth error: %v", err)
			}
			if config.FormatInPod {
				jfsSetting.FormatCmd = res
			}
		}
		jfsSetting.UUID = secrets["name"]
	} else {
		noUpdate := false
		if secrets["storage"] == "" || secrets["bucket"] == "" {
			klog.V(5).Infof("JfsMount: storage or bucket is empty, format --no-update.")
			noUpdate = true
		}
		if config.FormatInPod && (secrets["storage"] == "ceph" || secrets["storage"] == "gs") {
			jfsSetting.Envs["JFS_NO_CHECK_OBJECT_STORAGE"] = "1"
		}
		res, err := j.ceFormat(ctx, secrets, noUpdate, jfsSetting)
		if err != nil {
			return nil, fmt.Errorf("juicefs format error: %v", err)
		}
		if config.FormatInPod {
			jfsSetting.FormatCmd = res
		}
	}

	uniqueId, err := j.getUniqueId(ctx, volumeID)
	if err != nil {
		klog.Errorf("Get volume name by volume id %s error: %v", volumeID, err)
		return nil, err
	}
	klog.V(6).Infof("Get uniqueId of volume [%s]: %s", volumeID, uniqueId)
	jfsSetting.UniqueId = uniqueId
	if jfsSetting.CleanCache {
		uuid := jfsSetting.Name
		if jfsSetting.IsCe {
			if uuid, err = j.GetJfsVolUUID(ctx, jfsSetting.Source); err != nil {
				return nil, err
			}
		}
		jfsSetting.UUID = uuid
		if config.ByProcess {
			j.Lock()
			j.UUIDMaps[uniqueId] = uuid
			j.CacheDirMaps[uniqueId] = jfsSetting.CacheDirs
			j.Unlock()
		}
		klog.V(6).Infof("Get uuid of volume [%s]: %s", volumeID, uuid)
	}
	return jfsSetting, nil
}

// getUniqueId: get UniqueId from volumeId (volumeHandle of PV)
// When STORAGE_CLASS_SHARE_MOUNT env is set:
//
//	in dynamic provision, UniqueId set as SC name
//	in static provision, UniqueId set as volumeId
//
// When STORAGE_CLASS_SHARE_MOUNT env not set:
//
//	UniqueId set as volumeId
func (j *juicefs) getUniqueId(ctx context.Context, volumeId string) (string, error) {
	if os.Getenv("STORAGE_CLASS_SHARE_MOUNT") == "true" && !config.ByProcess {
		pv, err := j.K8sClient.GetPersistentVolume(ctx, volumeId)
		// In static provision, volumeId may not be PV name, it is expected that PV cannot be found by volumeId
		if err != nil && !k8serrors.IsNotFound(err) {
			return "", err
		}
		// In dynamic provision, PV.spec.StorageClassName is which SC(StorageClass) it belongs to.
		if err == nil && pv.Spec.StorageClassName != "" {
			return pv.Spec.StorageClassName, nil
		}
	}
	return volumeId, nil
}

// GetJfsVolUUID get UUID from result of `juicefs status <volumeName>`
func (j *juicefs) GetJfsVolUUID(ctx context.Context, name string) (string, error) {
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 8*defaultCheckTimeout)
	defer cmdCancel()
	stdout, err := j.Exec.CommandContext(cmdCtx, config.CeCliPath, "status", name).CombinedOutput()
	if err != nil {
		re := string(stdout)
		klog.Infof("juicefs status error: %v, output: '%s'", err, re)
		if cmdCtx.Err() == context.DeadlineExceeded {
			re = fmt.Sprintf("juicefs status %s timed out", 8*defaultCheckTimeout)
			return "", errors.New(re)
		}
		return "", errors.Wrap(err, re)
	}

	matchExp := regexp.MustCompile(`"UUID": "(.*)"`)
	idStr := matchExp.FindString(string(stdout))
	idStrs := strings.Split(idStr, "\"")
	if len(idStrs) < 4 {
		return "", fmt.Errorf("get uuid of %s error", name)
	}

	return idStrs[3], nil
}

func (j *juicefs) validTarget(target string) error {
	if config.ByProcess {
		// do not check target when by process, because it may not in kubernetes
		return nil
	}
	kubeletDir := "/var/lib/kubelet"
	for _, v := range config.CSIPod.Spec.Volumes {
		if v.Name == "kubelet-dir" {
			kubeletDir = v.HostPath.Path
			break
		}
	}
	dirs := strings.Split(target, "/pods/")
	if len(dirs) == 0 {
		return fmt.Errorf("can't parse kubelet rootdir from target %s", target)
	}
	if kubeletDir != dirs[0] {
		return fmt.Errorf("target kubelet rootdir %s is not equal csi mounted kubelet root-dir %s", dirs[0], kubeletDir)
	}
	return nil
}

func (j *juicefs) validOptions(volumeId string, options []string) error {
	for _, option := range options {
		if option == "writeback" {
			klog.Warningf("writeback is not suitable in CSI, please do not use it. volumeId: %s", volumeId)
		}
	}
	return nil
}

func (j *juicefs) JfsUnmount(ctx context.Context, volumeId, mountPath string) error {
	uniqueId, err := j.getUniqueId(ctx, volumeId)
	if err != nil {
		klog.Errorf("Get volume name by volume id %s error: %v", volumeId, err)
		return err
	}
	if config.ByProcess {
		ref, err := j.processMount.GetMountRef(ctx, mountPath, "")
		if err != nil {
			klog.Errorf("Get mount ref error: %v", err)
		}
		err = j.processMount.JUmount(ctx, mountPath, "")
		if err != nil {
			klog.Errorf("Get mount ref error: %v", err)
		}
		if ref == 1 {
			func() {
				j.Lock()
				defer j.Unlock()
				uuid := j.UUIDMaps[uniqueId]
				cacheDirs := j.CacheDirMaps[uniqueId]
				if uuid == "" && len(cacheDirs) == 0 {
					klog.Infof("Can't get uuid and cacheDirs of %s. skip cache clean.", uniqueId)
					return
				}
				delete(j.UUIDMaps, uniqueId)
				delete(j.CacheDirMaps, uniqueId)

				klog.V(5).Infof("Cleanup cache of volume %s in node %s", uniqueId, config.NodeName)
				// clean cache should be done even when top context timeout
				go j.processMount.CleanCache(context.TODO(), uuid, uniqueId, cacheDirs)
			}()
		}
		return err
	}

	mnt := j.podMount
	mountPods := []corev1.Pod{}
	var mountPod *corev1.Pod
	var podName string
	// get pod by exact name
	oldPodName := podmount.GenPodNameByUniqueId(uniqueId, false)
	pod, err := j.K8sClient.GetPod(ctx, oldPodName, config.Namespace)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			klog.Errorf("JfsUnmount: Get mount pod %s err %v", oldPodName, err)
			return err
		}
	}
	if pod != nil {
		mountPods = append(mountPods, *pod)
	}
	// get pod by label
	labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{
		config.PodTypeKey:          config.PodTypeValue,
		config.PodUniqueIdLabelKey: uniqueId,
	}}
	fieldSelector := &fields.Set{"spec.nodeName": config.NodeName}
	pods, err := j.K8sClient.ListPod(ctx, config.Namespace, labelSelector, fieldSelector)
	if err != nil {
		klog.Errorf("List pods of uniqueId %s error: %v", uniqueId, err)
		return err
	}
	mountPods = append(mountPods, pods...)
	// find pod by target
	key := util.GetReferenceKey(mountPath)
	for _, po := range mountPods {
		if _, ok := po.Annotations[key]; ok {
			mountPod = &po
			break
		}
	}
	if mountPod != nil {
		podName = mountPod.Name
	}

	lock := config.GetPodLock(podName)
	lock.Lock()
	defer lock.Unlock()

	// umount target path
	if err = mnt.UmountTarget(ctx, mountPath, podName); err != nil {
		return err
	}
	if podName == "" {
		return nil
	}
	// get refs of mount pod
	refs, err := mnt.GetMountRef(ctx, mountPath, podName)
	if err != nil {
		return err
	}
	if refs == 0 {
		// if refs is none, umount
		return j.podMount.JUmount(ctx, mountPath, podName)
	}
	return nil
}

func (j *juicefs) JfsCleanupMountPoint(ctx context.Context, mountPath string) error {
	klog.V(5).Infof("JfsCleanupMountPoint: clean up mount point: %q", mountPath)
	return util.DoWithTimeout(ctx, 2*defaultCheckTimeout, func() (err error) {
		return mount.CleanupMountPoint(mountPath, j.SafeFormatAndMount.Interface, false)
	})
}

// AuthFs authenticates JuiceFS, enterprise edition only
func (j *juicefs) AuthFs(ctx context.Context, secrets map[string]string, setting *config.JfsSetting) (string, error) {
	if secrets == nil {
		return "", status.Errorf(codes.InvalidArgument, "Nil secrets")
	}

	if secrets["name"] == "" {
		return "", status.Errorf(codes.InvalidArgument, "Empty name")
	}

	args := []string{"auth", secrets["name"]}
	cmdArgs := []string{config.CliPath, "auth", secrets["name"]}

	keysCompatible := map[string]string{
		"access-key":  "accesskey",
		"access-key2": "accesskey2",
		"secret-key":  "secretkey",
		"secret-key2": "secretkey2",
	}
	// compatible
	for compatibleKey, realKey := range keysCompatible {
		if value, ok := secrets[compatibleKey]; ok {
			klog.Infof("transform key [%s] to [%s]", compatibleKey, realKey)
			secrets[realKey] = value
			delete(secrets, compatibleKey)
		}
	}

	keys := []string{
		"accesskey",
		"accesskey2",
		"bucket",
		"bucket2",
		"subdir",
	}
	keysStripped := []string{
		"token",
		"secretkey",
		"secretkey2",
		"passphrase"}
	for _, k := range keys {
		if secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, secrets[k]))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	for _, k := range keysStripped {
		if secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=${%s}", k, k))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	if v, ok := os.LookupEnv("JFS_NO_UPDATE_CONFIG"); ok && v == "enabled" {
		cmdArgs = append(cmdArgs, "--no-update")
		args = append(args, "--no-update")
		if secrets["bucket"] == "" {
			return "", fmt.Errorf("bucket argument is required when --no-update option is provided")
		}
		if !config.FormatInPod && secrets["initconfig"] != "" {
			conf := secrets["name"] + ".conf"
			confPath := filepath.Join("/root/.juicefs", conf)
			if _, err := os.Stat(confPath); os.IsNotExist(err) {
				err = ioutil.WriteFile(confPath, []byte(secrets["initconfig"]), 0644)
				if err != nil {
					return "", fmt.Errorf("create config file %q failed: %v", confPath, err)
				}
				klog.V(5).Infof("Create config file: %q success", confPath)
			}
		}
	}
	if setting.FormatOptions != "" {
		options, err := setting.ParseFormatOptions()
		if err != nil {
			return "", status.Errorf(codes.InvalidArgument, "Parse format options error: %v", err)
		}
		args = append(args, setting.RepresentFormatOptions(options)...)
		stripped := setting.StripFormatOptions(options, []string{"session-token"})
		cmdArgs = append(cmdArgs, stripped...)
	}
	klog.V(5).Infof("AuthFs cmd: %v", cmdArgs)

	if config.FormatInPod {
		cmd := strings.Join(cmdArgs, " ")
		return cmd, nil
	}

	cmdCtx, cmdCancel := context.WithTimeout(ctx, 8*defaultCheckTimeout)
	defer cmdCancel()
	authCmd := j.Exec.CommandContext(cmdCtx, config.CliPath, args...)
	envs := syscall.Environ()
	for key, val := range setting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", key, val))
	}
	authCmd.SetEnv(envs)
	res, err := authCmd.CombinedOutput()
	klog.Infof("Auth output is %s", res)
	if err != nil {
		re := string(res)
		klog.Infof("Auth error: %v", err)
		if cmdCtx.Err() == context.DeadlineExceeded {
			re = fmt.Sprintf("juicefs auth %s timed out", 8*defaultCheckTimeout)
			return "", errors.New(re)
		}
		return "", errors.Wrap(err, re)
	}
	return string(res), nil
}

// MountFs mounts JuiceFS with idempotency
func (j *juicefs) MountFs(ctx context.Context, jfsSetting *config.JfsSetting) (string, error) {
	var mnt podmount.MntInterface
	if jfsSetting.UsePod {
		jfsSetting.MountPath = filepath.Join(config.PodMountBase, jfsSetting.UniqueId)
		mnt = j.podMount
	} else {
		jfsSetting.MountPath = filepath.Join(config.MountBase, jfsSetting.UniqueId)
		mnt = j.processMount
	}

	err := mnt.JMount(ctx, jfsSetting)
	if err != nil {
		return "", err
	}
	klog.V(5).Infof("Mount: mounting %q at %q with options %v", util.StripPasswd(jfsSetting.Source), jfsSetting.MountPath, jfsSetting.Options)
	return jfsSetting.MountPath, nil
}

// Upgrade upgrades binary file in `cliPath` to newest version
func (j *juicefs) Upgrade() {
	if v, ok := os.LookupEnv("JFS_AUTO_UPGRADE"); !ok || v != "enabled" {
		return
	}

	timeout := 10
	if t, ok := os.LookupEnv("JFS_AUTO_UPGRADE_TIMEOUT"); ok {
		if v, err := strconv.Atoi(t); err == nil {
			timeout = v
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	err := exec.CommandContext(ctx, config.CliPath, "version", "-u").Run()
	if ctx.Err() == context.DeadlineExceeded {
		klog.V(5).Infof("Upgrade: did not finish in %v", timeout)
		return
	}

	if err != nil {
		klog.V(5).Infof("Upgrade: err %v", err)
		return
	}

	klog.V(5).Infof("Upgrade: successfully upgraded to newest version")
}

func (j *juicefs) ceFormat(ctx context.Context, secrets map[string]string, noUpdate bool, setting *config.JfsSetting) (string, error) {
	if secrets == nil {
		return "", status.Errorf(codes.InvalidArgument, "Nil secrets")
	}

	if secrets["name"] == "" {
		return "", status.Errorf(codes.InvalidArgument, "Empty name")
	}

	if secrets["metaurl"] == "" {
		return "", status.Errorf(codes.InvalidArgument, "Empty metaurl")
	}

	args := []string{"format"}
	cmdArgs := []string{config.CeCliPath, "format"}
	if noUpdate {
		cmdArgs = append(cmdArgs, "--no-update")
		args = append(args, "--no-update")
	}
	keys := []string{
		"storage",
		"bucket",
		"access-key",
		"block-size",
		"compress",
		"trash-days",
		"capacity",
		"inodes",
		"shards",
	}
	keysStripped := map[string]string{"secret-key": "secretkey"}
	for _, k := range keys {
		if secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, secrets[k]))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	for k, v := range keysStripped {
		if secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=${%s}", k, v))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	cmdArgs = append(cmdArgs, "${metaurl}", secrets["name"])
	args = append(args, secrets["metaurl"], secrets["name"])

	if setting.FormatOptions != "" {
		options, err := setting.ParseFormatOptions()
		if err != nil {
			return "", status.Errorf(codes.InvalidArgument, "Parse format options error: %v", err)
		}
		args = append(args, setting.RepresentFormatOptions(options)...)
		stripped := setting.StripFormatOptions(options, []string{"session-token"})
		cmdArgs = append(cmdArgs, stripped...)
	}

	klog.V(5).Infof("ceFormat cmd: %v", cmdArgs)

	if config.FormatInPod {
		cmd := strings.Join(cmdArgs, " ")
		return cmd, nil
	}

	cmdCtx, cmdCancel := context.WithTimeout(ctx, 8*defaultCheckTimeout)
	defer cmdCancel()
	formatCmd := j.Exec.CommandContext(cmdCtx, config.CeCliPath, args...)
	envs := syscall.Environ()
	for key, val := range setting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", key, val))
	}
	if secrets["storage"] == "ceph" || secrets["storage"] == "gs" {
		envs = append(envs, "JFS_NO_CHECK_OBJECT_STORAGE=1")
	}
	formatCmd.SetEnv(envs)
	res, err := formatCmd.CombinedOutput()
	klog.Infof("Format output is %s", res)
	if err != nil {
		re := string(res)
		klog.Infof("Format error: %v", err)
		if cmdCtx.Err() == context.DeadlineExceeded {
			re = fmt.Sprintf("juicefs format %s timed out", 8*defaultCheckTimeout)
			return "", errors.New(re)
		}
		return "", errors.Wrap(err, re)
	}
	return string(res), nil
}
