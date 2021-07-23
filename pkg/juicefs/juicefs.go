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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
)

const (
	cliPath      = "/usr/bin/juicefs"
	ceCliPath    = "/usr/local/bin/juicefs"
	ceMountPath  = "/bin/mount.juicefs"
	jfsMountPath = "/sbin/mount.juicefs"
	mountBase    = "/jfs"
)

// Interface of juicefs provider
type Interface interface {
	mount.Interface
	JfsMount(volumeID string, target string, secrets, volCtx map[string]string, options []string) (Jfs, error)
	JfsUnmount(mountPath string) error
	DelRefOfMountPod(volumeId, target string) error
	AuthFs(secrets map[string]string) ([]byte, error)
	MountFs(volumeID string, target string, options []string, jfsSetting *JfsSetting) (string, error)
	Version() ([]byte, error)
	ServeMetrics(port int)
}

type juicefs struct {
	mount.SafeFormatAndMount
	metricsProxy
	K8sClient
}

var _ Interface = &juicefs{}
var JLock = sync.RWMutex{}

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
	DeleteVol(volumeID string, secrets map[string]string) error
}

var _ Jfs = &jfs{}

func (fs *jfs) GetBasePath() string {
	return fs.MountPath
}

// CreateVol creates the directory needed
func (fs *jfs) CreateVol(volumeID, subPath string) (string, error) {
	volPath := filepath.Join(fs.MountPath, subPath)

	klog.V(5).Infof("CreateVol: checking %q exists in %v", volPath, fs)
	exists, err := mount.PathExists(volPath)
	if err != nil {
		return "", status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	}
	if !exists {
		klog.V(5).Infof("CreateVol: volume not existed")
		err := os.MkdirAll(volPath, os.FileMode(0777))
		if err != nil {
			return "", status.Errorf(codes.Internal, "Could not make directory for meta %q", volPath)
		}
	}
	if fi, err := os.Stat(volPath); err != nil {
		return "", status.Errorf(codes.Internal, "Could not stat directory %s: %q", volPath, err)
	} else if fi.Mode().Perm() != 0777 { // The perm of `volPath` may not be 0777 when the umask applied
		err = os.Chmod(volPath, os.FileMode(0777))
		if err != nil {
			return "", status.Errorf(codes.Internal, "Could not chmod directory %s: %q", volPath, err)
		}
	}

	return volPath, nil
}

func (fs *jfs) DeleteVol(volumeID string, secrets map[string]string) error {
	volPath := filepath.Join(fs.MountPath, volumeID)
	if existed, err := mount.PathExists(volPath); err != nil {
		return status.Errorf(codes.Internal, "Could not check volume path %q exists: %v", volPath, err)
	} else if existed {
		_, isCeMount := secrets["metaurl"]
		stdoutStderr, err := fs.Provider.RmrDir(volPath, isCeMount)
		klog.V(5).Infof("DeleteVol: rmr output is '%s'", stdoutStderr)
		if err != nil {
			return status.Errorf(codes.Internal, "Could not delete volume path %q: %v", volPath, err)
		}
	}
	return nil
}

// NewJfsProvider creates a provider for JuiceFS file system
func NewJfsProvider(mounter *mount.SafeFormatAndMount) (Interface, error) {
	if mounter == nil {
		mounter = &mount.SafeFormatAndMount{
			Interface: mount.New(""),
			Exec:      k8sexec.New(),
		}
	}
	k8sClient, err := NewClient()
	if err != nil {
		klog.V(5).Infof("Can't get k8s client: %v", err)
		return nil, err
	}

	return &juicefs{*mounter, *newMetricsProxy(), k8sClient}, nil
}

func (j *juicefs) IsNotMountPoint(dir string) (bool, error) {
	return mount.IsNotMountPoint(j, dir)
}

