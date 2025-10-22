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
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
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
	SetQuota(ctx context.Context, secrets map[string]string, jfsSetting *config.JfsSetting, quotaPath string, capacity int64) error
	Settings(ctx context.Context, volumeID, uniqueId, uuid string, secrets, volCtx map[string]string, options []string) (*config.JfsSetting, error)
	GetSubPath(ctx context.Context, volumeID string) (string, error)
	CreateTarget(ctx context.Context, target string) error
	AuthFs(ctx context.Context, secrets map[string]string, jfsSetting *config.JfsSetting, force bool) (string, error)
	Status(ctx context.Context, metaUrl string) error
	CreateSnapshot(ctx context.Context, snapshotID, sourceVolumeID, sourcePath string, secrets map[string]string, volCtx map[string]string) error
	DeleteSnapshot(ctx context.Context, snapshotID, snapshotPath string, secrets map[string]string) error
	RestoreSnapshot(ctx context.Context, snapshotID, targetVolumeID, targetVolumePath string, secrets map[string]string, volCtx map[string]string) error
}

type juicefs struct {
	sync.Mutex
	mount.SafeFormatAndMount
	*k8sclient.K8sClient

	mnt          podmount.MntInterface
	UUIDMaps     map[string]string
	CacheDirMaps map[string][]string
}

var _ Interface = &juicefs{}

