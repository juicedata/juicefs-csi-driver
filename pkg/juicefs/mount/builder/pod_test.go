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
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse"
)

var (
	defaultCmd             = "exec /bin/mount.juicefs ${metaurl} /jfs/default-imagenet -o metrics=0.0.0.0:9567"
	defaultMountPath       = "/jfs/default-imagenet"
	isPrivileged           = true
	rootUser         int64 = 0
	mp                     = corev1.MountPropagationBidirectional
	dir                    = corev1.HostPathDirectoryOrCreate
	file                   = corev1.HostPathFileOrCreate
	gracePeriod            = int64(10)
	podDefaultTest         = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "juicefs-node-test",
			Namespace: config.Namespace,
			Labels: map[string]string{
				common.PodTypeKey:          common.PodTypeValue,
				common.PodUniqueIdLabelKey: "",
			},
			Annotations: map[string]string{
				common.JuiceFSUUID: "",
				common.UniqueId:    "",
			},
			Finalizers: []string{common.Finalizer},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: JfsDirName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: config.MountPointPath,
							Type: &dir,
						},
					},
				}, {
					Name: JfsFuseFdPathName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: path.Join(JfsFuseFsPathInHost, "test"),
							Type: &dir,
						},
					},
				}, {
					Name: UpdateDBDirName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: UpdateDBCfgFile,
							Type: &file,
						},
					},
				},
			},
			Containers: []corev1.Container{{
				Name:    "jfs-mount",
				Image:   config.DefaultCEMountImage,
				Command: []string{"sh", "-c", defaultCmd},
				Env: []corev1.EnvVar{{
					Name:  "JFS_FOREGROUND",
					Value: "1",
				}, {
					Name:  JfsCommEnv,
					Value: "tmp/fuse_fd_csi_comm.sock",
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
						Name:             JfsDirName,
						MountPath:        config.PodMountBase,
						MountPropagation: &mp,
					}, {
						Name:      JfsFuseFdPathName,
						MountPath: "/tmp",
					}, {

						Name:      UpdateDBDirName,
						MountPath: UpdateDBCfgFile,
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Privileged: &isPrivileged,
					RunAsUser:  &rootUser,
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "metrics",
						ContainerPort: 9567,
					},
				},
			}},
			TerminationGracePeriodSeconds: &gracePeriod,
			RestartPolicy:                 corev1.RestartPolicyAlways,
			NodeName:                      "node",
			Hostname:                      "test",
			PriorityClassName:             config.JFSMountPriorityName,
		},
	}
)

func deepcopyPodFromDefault(pod *corev1.Pod) {
	defaultValue, _ := json.Marshal(podDefaultTest)
	defaultValueMap := make(map[string]interface{})
	_ = json.Unmarshal(defaultValue, &defaultValueMap)
	resMap := runtime.DeepCopyJSON(defaultValueMap)
	resValue, _ := json.Marshal(resMap)
	_ = json.Unmarshal(resValue, pod)
}

func Test_getCacheDirVolumes(t *testing.T) {
	optionWithoutCacheDir := []string{}
	optionWithCacheDir := []string{"cache-dir=/dev/shm/imagenet"}
	optionWithCacheDir2 := []string{"cache-dir=/dev/shm/imagenet-0:/dev/shm/imagenet-1"}
	optionWithCacheDir3 := []string{"cache-dir"}

	r := PodBuilder{
		BaseBuilder: BaseBuilder{nil, 0},
	}

	dir := corev1.HostPathDirectory
	volumeMounts := []corev1.VolumeMount{{
		Name:      JfsDirName,
		MountPath: config.PodMountBase,
	}}

	volumes := []corev1.Volume{{
		Name: JfsDirName,
		VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{
			Path: config.MountPointPath,
			Type: &dir,
		}}}}

	s, _ := config.ParseSetting(map[string]string{"name": "test"}, nil, optionWithoutCacheDir, true, nil, nil)
	s.HashVal = "test"
	r.jfsSetting = s
	cacheVolumes, cacheVolumeMounts := r.genCacheDirVolumes()
	volumes = append(volumes, cacheVolumes...)
	volumeMounts = append(volumeMounts, cacheVolumeMounts...)
	if len(volumes) != 2 || len(volumeMounts) != 2 {
		t.Error("getCacheDirVolumes can't work properly")
	}

	s, _ = config.ParseSetting(map[string]string{"name": "test"}, nil, optionWithCacheDir, true, nil, nil)
	s.HashVal = "test"
	r.jfsSetting = s
	cacheVolumes, cacheVolumeMounts = r.genCacheDirVolumes()
	volumes = append(volumes, cacheVolumes...)
	volumeMounts = append(volumeMounts, cacheVolumeMounts...)
	if len(volumes) != 3 || len(volumeMounts) != 3 {
		t.Error("getCacheDirVolumes can't work properly")
	}

	s, _ = config.ParseSetting(map[string]string{"name": "test"}, nil, optionWithCacheDir2, true, nil, nil)
	s.HashVal = "test"
	r.jfsSetting = s
	cacheVolumes, cacheVolumeMounts = r.genCacheDirVolumes()
	volumes = append(volumes, cacheVolumes...)
	volumeMounts = append(volumeMounts, cacheVolumeMounts...)
	if len(volumes) != 5 || len(volumeMounts) != 5 {
		t.Error("getCacheDirVolumes can't work properly")
	}

	s, _ = config.ParseSetting(map[string]string{"name": "test"}, nil, optionWithCacheDir3, true, nil, nil)
	s.HashVal = "test"
	r.jfsSetting = s
	cacheVolumes, cacheVolumeMounts = r.genCacheDirVolumes()
	volumes = append(volumes, cacheVolumes...)
	volumeMounts = append(volumeMounts, cacheVolumeMounts...)
	if len(volumes) != 6 || len(volumeMounts) != 6 {
		t.Error("getCacheDirVolumes can't work properly")
	}
}

