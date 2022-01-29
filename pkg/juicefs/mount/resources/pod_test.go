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

package resources

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var (
	defaultCmd       = "/bin/mount.juicefs redis://127.0.0.1:6379/0 /jfs/default-imagenet -o metrics=0.0.0.0:9567"
	defaultMountPath = "/jfs/default-imagenet"
	podLimit         = map[corev1.ResourceName]resource.Quantity{
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
			Name:      "juicefs-node-test",
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
				},
			},
			Containers: []corev1.Container{{
				Name:    "jfs-mount",
				Image:   config.MountImage,
				Command: []string{"sh", "-c", defaultCmd},
				Env: []corev1.EnvVar{{
					Name:  "JFS_FOREGROUND",
					Value: "1",
				}},
				EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "juicefs-node-test",
						},
					},
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
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Privileged: &isPrivileged,
				},
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", fmt.Sprintf(
							"if [ x$(%v) = x1 ]; then exit 0; else exit 1; fi ", "stat -c %i /jfs/default-imagenet")},
						}},
					InitialDelaySeconds: 1,
					PeriodSeconds:       1,
				},
				Lifecycle: &corev1.Lifecycle{
					PreStop: &corev1.Handler{
						Exec: &corev1.ExecAction{Command: []string{"sh", "-c", fmt.Sprintf("umount %s && rmdir %s", "/jfs/default-imagenet", "/jfs/default-imagenet")}},
					},
				},
			}},
			RestartPolicy:     corev1.RestartPolicyAlways,
			NodeName:          "node",
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

