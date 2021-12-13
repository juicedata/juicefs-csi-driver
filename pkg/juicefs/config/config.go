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

package config

import (
	corev1 "k8s.io/api/core/v1"
	"sync"
)

var JLock = sync.RWMutex{}

var (
	NodeName              = ""
	Namespace             = ""
	PodName               = ""
	PodServiceAccountName = ""
	MountImage            = ""
	MountLabels           = ""

	CSINodePod = corev1.Pod{}

	MountPointPath       = "/var/lib/juicefs/volume"
	JFSConfigPath        = "/var/lib/juicefs/config"
	JFSMountPriorityName = "system-node-critical"

	PodMountBase = "/jfs"
	MountBase    = "/var/lib/jfs"
	FsType       = "juicefs"
	CliPath      = "/usr/bin/juicefs"
	CeCliPath    = "/usr/local/bin/juicefs"
	CeMountPath  = "/bin/mount.juicefs"
	JfsMountPath = "/sbin/mount.juicefs"
)

const (
	PodTypeKey   = "app.kubernetes.io/name"
	PodTypeValue = "juicefs-mount"
	Finalizer    = "juicefs.com/finalizer"

	mountPodCpuLimitKey    = "juicefs/mount-cpu-limit"
	mountPodMemLimitKey    = "juicefs/mount-memory-limit"
	mountPodCpuRequestKey  = "juicefs/mount-cpu-request"
	mountPodMemRequestKey  = "juicefs/mount-memory-request"
	mountPodLabelKey       = "juicefs/mount-labels"
	mountPodAnnotationKey  = "juicefs/mount-annotations"
	mountPodServiceAccount = "juicefs/mount-service-account"
)