type jfs struct {
	Provider  *juicefs
	Name      string
	MountPath string
	Options   []string
	Setting   *config.JfsSetting `json:"-"`
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
	if !config.StorageClassShareMount && !config.FSShareMount && !config.ByProcess {
		return fs.MountPath, nil
	}
	volPath := filepath.Join(fs.MountPath, subPath)
	log.V(1).Info("checking volPath exists", "volPath", volPath, "fs", fs)
	var exists bool
	if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
		exists, err = mount.PathExists(volPath)
		return
	}); err != nil {
		return "", fmt.Errorf("could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		log.Info("volume not existed")
		if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
			return os.MkdirAll(volPath, os.FileMode(0777))
		}); err != nil {
			return "", fmt.Errorf("could not make directory for meta %q: %v", volPath, err)
		}
		var fi os.FileInfo
		if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
			fi, err = os.Stat(volPath)
			return err
		}); err != nil {
			return "", fmt.Errorf("could not stat directory %s: %q", volPath, err)
		} else if fi.Mode().Perm() != 0777 { // The perm of `volPath` may not be 0777 when the umask applied
			if err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
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
		_ = util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) error {
			return util.UmountPath(ctx, target, false)
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
	var mnt podmount.MntInterface
	if config.ByProcess {
		mnt = podmount.NewProcessMount(*mounter)
	} else {
		mnt = podmount.NewPodMount(k8sClient, *mounter)
	}

	uuidMaps := make(map[string]string)
	cacheDirMaps := make(map[string][]string)
	return &juicefs{
		Mutex:              sync.Mutex{},
		SafeFormatAndMount: *mounter,
		K8sClient:          k8sClient,
		mnt:                mnt,
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
	return j.mnt.JCreateVolume(ctx, jfsSetting)
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

	if err := j.mnt.JDeleteVolume(ctx, jfsSetting); err != nil {
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
// do format/auth only in process mode
// volumeID: volumeHandle of PV
// uniqueId: volumeId or storageClassName
// uuid: uuid of juicefs volume. If equals "", will be generated by juicefs status
// secrets: customs secrets
// volCtx: volume context
// options: mount options
func (j *juicefs) Settings(ctx context.Context, volumeID, uniqueId, uuid string, secrets, volCtx map[string]string, options []string) (*config.JfsSetting, error) {
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

	jfsSetting, err := config.ParseSetting(ctx, secrets, volCtx, options, volumeID, uniqueId, uuid, pv, pvc)
	if err != nil {
		log.Error(err, "Parse config error", "secret", secrets["name"])
		return nil, err
	}

	if jfsSetting.FormatCmd != "" {
		log.Info("Format/Auth command", "cmd", jfsSetting.FormatCmd)
	} else {
		log.Info("Skip auth/format")
	}

	// do format/auth in process mode
	if config.ByProcess {
		if !jfsSetting.IsCe {
			if secrets["token"] == "" {
				log.Info("token is empty, skip authfs.")
			} else {
				_, err := j.AuthFs(ctx, secrets, jfsSetting, false)
				if err != nil {
					return nil, fmt.Errorf("juicefs auth error: %v", err)
				}
			}
		} else {
			noUpdate := false
			if secrets["storage"] == "" || secrets["bucket"] == "" {
				log.Info("JfsMount: storage or bucket is empty, format --no-update.")
				noUpdate = true
			}
			_, err := j.ceFormat(ctx, secrets, noUpdate, jfsSetting)
			if err != nil {
				return nil, fmt.Errorf("juicefs format error: %v", err)
			}
		}
	}
	return jfsSetting, nil
}

// genJfsSettings get jfs settings and unique id
func (j *juicefs) genJfsSettings(ctx context.Context, volumeID string, target string, secrets, volCtx map[string]string, options []string) (*config.JfsSetting, error) {
	log := util.GenLog(ctx, jfsLog, "Settings")
	// get unique id
	uniqueId, err := j.getUniqueId(ctx, volumeID, secrets)
	if err != nil {
		log.Error(err, "Get volume name by volume id error", "volumeID", volumeID)
		return nil, err
	}
	log.V(1).Info("Get uniqueId of volume", "volumeId", volumeID, "uniqueId", uniqueId)
	// get settings
	jfsSetting, err := j.Settings(ctx, volumeID, uniqueId, "", secrets, volCtx, options)
	if err != nil {
		return nil, err
	}
	jfsSetting.TargetPath = target

	if jfsSetting.CleanCache {
		uuid := jfsSetting.UUID
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

// shouldUseFSNameAsUniqueId checks if the file system name (`fsname`) can be used as the unique ID.
//
// Using `fsname` as the unique ID is possible under the following conditions:
// 1. If no other secret with the same name exists in the cluster.
// 2. If a secret with the same name exists, the configuration must be consistent:
//   - For Community Edition (CE): The `metaurl` must be the same.
//   - For Enterprise Edition (EE):
//   - The `token` must be the same.
//   - The console URL (`BASE_URL`) must either not exist or be the same.
//
// If these conditions are not met, the function returns `false`, and the system should
// fall back to using the `volumeId` as the unique ID.
func (j *juicefs) shouldUseFSNameAsUniqueId(ctx context.Context, fsname string, secrets map[string]string) (bool, error) {
	log := util.GenLog(ctx, jfsLog, "shouldUseFSNameAsUniqueId")
	if fsname == "" {
		return false, nil
	}

	secretName := fmt.Sprintf("juicefs-%s-secret", fsname)
	existSecret, err := j.K8sClient.GetSecret(ctx, secretName, config.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	v1, isCe := secrets["metaurl"]
	v2, existIsCe := existSecret.Data["metaurl"]

	if isCe != existIsCe {
		log.Info("fallback to volumeId", "secretName", secretName, "fsname", fsname, "isCe", isCe, "existIsCe", existIsCe)
		return false, nil
	}

	if isCe {
		r := v1 == string(v2)
		if !r {
			log.Info("metaurl is not equal with exist secret, fallback to volumeId", "secretName", secretName, "fsname", fsname)
		}
		return r, nil
	}

	// EE
	if secrets["token"] != string(existSecret.Data["token"]) {
		log.V(1).Info("token is not equal with exist secret, fallback to volumeId", "secretName", secretName, "fsname", fsname)
		return false, nil
	}

	consoleUrl := ""
	if envs, ok := secrets["envs"]; ok {
		var envsMap map[string]string
		if err := config.ParseYamlOrJson(envs, &envsMap); err != nil {
			return false, err
		}
		if val, ok := envsMap["BASE_URL"]; ok {
			consoleUrl = val
		}
	}

	existConsoleUrl := ""
	if val, ok := existSecret.Data["BASE_URL"]; ok {
		existConsoleUrl = string(val)
	}
	r := consoleUrl == existConsoleUrl
	if !r {
		log.Info("console url is not equal with exist secret, fallback to volumeId", "secretName", secretName, "consoleUrl", consoleUrl)
	}
	return r, nil
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
func (j *juicefs) getUniqueId(ctx context.Context, volumeId string, secrets map[string]string) (string, error) {
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
	if config.FSShareMount && !config.ByProcess {
		if fsname, ok := secrets["name"]; ok {
			ok, err := j.shouldUseFSNameAsUniqueId(ctx, fsname, secrets)
			if err != nil {
				return "", err
			}
			if ok {
				return fsname, nil
			}
			return volumeId, nil
		}
		pv, err := j.K8sClient.GetPersistentVolume(ctx, volumeId)
		// In static provision, volumeId may not be PV name, it is expected that PV cannot be found by volumeId
		if err != nil {
			pvs, err := j.K8sClient.ListPersistentVolumesByVolumeHandle(ctx, volumeId)
			if err != nil {
				return "", err
			}
			if len(pvs) == 0 {
				log.Info("no persistent volume found for volumeHandle, fallback to volumeId", "volumeHandle", volumeId)
				return volumeId, nil
			}
			pv = &pvs[0]
		}
		// get secret
		if pv.Spec.CSI != nil && pv.Spec.CSI.NodePublishSecretRef != nil {
			secretName := pv.Spec.CSI.NodePublishSecretRef.Name
			secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Namespace
			log.V(1).Info("Get secret from PV", "secretName", secretName, "secretNamespace", secretNamespace)
			secret, err := j.K8sClient.GetSecret(ctx, secretName, secretNamespace)
			if err != nil {
				return "", err
			}
			secretData := make(map[string]string)
			for k, v := range secret.Data {
				secretData[k] = string(v)
			}
			if fsname, ok := secretData["name"]; ok {
				ok, err := j.shouldUseFSNameAsUniqueId(ctx, string(fsname), secretData)
				if err != nil {
					return "", err
				}
				if ok {
					return string(fsname), nil
				}
				return volumeId, nil
			}
		}
	}
	return volumeId, nil
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

var errorNotFound = fmt.Errorf("not found")

func (j *juicefs) findMountPod(ctx context.Context, uniqueId, mountPath string) (*corev1.Pod, error) {
	log := util.GenLog(ctx, jfsLog, "JfsUmount/findMountPod")
	mountPods := []corev1.Pod{}
	var mountPod *corev1.Pod
	// get pod by exact name
	oldPodName := podmount.GenPodNameByUniqueId(uniqueId, false)
	pod, err := j.K8sClient.GetPod(ctx, oldPodName, config.Namespace)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Error(err, "Get mount pod error", "pod", oldPodName)
			return nil, err
		}
	}
	if pod != nil {
		mountPods = append(mountPods, *pod)
	}
	labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{
		common.PodTypeKey:          common.PodTypeValue,
		common.PodUniqueIdLabelKey: uniqueId,
	}}
	fieldSelector := &fields.Set{"spec.nodeName": config.NodeName}
	pods, err := j.K8sClient.ListPod(ctx, config.Namespace, labelSelector, fieldSelector)
	if err != nil {
		log.Error(err, "List pods of uniqueId error", "uniqueId", uniqueId)
		return nil, err
	}
	mountPods = append(mountPods, pods...)
	key := util.GetReferenceKey(mountPath)
	for _, po := range mountPods {
		if po.DeletionTimestamp != nil || resource.IsPodComplete(&po) {
			continue
		}
		if _, ok := po.Annotations[key]; ok {
			mountPod = &po
			return mountPod, nil
		}
	}

	return nil, errorNotFound
}

func (j *juicefs) JfsUnmount(ctx context.Context, volumeId, mountPath string) error {
	log := util.GenLog(ctx, jfsLog, "JfsUmount")
	// umount target path
	if err := j.mnt.UmountTarget(ctx, mountPath, ""); err != nil {
		return err
	}
	// umount mount pod
	uniqueId, err := j.getUniqueId(ctx, volumeId, nil)
	if err != nil {
		log.Error(err, "Get volume name by volume id error", "volumeId", volumeId)
		return err
	}
	if config.ByProcess {
		ref, err := j.mnt.GetMountRef(ctx, mountPath, "")
		if err != nil {
			log.Error(err, "Get mount ref error")
		}
		err = j.mnt.JUmount(ctx, mountPath, "")
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
					_ = j.mnt.CleanCache(context.TODO(), "", uuid, uniqueId, cacheDirs)
				}()
			}()
		}
		return err
	}

	mountPod, err := j.findMountPod(ctx, uniqueId, mountPath)
	if err != nil && errors.Is(err, errorNotFound) && volumeId != uniqueId {
		mountPod, err = j.findMountPod(ctx, volumeId, mountPath)
	}
	if err != nil && !errors.Is(err, errorNotFound) {
		return err
	}
	if mountPod == nil {
		log.Info("No mount pod found, skip umount mount pod", "mountPath", mountPath, "uniqueId", uniqueId, "volumeId", volumeId)
		return nil
	}
	lock := config.GetPodLock(config.GetPodLockKey(mountPod, ""))
	lock.Lock()
	defer lock.Unlock()
	return j.mnt.JUmount(ctx, mountPath, mountPod.Name)
}

func (j *juicefs) CreateTarget(ctx context.Context, target string) error {
	var corruptedMnt bool

	for {
		err := util.DoWithTimeout(ctx, defaultCheckTimeout, func(ctx context.Context) (err error) {
			_, err = mount.PathExists(target)
			return
		})
		if err == nil {
			return os.MkdirAll(target, os.FileMode(0755))
		} else if corruptedMnt = mount.IsCorruptedMnt(err); corruptedMnt {
			// if target is a corrupted mount, umount it
			_ = util.DoWithTimeout(ctx, defaultCheckTimeout*2, func(ctx context.Context) error {
				return util.UmountPath(ctx, target, false)
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
	return util.DoWithTimeout(ctx, 2*defaultCheckTimeout, func(ctx context.Context) (err error) {
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

	log.Info("AuthFs cmd", "args", cmdArgs)
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 8*defaultCheckTimeout)
	defer cmdCancel()
	authCmd := j.Exec.CommandContext(cmdCtx, config.CliPath, args...)
	envs := syscall.Environ()
	for key, val := range setting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", security.EscapeBashStr(key), security.EscapeBashStr(val)))
	}
	if secrets["storage"] == "ceph" || secrets["storage"] == "gs" {
		envs = append(envs, "JFS_NO_CHECK_OBJECT_STORAGE=1")
	}
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
		args = []string{"quota", "set", fmt.Sprintf("'%s'", secrets["metaurl"]), "--path", quotaPath, "--capacity", strconv.FormatInt(cap, 10)}
		cmdArgs = []string{config.CeCliPath, "quota", "set", "${metaurl}", "--path", quotaPath, "--capacity", strconv.FormatInt(cap, 10)}
		if util.SupportQuotaPathCreate(true, config.BuiltinCeVersion) {
			args = append(args, "--create")
			cmdArgs = append(cmdArgs, "--create")
		}
	} else {
		args = []string{"quota", "set", secrets["name"], "--path", quotaPath, "--capacity", strconv.FormatInt(cap, 10)}
		cmdArgs = []string{config.CliPath, "quota", "set", secrets["name"], "--path", quotaPath, "--capacity", strconv.FormatInt(cap, 10)}
		if util.SupportQuotaPathCreate(false, config.BuiltinEeVersion) {
			args = append(args, "--create")
			cmdArgs = append(cmdArgs, "--create")
		}
	}
	log.Info("quota cmd", "command", strings.Join(cmdArgs, " "))
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 10*defaultCheckTimeout)
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
		cmdStr := fmt.Sprintf("umask 000; %s %s", config.CliPath, strings.Join(args, " "))
		quotaCmd := j.Exec.CommandContext(cmdCtx, "sh", "-c", cmdStr)
		quotaCmd.SetEnv(envs)
		res, err := quotaCmd.CombinedOutput()

		if err == nil {
			log.Info("quota set success", "output", string(res))
		}
		return wrapSetQuotaErr(string(res), err)
	}

	cmdStr := fmt.Sprintf("umask 000; %s %s", config.CeCliPath, strings.Join(args, " "))
	quotaCmd := j.Exec.CommandContext(ctx, "sh", "-c", cmdStr)
	quotaCmd.SetEnv(envs)
	res, err := quotaCmd.CombinedOutput()
	if err == nil {
		log.Info("quota set success", "output", string(res))
	}

	return wrapSetQuotaErr(string(res), err)
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
	if jfsSetting.UsePod {
		jfsSetting.MountPath = filepath.Join(config.PodMountBase, jfsSetting.UniqueId)
	} else {
		jfsSetting.MountPath = filepath.Join(config.MountBase, jfsSetting.UniqueId)
	}

	err := j.mnt.JMount(ctx, appInfo, jfsSetting)
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

	log.Info("ce format cmd", "args", cmdArgs)
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

// CreateSnapshot creates a snapshot using JuiceFS CLI clone command via a Job
func (j *juicefs) CreateSnapshot(ctx context.Context, snapshotID, sourceVolumeID, sourcePath string, secrets map[string]string, volCtx map[string]string) error {
	log := util.GenLog(ctx, jfsLog, "CreateSnapshot")
	log.Info("creating snapshot", "snapshotID", snapshotID, "sourceVolumeID", sourceVolumeID, "sourcePath", sourcePath)

	// Get proper JfsSetting using Settings method
	jfsSetting, err := j.Settings(ctx, sourceVolumeID, sourceVolumeID, "", secrets, volCtx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to get settings")
	}

	// Get the actual secret name from the PV (instead of creating a new one)
	pv, _, err := resource.GetPVWithVolumeHandleOrAppInfo(ctx, j.K8sClient, sourceVolumeID, volCtx)
	if err != nil {
		return errors.Wrap(err, "failed to get PV for secret reference")
	}
	if pv == nil || pv.Spec.CSI == nil || pv.Spec.CSI.NodePublishSecretRef == nil {
		return errors.New("PV does not have a secret reference")
	}

	secretName := pv.Spec.CSI.NodePublishSecretRef.Name
	secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Namespace
	if secretNamespace == "" {
		secretNamespace = config.Namespace
	}

	// Fetch the existing secret to populate jfsSetting fields needed for GetEnvKey()
	secret, err := j.K8sClient.GetSecret(ctx, secretName, secretNamespace)
	if err != nil {
		return errors.Wrapf(err, "failed to get secret %s/%s", secretNamespace, secretName)
	}

	// Populate jfsSetting from secret so GetEnvKey() returns correct keys
	// For minimal code changes, we populate the standard internal fields
	if val, ok := secret.Data["metaurl"]; ok {
		jfsSetting.MetaUrl = string(val)
	}
	if val, ok := secret.Data["secretkey"]; ok {
		jfsSetting.SecretKey = string(val)
	} else if val, ok := secret.Data["secret-key"]; ok {
		// User secrets use "secret-key", normalize to internal "secretkey"
		jfsSetting.SecretKey = string(val)
	}
	if val, ok := secret.Data["token"]; ok {
		jfsSetting.Token = string(val)
	}
	if val, ok := secret.Data["passphrase"]; ok {
		jfsSetting.Passphrase = string(val)
	}
	if val, ok := secret.Data["encrypt_rsa_key"]; ok {
		jfsSetting.EncryptRsaKey = string(val)
	}
	if val, ok := secret.Data["initconfig"]; ok {
		jfsSetting.InitConfig = string(val)
	}

	// Set the secret name
	jfsSetting.SecretName = secretName
	log.Info("using existing secret from PV", "secretName", secretName, "namespace", secretNamespace)

	// If the secret is in a different namespace than the job (kube-system), copy it
	if secretNamespace != config.Namespace {
		// Create a copy of the secret in the job's namespace (kube-system)
		// Normalize secret keys to match internal convention (secret-key → secretkey)
		jobSecretName := fmt.Sprintf("juicefs-snapshot-%s-secret", snapshotID[:8])
		normalizedData := make(map[string][]byte)
		for k, v := range secret.Data {
			if k == "secret-key" {
				// Normalize to internal format
				normalizedData["secretkey"] = v
			} else if k == "access-key" {
				// Keep access-key as-is (it's used in format command directly)
				normalizedData["access-key"] = v
			} else {
				normalizedData[k] = v
			}
		}

		jobSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobSecretName,
				Namespace: config.Namespace,
			},
			Data: normalizedData,
		}

		_, err = j.K8sClient.CreateSecret(ctx, jobSecret)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return errors.Wrap(err, "failed to create secret copy for snapshot job")
		}

		// Update jfsSetting to use the copied secret
		jfsSetting.SecretName = jobSecretName
		log.Info("created secret copy in job namespace", "originalSecret", secretName, "jobSecret", jobSecretName, "namespace", config.Namespace)
	}

	// Use JobBuilder to create snapshot job
	jobBuilder := builder.NewJobBuilder(jfsSetting, 0)
	job := jobBuilder.NewJobForSnapshot(snapshotID, sourceVolumeID, secrets)

	jobName := job.Name
	log.Info("creating snapshot job", "jobName", jobName, "sourceVolume", sourceVolumeID, "snapshot", snapshotID)

	// Create the job and wait for completion
	_, err = j.K8sClient.CreateJob(ctx, job)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Info("snapshot job already exists, waiting for completion")
		} else {
			return errors.Wrap(err, "failed to create snapshot job")
		}
	}

	// Wait for job to complete (with timeout)
	log.Info("waiting for snapshot job to complete", "jobName", jobName)
	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return errors.New("snapshot job timed out after 120 seconds")
		case <-ticker.C:
			jobStatus, err := j.K8sClient.GetJob(ctx, jobName, config.Namespace)
			if err != nil {
				log.Info("waiting for job to be created", "jobName", jobName)
				continue
			}

			if jobStatus.Status.Succeeded > 0 {
				log.Info("snapshot job completed successfully", "jobName", jobName)
				return nil
			}

			if jobStatus.Status.Failed > 0 {
				pods, _ := j.K8sClient.ListPod(ctx, config.Namespace, &metav1.LabelSelector{
					MatchLabels: map[string]string{"job": jobName},
				}, nil)
				if len(pods) > 0 {
					logs, _ := j.K8sClient.GetPodLog(ctx, pods[0].Name, pods[0].Namespace, pods[0].Spec.Containers[0].Name)
					log.Error(nil, "snapshot job failed", "logs", logs)
				}
				return errors.New("snapshot job failed")
			}

			log.Info("snapshot job still running", "jobName", jobName)
		}
	}
}