func putDefaultCacheDir(pod *corev1.Pod) {
	volume := corev1.Volume{
		Name: "jfs-default-cache",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/jfsCache",
				Type: &dir,
			},
		},
	}
	volumeMount := corev1.VolumeMount{
		Name:             "jfs-default-cache",
		MountPath:        "/var/jfsCache",
		MountPropagation: &mp,
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, volumeMount)
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
	cmdWithCacheDir3 := `/bin/mount.juicefs redis://127.0.0.1:6379/0 /jfs/default-imagenet -o cache-dir`

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

	cacheVolumes, cacheVolumeMounts = getCacheDirVolumes(cmdWithCacheDir3)
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
						Annotations: map[string]string{"a": "b", util.GetReferenceKey("aa"): "aa"},
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
			if got := HasRef(tt.args.pod); got != tt.want {
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
	config.NodeName = "node"
	config.Namespace = ""
	podLabelTest := corev1.Pod{}
	deepcopyPodFromDefault(&podLabelTest)
	podLabelTest.Labels["a"] = "b"
	podLabelTest.Labels["c"] = "d"
	putDefaultCacheDir(&podLabelTest)

	podAnnoTest := corev1.Pod{}
	deepcopyPodFromDefault(&podAnnoTest)
	podAnnoTest.Annotations = make(map[string]string)
	podAnnoTest.Annotations["a"] = "b"
	putDefaultCacheDir(&podAnnoTest)

	podSATest := corev1.Pod{}
	deepcopyPodFromDefault(&podSATest)
	podSATest.Spec.ServiceAccountName = "test"
	putDefaultCacheDir(&podSATest)

	podEnvTest := corev1.Pod{}
	deepcopyPodFromDefault(&podEnvTest)
	putDefaultCacheDir(&podEnvTest)

	podConfigTest := corev1.Pod{}
	deepcopyPodFromDefault(&podConfigTest)
	podConfigTest.Spec.Volumes = append(podConfigTest.Spec.Volumes, corev1.Volume{
		Name:         "config-1",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "secret-test"}},
	})
	podConfigTest.Spec.Containers[0].VolumeMounts = append(podConfigTest.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      "config-1",
		MountPath: "/test",
	})
	putDefaultCacheDir(&podConfigTest)

	cmdWithCacheDir := `/bin/mount.juicefs redis://127.0.0.1:6379/0 /jfs/default-imagenet -o cache-dir=/dev/shm/imagenet-0:/dev/shm/imagenet-1,cache-size=10240,metrics=0.0.0.0:9567`
	cacheVolumes, cacheVolumeMounts := getCacheDirVolumes(cmdWithCacheDir)
	podCacheTest := corev1.Pod{}
	deepcopyPodFromDefault(&podCacheTest)
	podCacheTest.Spec.Containers[0].Command = []string{"sh", "-c", cmdWithCacheDir}
	podCacheTest.Spec.Volumes = append(podCacheTest.Spec.Volumes, cacheVolumes...)
	podCacheTest.Spec.Containers[0].VolumeMounts = append(podCacheTest.Spec.Containers[0].VolumeMounts, cacheVolumeMounts...)
	type args struct {
		name           string
		cmd            string
		mountPath      string
		configs        map[string]string
		env            map[string]string
		labels         map[string]string
		annotations    map[string]string
		serviceAccount string
		options        []string
	}
	tests := []struct {
		name string
		args args
		want corev1.Pod
	}{
		{
			name: "test-labels",
			args: args{
				name:      "test",
				labels:    map[string]string{"a": "b", "c": "d"},
				cmd:       defaultCmd,
				mountPath: defaultMountPath,
			},
			want: podLabelTest,
		},
		{
			name: "test-annotation",
			args: args{
				name:        "test",
				annotations: map[string]string{"a": "b"},
				cmd:         defaultCmd,
				mountPath:   defaultMountPath,
			},
			want: podAnnoTest,
		},
		{
			name: "test-serviceaccount",
			args: args{
				name:           "test",
				serviceAccount: "test",
				cmd:            defaultCmd,
				mountPath:      defaultMountPath,
			},
			want: podSATest,
		},
		{
			name: "test-cache-dir",
			args: args{
				name:      "test",
				mountPath: defaultMountPath,
				cmd:       cmdWithCacheDir,
				options:   []string{"cache-dir=/dev/shm/imagenet-0:/dev/shm/imagenet-1", "cache-size=10240"},
			},
			want: podCacheTest,
		},
		{
			name: "test-config",
			args: args{
				name:      "test",
				cmd:       defaultCmd,
				mountPath: defaultMountPath,
				configs:   map[string]string{"secret-test": "/test"},
			},
			want: podConfigTest,
		},
		{
			name: "test-env",
			args: args{
				name:      "test",
				cmd:       defaultCmd,
				mountPath: defaultMountPath,
				env:       map[string]string{"a": "b"},
			},
			want: podEnvTest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jfsSetting := &config.JfsSetting{
				IsCe:                   true,
				Name:                   tt.args.name,
				Source:                 "redis://127.0.0.1:6379/0",
				Configs:                tt.args.configs,
				Envs:                   tt.args.env,
				MountPodLabels:         tt.args.labels,
				MountPodAnnotations:    tt.args.annotations,
				MountPodServiceAccount: tt.args.serviceAccount,
				MountPath:              tt.args.mountPath,
				VolumeId:               tt.args.name,
				Options:                tt.args.options,
			}
			got := NewMountPod(jfsSetting)
			gotStr, _ := json.Marshal(got)
			wantStr, _ := json.Marshal(tt.want)
			if string(gotStr) != string(wantStr) {
				t.Errorf("NewMountPod() = %v \n want %v", string(gotStr), string(wantStr))
			}
		})
	}
}

func TestPodMount_getCommand(t *testing.T) {
	type args struct {
		mountPath string
		options   []string
	}
	tests := []struct {
		name   string
		isCe   bool
		source string
		args   args
		want   string
	}{
		{
			name:   "test-ce",
			isCe:   true,
			source: "redis://127.0.0.1:6379/0",
			args: args{
				mountPath: "/jfs/test-volume",
				options:   []string{"debug"},
			},
			want: "/bin/mount.juicefs redis://127.0.0.1:6379/0 /jfs/test-volume -o debug,metrics=0.0.0.0:9567",
		},
		{
			name:   "test-ee",
			isCe:   false,
			source: "test",
			args: args{
				mountPath: "/jfs/test-volume",
				options:   []string{"debug"},
			},
			want: "/sbin/mount.juicefs test /jfs/test-volume -o debug,foreground",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jfsSetting := &config.JfsSetting{
				Name:      tt.name,
				Source:    tt.source,
				IsCe:      tt.isCe,
				MountPath: tt.args.mountPath,
				Options:   tt.args.options,
			}
			if got := getCommand(jfsSetting); got != tt.want {
				t.Errorf("getCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
