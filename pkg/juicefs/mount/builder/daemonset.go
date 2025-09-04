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

package builder

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

type DaemonSetBuilder struct {
	BaseBuilder
}

func NewDaemonSetBuilder(setting *config.JfsSetting, capacity int64) *DaemonSetBuilder {
	return &DaemonSetBuilder{
		BaseBuilder: BaseBuilder{
			jfsSetting: setting,
			capacity:   capacity,
		},
	}
}

// NewMountDaemonSet generates a DaemonSet with juicefs client for storage class sharing
func (d *DaemonSetBuilder) NewMountDaemonSet(dsName string) (*appsv1.DaemonSet, error) {
	// Fix tolerations for DaemonSet pods BEFORE building to ensure consistent hash
	// During node shutdown, we need the mount pods to stay alive longer than application pods
	d.ensureDaemonSetTolerations()
	
	podBuilder := NewPodBuilder(d.jfsSetting, d.capacity)
	
	// Create template pod for DaemonSet
	pod := podBuilder.genCommonJuicePod(podBuilder.genCommonContainer)
	pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	// Remove NodeName from DaemonSet pod template - DaemonSet controller manages pod placement
	pod.Spec.NodeName = ""

	// Generate mount command
	mountCmd := d.genMountCommand()
	cmd := mountCmd
	initCmd := d.genInitCommand()
	if initCmd != "" {
		cmd = strings.Join([]string{initCmd, mountCmd}, "\n")
	}
	pod.Spec.Containers[0].Command = []string{"sh", "-c", cmd}
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "JFS_FOREGROUND",
		Value: "1",
	})

	// Generate volumes and volumeMounts using PodBuilder
	volumes, volumeMounts := podBuilder.genPodVolumes()
	pod.Spec.Volumes = append(pod.Spec.Volumes, volumes...)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, volumeMounts...)

	// Add cache-dir hostpath & PVC volume
	cacheVolumes, cacheVolumeMounts := podBuilder.genCacheDirVolumes()
	pod.Spec.Volumes = append(pod.Spec.Volumes, cacheVolumes...)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, cacheVolumeMounts...)

	// Add mount path host path volume
	mountVolumes, mountVolumeMounts := podBuilder.genHostPathVolumes()
	pod.Spec.Volumes = append(pod.Spec.Volumes, mountVolumes...)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, mountVolumeMounts...)

	// Add users custom volumes, volumeMounts, volumeDevices
	if d.jfsSetting.Attr.Volumes != nil {
		pod.Spec.Volumes = append(pod.Spec.Volumes, d.jfsSetting.Attr.Volumes...)
	}
	if d.jfsSetting.Attr.VolumeMounts != nil {
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, d.jfsSetting.Attr.VolumeMounts...)
	}
	if d.jfsSetting.Attr.VolumeDevices != nil {
		pod.Spec.Containers[0].VolumeDevices = append(pod.Spec.Containers[0].VolumeDevices, d.jfsSetting.Attr.VolumeDevices...)
	}

	// Create DaemonSet
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsName,
			Namespace: config.Namespace,
			Labels: map[string]string{
				common.PodTypeKey:           common.PodTypeValue,
				common.PodUniqueIdLabelKey:  d.jfsSetting.UniqueId,
				common.PodJuiceHashLabelKey: d.jfsSetting.HashVal,
			},
			Annotations: map[string]string{},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.PodTypeKey:          common.PodTypeValue,
					common.PodUniqueIdLabelKey: d.jfsSetting.UniqueId,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.PodTypeKey:           common.PodTypeValue,
						common.PodUniqueIdLabelKey:  d.jfsSetting.UniqueId,
						common.PodJuiceHashLabelKey: d.jfsSetting.HashVal,
					},
					Annotations: map[string]string{},
				},
				Spec: pod.Spec,
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
		},
	}

	// Add node affinity if needed for specific storage class
	if d.jfsSetting.StorageClassNodeAffinity != nil {
		if ds.Spec.Template.Spec.Affinity == nil {
			ds.Spec.Template.Spec.Affinity = &corev1.Affinity{}
		}
		ds.Spec.Template.Spec.Affinity.NodeAffinity = d.jfsSetting.StorageClassNodeAffinity
	}

	return ds, nil
}

