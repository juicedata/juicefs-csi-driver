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
	"crypto/sha256"
	"sync"

	corev1 "k8s.io/api/core/v1"
)

type PodLock struct {
	podLocks map[string]*sync.RWMutex // One pod corresponds to one lock
	lock     sync.RWMutex
}

func NewPodLock() *PodLock {
	podLocks := make(map[string]*sync.RWMutex)
	return &PodLock{
		podLocks: podLocks,
		lock:     sync.RWMutex{},
	}
}

var (
	NodeName              = ""
	Namespace             = ""
	PodName               = ""
	PodServiceAccountName = ""
	MountImage            = ""
	MountLabels           = ""
	HostIp                = ""
	KubeletPort           = ""

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

	ReconcilerInterval = 5
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

var JLock *PodLock

func (j *PodLock) GetPodLock(podName string) *sync.RWMutex {
	j.lock.Lock()
	defer j.lock.Unlock()

	h := sha256.New()
	h.Write([]byte(podName))
	key := h.Sum(nil)
	_, ok := j.podLocks[string(key)]
	if !ok {
		j.podLocks[string(key)] = &sync.RWMutex{}
	}
	return j.podLocks[string(key)]
}
