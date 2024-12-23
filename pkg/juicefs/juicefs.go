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
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog/v2"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	podmount "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/security"
)

const (
	defaultCheckTimeout = 2 * time.Second
	fsTypeNone          = "none"
	procMountInfoPath   = "/proc/self/mountinfo"
)

var jfsLog = klog.NewKlogr().WithName("juicefs")

// Interface of juicefs provider
type Interface interface {
	mount.Interface
	JfsMount(ctx context.Context, volumeID string, target string, secrets, volCtx map[string]string, options []string) (Jfs, error)
	JfsCreateVol(ctx context.Context, volumeID string, subPath string, secrets, volCtx map[string]string) error
	JfsDeleteVol(ctx context.Context, volumeID string, target string, secrets, volCtx map[string]string, options []string) error
	JfsUnmount(ctx context.Context, volumeID, mountPath string) error
	JfsCleanupMountPoint(ctx context.Context, mountPath string) error
	GetJfsVolUUID(ctx context.Context, jfsSetting *config.JfsSetting) (string, error)
	SetQuota(ctx context.Context, secrets map[string]string, jfsSetting *config.JfsSetting, quotaPath string, capacity int64) error
	Settings(ctx context.Context, volumeID string, secrets, volCtx map[string]string, options []string) (*config.JfsSetting, error)
	GetSubPath(ctx context.Context, volumeID string) (string, error)
	CreateTarget(ctx context.Context, target string) error
	AuthFs(ctx context.Context, secrets map[string]string, jfsSetting *config.JfsSetting, force bool) (string, error)
	Status(ctx context.Context, metaUrl string) error
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
	Setting   *config.JfsSetting
}

// Jfs is the interface of a mounted file system
type Jfs interface {
	GetBasePath() string
	GetSetting() *config.JfsSetting
	CreateVol(ctx context.Context, volumeID, subPath string) (string, error)
	BindTarget(ctx context.Context, bindSource, target string) error
}

var _ Jfs = &jfs{}

func (fs *jfs) GetBasePath() string {
	return fs.MountPath
}

// CreateVol creates the directory needed
func (fs *jfs) CreateVol(ctx context.Context, volumeID, subPath string) (string, error) {
	log := util.GenLog(ctx, jfsLog, "CreateVol")
	volPath := filepath.Join(fs.MountPath, subPath)
	log.V(1).Info("checking volPath exists", "volPath", volPath, "fs", fs)
	var exists bool
	if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func() (err error) {
		exists, err = mount.PathExists(volPath)
		return
	}); err != nil {
		return "", fmt.Errorf("could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		log.Info("volume not existed")
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
	log := util.GenLog(ctx, jfsLog, "BindTarget")
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
			log.V(1).Info("target already bind mounted.", "target", target, "mountPath", fs.MountPath)
			return nil
		}
		// target is bind by other path, umount it
		log.Info("target bind mount to other path, umount it", "target", target)
		_ = util.DoWithTimeout(ctx, defaultCheckTimeout, func() error {
			util.UmountPath(ctx, target)
			return nil
		})
	}
	// bind target to mountpath
	log.Info("binding source at target", "source", bindSource, "target", target)
	if err := fs.Provider.Mount(bindSource, target, fsTypeNone, []string{"bind"}); err != nil {
		os.Remove(target)
		return err
	}
	return nil
}

