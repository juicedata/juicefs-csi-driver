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
	"encoding/json"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"
	"testing"
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
)

func TestParseSecret(t *testing.T) {
	s := map[string]string{
		"GOOGLE_APPLICATION_CREDENTIALS": "/root/.config/gcloud/application_default_credentials.json",
		"a":                              "b",
		"c":                              "d",
	}

	type args struct {
		secrets     map[string]string
		volCtx      map[string]string
		options     []string
		usePod      bool
		MountLabels string
	}
	tests := []struct {
		name    string
		args    args
		want    *JfsSetting
		wantErr bool
	}{
		{
			name:    "test-nil",
			args:    args{},
			want:    &JfsSetting{Options: []string{}},
			wantErr: false,
		},
		{
			name:    "test-no-name",
			args:    args{secrets: map[string]string{}},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-env",
			args: args{
				secrets: map[string]string{"name": "test", "envs": "{GOOGLE_APPLICATION_CREDENTIALS: \"/root/.config/gcloud/application_default_credentials.json\", a: b, c: d}"},
				usePod:  true,
			},
			want: &JfsSetting{
				Name:      "test",
				UsePod:    true,
				Source:    "test",
				CachePVCs: []CachePVC{},
				CacheDirs: []string{"/var/jfsCache"},
				Envs:      s,
				Configs:   map[string]string{},
				Options:   []string{},
				Resources: corev1.ResourceRequirements{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-env-error",
			args: args{
				secrets: map[string]string{"name": "test", "envs": "-"},
				usePod:  true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-storage-nil",
			args: args{secrets: map[string]string{"name": "test"}, usePod: true},
			want: &JfsSetting{
				Name:      "test",
				Source:    "test",
				Configs:   map[string]string{},
				Envs:      map[string]string{},
				Options:   []string{},
				UsePod:    true,
				CacheDirs: []string{"/var/jfsCache"},
				CachePVCs: []CachePVC{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-storage",
			args: args{secrets: map[string]string{"name": "test", "storage": "ceph"}, usePod: true},
			want: &JfsSetting{
				Name:      "test",
				Source:    "test",
				Storage:   "ceph",
				Configs:   map[string]string{},
				Envs:      map[string]string{},
				UsePod:    true,
				Options:   []string{},
				CacheDirs: []string{"/var/jfsCache"},
				CachePVCs: []CachePVC{},
				Resources: corev1.ResourceRequirements{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-cpu-limit",
			args: args{
				secrets: map[string]string{"name": "test", "storage": "s3"},
				volCtx:  map[string]string{mountPodCpuLimitKey: "1"},
				usePod:  true,
			},
			want: &JfsSetting{
				Name:      "test",
				Source:    "test",
				Storage:   "s3",
				UsePod:    true,
				Configs:   map[string]string{},
				Envs:      map[string]string{},
				Options:   []string{},
				CacheDirs: []string{"/var/jfsCache"},
				CachePVCs: []CachePVC{},
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-mem-limit",
			args: args{
				secrets: map[string]string{"name": "test", "storage": "s3"},
				volCtx:  map[string]string{mountPodMemLimitKey: "1G"},
				usePod:  true,
			},
			want: &JfsSetting{
				Name:      "test",
				Source:    "test",
				Storage:   "s3",
				UsePod:    true,
				Configs:   map[string]string{},
				Envs:      map[string]string{},
				Options:   []string{},
				CacheDirs: []string{"/var/jfsCache"},
				CachePVCs: []CachePVC{},
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceMemory: resource.MustParse("1G"),
					},
				},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-mem-request",
			args: args{
				secrets: map[string]string{"name": "test", "storage": "s3"},
				volCtx:  map[string]string{mountPodMemRequestKey: "1G"},
				usePod:  true,
			},
			want: &JfsSetting{
				Name:      "test",
				Source:    "test",
				Storage:   "s3",
				UsePod:    true,
				Configs:   map[string]string{},
				Envs:      map[string]string{},
				Options:   []string{},
				CacheDirs: []string{"/var/jfsCache"},
				CachePVCs: []CachePVC{},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceMemory: resource.MustParse("1G"),
					},
				},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-cpu-request",
			args: args{
				secrets: map[string]string{"name": "test"},
				volCtx:  map[string]string{mountPodCpuRequestKey: "1"},
			},
			want: &JfsSetting{
				Name:      "test",
				Source:    "test",
				Configs:   map[string]string{},
				Envs:      map[string]string{},
				Options:   []string{},
				CacheDirs: []string{"/var/jfsCache"},
				CachePVCs: []CachePVC{},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-labels",
			args: args{
				secrets: map[string]string{"name": "test"},
				volCtx:  map[string]string{mountPodLabelKey: "a: b"},
			},
			want: &JfsSetting{
				Name:           "test",
				Source:         "test",
				MountPodLabels: map[string]string{"a": "b"},
				Configs:        map[string]string{},
				Envs:           map[string]string{},
				Options:        []string{},
				CacheDirs:      []string{"/var/jfsCache"},
				CachePVCs:      []CachePVC{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-labels-error",
			args: args{
				secrets: map[string]string{"name": "test"},
				volCtx:  map[string]string{mountPodLabelKey: "-"},
			},
			wantErr: true,
		},
		{
			name: "test-labels-json",
			args: args{
				secrets: map[string]string{"name": "test"},
				volCtx:  map[string]string{mountPodLabelKey: "{\"a\": \"b\"}"},
			},
			want: &JfsSetting{
				Name:           "test",
				Source:         "test",
				MountPodLabels: map[string]string{"a": "b"},
				Configs:        map[string]string{},
				Envs:           map[string]string{},
				Options:        []string{},
				CacheDirs:      []string{"/var/jfsCache"},
				CachePVCs:      []CachePVC{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-annotation",
			args: args{
				secrets: map[string]string{"name": "test"},
				volCtx:  map[string]string{mountPodAnnotationKey: "a: b"},
			},
			want: &JfsSetting{
				Name:                "test",
				Source:              "test",
				MountPodAnnotations: map[string]string{"a": "b"},
				Configs:             map[string]string{},
				Envs:                map[string]string{},
				Options:             []string{},
				CacheDirs:           []string{"/var/jfsCache"},
				CachePVCs:           []CachePVC{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-annotation-error",
			args: args{
				secrets: map[string]string{"name": "test"},
				volCtx:  map[string]string{mountPodAnnotationKey: "-"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-serviceaccount",
			args: args{
				secrets: map[string]string{"name": "test", "storage": "s3"},
				volCtx:  map[string]string{mountPodServiceAccount: "test"},
				usePod:  true,
			},
			want: &JfsSetting{
				UsePod:             true,
				Name:               "test",
				Source:             "test",
				Storage:            "s3",
				ServiceAccountName: "test",
				Configs:            map[string]string{},
				Envs:               map[string]string{},
				Options:            []string{},
				CacheDirs:          []string{"/var/jfsCache"},
				CachePVCs:          []CachePVC{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-config",
			args: args{secrets: map[string]string{"configs": "a: b", "name": "test"}},
			want: &JfsSetting{
				Name:      "test",
				Source:    "test",
				Configs:   map[string]string{"a": "b"},
				Envs:      map[string]string{},
				Options:   []string{},
				CacheDirs: []string{"/var/jfsCache"},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
				CachePVCs: []CachePVC{},
			},
			wantErr: false,
		},
		{
			name:    "test-config-error",
			args:    args{secrets: map[string]string{"configs": "-", "name": "test"}},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-mountLabel",
			args: args{
				secrets:     map[string]string{"name": "test"},
				MountLabels: "{a: b, c: d, e: f}",
			},
			want: &JfsSetting{
				Name:           "test",
				Source:         "test",
				MountPodLabels: map[string]string{"a": "b", "c": "d", "e": "f"},
				Configs:        map[string]string{},
				Envs:           map[string]string{},
				Options:        []string{},
				CacheDirs:      []string{"/var/jfsCache"},
				CachePVCs:      []CachePVC{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-mountLabel-error",
			args: args{
				secrets:     map[string]string{"name": "test"},
				MountLabels: "-",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-parse-secret",
			args: args{
				secrets: map[string]string{
					"name":            "abc",
					"token":           "abc",
					"secret-key":      "abc",
					"secret-key2":     "abc",
					"passphrase":      "abc",
					"encrypt_rsa_key": "abc",
					"initconfig":      "abc",
					"format-options":  "xxx",
				},
			},
			want: &JfsSetting{
				Name:          "abc",
				Source:        "abc",
				SecretKey:     "abc",
				SecretKey2:    "abc",
				Token:         "abc",
				Passphrase:    "abc",
				EncryptRsaKey: "abc",
				InitConfig:    "abc",
				Envs:          map[string]string{},
				Configs:       map[string]string{},
				Options:       []string{},
				FormatOptions: "xxx",
				CacheDirs:     []string{"/var/jfsCache"},
				CachePVCs:     []CachePVC{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "cache-pvc1",
			args: args{
				secrets: map[string]string{"name": "abc"},
				volCtx:  map[string]string{"juicefs/mount-cache-pvc": "abc,def"},
				options: []string{"cache-dir=/abc"},
				usePod:  true,
			},
			want: &JfsSetting{
				IsCe:   false,
				UsePod: true,
				Name:   "abc",
				Source: "abc",
				CachePVCs: []CachePVC{{
					PVCName: "abc",
					Path:    "/var/jfsCache-0",
				}, {
					PVCName: "def",
					Path:    "/var/jfsCache-1",
				}},
				CacheDirs: []string{"/abc"},
				Options:   []string{"cache-dir=/var/jfsCache-0:/var/jfsCache-1:/abc"},
				Envs:      map[string]string{},
				Configs:   map[string]string{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "cache-pvc2",
			args: args{
				secrets: map[string]string{"name": "abc"},
				volCtx:  map[string]string{"juicefs/mount-cache-pvc": "abc"},
				options: []string{},
				usePod:  true,
			},
			want: &JfsSetting{
				IsCe:   false,
				UsePod: true,
				Name:   "abc",
				Source: "abc",
				CachePVCs: []CachePVC{{
					PVCName: "abc",
					Path:    "/var/jfsCache-0",
				}},
				CacheDirs: []string{},
				Options:   []string{"cache-dir=/var/jfsCache-0"},
				Envs:      map[string]string{},
				Configs:   map[string]string{},
				Attr: PodAttr{
					JFSConfigPath:        JFSConfigPath,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MountLabels = ""
			if tt.args.MountLabels != "" {
				MountLabels = tt.args.MountLabels
			}
			got, err := ParseSetting(tt.args.secrets, tt.args.volCtx, tt.args.options, tt.args.usePod)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotStr, err := json.Marshal(got)
			if err != nil {
				t.Errorf("Marshal got error = %v", err)
				return
			}
			wantStr, err := json.Marshal(tt.want)
			if err != nil {
				t.Errorf("Marshal want error = %v", err)
				return
			}
			if string(gotStr) != string(wantStr) {
				t.Errorf("ParseSecret() got = \n%v\n, want \n%v\n", got, tt.want)
			}
		})
	}
}

func Test_parseYamlOrJson(t *testing.T) {
	jsonDst := make(map[string]string)
	yamlDst := make(map[string]string)
	type args struct {
		source string
		dst    interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		wantDst interface{}
	}{
		{
			name: "test-json",
			args: args{
				source: "{\"a\": \"b\", \"c\": \"d\"}",
				dst:    &jsonDst,
			},
			wantErr: false,
			wantDst: map[string]string{
				"a": "b",
				"c": "d",
			},
		},
		{
			name: "test-yaml",
			args: args{
				source: "c: d\ne: f",
				dst:    &yamlDst,
			},
			wantErr: false,
			wantDst: map[string]string{
				"c": "d",
				"e": "f",
			},
		},
		{
			name: "test-wrong",
			args: args{
				source: ":",
				dst:    nil,
			},
			wantErr: true,
			wantDst: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parseYamlOrJson(tt.args.source, tt.args.dst); (err != nil) != tt.wantErr {
				t.Errorf("parseYamlOrJson() error = %v, wantErr %v", err, tt.wantErr)
			}
			wantString, _ := json.Marshal(tt.wantDst)
			gotString, _ := json.Marshal(tt.args.dst)
			if string(wantString) != string(gotString) {
				t.Errorf("parseYamlOrJson() parse error, wantDst %v, gotDst %v", tt.wantDst, tt.args.dst)
			}
		})
	}
}

func Test_parsePodResources(t *testing.T) {
	type args struct {
		cpuLimit      string
		memoryLimit   string
		cpuRequest    string
		memoryRequest string
	}
	tests := []struct {
		name    string
		args    args
		want    corev1.ResourceRequirements
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				cpuLimit:      "1",
				memoryLimit:   "2G",
				cpuRequest:    "3",
				memoryRequest: "4G",
			},
			want:    testResources,
			wantErr: false,
		},
		{
			name: "test-err",
			args: args{
				cpuLimit: "aaa",
			},
			want:    corev1.ResourceRequirements{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePodResources(tt.args.cpuLimit, tt.args.memoryLimit, tt.args.cpuRequest, tt.args.memoryRequest)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePodResources() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePodResources() got = %v, want %v", got, tt.want)
			}
		})
	}
}
