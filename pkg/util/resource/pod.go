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
	"k8s.io/apimachinery/pkg/types"

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
	if err := client.PatchPod(ctx, pod, payloadBytes, types.JSONPatchType); err != nil {
		resourceLog.Error(err, "Patch pod err")
		return err
	}
	return nil
}

func AddPodLabel(ctx context.Context, client *k8sclient.K8sClient, pod *corev1.Pod, addLabels map[string]string) error {
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
	log.V(1).Info("add labels in pod", "labels", addLabels, "pod", pod.Name)
	if err := client.PatchPod(ctx, pod, payloadBytes, types.StrategicMergePatchType); err != nil {
		log.Error(err, "Patch pod error", "podName", pod.Name)
		return err
	}
	return nil
}

func AddPodAnnotation(ctx context.Context, client *k8sclient.K8sClient, pod *corev1.Pod, addAnnotations map[string]string) error {
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
	resourceLog.V(1).Info("add annotation in pod", "annotations", addAnnotations, "podName", pod.Name)
	if err := client.PatchPod(ctx, pod, payloadBytes, types.StrategicMergePatchType); err != nil {
		resourceLog.Error(err, "Patch pod error", "podName", pod.Name)
		return err
	}
	return nil
}

func DelPodAnnotation(ctx context.Context, client *k8sclient.K8sClient, pod *corev1.Pod, delAnnotations []string) error {
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
	resourceLog.V(1).Info("remove annotations of pod", "annotations", delAnnotations, "podName", pod.Name)
	if err := client.PatchPod(ctx, pod, payloadBytes, types.JSONPatchType); err != nil {
		resourceLog.Error(err, "Patch pod error", "podName", pod.Name)
		return err
	}
	return nil
}

func ReplacePodAnnotation(ctx context.Context, client *k8sclient.K8sClient, pod *corev1.Pod, annotation map[string]string) error {
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
	resourceLog.V(1).Info("Replace annotations of pod", "annotations", annotation, "podName", pod.Name)
	if err := client.PatchPod(ctx, pod, payloadBytes, types.JSONPatchType); err != nil {
		resourceLog.Error(err, "Patch pod error", "podName", pod.Name)
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
	log.Info("waiting for mount point ready", "podName", podName)
	for {
		var finfo os.FileInfo
		if err := util.DoWithTimeout(waitCtx, timeout, func() (err error) {
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
	delayStr, delayExist := pod.Annotations[config.DeleteDelayTimeKey]
	if !delayExist {
		// not set delete delay
		return false, nil
	}
	delayAtStr, delayAtExist := pod.Annotations[config.DeleteDelayAtKey]
	if !delayAtExist {
		// need to add delayAt annotation
		d, err := util.GetTimeAfterDelay(delayStr)
		if err != nil {
			resourceLog.Error(err, "delayDelete: can't parse delay time", "time", d)
			return false, nil
		}
		addAnnotation := map[string]string{config.DeleteDelayAtKey: d}
		resourceLog.Info("delayDelete: add annotation to pod", "annotations", addAnnotation, "podName", pod.Name)
		if err := AddPodAnnotation(ctx, Client, pod, addAnnotation); err != nil {
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
	if len(args) < 3 || !strings.HasPrefix(args[2], config.PodMountBase) {
		return "", "", fmt.Errorf("err cmd:%s", cmd)
	}
	argSlice := strings.Split(args[2], "/")
	if len(argSlice) < 3 {
		return "", "", fmt.Errorf("err mntPath:%s", args[2])
	}
	return args[2], argSlice[2], nil
}

func GetPVWithVolumeHandleOrAppInfo(ctx context.Context, client *k8s.K8sClient, volumeHandle string, volCtx map[string]string) (*corev1.PersistentVolume, *corev1.PersistentVolumeClaim, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("k8s client is nil")
	}
	pv, err := client.GetPersistentVolume(ctx, volumeHandle)
	if k8serrors.IsNotFound(err) {
		// failed to get pv by volumeHandle, try to get pv by appName and appNamespace
		appName, appNamespace := volCtx[config.PodInfoName], volCtx[config.PodInfoNamespace]
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
	hashVal := pod.Labels[config.PodJuiceHashLabelKey]
	if hashVal == "" {
		return "", fmt.Errorf("pod %s/%s has no hash label", pod.Namespace, pod.Name)
	}
	return path.Join(basePath, hashVal, "fuse_fd_comm.1"), nil
}