func (fs *jfs) GetSetting() *config.JfsSetting {
	return fs.Setting
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

// unused for now
func (j *juicefs) JfsCreateVol(ctx context.Context, volumeID string, subPath string, secrets, volCtx map[string]string) error {
	jfsSetting, err := j.genJfsSettings(ctx, volumeID, "", secrets, volCtx, []string{})
	if err != nil {
		return err
	}
	jfsSetting.SubPath = subPath
	jfsSetting.MountPath = filepath.Join(config.TmpPodMountBase, jfsSetting.VolumeId)
	if !config.ByProcess {
		return j.podMount.JCreateVolume(ctx, jfsSetting)
	}
	return j.processMount.JCreateVolume(ctx, jfsSetting)
}

func (j *juicefs) JfsDeleteVol(ctx context.Context, volumeID string, subPath string, secrets, volCtx map[string]string, options []string) error {
	// if not process mode, get pv by volumeId
	if !config.ByProcess {
		pv, err := j.K8sClient.GetPersistentVolume(ctx, volumeID)
		if err != nil {
			return err
		}
		volCtx = pv.Spec.CSI.VolumeAttributes
		options = pv.Spec.MountOptions
	}
	jfsSetting, err := j.genJfsSettings(ctx, volumeID, "", secrets, volCtx, options)
	if err != nil {
		return err
	}
	jfsSetting.SubPath = subPath
	jfsSetting.MountPath = filepath.Join(config.TmpPodMountBase, jfsSetting.VolumeId)

	mnt := j.processMount
	if !config.ByProcess {
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
	jfsSetting, err := j.genJfsSettings(ctx, volumeID, target, secrets, volCtx, options)
	if err != nil {
		return nil, err
	}
	appInfo, err := config.ParseAppInfo(volCtx)
	if err != nil {
		return nil, err
	}
	mountPath, err := j.MountFs(ctx, appInfo, jfsSetting)
	if err != nil {
		return nil, err
	}

	return &jfs{
		Provider:  j,
		Name:      secrets["name"],
		MountPath: mountPath,
		Options:   options,
		Setting:   jfsSetting,
	}, nil
}

// Settings get all jfs settings and generate format/auth command
func (j *juicefs) Settings(ctx context.Context, volumeID string, secrets, volCtx map[string]string, options []string) (*config.JfsSetting, error) {
	log := util.GenLog(ctx, jfsLog, "Settings")
	pv, pvc, err := resource.GetPVWithVolumeHandleOrAppInfo(ctx, j.K8sClient, volumeID, volCtx)
	if err != nil {
		log.Error(err, "Get PV with volumeID error", "volumeId", volumeID)
	}
	// overwrite volCtx with pvc annotations
	if pvc != nil {
		if volCtx == nil {
			volCtx = make(map[string]string)
		}
		for k, v := range pvc.Annotations {
			if !strings.HasPrefix(k, "juicefs") {
				continue
			}
			volCtx[k] = v
		}
	}

	jfsSetting, err := config.ParseSetting(secrets, volCtx, options, !config.ByProcess, pv, pvc)
	if err != nil {
		log.Error(err, "Parse config error", "secret", secrets["name"])
		return nil, err
	}
	jfsSetting.VolumeId = volumeID
	if !jfsSetting.IsCe {
		if secrets["token"] == "" {
			log.Info("token is empty, skip authfs.")
		} else {
			res, err := j.AuthFs(ctx, secrets, jfsSetting, false)
			if err != nil {
				return nil, fmt.Errorf("juicefs auth error: %v", err)
			}
			jfsSetting.FormatCmd = res
		}
		jfsSetting.UUID = secrets["name"]
		jfsSetting.InitConfig = secrets["initconfig"]
	} else {
		noUpdate := false
		if secrets["storage"] == "" || secrets["bucket"] == "" {
			log.Info("JfsMount: storage or bucket is empty, format --no-update.")
			noUpdate = true
		}
		res, err := j.ceFormat(ctx, secrets, noUpdate, jfsSetting)
		if err != nil {
			return nil, fmt.Errorf("juicefs format error: %v", err)
		}
		jfsSetting.FormatCmd = res
	}
	return jfsSetting, nil
}

// genJfsSettings get jfs settings and unique id
func (j *juicefs) genJfsSettings(ctx context.Context, volumeID string, target string, secrets, volCtx map[string]string, options []string) (*config.JfsSetting, error) {
	log := util.GenLog(ctx, jfsLog, "Settings")
	// get settings
	jfsSetting, err := j.Settings(ctx, volumeID, secrets, volCtx, options)
	if err != nil {
		return nil, err
	}
	jfsSetting.TargetPath = target
	// get unique id
	uniqueId, err := j.getUniqueId(ctx, volumeID)
	if err != nil {
		log.Error(err, "Get volume name by volume id error", "volumeID", volumeID)
		return nil, err
	}
	log.V(1).Info("Get uniqueId of volume", "volumeId", volumeID, "uniqueId", uniqueId)
	jfsSetting.UniqueId = uniqueId
	jfsSetting.SecretName = fmt.Sprintf("juicefs-%s-secret", jfsSetting.UniqueId)
	if jfsSetting.CleanCache {
		uuid := jfsSetting.Name
		if jfsSetting.IsCe {
			if uuid, err = j.GetJfsVolUUID(ctx, jfsSetting); err != nil {
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
		log.V(1).Info("Get uuid of volume", "volumeId", volumeID, "uuid", uuid)
	}
	return jfsSetting, nil
}

// getUniqueId: get UniqueId from volumeId (volumeHandle of PV)
// When STORAGE_CLASS_SHARE_MOUNT env is set:
//
//	in dynamic provision, UniqueId set as SC name
//	if sc secrets is template. UniqueId set as volumeId
//	in static provision, UniqueId set as volumeId
//
// When STORAGE_CLASS_SHARE_MOUNT env not set:
//
//	UniqueId set as volumeId
func (j *juicefs) getUniqueId(ctx context.Context, volumeId string) (string, error) {
	log := util.GenLog(ctx, jfsLog, "getUniqueId")
	if config.StorageClassShareMount && !config.ByProcess {
		pv, err := j.K8sClient.GetPersistentVolume(ctx, volumeId)
		// In static provision, volumeId may not be PV name, it is expected that PV cannot be found by volumeId
		if err != nil && !k8serrors.IsNotFound(err) {
			return "", err
		}
		// In dynamic provision, PV.spec.StorageClassName is which SC(StorageClass) it belongs to.
		// if SC has template secrets, UniqueId set as volumeId
		if err == nil && pv.Spec.StorageClassName != "" {
			if sc, err := j.K8sClient.GetStorageClass(ctx, pv.Spec.StorageClassName); err != nil {
				log.Error(err, "Get storage class error", "sc", pv.Spec.StorageClassName)
				return "", err
			} else {
				secret := sc.Parameters[common.PublishSecretName]
				secretNamespace := sc.Parameters[common.PublishSecretNamespace]
				if strings.Contains(secret, "$") || strings.Contains(secretNamespace, "$") {
					log.Info("storageClass has template secrets, cannot use `STORAGE_CLASS_SHARE_MOUNT`", "volumeId", volumeId)
					return volumeId, nil
				}
			}
			return pv.Spec.StorageClassName, nil
		}
	}
	return volumeId, nil
}

// GetJfsVolUUID get UUID from result of `juicefs status <volumeName>`
func (j *juicefs) GetJfsVolUUID(ctx context.Context, jfsSetting *config.JfsSetting) (string, error) {
	log := util.GenLog(ctx, jfsLog, "GetJfsVolUUID")
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 8*defaultCheckTimeout)
	defer cmdCancel()
	statusCmd := j.Exec.CommandContext(cmdCtx, config.CeCliPath, "status", jfsSetting.Source)
	envs := syscall.Environ()
	for key, val := range jfsSetting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", security.EscapeBashStr(key), security.EscapeBashStr(val)))
	}
	statusCmd.SetEnv(envs)
	stdout, err := statusCmd.CombinedOutput()
	if err != nil {
		re := string(stdout)
		if strings.Contains(re, "database is not formatted") {
			log.V(1).Info("juicefs not formatted.", "name", jfsSetting.Source)
			return "", nil
		}
		log.Error(err, "juicefs status error", "output", re)
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
		return "", fmt.Errorf("get uuid of %s error", jfsSetting.Source)
	}

	return idStrs[3], nil
}

func (j *juicefs) validTarget(target string) error {
	var msg string
	if strings.Contains(target, "../") || strings.Contains(target, "/..") || strings.Contains(target, "..") {
		msg = msg + fmt.Sprintf("Path %s has illegal access.", target)
		return errors.New(msg)
	}
	if strings.Contains(target, "./") || strings.Contains(target, "/.") {
		msg = msg + fmt.Sprintf("Path %s has illegal access.", target)
		return errors.New(msg)
	}
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

func (j *juicefs) JfsUnmount(ctx context.Context, volumeId, mountPath string) error {
	log := util.GenLog(ctx, jfsLog, "JfsUmount")
	uniqueId, err := j.getUniqueId(ctx, volumeId)
	if err != nil {
		log.Error(err, "Get volume name by volume id error", "volumeId", volumeId)
		return err
	}
	if config.ByProcess {
		ref, err := j.processMount.GetMountRef(ctx, mountPath, "")
		if err != nil {
			log.Error(err, "Get mount ref error")
		}
		err = j.processMount.JUmount(ctx, mountPath, "")
		if err != nil {
			log.Error(err, "umount error")
		}
		if ref == 1 {
			func() {
				j.Lock()
				defer j.Unlock()
				uuid := j.UUIDMaps[uniqueId]
				cacheDirs := j.CacheDirMaps[uniqueId]
				if uuid == "" && len(cacheDirs) == 0 {
					log.Info("Can't get uuid and cacheDirs. skip cache clean.", "uniqueId", uniqueId)
					return
				}
				delete(j.UUIDMaps, uniqueId)
				delete(j.CacheDirMaps, uniqueId)

				log.Info("Cleanup cache of volume", "uniqueId", uniqueId, "node", config.NodeName)
				// clean cache should be done even when top context timeout
				go func() {
					_ = j.processMount.CleanCache(context.TODO(), "", uuid, uniqueId, cacheDirs)
				}()
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
			log.Error(err, "Get mount pod error", "pod", oldPodName)
			return err
		}
	}
	if pod != nil {
		mountPods = append(mountPods, *pod)
	}
	// get pod by label
	labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{
		common.PodTypeKey:          common.PodTypeValue,
		common.PodUniqueIdLabelKey: uniqueId,
	}}
	fieldSelector := &fields.Set{"spec.nodeName": config.NodeName}
	pods, err := j.K8sClient.ListPod(ctx, config.Namespace, labelSelector, fieldSelector)
	if err != nil {
		log.Error(err, "List pods of uniqueId error", "uniqueId", uniqueId)
		return err
	}
	mountPods = append(mountPods, pods...)
	// find pod by target
	key := util.GetReferenceKey(mountPath)
	for _, po := range mountPods {
		if po.DeletionTimestamp != nil || resource.IsPodComplete(&po) {
			continue
		}
		if _, ok := po.Annotations[key]; ok {
			mountPod = &po
			break
		}
	}
	if mountPod != nil {
		podName = mountPod.Name
	}
	lock := config.GetPodLock(config.GetPodLockKey(mountPod, ""))
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

func (j *juicefs) CreateTarget(ctx context.Context, target string) error {
	var corruptedMnt bool

	for {
		err := util.DoWithTimeout(ctx, defaultCheckTimeout, func() (err error) {
			_, err = mount.PathExists(target)
			return
		})
		if err == nil {
			return os.MkdirAll(target, os.FileMode(0755))
		} else if corruptedMnt = mount.IsCorruptedMnt(err); corruptedMnt {
			// if target is a corrupted mount, umount it
			_ = util.DoWithTimeout(ctx, defaultCheckTimeout, func() error {
				util.UmountPath(ctx, target)
				return nil
			})
			continue
		} else {
			return err
		}
	}
}

func (j *juicefs) JfsCleanupMountPoint(ctx context.Context, mountPath string) error {
	log := util.GenLog(ctx, jfsLog, "JfsCleanupMountPoint")
	log.Info("clean up mount point", "mountPath", mountPath)
	return util.DoWithTimeout(ctx, 2*defaultCheckTimeout, func() (err error) {
		return mount.CleanupMountPoint(mountPath, j.SafeFormatAndMount.Interface, false)
	})
}

// AuthFs authenticates JuiceFS, enterprise edition only
func (j *juicefs) AuthFs(ctx context.Context, secrets map[string]string, setting *config.JfsSetting, force bool) (string, error) {
	log := util.GenLog(ctx, jfsLog, "AuthFs")
	args, cmdArgs, err := config.GenAuthCmd(secrets, setting)
	if err != nil {
		return "", err
	}

	// only run command when in process mode
	if !force && !config.ByProcess {
		cmd := strings.Join(cmdArgs, " ")
		return cmd, nil
	}

	cmdCtx, cmdCancel := context.WithTimeout(ctx, 8*defaultCheckTimeout)
	defer cmdCancel()
	authCmd := j.Exec.CommandContext(cmdCtx, config.CliPath, args...)
	envs := syscall.Environ()
	for key, val := range setting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", security.EscapeBashStr(key), security.EscapeBashStr(val)))
	}
	envs = append(envs, "JFS_NO_CHECK_OBJECT_STORAGE=1")
	authCmd.SetEnv(envs)
	res, err := authCmd.CombinedOutput()
	log.Info("auth output", "output", res)
	if err != nil {
		re := string(res)
		log.Error(err, "auth error")
		if cmdCtx.Err() == context.DeadlineExceeded {
			re = fmt.Sprintf("juicefs auth %s timed out", 8*defaultCheckTimeout)
			return "", errors.New(re)
		}
		return "", errors.Wrap(err, re)
	}
	return string(res), nil
}

func (j *juicefs) SetQuota(ctx context.Context, secrets map[string]string, jfsSetting *config.JfsSetting, quotaPath string, capacity int64) error {
	log := util.GenLog(ctx, jfsLog, "SetQuota")
	cap := capacity / 1024 / 1024 / 1024
	if cap <= 0 {
		return fmt.Errorf("capacity %d is too small, at least 1GiB for quota", capacity)
	}

	var args, cmdArgs []string
	if jfsSetting.IsCe {
		args = []string{"quota", "set", secrets["metaurl"], "--path", quotaPath, "--capacity", strconv.FormatInt(cap, 10)}
		cmdArgs = []string{config.CeCliPath, "quota", "set", "${metaurl}", "--path", quotaPath, "--capacity", strconv.FormatInt(cap, 10)}
	} else {
		args = []string{"quota", "set", secrets["name"], "--path", quotaPath, "--capacity", strconv.FormatInt(cap, 10)}
		cmdArgs = []string{config.CliPath, "quota", "set", secrets["name"], "--path", quotaPath, "--capacity", strconv.FormatInt(cap, 10)}
	}
	log.Info("quota cmd", "command", strings.Join(cmdArgs, " "))
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 5*defaultCheckTimeout)
	defer cmdCancel()
	envs := syscall.Environ()
	for key, val := range jfsSetting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", security.EscapeBashStr(key), security.EscapeBashStr(val)))
	}
	var err error
	if !jfsSetting.IsCe {
		var authRes string
		authRes, err = j.AuthFs(ctx, secrets, jfsSetting, true)
		if err != nil {
			return errors.Wrap(err, authRes)
		}
		quotaCmd := j.Exec.CommandContext(cmdCtx, config.CliPath, args...)
		quotaCmd.SetEnv(envs)
		res, err := quotaCmd.CombinedOutput()

		if err == nil {
			log.Info("quota set success", "output", string(res))
		}
		return wrapSetQuotaErr(string(res), err)
	}

	done := make(chan error, 1)
	go func() {
		// ce cli will block until quota is set
		quotaCmd := j.Exec.CommandContext(context.Background(), config.CeCliPath, args...)
		quotaCmd.SetEnv(envs)
		res, err := quotaCmd.CombinedOutput()
		if err == nil {
			log.Info("quota set success", "output", string(res))
		}
		done <- wrapSetQuotaErr(string(res), err)
		close(done)
	}()
	select {
	case <-cmdCtx.Done():
		log.Info("quota set timeout, runs in background")
		return nil
	case err = <-done:
		return err
	}
}

func wrapSetQuotaErr(res string, err error) error {
	if err != nil {
		re := string(res)
		if strings.Contains(re, "invalid command: quota") || strings.Contains(re, "No help topic for 'quota'") {
			jfsLog.Info("juicefs inside do not support quota, skip it.")
			return nil
		}
		return errors.Wrap(err, re)
	}
	return err
}

func wrapStatusErr(res string, err error) error {
	if err != nil {
		re := string(res)
		if strings.Contains(re, "database is not formatted") {
			jfsLog.Info("juicefs not formatted, ignore status command error")
			return nil
		}
		return errors.Wrap(err, re)
	}
	return err
}

func (j *juicefs) GetSubPath(ctx context.Context, volumeID string) (string, error) {
	if config.Provisioner {
		pv, err := j.K8sClient.GetPersistentVolume(ctx, volumeID)
		if err != nil {
			return "", err
		}
		return pv.Spec.CSI.VolumeAttributes["subPath"], nil
	}
	return volumeID, nil
}

// MountFs mounts JuiceFS with idempotency
func (j *juicefs) MountFs(ctx context.Context, appInfo *config.AppInfo, jfsSetting *config.JfsSetting) (string, error) {
	log := util.GenLog(ctx, jfsLog, "MountFs")
	var mnt podmount.MntInterface
	if jfsSetting.UsePod {
		jfsSetting.MountPath = filepath.Join(config.PodMountBase, jfsSetting.UniqueId)
		mnt = j.podMount
	} else {
		jfsSetting.MountPath = filepath.Join(config.MountBase, jfsSetting.UniqueId)
		mnt = j.processMount
	}

	err := mnt.JMount(ctx, appInfo, jfsSetting)
	if err != nil {
		return "", err
	}
	log.Info("mounting with options", "source", util.StripPasswd(jfsSetting.Source), "mountPath", jfsSetting.MountPath, "options", jfsSetting.Options)
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
		jfsLog.Info("Upgrade: did not finish", "time", timeout)
		return
	}

	if err != nil {
		jfsLog.Error(err, "Upgrade juicefs err")
		return
	}

	jfsLog.Info("Upgrade: successfully upgraded to newest version")
}