func TestNewMountPod(t *testing.T) {
	defer func() { _ = os.RemoveAll("tmp") }()
	fuse.InitTestFds()
	config.NodeName = "node"
	config.Namespace = ""
	podLabelTest := corev1.Pod{}
	deepcopyPodFromDefault(&podLabelTest)
	podLabelTest.Labels["a"] = "b"
	podLabelTest.Labels["c"] = "d"

	podAnnoTest := corev1.Pod{}
	deepcopyPodFromDefault(&podAnnoTest)
	podAnnoTest.Annotations["a"] = "b"

	podSATest := corev1.Pod{}
	deepcopyPodFromDefault(&podSATest)
	podSATest.Spec.ServiceAccountName = "test"

	podEnvTest := corev1.Pod{}
	deepcopyPodFromDefault(&podEnvTest)

	podConfigTest := corev1.Pod{}
	deepcopyPodFromDefault(&podConfigTest)
	podConfigTest.Spec.Volumes = append([]corev1.Volume{{
		Name:         "config-1",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "secret-test"}},
	}}, podConfigTest.Spec.Volumes...)
	podConfigTest.Spec.Containers[0].VolumeMounts = append([]corev1.VolumeMount{{
		Name:      "config-1",
		MountPath: "/test",
	}}, podConfigTest.Spec.Containers[0].VolumeMounts...)

	s, _ := config.ParseSetting(map[string]string{"name": "test"}, nil, []string{"cache-dir=/dev/shm/imagenet-0:/dev/shm/imagenet-1", "cache-size=10240", "metrics=0.0.0.0:9567"}, true, nil, nil)
	s.HashVal = "test"
	r := PodBuilder{
		BaseBuilder: BaseBuilder{s, 0},
	}
	cmdWithCacheDir := `exec /bin/mount.juicefs ${metaurl} /jfs/default-imagenet -o cache-dir=/dev/shm/imagenet-0:/dev/shm/imagenet-1,cache-size=10240,metrics=0.0.0.0:9567`
	cacheVolumes, cacheVolumeMounts := r.genCacheDirVolumes()
	podCacheTest := corev1.Pod{}
	deepcopyPodFromDefault(&podCacheTest)
	podCacheTest.Spec.Containers[0].Command = []string{"sh", "-c", cmdWithCacheDir}
	podCacheTest.Spec.Volumes = append(podCacheTest.Spec.Volumes, cacheVolumes...)
	podCacheTest.Spec.Containers[0].VolumeMounts = append(podCacheTest.Spec.Containers[0].VolumeMounts, cacheVolumeMounts...)

	podMetricTest := corev1.Pod{}
	cmdWithMetrics := `exec /bin/mount.juicefs ${metaurl} /jfs/default-imagenet -o metrics=0.0.0.0:9999`
	deepcopyPodFromDefault(&podMetricTest)
	podMetricTest.Spec.Containers[0].Command = []string{"sh", "-c", cmdWithMetrics}
	podMetricTest.Spec.Containers[0].Ports = []corev1.ContainerPort{
		{Name: "metrics", ContainerPort: 9999},
	}

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
		cacheDirs      []string
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
				cacheDirs: []string{"/dev/shm/imagenet-0", "/dev/shm/imagenet-1"},
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
		{
			name: "test-metrics",
			args: args{
				name:      "test",
				cmd:       defaultCmd,
				mountPath: defaultMountPath,
				options:   []string{"metrics=0.0.0.0:9999"},
			},
			want: podMetricTest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podName := fmt.Sprintf("juicefs-%s-%s", config.NodeName, tt.args.name)
			jfsSetting := &config.JfsSetting{
				IsCe:       true,
				Name:       tt.args.name,
				HashVal:    "test",
				Source:     "redis://127.0.0.1:6379/0",
				Configs:    tt.args.configs,
				Envs:       tt.args.env,
				MountPath:  tt.args.mountPath,
				VolumeId:   tt.args.name,
				Options:    tt.args.options,
				CacheDirs:  tt.args.cacheDirs,
				SecretName: podName,
				Attr: &config.PodAttr{
					Labels:             tt.args.labels,
					Annotations:        tt.args.annotations,
					ServiceAccountName: tt.args.serviceAccount,
					Namespace:          config.Namespace,
					MountPointPath:     config.MountPointPath,
					JFSConfigPath:      config.JFSConfigPath,
					Image:              config.DefaultCEMountImage,
				},
			}
			r := PodBuilder{
				BaseBuilder: BaseBuilder{jfsSetting, 0},
			}
			got, _ := r.NewMountPod(podName)
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
			want: "exec /bin/mount.juicefs ${metaurl} /jfs/test-volume -o debug,metrics=0.0.0.0:9567",
		},
		{
			name:   "test-ee",
			isCe:   false,
			source: "test",
			args: args{
				mountPath: "/jfs/test-volume",
				options:   []string{"debug"},
			},
			want: "exec /sbin/mount.juicefs test /jfs/test-volume -o foreground,no-update,debug",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jfsSetting := &config.JfsSetting{
				Name:      tt.name,
				HashVal:   "test",
				Source:    tt.source,
				IsCe:      tt.isCe,
				MountPath: tt.args.mountPath,
				Options:   tt.args.options,
				Attr:      &config.PodAttr{},
			}
			r := PodBuilder{
				BaseBuilder: BaseBuilder{jfsSetting, 0},
			}
			if got := r.genMountCommand(); got != tt.want {
				t.Errorf("getCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodMount_getMetricsPort(t *testing.T) {
	type args struct {
		options []string
	}
	tests := []struct {
		name string
		args args
		want int32
	}{
		{
			name: "test-metrics",
			args: args{},
			want: int32(9567),
		},
		{
			name: "test-metrics-port",
			args: args{
				options: []string{"metrics=0.0.0.0:9999"},
			},
			want: int32(9999),
		},
		{
			name: "test-metrics-port-string",
			args: args{
				options: []string{"metrics=0.0.0.0:foo"},
			},
			want: int32(9567),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jfsSetting := &config.JfsSetting{
				Name:    tt.name,
				Options: tt.args.options,
				HashVal: "test",
			}
			r := PodBuilder{
				BaseBuilder: BaseBuilder{jfsSetting, 0},
			}
			if got := r.genMetricsPort(); got != tt.want {
				t.Errorf("getMetricsPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuilder_genHostPathVolumes(t *testing.T) {
	type fields struct {
		jfsSetting *config.JfsSetting
	}
	tests := []struct {
		name             string
		fields           fields
		wantVolumes      []corev1.Volume
		wantVolumeMounts []corev1.VolumeMount
	}{
		{
			name: "test",
			fields: fields{
				jfsSetting: &config.JfsSetting{
					HostPath: []string{"/tmp"},
				},
			},
			wantVolumes: []corev1.Volume{{
				Name: "hostpath-0",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/tmp",
					},
				},
			}},
			wantVolumeMounts: []corev1.VolumeMount{{
				Name:      "hostpath-0",
				MountPath: "/tmp",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fields.jfsSetting.HashVal = "test"
			r := &PodBuilder{
				BaseBuilder: BaseBuilder{
					jfsSetting: tt.fields.jfsSetting,
				},
			}
			gotVolumes, gotVolumeMounts := r.genHostPathVolumes()
			if !reflect.DeepEqual(gotVolumes, tt.wantVolumes) {
				t.Errorf("genHostPathVolumes() gotVolumes = %v, want %v", gotVolumes, tt.wantVolumes)
			}
			if !reflect.DeepEqual(gotVolumeMounts, tt.wantVolumeMounts) {
				t.Errorf("genHostPathVolumes() gotVolumeMounts = %v, want %v", gotVolumeMounts, tt.wantVolumeMounts)
			}
		})
	}
}