// RestoreSnapshot restores a volume from a snapshot (background/async)
func (j *juicefs) RestoreSnapshot(ctx context.Context, snapshotID, targetVolumeID, targetVolumePath string, secrets map[string]string, volCtx map[string]string) error {
	log := util.GenLog(ctx, jfsLog, "RestoreSnapshot")
	log.Info("restoring volume from snapshot", "snapshotID", snapshotID, "targetVolumeID", targetVolumeID, "targetVolumePath", targetVolumePath)

	parts := strings.Split(snapshotID, "|")
	if len(parts) != 2 {
		return errors.Errorf("invalid snapshot ID format: %s, expected format: snapshot-uuid|source-volume-id", snapshotID)
	}
	actualSnapshotID := parts[0]
	sourceVolumeID := parts[1]

	log.Info("parsed snapshot ID", "actualSnapshotID", actualSnapshotID, "sourceVolumeID", sourceVolumeID, "targetVolumeID", targetVolumeID)

	// Create background restore job
	err := j.createRestoreJob(ctx, actualSnapshotID, sourceVolumeID, targetVolumeID, secrets)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Info("restore job already exists, restore in progress")
			return nil
		}
		return errors.Wrap(err, "failed to create restore job")
	}

	log.Info("restore job created, background restore started", "snapshotID", actualSnapshotID, "targetVolumeID", targetVolumeID)
	return nil
}