func (j *juicefs) ceFormat(ctx context.Context, secrets map[string]string, noUpdate bool, setting *config.JfsSetting) (string, error) {
	log := util.GenLog(ctx, jfsLog, "ceFormat")
	args, cmdArgs, err := config.GenFormatCmd(secrets, noUpdate, setting)
	if err != nil {
		return "", err
	}

	// only run command when in process mode
	if !config.ByProcess {
		cmd := strings.Join(cmdArgs, " ")
		return cmd, nil
	}

	cmdCtx, cmdCancel := context.WithTimeout(ctx, 8*defaultCheckTimeout)
	defer cmdCancel()

	shArgs := append([]string{config.CeCliPath}, args...)
	formatCmd := j.Exec.CommandContext(cmdCtx, "/bin/bash", "-c", strings.Join(shArgs, " "))
	envs := syscall.Environ()
	for key, val := range setting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", security.EscapeBashStr(key), security.EscapeBashStr(val)))
	}
	if secrets["storage"] == "ceph" || secrets["storage"] == "gs" {
		envs = append(envs, "JFS_NO_CHECK_OBJECT_STORAGE=1")
	}
	formatCmd.SetEnv(envs)
	res, err := formatCmd.CombinedOutput()
	log.Info("format output", "output", res)
	if err != nil {
		re := string(res)
		log.Error(err, "format error")
		if cmdCtx.Err() == context.DeadlineExceeded {
			re = fmt.Sprintf("juicefs format %s timed out", 8*defaultCheckTimeout)
			return "", errors.New(re)
		}
		return "", errors.Wrap(err, re)
	}
	return string(res), nil
}

// Status checks the status of JuiceFS, only for community edition
func (j *juicefs) Status(ctx context.Context, metaUrl string) error {
	log := util.GenLog(ctx, jfsLog, "status")
	args := []string{"status", metaUrl}
	cmdArgs := []string{config.CeCliPath, "status", "${metaurl}"}

	log.Info("juicefs status cmd", "command", strings.Join(cmdArgs, " "))
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 2*defaultCheckTimeout)
	defer cmdCancel()

	done := make(chan error, 1)
	go func() {
		res, err := j.Exec.CommandContext(context.Background(), config.CeCliPath, args...).CombinedOutput()
		done <- wrapStatusErr(string(res), err)
		close(done)
	}()

	select {
	case <-cmdCtx.Done():
		err := fmt.Errorf("juicefs status %s timed out", 2*defaultCheckTimeout)
		return err
	case err := <-done:
		return err
	}
}
