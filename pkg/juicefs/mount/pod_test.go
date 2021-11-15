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

package mount

import (
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"testing"

	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
)

var (
	podLimit = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("2G"),
	}
	podRequest = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU:    resource.MustParse("3"),
		corev1.ResourceMemory: resource.MustParse("4G"),
	}
	testResources = corev1.ResourceRequirements{
		Limits:   podLimit,
		Requests: podRequest,
	}
	isPrivileged   = true
	mp             = corev1.MountPropagationBidirectional
	dir            = corev1.HostPathDirectoryOrCreate
	podDefaultTest = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: config.Namespace,
			Labels: map[string]string{
				config.PodTypeKey: config.PodTypeValue,
			},
			Finalizers: []string{config.Finalizer},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "jfs-dir",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: config.MountPointPath,
							Type: &dir,
						},
					},
				}, {
					Name: "jfs-root-dir",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: config.JFSConfigPath,
							Type: &dir,
						},
					},
				}, {
					Name: "jfs-default-cache",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/jfsCache",
							Type: &dir,
						},
					},
				},
			},
			Containers: []corev1.Container{{
				Name:    "jfs-mount",
				Image:   config.MountImage,
				Command: []string{"sh", "-c", ""},
				Ports: []corev1.ContainerPort{{
					Name:          "metrics",
					ContainerPort: 9567,
				}},
				Env: []corev1.EnvVar{{
					Name:  "JFS_FOREGROUND",
					Value: "1",
				}},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:             "jfs-dir",
						MountPath:        config.PodMountBase,
						MountPropagation: &mp,
					}, {
						Name:             "jfs-root-dir",
						MountPath:        "/root/.juicefs",
						MountPropagation: &mp,
					}, {
						Name:             "jfs-default-cache",
						MountPath:        "/var/jfsCache",
						MountPropagation: &mp,
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Privileged: &isPrivileged,
				},
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", fmt.Sprintf(
							"if [ x$(%v) = x1 ]; then exit 0; else exit 1; fi ", "stat -c %i ")},
						}},
					InitialDelaySeconds: 1,
					PeriodSeconds:       1,
				},
				Lifecycle: &corev1.Lifecycle{
					PreStop: &corev1.Handler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", fmt.Sprintf("umount %s", "")}},
					},
				},
			}},
			RestartPolicy:     corev1.RestartPolicyNever,
			NodeName:          config.NodeName,
			PriorityClassName: config.JFSMountPriorityName,
		},
	}
)

func deepcopyPodFromDefault(pod *corev1.Pod) {
	defaultValue, _ := json.Marshal(podDefaultTest)
	defaultValueMap := make(map[string]interface{})
	json.Unmarshal(defaultValue, &defaultValueMap)
	resMap := runtime.DeepCopyJSON(defaultValueMap)
	resValue, _ := json.Marshal(resMap)
	json.Unmarshal(resValue, pod)
}

func Test_parsePodResources(t *testing.T) {
	type args struct {
		MountPodCpuLimit   string
		MountPodMemLimit   string
		MountPodCpuRequest string
		MountPodMemRequest string
	}
	tests := []struct {
		name string
		args args
		want corev1.ResourceRequirements
	}{
		{
			name: "test",
			args: args{
				MountPodCpuLimit:   "1",
				MountPodMemLimit:   "2G",
				MountPodCpuRequest: "3",
				MountPodMemRequest: "4G",
			},
			want: testResources,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePodResources(tt.args.MountPodCpuLimit, tt.args.MountPodMemLimit, tt.args.MountPodCpuRequest, tt.args.MountPodMemRequest); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePodResources() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getCacheDirVolumes(t *testing.T) {
	cmdWithoutCacheDir := `/bin/mount.juicefs redis://127.0.0.1:6379/0 /jfs/default-imagenet`
	cmdWithCacheDir := `/bin/mount.juicefs redis://127.0.0.1:6379/0 /jfs/default-imagenet -o prefetch=1,cache-dir=/dev/shm/imagenet,cache-size=10240,open-cache=7200,metrics=0.0.0.0:9567`
	cmdWithCacheDir2 := `/bin/mount.juicefs redis://127.0.0.1:6379/0 /jfs/default-imagenet -o cache-dir=/dev/shm/imagenet-0:/dev/shm/imagenet-1,cache-size=10240,metrics=0.0.0.0:9567`

	mp := corev1.MountPropagationBidirectional
	dir := corev1.HostPathDirectory
	volumeMounts := []corev1.VolumeMount{{
		Name:             "jfs-dir",
		MountPath:        config.PodMountBase,
		MountPropagation: &mp,
	}, {
		Name:             "jfs-root-dir",
		MountPath:        "/root/.juicefs",
		MountPropagation: &mp,
	}}

	volumes := []corev1.Volume{{
		Name: "jfs-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: config.MountPointPath,
				Type: &dir,
			},
		},
	}, {
		Name: "jfs-root-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: config.JFSConfigPath,
				Type: &dir,
			},
		},
	}}

	cacheVolumes, cacheVolumeMounts := getCacheDirVolumes(cmdWithoutCacheDir)
	volumes = append(volumes, cacheVolumes...)
	volumeMounts = append(volumeMounts, cacheVolumeMounts...)
	if len(volumes) != 2 || len(volumeMounts) != 2 {
		t.Error("getCacheDirVolumes can't work properly")
	}

	cacheVolumes, cacheVolumeMounts = getCacheDirVolumes(cmdWithCacheDir)
	volumes = append(volumes, cacheVolumes...)
	volumeMounts = append(volumeMounts, cacheVolumeMounts...)
	if len(volumes) != 3 || len(volumeMounts) != 3 {
		t.Error("getCacheDirVolumes can't work properly")
	}

	cacheVolumes, cacheVolumeMounts = getCacheDirVolumes(cmdWithCacheDir2)
	volumes = append(volumes, cacheVolumes...)
	volumeMounts = append(volumeMounts, cacheVolumeMounts...)
	if len(volumes) != 5 || len(volumeMounts) != 5 {
		t.Error("getCacheDirVolumes can't work properly")
	}
}

