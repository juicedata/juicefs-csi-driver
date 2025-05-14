/*
 Copyright 2023 Juicedata Inc

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

package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func IsPodReady(pod *corev1.Pod) bool {
	conditionsTrue := 0
	for _, cond := range pod.Status.Conditions {
		if cond.Status == corev1.ConditionTrue && (cond.Type == corev1.ContainersReady || cond.Type == corev1.PodReady) {
			conditionsTrue++
		}
	}
	return conditionsTrue == 2
}

func containError(statuses []corev1.ContainerStatus) bool {
	for _, status := range statuses {
		if (status.State.Waiting != nil && status.State.Waiting.Reason != "ContainerCreating") ||
			(status.State.Terminated != nil && status.State.Terminated.ExitCode != 0) {
			return true
		}
	}
	return false
}

func IsPodError(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown {
		return true
	}
	return containError(pod.Status.ContainerStatuses)
}

func IsPodComplete(pod *corev1.Pod) bool {
	var reason string
	for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
		container := pod.Status.ContainerStatuses[i]

		if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
			reason = container.State.Waiting.Reason
		} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
			reason = container.State.Terminated.Reason
		} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
			if container.State.Terminated.Signal != 0 {
				reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
			} else {
				reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
			}
		}
	}
	return reason == "Completed"
}

func IsPodResourceError(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed {
		if strings.Contains(pod.Status.Reason, "OutOf") {
			return true
		}
		if pod.Status.Reason == "UnexpectedAdmissionError" &&
			strings.Contains(pod.Status.Message, "to reclaim resources") {
			return true
		}
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Status == corev1.ConditionFalse && cond.Type == corev1.PodScheduled && cond.Reason == corev1.PodReasonUnschedulable &&
			(strings.Contains(cond.Message, "Insufficient cpu") || strings.Contains(cond.Message, "Insufficient memory")) {
			return true
		}
	}
	return false
}

func DeleteResourceOfPod(pod *corev1.Pod) {
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].Resources.Requests = nil
		pod.Spec.Containers[i].Resources.Limits = nil
	}
}

func SetRequestToZeroOfPod(pod *corev1.Pod) {
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("0"),
			corev1.ResourceMemory: resource.MustParse("0"),
		}
	}
}

func IsPodHasResource(pod corev1.Pod) bool {
	for _, cn := range pod.Spec.Containers {
		if len(cn.Resources.Requests) != 0 {
			return true
		}
	}
	return false
}

func RemoveFinalizer(ctx context.Context, client *k8sclient.K8sClient, pod *corev1.Pod, finalizer string) error {
	f := pod.GetFinalizers()
	for i := 0; i < len(f); i++ {
		if f[i] == finalizer {
			f = append(f[:i], f[i+1:]...)
			i--
		}
	}
	payload := []k8sclient.PatchListValue{{
		Op:    "replace",
		Path:  "/metadata/finalizers",
		Value: f,
	}}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		resourceLog.Error(err, "Parse json error")
		return err
	}
	if err := client.PatchPod(ctx, pod.Name, pod.Namespace, payloadBytes, types.JSONPatchType); err != nil {
		resourceLog.Error(err, "Patch pod err")
		return err
	}
	return nil
}

func AddPodLabel(ctx context.Context, client *k8sclient.K8sClient, podName, namespace string, addLabels map[string]string) error {
	log := util.GenLog(ctx, resourceLog, "AddPodLabel")
	payloads := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": addLabels,
		},
	}

	payloadBytes, err := json.Marshal(payloads)
	if err != nil {
		log.Error(err, "Parse json error")
		return err
	}
	log.V(1).Info("add labels in pod", "labels", addLabels, "pod", podName)
	if err := client.PatchPod(ctx, podName, namespace, payloadBytes, types.StrategicMergePatchType); err != nil {
		log.Error(err, "Patch pod error", "podName", podName)
		return err
	}
	return nil
}

func AddPodAnnotation(ctx context.Context, client *k8sclient.K8sClient, podName, namespace string, addAnnotations map[string]string) error {
	payloads := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": addAnnotations,
		},
	}
	payloadBytes, err := json.Marshal(payloads)
	if err != nil {
		resourceLog.Error(err, "Parse json error")
		return err
	}
	resourceLog.V(1).Info("add annotation in pod", "annotations", addAnnotations, "podName", podName)
	if err := client.PatchPod(ctx, podName, namespace, payloadBytes, types.StrategicMergePatchType); err != nil {
		resourceLog.Error(err, "Patch pod error", "podName", podName)
		return err
	}
	return nil
}

func DelPodAnnotation(ctx context.Context, client *k8sclient.K8sClient, podName, namespace string, delAnnotations []string) error {
	payloads := []k8sclient.PatchDelValue{}
	for _, k := range delAnnotations {
		payloads = append(payloads, k8sclient.PatchDelValue{
			Op:   "remove",
			Path: fmt.Sprintf("/metadata/annotations/%s", k),
		})
	}
	payloadBytes, err := json.Marshal(payloads)
	if err != nil {
		resourceLog.Error(err, "Parse json error")
		return err
	}
	resourceLog.V(1).Info("remove annotations of pod", "annotations", delAnnotations, "podName", podName)
	if err := client.PatchPod(ctx, podName, namespace, payloadBytes, types.JSONPatchType); err != nil {
		resourceLog.Error(err, "Patch pod error", "podName", podName)
		return err
	}
	return nil
}

func ReplacePodAnnotation(ctx context.Context, client *k8sclient.K8sClient, podName, namespace string, annotation map[string]string) error {
	payload := []k8sclient.PatchMapValue{{
		Op:    "replace",
		Path:  "/metadata/annotations",
		Value: annotation,
	}}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		resourceLog.Error(err, "Parse json error")
		return err
	}
	resourceLog.V(1).Info("Replace annotations of pod", "annotations", annotation, "podName", podName)
	if err := client.PatchPod(ctx, podName, namespace, payloadBytes, types.JSONPatchType); err != nil {
		resourceLog.Error(err, "Patch pod error", "podName", podName)
		return err
	}
	return nil
}

func GetAllRefKeys(pod corev1.Pod) map[string]string {
	annos := make(map[string]string)
	for k, v := range pod.Annotations {
		if k == util.GetReferenceKey(v) {
			annos[k] = v
		}
	}
	return annos
}

func WaitUtilMountReady(ctx context.Context, podName, mntPath string, timeout time.Duration) error {
	log := util.GenLog(ctx, resourceLog, "")
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	// Wait until the mount point is ready
	for {
		var finfo os.FileInfo
		if err := util.DoWithTimeout(waitCtx, timeout, func(ctx context.Context) (err error) {
			finfo, err = os.Stat(mntPath)
			return err
		}); err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				break
			}
			log.V(1).Info("Mount path is not ready, wait for it.", "mountPath", mntPath, "podName", podName, "error", err)
			time.Sleep(time.Millisecond * 500)
			continue
		}
		var dev uint64
		if st, ok := finfo.Sys().(*syscall.Stat_t); ok {
			if st.Ino == 1 {
				dev = uint64(st.Dev)
				util.DevMinorTableStore(mntPath, dev)
				log.Info("Mount point is ready", "podName", podName)
				return nil
			}
			log.V(1).Info("Mount point is not ready, wait for it", "mountPath", mntPath, "podName", podName)
		}
		time.Sleep(time.Millisecond * 500)
	}

	return fmt.Errorf("mount point is not ready eventually, mountpod: %s", podName)
}

func ShouldDelay(ctx context.Context, pod *corev1.Pod, Client *k8s.K8sClient) (shouldDelay bool, err error) {
	delayStr, delayExist := pod.Annotations[common.DeleteDelayTimeKey]
	if !delayExist {
		// not set delete delay
		return false, nil
	}
	delayAtStr, delayAtExist := pod.Annotations[common.DeleteDelayAtKey]
	if !delayAtExist {
		// need to add delayAt annotation
		d, err := util.GetTimeAfterDelay(delayStr)
		if err != nil {
			resourceLog.Error(err, "delayDelete: can't parse delay time", "time", d)
			return false, nil
		}
		addAnnotation := map[string]string{common.DeleteDelayAtKey: d}
		resourceLog.Info("delayDelete: add annotation to pod", "annotations", addAnnotation, "podName", pod.Name)
		if err := AddPodAnnotation(ctx, Client, pod.Name, pod.Namespace, addAnnotation); err != nil {
			resourceLog.Error(err, "delayDelete: Update pod error", "podName", pod.Name)
			return true, err
		}
		return true, nil
	}
	delayAt, err := util.GetTime(delayAtStr)
	if err != nil {
		resourceLog.Error(err, "delayDelete: can't parse delayAt", "delayAt", delayAtStr)
		return false, nil
	}
	return time.Now().Before(delayAt), nil
}

func GetPVWithVolumeHandleOrAppInfo(ctx context.Context, client *k8s.K8sClient, volumeHandle string, volCtx map[string]string) (*corev1.PersistentVolume, *corev1.PersistentVolumeClaim, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("k8s client is nil")
	}
	pv, err := client.GetPersistentVolume(ctx, volumeHandle)
	if k8serrors.IsNotFound(err) {
		// failed to get pv by volumeHandle, try to get pv by appName and appNamespace
		appName, appNamespace := volCtx[common.PodInfoName], volCtx[common.PodInfoNamespace]
		appPod, err := client.GetPod(ctx, appName, appNamespace)
		if err != nil {
			return nil, nil, err
		}
		for _, ref := range appPod.Spec.Volumes {
			if ref.PersistentVolumeClaim != nil {
				pvc, err := client.GetPersistentVolumeClaim(ctx, ref.PersistentVolumeClaim.ClaimName, appNamespace)
				if err != nil {
					return nil, nil, err
				}
				if pvc.Spec.VolumeName == "" {
					continue
				}
				appPV, err := client.GetPersistentVolume(ctx, pvc.Spec.VolumeName)
				if err != nil {
					return nil, nil, err
				}
				if appPV.Spec.CSI != nil && appPV.Spec.CSI.Driver == config.DriverName && appPV.Spec.CSI.VolumeHandle == volumeHandle {
					return appPV, pvc, nil
				}
			}
		}
	} else if err != nil {
		return nil, nil, err
	}

	if pv == nil {
		return nil, nil, fmt.Errorf("pv not found by volumeHandle %s", volumeHandle)
	}

	pvc, err := client.GetPersistentVolumeClaim(ctx, pv.Spec.ClaimRef.Name, pv.Spec.ClaimRef.Namespace)
	if err != nil {
		return nil, nil, err
	}
	return pv, pvc, nil
}

func GetCommPath(basePath string, pod corev1.Pod) (string, error) {
	upgradeUUID := GetUpgradeUUID(&pod)
	if upgradeUUID == "" {
		return "", fmt.Errorf("pod %s/%s has no hash label", pod.Namespace, pod.Name)
	}
	return path.Join(basePath, upgradeUUID, "fuse_fd_comm.1"), nil
}

func GetUniqueId(pod corev1.Pod) string {
	if pod.Labels[common.PodUniqueIdLabelKey] != "" {
		return pod.Labels[common.PodUniqueIdLabelKey]
	}

	// for backward compatibility
	// pod created by version before: https://github.com/juicedata/juicefs-csi-driver/pull/370
	nodeName := pod.Spec.NodeName
	uniqueId := strings.SplitN(pod.Name, fmt.Sprintf("%s-", nodeName), 2)[1]
	return uniqueId
}

func MergeEnvs(pod *corev1.Pod, env []corev1.EnvVar) {
	newEnvs := []corev1.EnvVar{}
	for _, existsEnv := range pod.Spec.Containers[0].Env {
		if _, ok := config.CSISetEnvMap[existsEnv.Name]; ok {
			if !util.ContainsEnv(env, existsEnv.Name) {
				newEnvs = append(newEnvs, existsEnv)
			}
		}
	}
	newEnvs = append(newEnvs, env...)
	pod.Spec.Containers[0].Env = newEnvs
}

func MergeMountOptions(pod *corev1.Pod, jfsSetting *config.JfsSetting) {
	newOpts := []string{}
	for _, existsOpt := range util.GetMountOptionsOfPod(pod) {
		pair := strings.Split(existsOpt, "=")
		if _, ok := config.CSISetOptsMap[pair[0]]; ok {
			if !util.ContainsPrefix(jfsSetting.Options, pair[0]) {
				newOpts = append(newOpts, existsOpt)
			}
		}
	}
	newOpts = append(newOpts, jfsSetting.Options...)

	if len(pod.Spec.Containers[0].Command) < 3 {
		return
	}
	command := strings.Split(pod.Spec.Containers[0].Command[2], "\n")
	mountCmds := strings.Fields(command[len(command)-1])

	// not valid cmd
	if len(mountCmds) < 3 || mountCmds[len(mountCmds)-2] != "-o" {
		return
	}
	mountCmds[len(mountCmds)-1] = strings.Join(newOpts, ",")
	command[len(command)-1] = strings.Join(mountCmds, " ")
	pod.Spec.Containers[0].Command[2] = strings.Join(command, "\n")
}

// MergeVolumes merges the cache volumes and volume mounts specified in the JfsSetting
// into the given pod's spec.
func MergeVolumes(pod *corev1.Pod, jfsSetting *config.JfsSetting) {
	cacheVolumes := []corev1.Volume{}
	cacheVolumeMounts := []corev1.VolumeMount{}
	hostPathType := corev1.HostPathDirectoryOrCreate
	for idx, cacheDir := range jfsSetting.CacheDirs {
		name := fmt.Sprintf("cachedir-%d", idx)
		cacheVolumes = append(cacheVolumes, corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: cacheDir,
					Type: &hostPathType,
				},
			},
		})
		cacheVolumeMounts = append(cacheVolumeMounts, corev1.VolumeMount{
			Name:      name,
			MountPath: cacheDir,
		})
	}

	for i, cache := range jfsSetting.CachePVCs {
		name := fmt.Sprintf("cachedir-pvc-%d", i)
		cacheVolumes = append(cacheVolumes, corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: cache.PVCName,
					ReadOnly:  false,
				},
			},
		})
		cacheVolumeMounts = append(cacheVolumeMounts, corev1.VolumeMount{
			Name:      name,
			ReadOnly:  false,
			MountPath: cache.Path,
		})
	}
	volumes := cacheVolumes
	for _, volume := range pod.Spec.Volumes {
		if !strings.HasPrefix(volume.Name, "cachedir-") &&
			(jfsSetting.Attr == nil || !util.ContainsVolumes(jfsSetting.Attr.Volumes, volume.Name)) {
			volumes = append(volumes, volume)
		}
	}
	vms := cacheVolumeMounts
	for _, vm := range pod.Spec.Containers[0].VolumeMounts {
		if !strings.HasPrefix(vm.Name, "cachedir-") &&
			(jfsSetting.Attr == nil || !util.ContainsVolumeMounts(jfsSetting.Attr.VolumeMounts, vm.Name)) {
			vms = append(vms, vm)
		}
	}
	vds := []corev1.VolumeDevice{}
	for i, vd := range pod.Spec.Containers[0].VolumeDevices {
		if !util.ContainsVolumeDevices(jfsSetting.Attr.VolumeDevices, vd.Name) {
			vds = append(vds, pod.Spec.Containers[0].VolumeDevices[i])
		}
	}
	if jfsSetting.Attr != nil {
		volumes = append(volumes, jfsSetting.Attr.Volumes...)
		vms = append(vms, jfsSetting.Attr.VolumeMounts...)
		vds = append(vds, jfsSetting.Attr.VolumeDevices...)
	}
	pod.Spec.Volumes = volumes
	pod.Spec.Containers[0].VolumeMounts = vms
	pod.Spec.Containers[0].VolumeDevices = vds
}

func FilterVars[T any](vars []T, excludeName string, getName func(T) string) []T {
	var filteredVars []T
	for _, v := range vars {
		if getName(v) != excludeName {
			filteredVars = append(filteredVars, v)
		}
	}
	return filteredVars
}

func FilterPodsToUpgrade(podLists corev1.PodList, recreate bool) []corev1.Pod {
	var pods = []corev1.Pod{}
	for _, pod := range podLists.Items {
		canUpgrade, reason, err := CanUpgrade(pod, recreate)
		if err != nil {
			log.Error(err, "check pod upgrade error", "pod", pod.Name)
			continue
		}
		if !canUpgrade {
			log.V(1).Info("pod can not upgrade", "pod", pod.Name, "reason", reason)
			continue
		}
		pods = append(pods, pod)
	}
	return pods
}

// CanUpgrade if the pod can be upgraded
// 1. pod has hash label
// 2. pod image support upgrade
// 3. pod is ready
func CanUpgrade(pod corev1.Pod, recreate bool) (bool, string, error) {
	if len(pod.Spec.Containers) == 0 {
		return false, fmt.Sprintf("pod %s has no container", pod.Name), nil
	}
	hashVal := pod.Labels[common.PodJuiceHashLabelKey]
	if hashVal == "" {
		return false, "pod has no hash label", nil
	}
	// check mount pod now support upgrade or not
	if !recreate && !util.ImageSupportBinary(pod.Spec.Containers[0].Image) {
		return false, fmt.Sprintf("image %s do not support smooth binary upgrade", pod.Spec.Containers[0].Image), nil
	}
	if recreate && !util.SupportFusePass(pod.Spec.Containers[0].Image) {
		return false, fmt.Sprintf("image %s do not support recreate smooth upgrade", pod.Spec.Containers[0].Image), nil
	}

	// check prestop hook
	if pod.Spec.Containers[0].Lifecycle != nil && pod.Spec.Containers[0].Lifecycle.PreStop != nil && pod.Spec.Containers[0].Lifecycle.PreStop.Exec != nil {
		prestopCmd := pod.Spec.Containers[0].Lifecycle.PreStop.Exec.Command
		for _, cmd := range prestopCmd {
			if strings.Contains(cmd, "umount") {
				return false, "mount pod has umount prestop hook, can not upgrade", nil
			}
		}
	}

	// check status
	if !IsPodReady(&pod) {
		return false, "mount pod is not ready yet", nil
	}
	return true, "", nil
}

func CanUpgradeWithHash(ctx context.Context, client *k8sclient.K8sClient, pod corev1.Pod, recreate bool) (bool, string, error) {
	return CanUpgrade(pod, recreate)
}

func GetUpgradeUUID(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}
	if pod.Labels[common.PodUpgradeUUIDLabelKey] != "" {
		return pod.Labels[common.PodUpgradeUUIDLabelKey]
	}
	return pod.Labels[common.PodJuiceHashLabelKey]
}

func HandleCorruptedMountPath(client *k8sclient.K8sClient, volumeId string, volumePath string) error {
	if config.NodeName == "" {
		resourceLog.Info("node name is empty, skip handle corrupted mount path", "volumeId", volumeId, "volumePath", volumePath)
		return nil
	}
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.PodTypeKey:          common.PodTypeValue,
			common.PodUniqueIdLabelKey: volumeId,
		},
	}
	fieldSelector := &fields.Set{
		"spec.nodeName": config.NodeName,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	mountpods, err := client.ListPod(ctx, config.Namespace, labelSelector, fieldSelector)
	if err != nil {
		return err
	}

	if len(mountpods) == 0 {
		resourceLog.Info("mount pod not found, skip handle corrupted mount path", "volumeId", volumeId, "volumePath", volumePath)
		return nil
	}

	for _, mountpod := range mountpods {
		if mountpod.Status.Phase != corev1.PodRunning {
			continue
		}
		if !mountpod.DeletionTimestamp.IsZero() {
			continue
		}
		if _, ok := mountpod.Annotations[common.ImmediateReconcilerKey]; ok {
			continue
		}
		for _, target := range mountpod.Annotations {
			if target == volumePath {
				if err := AddPodAnnotation(context.Background(), client, mountpod.Name, mountpod.Namespace,
					map[string]string{common.ImmediateReconcilerKey: time.Now().Format(time.RFC3339)}); err != nil {
					resourceLog.Error(err, "add annotation to pod error", "podName", mountpod.Name)
					return err
				}
				break
			}
		}
	}

	return nil
}