// createRestoreJob creates a Kubernetes Job to restore snapshot data in background
func (j *juicefs) createRestoreJob(ctx context.Context, snapshotID, sourceVolumeID, targetVolumeID string, secrets map[string]string) error {
	log := util.GenLog(ctx, jfsLog, "createRestoreJob")

	// Get proper JfsSetting using Settings method
	jfsSetting, err := j.Settings(ctx, targetVolumeID, targetVolumeID, "", secrets, nil, nil)
	if err != nil {
		return errors.Wrap(err, "failed to get settings")
	}

	// Get the actual secret name from the SOURCE PV (target PV doesn't exist yet during CreateVolume)
	pv, _, err := resource.GetPVWithVolumeHandleOrAppInfo(ctx, j.K8sClient, sourceVolumeID, nil)
	if err != nil {
		return errors.Wrap(err, "failed to get source PV for secret reference")
	}
	if pv == nil || pv.Spec.CSI == nil || pv.Spec.CSI.NodePublishSecretRef == nil {
		return errors.New("source PV does not have a secret reference")
	}

	secretName := pv.Spec.CSI.NodePublishSecretRef.Name
	secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Namespace
	if secretNamespace == "" {
		secretNamespace = config.Namespace
	}

	// Fetch the existing secret to populate jfsSetting fields needed for GetEnvKey()
	secret, err := j.K8sClient.GetSecret(ctx, secretName, secretNamespace)
	if err != nil {
		return errors.Wrapf(err, "failed to get secret %s/%s", secretNamespace, secretName)
	}

	// Populate jfsSetting from secret so GetEnvKey() returns correct keys
	// For minimal code changes, we populate the standard internal fields
	if val, ok := secret.Data["metaurl"]; ok {
		jfsSetting.MetaUrl = string(val)
	}
	if val, ok := secret.Data["secretkey"]; ok {
		jfsSetting.SecretKey = string(val)
	} else if val, ok := secret.Data["secret-key"]; ok {
		// User secrets use "secret-key", normalize to internal "secretkey"
		jfsSetting.SecretKey = string(val)
	}
	if val, ok := secret.Data["token"]; ok {
		jfsSetting.Token = string(val)
	}
	if val, ok := secret.Data["passphrase"]; ok {
		jfsSetting.Passphrase = string(val)
	}
	if val, ok := secret.Data["encrypt_rsa_key"]; ok {
		jfsSetting.EncryptRsaKey = string(val)
	}
	if val, ok := secret.Data["initconfig"]; ok {
		jfsSetting.InitConfig = string(val)
	}

	// Set the secret name
	jfsSetting.SecretName = secretName
	log.Info("using existing secret from target PV", "secretName", secretName, "namespace", secretNamespace)

	// If the secret is in a different namespace than the job (kube-system), copy it
	if secretNamespace != config.Namespace {
		// Create a copy of the secret in the job's namespace (kube-system)
		// Normalize secret keys to match internal convention (secret-key → secretkey)
		jobSecretName := fmt.Sprintf("juicefs-restore-%s-secret", targetVolumeID[:8])
		normalizedData := make(map[string][]byte)
		for k, v := range secret.Data {
			if k == "secret-key" {
				// Normalize to internal format
				normalizedData["secretkey"] = v
			} else if k == "access-key" {
				// Keep access-key as-is (it's used in format command directly)
				normalizedData["access-key"] = v
			} else {
				normalizedData[k] = v
			}
		}

		jobSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobSecretName,
				Namespace: config.Namespace,
			},
			Data: normalizedData,
		}

		_, err = j.K8sClient.CreateSecret(ctx, jobSecret)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return errors.Wrap(err, "failed to create secret copy for restore job")
		}

		// Update jfsSetting to use the copied secret
		jfsSetting.SecretName = jobSecretName
		log.Info("created secret copy in job namespace", "originalSecret", secretName, "jobSecret", jobSecretName, "namespace", config.Namespace)
	}

	// Use JobBuilder to create restore job
	jobBuilder := builder.NewJobBuilder(jfsSetting, 0)
	job := jobBuilder.NewJobForRestore(snapshotID, sourceVolumeID, targetVolumeID, secrets)

	jobName := job.Name
	log.Info("creating background restore job", "jobName", jobName, "sourceVolume", sourceVolumeID, "targetVolume", targetVolumeID, "snapshot", snapshotID)

	_, err = j.K8sClient.CreateJob(ctx, job)
	if err != nil {
		return errors.Wrap(err, "failed to create restore job")
	}

	log.Info("restore job created, will run in background", "jobName", jobName)
	return nil
}

