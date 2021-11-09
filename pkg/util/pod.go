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

package util

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"strings"
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

func GetMountPathOfPod(pod corev1.Pod) (string, string, error) {
	if len(pod.Spec.Containers) == 0 {
		return "", "", fmt.Errorf("pod %v has no container", pod.Name)
	}
	cmd := pod.Spec.Containers[0].Command
	if cmd == nil || len(cmd) < 3 {
		return "", "", fmt.Errorf("get error pod command:%v", cmd)
	}
	sourcePath, volumeId, err := ParseMntPath(cmd[2])
	if err != nil {
		return "", "", err
	}
	return sourcePath, volumeId, nil
}
