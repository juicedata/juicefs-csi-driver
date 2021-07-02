package util

import (
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

func IsPodError(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown {
		return true
	}
	conditionsFalse := 0
	for _, cond := range pod.Status.Conditions {
		if pod.Status.Phase == corev1.PodRunning && cond.Status == corev1.ConditionFalse && (cond.Type == corev1.ContainersReady || cond.Type == corev1.PodReady) {
			conditionsFalse++
		}
	}
	return conditionsFalse == 2
}

func IsPodResourceError(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed && strings.Contains(pod.Status.Reason, "OutOf") {
		return true
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
	for i, cn := range pod.Spec.Containers {
		cn.Resources = corev1.ResourceRequirements{}
		pod.Spec.Containers[i] = cn
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