// DeleteSnapshot deletes a snapshot from parent-level storage
func (j *juicefs) DeleteSnapshot(ctx context.Context, snapshotID, snapshotPath string, secrets map[string]string) error {
	log := util.GenLog(ctx, jfsLog, "DeleteSnapshot")
	log.Info("deleting snapshot", "snapshotID", snapshotID, "snapshotPath", snapshotPath)

	parts := strings.Split(snapshotID, "|")
	if len(parts) != 2 {
		return errors.Errorf("invalid snapshot ID format: %s, expected format: snapshot-uuid|source-volume-id", snapshotID)
	}
	actualSnapshotID := parts[0]
	sourceVolumeID := parts[1]

	log.Info("parsed snapshot ID", "actualSnapshotID", actualSnapshotID, "sourceVolumeID", sourceVolumeID)

	// Find any active mount pod to execute the deletion command
	log.Info("looking for any mount pod to execute deletion", "sourceVolumeID", sourceVolumeID)

	pods, err := j.K8sClient.ListPod(ctx, config.Namespace, nil, nil)
	if err != nil {
		return errors.Wrap(err, "failed to list pods")
	}

	var mountPod *corev1.Pod
	for i := range pods {
		if pods[i].Labels["app.kubernetes.io/name"] == "juicefs-mount" && pods[i].Status.Phase == corev1.PodRunning {
			// Prefer mount pod for source volume if we know it
			if sourceVolumeID != "" && strings.Contains(pods[i].Name, sourceVolumeID) {
				mountPod = &pods[i]
				break
			}
			// Otherwise use any running mount pod
			if mountPod == nil {
				mountPod = &pods[i]
			}
		}
	}

	if mountPod == nil {
		return errors.New("no running mount pod found to execute snapshot deletion")
	}

	log.Info("found mount pod for deletion", "podName", mountPod.Name)

	// Build deletion command for snapshot stored at filesystem root: /jfs/.snapshots/<volumeID>/<snapshotID>/
	delCmd := fmt.Sprintf("rm -rf /jfs/.snapshots/%s/%s 2>/dev/null || true", sourceVolumeID, actualSnapshotID)
	cmd := []string{"sh", "-c", delCmd}

	log.Info("executing delete command", "pod", mountPod.Name, "cmd", delCmd)

	// Execute deletion in the mount pod
	stdout, stderr, err := j.K8sClient.ExecuteInContainer(ctx, mountPod.Name, mountPod.Namespace, mountPod.Spec.Containers[0].Name, cmd)
	log.Info("delete command output", "stdout", stdout, "stderr", stderr)

	if err != nil {
		// Check if error is "not found" which is okay (idempotent)
		if strings.Contains(stderr, "No such file") || strings.Contains(stderr, "not found") {
			log.Info("snapshot already deleted or doesn't exist", "snapshotID", actualSnapshotID, "sourceVolumeID", sourceVolumeID)
			return nil
		}
		log.Error(err, "delete snapshot failed", "stdout", stdout, "stderr", stderr)
		return errors.Wrap(err, "failed to delete snapshot: "+stderr)
	}

	log.Info("snapshot deleted successfully", "snapshotID", actualSnapshotID, "sourceVolumeID", sourceVolumeID)
	return nil
}