func TestHasRef(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-true",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Annotations: map[string]string{"a": "b", "juicefs-aa": "bb"},
					},
				},
			},
			want: true,
		},
		{
			name: "test-false",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Annotations: map[string]string{"a": "b"},
					},
				},
			},
			want: false,
		},
		{
			name: "test-null",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Annotations: nil,
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasRef(tt.args.pod); got != tt.want {
				t.Errorf("HasRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_quoteForShell(t *testing.T) {
	type args struct {
		cmd string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test-(",
			args: args{
				cmd: "mysql://user@(127.0.0.1:3306)/juicefs",
			},
			want: "mysql://user@\\(127.0.0.1:3306\\)/juicefs",
		},
		{
			name: "test-none",
			args: args{
				cmd: "redis://127.0.0.1:6379/0",
			},
			want: "redis://127.0.0.1:6379/0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quoteForShell(tt.args.cmd); got != tt.want {
				t.Errorf("transformCmd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewMountPod(t *testing.T) {
	config.NodeName = ""
	config.Namespace = ""
	podLabelTest := corev1.Pod{}
	deepcopyPodFromDefault(&podLabelTest)
	podLabelTest.Labels["a"] = "b"
	podLabelTest.Labels["c"] = "d"

	podAnnoTest := corev1.Pod{}
	deepcopyPodFromDefault(&podAnnoTest)
	podAnnoTest.Annotations = make(map[string]string)
	podAnnoTest.Annotations["a"] = "b"
	type args struct {
		podName              string
		cmd                  string
		mountPath            string
		resourceRequirements corev1.ResourceRequirements
		configs              map[string]string
		env                  map[string]string
		labels               map[string]string
		annotations          map[string]string
	}
	tests := []struct {
		name string
		args args
		want corev1.Pod
	}{
		{
			name: "test-default",
			args: args{
				podName:              "test",
				cmd:                  "",
				mountPath:            "",
				resourceRequirements: corev1.ResourceRequirements{},
				configs:              nil,
				env:                  nil,
				labels:               nil,
				annotations:          nil,
			},
			want: podDefaultTest,
		},
		//{
		//	name: "test-labels",
		//	args: args{
		//		podName:              "test",
		//		cmd:                  "",
		//		mountPath:            "",
		//		resourceRequirements: corev1.ResourceRequirements{},
		//		configs:              nil,
		//		env:                  nil,
		//		labels:               map[string]string{"a": "b", "c": "d"},
		//		annotations:          nil,
		//	},
		//	want: podLabelTest,
		//},
		//{
		//	name: "test-annotation",
		//	args: args{
		//		podName:              "test",
		//		cmd:                  "",
		//		mountPath:            "",
		//		resourceRequirements: corev1.ResourceRequirements{},
		//		configs:              nil,
		//		env:                  nil,
		//		labels:               nil,
		//		annotations:          map[string]string{"a": "b"},
		//	},
		//	want: podAnnoTest,
		//},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewMountPod(tt.args.podName, tt.args.cmd, tt.args.mountPath, tt.args.resourceRequirements, tt.args.configs, tt.args.env, tt.args.labels, tt.args.annotations)
			gotStr, _ := json.Marshal(got)
			wantStr, _ := json.Marshal(tt.want)
			if string(gotStr) != string(wantStr) {
				t.Errorf("NewMountPod() = %v \n want %v", string(gotStr), string(wantStr))
			}
		})
	}
}
