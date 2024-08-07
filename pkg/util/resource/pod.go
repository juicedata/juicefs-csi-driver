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
	"strings"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

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
		klog.Errorf("Parse json error: %v", err)
		return err
	}
	if err := client.PatchPod(ctx, pod, payloadBytes, types.JSONPatchType); err != nil {
		klog.Errorf("Patch pod err:%v", err)
		return err
	}
	return nil
}

func AddPodLabel(ctx context.Context, client *k8sclient.K8sClient, pod *corev1.Pod, addLabels map[string]string) error {
	payloads := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": addLabels,
		},
	}

	payloadBytes, err := json.Marshal(payloads)
	if err != nil {
		klog.Errorf("Parse json error: %v", err)
		return err
	}
	klog.V(6).Infof("AddPodLabel: %v in pod %s", addLabels, pod.Name)
	if err := client.PatchPod(ctx, pod, payloadBytes, types.StrategicMergePatchType); err != nil {
		klog.Errorf("Patch pod %s error: %v", pod.Name, err)
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
		klog.Errorf("Parse json error: %v", err)
		return err
	}
	klog.V(6).Infof("AddPodAnnotation: %v in pod %s", addAnnotations, pod.Name)
	if err := client.PatchPod(ctx, pod, payloadBytes, types.StrategicMergePatchType); err != nil {
		klog.Errorf("Patch pod %s error: %v", pod.Name, err)
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
		klog.Errorf("Parse json error: %v", err)
		return err
	}
	klog.V(6).Infof("Remove annotations: %v of pod %s", delAnnotations, pod.Name)
	if err := client.PatchPod(ctx, pod, payloadBytes, types.JSONPatchType); err != nil {
		klog.Errorf("Patch pod %s error: %v", pod.Name, err)
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
		klog.Errorf("Parse json error: %v", err)
		return err
	}
	klog.V(6).Infof("Replace annotations: %v of pod %s", annotation, pod.Name)
	if err := client.PatchPod(ctx, pod, payloadBytes, types.JSONPatchType); err != nil {
		klog.Errorf("Patch pod %s error: %v", pod.Name, err)
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
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	// Wait until the mount point is ready
	klog.V(5).Infof("waiting for mount point %v ready, mountpod: %s", mntPath, podName)
	for {
		var finfo os.FileInfo
		if err := util.DoWithTimeout(waitCtx, timeout, func() (err error) {
			finfo, err = os.Stat(mntPath)
			return err
		}); err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				break
			}
			klog.V(6).Infof("Mount path %v is not ready, mountpod: %s, err: %v", mntPath, podName, err)
			time.Sleep(time.Millisecond * 500)
			continue
		}
		var dev uint64
		if st, ok := finfo.Sys().(*syscall.Stat_t); ok {
			if st.Ino == 1 {
				dev = uint64(st.Dev)
				util.DevMinorTableStore(mntPath, dev)
				klog.V(5).Infof("Mount point %v is ready, mountpod: %s", mntPath, podName)
				return nil
			}
			klog.V(6).Infof("Mount point %v is not ready, mountpod: %s", mntPath, podName)
		}
		time.Sleep(time.Millisecond * 500)
	}

	return fmt.Errorf("mount point %v is not ready, mountpod: %s", mntPath, podName)
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
	delayAt, err := util.GetTime(delayAtStr)
	if err != nil {
		klog.Errorf("delayDelete: can't parse delayAt %s: %v", delayAtStr, err)
		return false, nil
	}
	return time.Now().Before(delayAt), nil
}