// JfsMount auths and mounts JuiceFS
func (j *juicefs) JfsMount(volumeID string, target string, secrets, volCtx map[string]string, options []string) (Jfs, error) {
	jfsSecret, err := ParseSetting(secrets, volCtx)
	if err != nil {
		klog.V(5).Infof("Parse settings error: %v", err)
		return nil, err
	}
	source, isCe := secrets["metaurl"]
	var mountPath string
	if !isCe {
		j.Upgrade()
		stdoutStderr, err := j.AuthFs(secrets)
		klog.V(5).Infof("JfsMount: authentication output is '%s'\n", stdoutStderr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not auth juicefs: %v", err)
		}
		jfsSecret.Source = secrets["name"]
		mountPath, err = j.MountFs(volumeID, target, options, jfsSecret)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
		}
	} else {
		stdoutStderr, err := j.ceFormat(secrets)
		klog.V(5).Infof("JfsMount: format output is '%s'\n", stdoutStderr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not format juicefs: %v", err)
		}
		// Default use redis:// scheme
		if !strings.Contains(source, "://") {
			source = "redis://" + source
		}
		jfsSecret.Source = source
		mountPath, err = j.MountFs(volumeID, target, options, jfsSecret)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not mount juicefs: %v", err)
		}
	}

	return &jfs{
		Provider:  j,
		Name:      secrets["name"],
		MountPath: mountPath,
		Options:   options,
	}, nil
}

func (j *juicefs) JfsUnmount(mountPath string) (err error) {
	klog.V(5).Infof("JfsUnmount: umount %s", mountPath)
	if err = j.Unmount(mountPath); err != nil {
		klog.V(5).Infof("JfsUnmount: error umount %s, %v", mountPath, err)
	}
	j.mpLock.Lock()
	delete(j.mountedFs, mountPath)
	j.mpLock.Unlock()
	return
}

func (j *juicefs) DelRefOfMountPod(volumeId, target string) error {
	// check mount pod is need to delete
	klog.V(5).Infof("DeleteRefOfMountPod: Check mount pod is need to delete or not.")

	pod, err := j.GetPod(GeneratePodNameByVolumeId(volumeId), Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		klog.V(5).Infof("DeleteRefOfMountPod: Get pod of volumeId %s err: %v", volumeId, err)
		return err
	}

	// if mount pod not exists.
	if pod == nil {
		klog.V(5).Infof("DeleteRefOfMountPod: Mount pod of volumeId %v not exists.", volumeId)
		return nil
	}

	klog.V(5).Infof("DeleteRefOfMountPod: Delete target ref [%s] in pod [%s].", target, pod.Name)

	h := sha256.New()
	h.Write([]byte(target))
	key := fmt.Sprintf("juicefs-%x", h.Sum(nil))[:63]
	klog.V(5).Infof("DeleteRefOfMountPod: Target %v hash of target %v", target, key)

loop:
	po, err := j.GetPod(pod.Name, pod.Namespace)
	if err != nil {
		return err
	}
	annotation := po.Annotations
	if _, ok := annotation[key]; !ok {
		klog.V(5).Infof("DeleteRefOfMountPod: Target ref [%s] in pod [%s] already not exists.", target, pod.Name)
	} else {
		delete(annotation, key)
		klog.V(5).Infof("DeleteRefOfMountPod: Remove ref of volumeId %v, target %v", volumeId, target)
		po.Annotations = annotation
		err = j.UpdatePod(po)
		if err != nil && k8serrors.IsConflict(err) {
			// if can't update pod because of conflict, retry
			klog.V(5).Infof("DeleteRefOfMountPod: Update pod conflict, retry.")
			goto loop
		} else if err != nil {
			return err
		}
	}

	dealWithRefFunc := func(podName, namespace string) error {
		JLock.Lock()
		defer JLock.Unlock()

		po, err := j.GetPod(podName, namespace)
		if err != nil {
			return err
		}

		for _, a := range po.Annotations {
			if strings.HasPrefix(a, "juicefs-") {
				// if pod annotation is not none, ignore.
				klog.V(5).Infof("DeleteRefOfMountPod: pod still has juicefs- refs.")
				return nil
			}
		}

		klog.V(5).Infof("DeleteRefOfMountPod: Pod of volumeId %v has not refs, delete it.", volumeId)
		if err := j.DeletePod(po); err != nil {
			klog.V(5).Infof("DeleteRefOfMountPod: Delete pod of volumeId %s error: %v", volumeId, err)
			return err
		}
		return nil
	}

	newPod, err := j.GetPod(pod.Name, pod.Namespace)
	if err != nil {
		return err
	}
	for _, a := range newPod.Annotations {
		if strings.HasPrefix(a, "juicefs-") {
			klog.V(5).Infof("DeleteRefOfMountPod: pod still has juicefs- refs.")
			return nil
		}
	}
	klog.V(5).Infof("DeleteRefOfMountPod: pod has no juicefs- refs.")
	// if pod annotations has no "juicefs-" prefix, delete pod
	return dealWithRefFunc(pod.Name, pod.Namespace)
}