// ensureDaemonSetTolerations modifies the JfsSetting to ensure DaemonSet pods have proper tolerations
// This must be done before hash calculation to ensure consistency
func (d *DaemonSetBuilder) ensureDaemonSetTolerations() {
	if d.jfsSetting.Attr == nil {
		return
	}
	
	tolerations := d.jfsSetting.Attr.Tolerations
	
	// Check for existing tolerations to avoid duplicates
	hasTolerationAll := false
	hasNotReadyToleration := false
	hasUnreachableToleration := false
	hasDiskPressureToleration := false
	hasMemoryPressureToleration := false
	hasPidPressureToleration := false
	hasUnschedulableToleration := false
	hasNetworkUnavailableToleration := false
	
	for i := range tolerations {
		if tolerations[i].Operator == corev1.TolerationOpExists && tolerations[i].Key == "" {
			hasTolerationAll = true
		}
		if tolerations[i].Key == "node.kubernetes.io/not-ready" && tolerations[i].Effect == corev1.TaintEffectNoExecute {
			hasNotReadyToleration = true
		}
		if tolerations[i].Key == "node.kubernetes.io/unreachable" && tolerations[i].Effect == corev1.TaintEffectNoExecute {
			hasUnreachableToleration = true
		}
		if tolerations[i].Key == "node.kubernetes.io/disk-pressure" && tolerations[i].Effect == corev1.TaintEffectNoSchedule {
			hasDiskPressureToleration = true
		}
		if tolerations[i].Key == "node.kubernetes.io/memory-pressure" && tolerations[i].Effect == corev1.TaintEffectNoSchedule {
			hasMemoryPressureToleration = true
		}
		if tolerations[i].Key == "node.kubernetes.io/pid-pressure" && tolerations[i].Effect == corev1.TaintEffectNoSchedule {
			hasPidPressureToleration = true
		}
		if tolerations[i].Key == "node.kubernetes.io/unschedulable" && tolerations[i].Effect == corev1.TaintEffectNoSchedule {
			hasUnschedulableToleration = true
		}
		if tolerations[i].Key == "node.kubernetes.io/network-unavailable" && tolerations[i].Effect == corev1.TaintEffectNoSchedule {
			hasNetworkUnavailableToleration = true
		}
	}
	
	// Add missing tolerations
	if !hasTolerationAll {
		tolerations = append(tolerations, corev1.Toleration{
			Operator: corev1.TolerationOpExists,
		})
	}
	if !hasNotReadyToleration {
		tolerations = append(tolerations, corev1.Toleration{
			Key:      "node.kubernetes.io/not-ready",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		})
	}
	if !hasUnreachableToleration {
		tolerations = append(tolerations, corev1.Toleration{
			Key:      "node.kubernetes.io/unreachable",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		})
	}
	if !hasDiskPressureToleration {
		tolerations = append(tolerations, corev1.Toleration{
			Key:      "node.kubernetes.io/disk-pressure",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		})
	}
	if !hasMemoryPressureToleration {
		tolerations = append(tolerations, corev1.Toleration{
			Key:      "node.kubernetes.io/memory-pressure",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		})
	}
	if !hasPidPressureToleration {
		tolerations = append(tolerations, corev1.Toleration{
			Key:      "node.kubernetes.io/pid-pressure",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		})
	}
	if !hasUnschedulableToleration {
		tolerations = append(tolerations, corev1.Toleration{
			Key:      "node.kubernetes.io/unschedulable",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		})
	}
	if !hasNetworkUnavailableToleration {
		tolerations = append(tolerations, corev1.Toleration{
			Key:      "node.kubernetes.io/network-unavailable",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		})
	}
	
	d.jfsSetting.Attr.Tolerations = tolerations
}

// GenDaemonSetNameByUniqueId generates DaemonSet name by unique ID
func GenDaemonSetNameByUniqueId(uniqueId string) string {
	return fmt.Sprintf("juicefs-%s-mount-ds", uniqueId)
}