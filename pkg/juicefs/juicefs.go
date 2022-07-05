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
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	podmount "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
)

const defaultCheckTimeout = 2 * time.Second

// Interface of juicefs provider
type Interface interface {
	mount.Interface
	JfsMount(volumeID string, target string, secrets, volCtx map[string]string, options []string) (Jfs, error)
	JfsCreateVol(volumeID string, subPath string, secrets map[string]string) error
	JfsDeleteVol(volumeID string, target string, secrets map[string]string) error
	JfsUnmount(volumeID, mountPath string) error
	JfsCleanupMountPoint(mountPath string) error
	GetJfsVolUUID(name string) (string, error)
	Version() ([]byte, error)
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
	CreateVol(volumeID, subPath string) (string, error)
}

var _ Jfs = &jfs{}

func (fs *jfs) GetBasePath() string {
	return fs.MountPath
}

// CreateVol creates the directory needed
func (fs *jfs) CreateVol(volumeID, subPath string) (string, error) {
	volPath := filepath.Join(fs.MountPath, subPath)

	klog.V(6).Infof("CreateVol: checking %q exists in %v", volPath, fs)
	var exists bool
	if _, err := util.DoWithinTime(context.TODO(), defaultCheckTimeout, nil, func() (err error) {
		exists, err = mount.PathExists(volPath)
		return
	}); err != nil {
		return "", status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		klog.V(5).Infof("CreateVol: volume not existed")
		if _, err := util.DoWithinTime(context.TODO(), defaultCheckTimeout, nil, func() (err error) {
			return os.MkdirAll(volPath, os.FileMode(0777))
		}); err != nil {
			return "", status.Errorf(codes.Internal, "Could not make directory for meta %q: %v", volPath, err)
		}
		var fi os.FileInfo
		if _, err := util.DoWithinTime(context.TODO(), defaultCheckTimeout, nil, func() (err error) {
			fi, err = os.Stat(volPath)
			return err
		}); err != nil {
			return "", status.Errorf(codes.Internal, "Could not stat directory %s: %q", volPath, err)
		} else if fi.Mode().Perm() != 0777 { // The perm of `volPath` may not be 0777 when the umask applied
			if _, err := util.DoWithinTime(context.TODO(), defaultCheckTimeout, nil, func() (err error) {
				return os.Chmod(volPath, os.FileMode(0777))
			}); err != nil {
				return "", status.Errorf(codes.Internal, "Could not chmod directory %s: %q", volPath, err)
			}
		}
	}

	return volPath, nil
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

func (j *juicefs) JfsCreateVol(volumeID string, subPath string, secrets map[string]string) error {
	jfsSetting, err := j.getSettings(volumeID, "", secrets, nil, []string{})
	if err != nil {
		return err
	}
	jfsSetting.SubPath = subPath
	jfsSetting.MountPath = filepath.Join(config.PodMountBase, jfsSetting.VolumeId)
	if config.FormatInPod {
		return j.podMount.JCreateVolume(jfsSetting)
	}
	return j.processMount.JCreateVolume(jfsSetting)
}

func (j *juicefs) JfsDeleteVol(volumeID string, subPath string, secrets map[string]string) error {
	jfsSetting, err := j.getSettings(volumeID, "", secrets, nil, []string{})
	if err != nil {
		return err
	}
	jfsSetting.SubPath = subPath
	jfsSetting.MountPath = filepath.Join(config.PodMountBase, jfsSetting.VolumeId)

	mnt := j.processMount
	if config.FormatInPod {
		mnt = j.podMount
	}
	if err := mnt.JDeleteVolume(jfsSetting); err != nil {
		return err
	}
	return j.JfsCleanupMountPoint(jfsSetting.MountPath)
}

func (j *juicefs) JfsMount(volumeID string, target string, secrets, volCtx map[string]string, options []string) (Jfs, error) {
	jfsSetting, err := j.getSettings(volumeID, target, secrets, volCtx, options)
	if err != nil {
		return nil, err
	}
	mountPath, err := j.MountFs(jfsSetting)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	return &jfs{
		Provider:  j,
		Name:      secrets["name"],
		MountPath: mountPath,
		Options:   options,
	}, nil
}

// JfsMount auths and mounts JuiceFS
func (j *juicefs) getSettings(volumeID string, target string, secrets, volCtx map[string]string, options []string) (*config.JfsSetting, error) {
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
			res, err := j.AuthFs(secrets, jfsSetting)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Could not auth juicefs: %v", err)
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
		res, err := j.ceFormat(secrets, noUpdate, jfsSetting)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
		if config.FormatInPod {
			jfsSetting.FormatCmd = res
		}
	}

	uniqueId, err := j.getUniqueId(volumeID)
	if err != nil {
		klog.Errorf("Get volume name by volume id %s error: %v", volumeID, err)
		return nil, err
	}
	jfsSetting.UniqueId = uniqueId
	if jfsSetting.CleanCache {
		uuid := jfsSetting.Name
		if jfsSetting.IsCe {
			if uuid, err = j.GetJfsVolUUID(jfsSetting.Source); err != nil {
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
	}
	return jfsSetting, nil
}

// getUniqueId: get UniqueId from volumeId (volumeHandle of PV)
// When STORAGE_CLASS_SHARE_MOUNT env is set:
//		in dynamic provision, UniqueId set as SC name
//		in static provision, UniqueId set as volumeId
// When STORAGE_CLASS_SHARE_MOUNT env not set:
// 		UniqueId set as volumeId
func (j *juicefs) getUniqueId(volumeId string) (string, error) {
	if os.Getenv("STORAGE_CLASS_SHARE_MOUNT") == "true" && !config.ByProcess {
		pv, err := j.K8sClient.GetPersistentVolume(volumeId)
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
func (j *juicefs) GetJfsVolUUID(name string) (string, error) {
	stdout, err := j.Exec.Command(config.CeCliPath, "status", name).CombinedOutput()
	if err != nil {
		klog.Errorf("juicefs status: output is '%s'", stdout)
		return "", err
	}

	matchExp := regexp.MustCompile(`"UUID": "(.*)"`)
	idStr := matchExp.FindString(string(stdout))
	idStrs := strings.Split(idStr, "\"")
	if len(idStrs) < 4 {
		return "", status.Errorf(codes.Internal, "get uuid of %s error", name)
	}

	return idStrs[3], nil
}

func (j *juicefs) JfsUnmount(volumeId, mountPath string) error {
	uniqueId, err := j.getUniqueId(volumeId)
	if err != nil {
		klog.Errorf("Get volume name by volume id %s error: %v", volumeId, err)
		return err
	}
	if config.ByProcess {
		ref, err := j.processMount.GetMountRef(mountPath, "")
		if err != nil {
			klog.Errorf("Get mount ref error: %v", err)
		}
		err = j.processMount.JUmount(mountPath, "")
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
				go j.processMount.CleanCache(uuid, uniqueId, cacheDirs)
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
	pod, err := j.K8sClient.GetPod(oldPodName, config.Namespace)
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
		config.PodUniqueIdLabelKey: config.UniqueId,
	}}
	fieldSelector := &fields.Set{"spec.nodeName": config.NodeName}
	pods, err := j.K8sClient.ListPod(config.Namespace, labelSelector, fieldSelector)
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
	if err = mnt.UmountTarget(mountPath, podName); err != nil {
		return err
	}
	if podName == "" {
		return nil
	}
	// get refs of mount pod
	refs, err := mnt.GetMountRef(mountPath, podName)
	if err != nil {
		return err
	}
	if refs == 0 {
		// if refs is none, umount
		return j.podMount.JUmount(mountPath, podName)
	}
	return nil
}

func (j *juicefs) JfsCleanupMountPoint(mountPath string) error {
	klog.V(5).Infof("JfsCleanupMountPoint: clean up mount point: %q", mountPath)
	_, err := util.DoWithinTime(context.TODO(), 5*time.Second, nil, func() error {
		return mount.CleanupMountPoint(mountPath, j.SafeFormatAndMount.Interface, false)
	})
	return err
}

// AuthFs authenticates JuiceFS, enterprise edition only
func (j *juicefs) AuthFs(secrets map[string]string, setting *config.JfsSetting) (string, error) {
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
	isOptional := map[string]bool{
		"accesskey":  true,
		"accesskey2": true,
		"secretkey":  true,
		"secretkey2": true,
		"bucket":     true,
		"bucket2":    true,
		"passphrase": true,
		"subdir":     true,
	}
	for _, k := range keys {
		if !isOptional[k] || secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, secrets[k]))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	for _, k := range keysStripped {
		if !isOptional[k] || secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=${%s}", k, k))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	if v, ok := os.LookupEnv("JFS_NO_UPDATE_CONFIG"); ok && v == "enabled" {
		cmdArgs = append(cmdArgs, "--no-update")
		args = append(args, "--no-update")
		if secrets["bucket"] == "" {
			return "", status.Errorf(codes.InvalidArgument,
				"bucket argument is required when --no-update option is provided")
		}
		if !config.FormatInPod && secrets["initconfig"] != "" {
			conf := secrets["name"] + ".conf"
			confPath := filepath.Join("/root/.juicefs", conf)
			if _, err := os.Stat(confPath); os.IsNotExist(err) {
				err = ioutil.WriteFile(confPath, []byte(secrets["initconfig"]), 0644)
				if err != nil {
					return "", status.Errorf(codes.Internal,
						"Create config file %q failed: %v", confPath, err)
				}
				klog.V(5).Infof("Create config file: %q success", confPath)
			}
		}
	}
	if setting.FormatOptions != "" {
		formatOptions := strings.Split(setting.FormatOptions, ",")
		for _, option := range formatOptions {
			o := strings.TrimSpace(option)
			if o != "" {
				args = append(args, fmt.Sprintf("--%s", o))
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", o))
			}
		}
	}
	klog.V(5).Infof("AuthFs cmd: %v", cmdArgs)

	if config.FormatInPod {
		cmd := strings.Join(cmdArgs, " ")
		return cmd, nil
	}

	authCmd := j.Exec.Command(config.CliPath, args...)
	envs := syscall.Environ()
	for key, val := range setting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", key, val))
	}
	authCmd.SetEnv(envs)
	var res []byte
	_, err := util.DoWithinTime(context.TODO(), 5*time.Second, nil, func() (err error) {
		res, err = authCmd.CombinedOutput()
		return
	})
	klog.Infof("Auth output is %s", res)
	return string(res), err
}

// MountFs mounts JuiceFS with idempotency
func (j *juicefs) MountFs(jfsSetting *config.JfsSetting) (string, error) {
	var mnt podmount.MntInterface
	if jfsSetting.UsePod {
		jfsSetting.MountPath = filepath.Join(config.PodMountBase, jfsSetting.UniqueId)
		mnt = j.podMount
	} else {
		jfsSetting.MountPath = filepath.Join(config.MountBase, jfsSetting.UniqueId)
		mnt = j.processMount
	}

	err := mnt.JMount(jfsSetting)
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

func (j *juicefs) Version() ([]byte, error) {
	return j.Exec.Command(config.CliPath, "version").CombinedOutput()
}

func (j *juicefs) ceFormat(secrets map[string]string, noUpdate bool, setting *config.JfsSetting) (string, error) {
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
	isOptional := map[string]bool{
		"block-size": true,
		"compress":   true,
		"trash-days": true,
		"capacity":   true,
		"inodes":     true,
		"shards":     true,
	}
	for _, k := range keys {
		if !isOptional[k] || secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, secrets[k]))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	for k, v := range keysStripped {
		if !isOptional[k] || secrets[k] != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=${%s}", k, v))
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	cmdArgs = append(cmdArgs, "${metaurl}", secrets["name"])
	args = append(args, secrets["metaurl"], secrets["name"])

	if setting.FormatOptions != "" {
		formatOptions := strings.Split(setting.FormatOptions, ",")
		for _, option := range formatOptions {
			o := strings.TrimSpace(option)
			if o != "" {
				args = append(args, fmt.Sprintf("--%s", o))
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", o))
			}
		}
	}
	klog.V(5).Infof("ceFormat cmd: %v", cmdArgs)

	if config.FormatInPod {
		cmd := strings.Join(cmdArgs, " ")
		return cmd, nil
	}

	formatCmd := j.Exec.Command(config.CeCliPath, args...)
	envs := syscall.Environ()
	for key, val := range setting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", key, val))
	}
	if secrets["storage"] == "ceph" || secrets["storage"] == "gs" {
		envs = append(envs, "JFS_NO_CHECK_OBJECT_STORAGE=1")
	}
	formatCmd.SetEnv(envs)
	var res []byte
	_, err := util.DoWithinTime(context.TODO(), 5*time.Second, nil, func() (err error) {
		res, err = formatCmd.CombinedOutput()
		return
	})
	klog.Infof("Format output is %s", res)
	return string(res), err
}