func (j *juicefs) RmrDir(directory string, isCeMount bool) ([]byte, error) {
	klog.V(5).Infof("RmrDir: removing directory recursively: %q", directory)
	if isCeMount {
		return j.Exec.Command(ceCliPath, "rmr", directory).CombinedOutput()
	}
	return j.Exec.Command("rm", "-rf", directory).CombinedOutput()
}

// AuthFs authenticates JuiceFS, enterprise edition only
func (j *juicefs) AuthFs(secrets map[string]string) ([]byte, error) {
	if secrets == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Nil secrets")
	}

	if secrets["name"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty name")
	}

	if secrets["token"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty token")
	}

	args := []string{"auth", secrets["name"]}
	argsStripped := []string{"auth", secrets["name"]}
	keys := []string{
		"accesskey",
		"accesskey2",
		"bucket",
		"bucket2",
	}
	keysStripped := []string{
		"token",
		"secretkey",
		"secretkey2",
		"passphrase"}
	isOptional := map[string]bool{
		"accesskey2": true,
		"secretkey2": true,
		"bucket":     true,
		"bucket2":    true,
		"passphrase": true,
	}
	for _, k := range keys {
		if !isOptional[k] || secrets[k] != "" {
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
			argsStripped = append(argsStripped, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	for _, k := range keysStripped {
		if !isOptional[k] || secrets[k] != "" {
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
			argsStripped = append(argsStripped, fmt.Sprintf("--%s=[secret]", k))
		}
	}
	if v, ok := os.LookupEnv("JFS_NO_UPDATE_CONFIG"); ok && v == "enabled" {
		args = append(args, "--no-update")
		argsStripped = append(argsStripped, "--no-update")

		if secrets["bucket"] == "" {
			return nil, status.Errorf(codes.InvalidArgument,
				"bucket argument is required when --no-update option is provided")
		}
		if secrets["initconfig"] != "" {
			conf := secrets["name"] + ".conf"
			confPath := filepath.Join("/root/.juicefs", conf)
			if _, err := os.Stat(confPath); os.IsNotExist(err) {
				err = ioutil.WriteFile(confPath, []byte(secrets["initconfig"]), 0644)
				if err != nil {
					return nil, status.Errorf(codes.Internal,
						"Create config file %q failed: %v", confPath, err)
				}
				klog.V(5).Infof("Create config file: %q success", confPath)
			}
		}
	}
	klog.V(5).Infof("AuthFs: cmd %q, args %#v", cliPath, argsStripped)
	return j.Exec.Command(cliPath, args...).CombinedOutput()
}

// MountFs mounts JuiceFS with idempotency
func (j *juicefs) MountFs(volumeID, target string, options []string, jfsSetting *JfsSetting) (string, error) {
	mountPath := filepath.Join(mountBase, volumeID)

	exists, err := mount.PathExists(mountPath)
	if err != nil && mount.IsCorruptedMnt(err) {
		klog.V(5).Infof("MountFs: %s is a corrupted mountpoint, unmounting", mountPath)
		if err = j.Unmount(mountPath); err != nil {
			klog.V(5).Infof("Unmount corrupted mount point %s failed: %v", mountPath, err)
			return mountPath, err
		}
	} else if err != nil {
		return mountPath, status.Errorf(codes.Internal, "Could not check mount point %q exists: %v", mountPath, err)
	}

	if !exists {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", jfsSetting.Source, mountPath, options)
		err = j.jMount(volumeID, mountPath, target, options, jfsSetting)
		if err != nil {
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", jfsSetting.Source, mountPath, err)
		}
		return mountPath, nil
	}

	// path exists
	notMnt, err := j.IsLikelyNotMountPoint(mountPath)
	if err != nil {
		return mountPath, status.Errorf(codes.Internal, "Could not check %q IsLikelyNotMountPoint: %v", mountPath, err)
	}

	if notMnt {
		klog.V(5).Infof("Mount: mounting %q at %q with options %v", jfsSetting.Source, mountPath, options)
		err = j.jMount(volumeID, mountPath, target, options, jfsSetting)
		if err != nil {
			return "", status.Errorf(codes.Internal, "Could not mount %q at %q: %v", jfsSetting.Source, mountPath, err)
		}
		return mountPath, nil
	}

	klog.V(5).Infof("Mount: skip mounting for existing mount point %q", mountPath)

	pod, err := j.K8sClient.GetPod(GeneratePodNameByVolumeId(volumeID), Namespace)
	if err != nil {
		klog.V(5).Infof("Can't find pod of volumeId %s but mount point %q already exist.", volumeID, mountPath)
		return mountPath, err
	}
	klog.V(5).Infof("Mount: add mount ref of configMap of volumeId %q", volumeID)
	err = j.addRefOfMount(target, pod)
	return mountPath, err
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

	err := exec.CommandContext(ctx, cliPath, "version", "-u").Run()
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
	return j.Exec.Command(cliPath, "version").CombinedOutput()
}

func (j *juicefs) ceFormat(secrets map[string]string) ([]byte, error) {
	if secrets == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Nil secrets")
	}

	if secrets["name"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty name")
	}

	if secrets["metaurl"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty metaurl")
	}

	args := []string{"format", "--no-update"}
	argsStripped := []string{"format"}
	keys := []string{
		"storage",
		"bucket",
		"access-key",
		"block-size",
		"compress",
	}
	keysStripped := []string{"secret-key"}
	isOptional := map[string]bool{
		"block-size": true,
		"compress":   true,
	}
	for _, k := range keys {
		if !isOptional[k] || secrets[k] != "" {
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
			argsStripped = append(argsStripped, fmt.Sprintf("--%s=%s", k, secrets[k]))
		}
	}
	for _, k := range keysStripped {
		if !isOptional[k] || secrets[k] != "" {
			args = append(args, fmt.Sprintf("--%s=%s", k, secrets[k]))
			argsStripped = append(argsStripped, fmt.Sprintf("--%s=[secret]", k))
		}
	}
	args = append(args, secrets["metaurl"], secrets["name"])
	argsStripped = append(argsStripped, "[metaurl]", secrets["name"])
	klog.V(5).Infof("ceFormat: cmd %q, args %#v", ceCliPath, argsStripped)
	return j.Exec.Command(ceCliPath, args...).CombinedOutput()
}

func (j *juicefs) jMount(volumeId, mountPath string, target string, options []string, jfsSetting *JfsSetting) error {
	cmd := ""
	if jfsSetting.IsCe {
		klog.V(5).Infof("ceMount: mount %v at %v", jfsSetting.Source, mountPath)
		mountArgs := []string{ceMountPath, jfsSetting.Source, mountPath}
		options = append(options, "metrics=0.0.0.0:9567")
		mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
		cmd = strings.Join(mountArgs, " ")
	} else {
		klog.V(5).Infof("Mount: mount %v at %v", jfsSetting.Source, mountPath)
		mountArgs := []string{jfsMountPath, jfsSetting.Source, mountPath}
		options = append(options, "foreground")
		if len(options) > 0 {
			mountArgs = append(mountArgs, "-o", strings.Join(options, ","))
		}
		cmd = strings.Join(mountArgs, " ")
	}

	if exist, err := mount.PathExists(mountPath); err != nil {
		return status.Errorf(codes.Internal, "Could not check existence of dir %q: %v", mountPath, err)
	} else if !exist {
		if err = os.MkdirAll(mountPath, os.FileMode(0755)); err != nil {
			return status.Errorf(codes.Internal, "Could not create dir %q: %v", mountPath, err)
		}
	}

	if notMounted, err := j.IsLikelyNotMountPoint(mountPath); err != nil {
		return err
	} else if !notMounted {
		err = j.Unmount(mountPath)
		if err != nil {
			klog.V(5).Infof("Unmount before mount failed: %v", err)
			return err
		}
		klog.V(5).Infof("Unmount %v", mountPath)
	}

	return j.waitUntilMount(volumeId, target, mountPath, cmd, jfsSetting)
}

func (j *juicefs) waitUntilMount(volumeId, target, mountPath, cmd string, jfsSetting *JfsSetting) error {
	podName := GeneratePodNameByVolumeId(volumeId)
	klog.V(5).Infof("waitUtilMount: Mount pod cmd: %v", cmd)
	podResource := parsePodResources(
		jfsSetting.MountPodCpuLimit,
		jfsSetting.MountPodMemLimit,
		jfsSetting.MountPodCpuRequest,
		jfsSetting.MountPodMemRequest,
	)

	h := sha256.New()
	h.Write([]byte(target))
	key := fmt.Sprintf("juicefs-%x", h.Sum(nil))[:63]
	_, err := j.K8sClient.GetPod(podName, Namespace)
	if err != nil && k8serrors.IsNotFound(err) {
		// need create
		klog.V(5).Infof("waitUtilMount: Need to create pod %s.", podName)
		newPod := NewMountPod(podName, cmd, mountPath, podResource, jfsSetting.Configs, jfsSetting.Envs)
		if newPod.Annotations == nil {
			newPod.Annotations = make(map[string]string)
		}
		newPod.Annotations[key] = target
		if _, e := j.K8sClient.CreatePod(newPod); e != nil && k8serrors.IsAlreadyExists(e) {
			// add ref of pod when pod exists
			klog.V(5).Infof("waitUtilMount: Pod %s already exist.", podName)
			exist, err := j.K8sClient.GetPod(podName, Namespace)
			if err != nil {
				return err
			}
			klog.V(5).Infof("waitUtilMount: add mount ref in pod of volumeId %q", volumeId)
			if err = j.addRefOfMount(target, exist); err != nil {
				return err
			}
		} else if e != nil {
			return e
		}
	} else if err != nil {
		return err
	}

	// create pod successfully
	// Wait until the mount pod is ready
	for i := 0; i < 30; i++ {
		pod, err := j.K8sClient.GetPod(podName, Namespace)
		if err != nil {
			return status.Errorf(codes.Internal, "waitUtilMount: Get pod %v failed: %v", volumeId, err)
		}
		if util.IsPodReady(pod) {
			klog.V(5).Infof("waitUtilMount: Pod %v is successful", volumeId)
			// add volumeId ref in configMap
			klog.V(5).Infof("waitUtilMount: add mount ref in pod of volumeId %q", volumeId)
			return j.addRefOfMount(target, pod)
		} else if util.IsPodResourceError(pod) {
			klog.V(5).Infof("waitUtilMount: Pod is failed because of resource.")
			if !util.IsPodHasResource(*pod) {
				return status.Errorf(codes.Internal, "Pod %v is failed", volumeId)
			}

			// if pod is failed because of resource, delete resource and deploy pod again.
			klog.V(5).Infof("waitUtilMount: Delete it and deploy again with no resource.")
			if err := j.K8sClient.DeletePod(pod); err != nil {
				return status.Errorf(codes.Internal, "Can't delete Pod %v", volumeId)
			}

			time.Sleep(time.Second * 5)
			newPod := NewMountPod(podName, cmd, mountPath, podResource, jfsSetting.Configs, jfsSetting.Envs)
			newPod.Annotations = pod.Annotations
			util.DeleteResourceOfPod(newPod)
			klog.V(5).Infof("waitUtilMount: Deploy again with no resource.")
			if _, err := j.K8sClient.CreatePod(newPod); err != nil {
				return status.Errorf(codes.Internal, "waitUtilMount: Can't create Pod %v", volumeId)
			}
		}
		time.Sleep(time.Millisecond * 500)
	}
	return status.Errorf(codes.Internal, "Mount %v failed: mount pod isn't ready in 15 seconds", volumeId)
}

func (j *juicefs) addRefOfMount(target string, pod *corev1.Pod) error {
	// add volumeId ref in pod annotation
	// mount target hash as key
	h := sha256.New()
	h.Write([]byte(target))
	key := fmt.Sprintf("juicefs-%x", h.Sum(nil))[:63]

	JLock.Lock()
	defer JLock.Unlock()

	annotation := pod.Annotations
	if _, ok := annotation[key]; ok {
		klog.V(5).Infof("addRefOfMount: Target ref [%s] in pod [%s] already exists.", target, pod.Name)
		return nil
	}
	patchBody := make(map[string]interface{})
	patchBody["metadata"] = map[string]map[string]string{"annotations": {key: target}}
	payloadBytes, _ := json.Marshal(patchBody)
	klog.V(5).Infof("addRefOfMount: Add target ref in mount pod. mount pod: [%s], target: [%s]", pod.Name, target)
	if err := j.K8sClient.PatchPod(pod, payloadBytes); err != nil && k8serrors.IsConflict(err) {
		klog.V(5).Infof("addRefOfMount: Patch pod %s error: %v", pod.Name, err)
		return err
	}
	return nil
}

func (j *juicefs) ceCheckMetrics(name, mountPath string, metricsPort int) {
	j.mpLock.Lock()
	defer j.mpLock.Unlock()
	// If the mountPath already exist, it means mount is skipped in MountFs()
	if _, ok := j.mountedFs[mountPath]; !ok {
		j.mountedFs[mountPath] = &mountInfo{
			Name:        name,
			MetricsPort: metricsPort,
		}
	}
}

func (j *juicefs) ServeMetrics(port int) {
	http.HandleFunc("/metrics", j.serveMetricsHTTP)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		klog.V(5).Infof("Start metrics server :%d failed: %q", port, err)
	}
}
