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
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	"github.com/stretchr/testify/assert"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
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
	defaultResource := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(common.DefaultMountPodCpuLimit),
			corev1.ResourceMemory: resource.MustParse(common.DefaultMountPodMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(common.DefaultMountPodCpuRequest),
			corev1.ResourceMemory: resource.MustParse(common.DefaultMountPodMemRequest),
		},
	}

	type args struct {
		secrets  map[string]string
		volCtx   map[string]string
		options  []string
		volumeId string
		uniqueId string
		uuid     string
		patch    *MountPodPatch
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
				secrets:  map[string]string{"name": "test", "envs": "{GOOGLE_APPLICATION_CREDENTIALS: \"/root/.config/gcloud/application_default_credentials.json\", a: b, c: d}"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				UsePod:     true,
				Source:     "test",
				UniqueId:   "test",
				CachePVCs:  []CachePVC{},
				CacheDirs:  []string{"/var/jfsCache"},
				UUID:       "test",
				SecretName: "juicefs-test-secret",
				Envs:       s,
				Configs:    map[string]string{},
				Options:    []string{},
				Attr: &PodAttr{
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
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
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-storage-nil",
			args: args{
				secrets:  map[string]string{"name": "test"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				Source:     "test",
				UUID:       "test",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{},
				UsePod:     true,
				CacheDirs:  []string{"/var/jfsCache"},
				CachePVCs:  []CachePVC{},
				Attr: &PodAttr{
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-storage",
			args: args{
				secrets:  map[string]string{"name": "test", "storage": "ceph"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				Source:     "test",
				UUID:       "test",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				Storage:    "ceph",
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				UsePod:     true,
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				CachePVCs:  []CachePVC{},
				Attr: &PodAttr{
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-cpu-limit",
			args: args{
				secrets:  map[string]string{"name": "test", "storage": "s3"},
				volCtx:   map[string]string{common.MountPodCpuLimitKey: "1"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				Source:     "test",
				Storage:    "s3",
				UniqueId:   "test",
				UsePod:     true,
				UUID:       "test",
				SecretName: "juicefs-test-secret",
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				CachePVCs:  []CachePVC{},
				Attr: &PodAttr{
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse(common.DefaultMountPodMemLimit),
						},
						Requests: defaultResource.Requests,
					},
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-mem-limit",
			args: args{
				secrets:  map[string]string{"name": "test", "storage": "s3"},
				volCtx:   map[string]string{common.MountPodMemLimitKey: "1G"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				Source:     "test",
				Storage:    "s3",
				UUID:       "test",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				UsePod:     true,
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				CachePVCs:  []CachePVC{},
				Attr: &PodAttr{
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceMemory: resource.MustParse("1G"),
							corev1.ResourceCPU:    resource.MustParse(common.DefaultMountPodCpuLimit),
						},
						Requests: defaultResource.Requests,
					},
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-mem-request",
			args: args{
				secrets:  map[string]string{"name": "test", "storage": "s3"},
				volCtx:   map[string]string{common.MountPodMemRequestKey: "1G"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				Source:     "test",
				Storage:    "s3",
				UniqueId:   "test",
				UUID:       "test",
				SecretName: "juicefs-test-secret",
				UsePod:     true,
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				CachePVCs:  []CachePVC{},
				Attr: &PodAttr{
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceMemory: resource.MustParse("1G"),
							corev1.ResourceCPU:    resource.MustParse(common.DefaultMountPodCpuRequest),
						},
						Limits: defaultResource.Limits,
					},
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-cpu-request",
			args: args{
				secrets:  map[string]string{"name": "test"},
				volCtx:   map[string]string{common.MountPodCpuRequestKey: "1"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				Source:     "test",
				UUID:       "test",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				UsePod:     true,
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				CachePVCs:  []CachePVC{},
				Attr: &PodAttr{
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse(common.DefaultMountPodMemRequest),
						},
						Limits: defaultResource.Limits,
					},
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-labels",
			args: args{
				secrets:  map[string]string{"name": "test"},
				volCtx:   map[string]string{common.MountPodLabelKey: "a: b"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				Source:     "test",
				UUID:       "test",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				UsePod:     true,
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				CachePVCs:  []CachePVC{},
				Attr: &PodAttr{
					Labels:               map[string]string{"a": "b"},
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
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
				volCtx:  map[string]string{common.MountPodLabelKey: "-"},
			},
			wantErr: true,
		},
		{
			name: "test-labels-json",
			args: args{
				secrets:  map[string]string{"name": "test"},
				volCtx:   map[string]string{common.MountPodLabelKey: "{\"a\": \"b\"}"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				Source:     "test",
				UUID:       "test",
				UniqueId:   "test",
				UsePod:     true,
				SecretName: "juicefs-test-secret",
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				CachePVCs:  []CachePVC{},
				Attr: &PodAttr{
					Resources:            defaultResource,
					Labels:               map[string]string{"a": "b"},
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-annotation",
			args: args{
				secrets:  map[string]string{"name": "test"},
				volCtx:   map[string]string{common.MountPodAnnotationKey: "a: b"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				Source:     "test",
				UUID:       "test",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				UsePod:     true,
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				CachePVCs:  []CachePVC{},
				Attr: &PodAttr{
					Annotations:          map[string]string{"a": "b"},
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
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
				volCtx:  map[string]string{common.MountPodAnnotationKey: "-"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-serviceaccount",
			args: args{
				secrets:  map[string]string{"name": "test", "storage": "s3"},
				volCtx:   map[string]string{common.MountPodServiceAccount: "test"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				UsePod:     true,
				Name:       "test",
				Source:     "test",
				UUID:       "test",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				Storage:    "s3",
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				CachePVCs:  []CachePVC{},
				Attr: &PodAttr{
					Resources:            defaultResource,
					ServiceAccountName:   "test",
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "test-config",
			args: args{
				secrets:  map[string]string{"configs": "a: b", "name": "test"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				Source:     "test",
				UUID:       "test",
				UniqueId:   "test",
				UsePod:     true,
				SecretName: "juicefs-test-secret",
				Configs:    map[string]string{"a": "b"},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				Attr: &PodAttr{
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
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
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:          "abc",
				Source:        "abc",
				UUID:          "abc",
				UniqueId:      "test",
				SecretName:    "juicefs-test-secret",
				UsePod:        true,
				SecretKey:     "abc",
				SecretKey2:    "abc",
				Token:         "abc",
				Passphrase:    "abc",
				EncryptRsaKey: "abc",
				InitConfig:    "abc",
				Envs:          map[string]string{},
				Configs:       map[string]string{},
				Options:       []string{},
				FormatCmd:     "/usr/bin/juicefs auth abc --token=${token} --secret-key=${secretkey} --secret-key2=${secretkey2} --passphrase=${passphrase} --xxx --conf-dir=/root/.juicefs",
				FormatOptions: "xxx",
				CacheDirs:     []string{"/var/jfsCache"},
				CachePVCs:     []CachePVC{},
				Attr: &PodAttr{
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "cache-pvc1",
			args: args{
				secrets:  map[string]string{"name": "abc"},
				volCtx:   map[string]string{"juicefs/mount-cache-pvc": "abc,def"},
				options:  []string{"cache-dir=/abc"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				IsCe:       false,
				UsePod:     true,
				Name:       "abc",
				UUID:       "abc",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				Source:     "abc",
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
				Attr: &PodAttr{
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "cache-pvc2",
			args: args{
				secrets:  map[string]string{"name": "abc"},
				volCtx:   map[string]string{"juicefs/mount-cache-pvc": "abc"},
				options:  []string{},
				uniqueId: "test",
			},
			want: &JfsSetting{
				IsCe:       false,
				UsePod:     true,
				Name:       "abc",
				UUID:       "abc",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				Source:     "abc",
				CachePVCs: []CachePVC{{
					PVCName: "abc",
					Path:    "/var/jfsCache-0",
				}},
				CacheDirs: []string{},
				Options:   []string{"cache-dir=/var/jfsCache-0"},
				Envs:      map[string]string{},
				Configs:   map[string]string{},
				Attr: &PodAttr{
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                DefaultEEMountImage,
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
			},
			wantErr: false,
		},
		{
			name: "specify mount image",
			args: args{
				secrets:  map[string]string{"configs": "a: b", "name": "test"},
				volCtx:   map[string]string{common.MountPodImageKey: "abc"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				UUID:       "test",
				Source:     "test",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				UsePod:     true,
				Configs:    map[string]string{"a": "b"},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				Attr: &PodAttr{
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                "abc",
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
				CachePVCs: []CachePVC{},
			},
			wantErr: false,
		},
		{
			name: "specify host path",
			args: args{
				secrets:  map[string]string{"name": "test"},
				volCtx:   map[string]string{common.MountPodHostPath: "/abc"},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				UUID:       "test",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				Source:     "test",
				UsePod:     true,
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{},
				CacheDirs:  []string{"/var/jfsCache"},
				Attr: &PodAttr{
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                "juicedata/mount:ee-nightly",
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
				CachePVCs: []CachePVC{},
				HostPath:  []string{"/abc"},
			},
			wantErr: false,
		},
		{
			name: "overwrite options",
			args: args{
				secrets:  map[string]string{"name": "test"},
				options:  []string{"buffer-size=10G"},
				patch:    &MountPodPatch{MountOptions: []string{"buffer-size=100"}},
				uniqueId: "test",
			},
			want: &JfsSetting{
				Name:       "test",
				UUID:       "test",
				UniqueId:   "test",
				SecretName: "juicefs-test-secret",
				UsePod:     true,
				Source:     "test",
				Configs:    map[string]string{},
				Envs:       map[string]string{},
				Options:    []string{"buffer-size=100"},
				CacheDirs:  []string{"/var/jfsCache"},
				Attr: &PodAttr{
					Resources:            defaultResource,
					JFSConfigPath:        JFSConfigPath,
					Image:                "juicedata/mount:ee-nightly",
					MountPointPath:       MountPointPath,
					JFSMountPriorityName: JFSMountPriorityName,
				},
				CachePVCs: []CachePVC{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer GlobalConfig.Reset()
			if tt.args.patch != nil {
				GlobalConfig.MountPodPatch = []MountPodPatch{*tt.args.patch}
			}
			got, err := ParseSetting(context.TODO(), tt.args.secrets, tt.args.volCtx, tt.args.options, tt.args.volumeId, tt.args.uniqueId, tt.args.uuid, nil, nil)
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
				t.Errorf("ParseSecret() got = \n%s\n, want \n%s\n", gotStr, wantStr)
			}
		})
	}
}

func Test_genCacheDirs(t *testing.T) {
	type args struct {
		JfsSetting JfsSetting
		volCtx     map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    JfsSetting
		wantErr bool
	}{
		{
			name: "test-default",
			args: args{
				JfsSetting: JfsSetting{},
			},
			want: JfsSetting{
				CacheDirs: []string{
					"/var/jfsCache",
				},
				CachePVCs: []CachePVC{},
				// default cache-dir is /var/jfsCache
				// Options: []string{"cache-dir=/var/jfsCache"},
			},
			wantErr: false,
		},
		{
			name: "test-cache-pvcs",
			args: args{
				JfsSetting: JfsSetting{
					Attr: &PodAttr{
						CacheDirs: []MountPatchCacheDir{
							{
								Type: "PVC",
								Name: "sss",
							},
						},
					},
				},
				volCtx: map[string]string{"juicefs/mount-cache-pvc": "abc,def"},
			},
			want: JfsSetting{
				CachePVCs: []CachePVC{{PVCName: "abc", Path: "/var/jfsCache-0"}, {PVCName: "def", Path: "/var/jfsCache-1"}, {PVCName: "sss", Path: "/var/jfsCache-2"}},
				Options:   []string{"cache-dir=/var/jfsCache-0:/var/jfsCache-1:/var/jfsCache-2"},
				Attr: &PodAttr{
					CacheDirs: []MountPatchCacheDir{
						{
							Type: "PVC",
							Name: "sss",
						},
					},
				},
				CacheDirs: []string{},
			},
			wantErr: false,
		},
		{
			name: "test-empty-dirs",
			args: args{
				JfsSetting: JfsSetting{},
				volCtx:     map[string]string{"juicefs/mount-cache-emptydir": "Memory:1Gi"},
			},
			want: JfsSetting{
				CacheEmptyDir: &CacheEmptyDir{
					Path:      "/var/jfsCache-emptyDir",
					SizeLimit: resource.MustParse("1Gi"),
					Medium:    "Memory",
				},
				Options:   []string{"cache-dir=/var/jfsCache-emptyDir"},
				CachePVCs: []CachePVC{},
				CacheDirs: []string{},
			},
			wantErr: false,
		},
		{
			name: "test-options",
			args: args{
				JfsSetting: JfsSetting{
					Options: []string{"cache-dir=/tmp/abc"},
				},
			},
			want: JfsSetting{
				CacheDirs: []string{
					"/tmp/abc",
				},
				CachePVCs: []CachePVC{},
				Options:   []string{"cache-dir=/tmp/abc"},
			},
			wantErr: false,
		},
		{
			name: "test-hostPath-with-pod-attr",
			args: args{
				JfsSetting: JfsSetting{
					Attr: &PodAttr{
						CacheDirs: []MountPatchCacheDir{
							{
								Type: "HostPath",
								Path: "/abc",
							},
						},
					},
				},
			},
			want: JfsSetting{
				Attr: &PodAttr{
					CacheDirs: []MountPatchCacheDir{
						{
							Type: "HostPath",
							Path: "/abc",
						},
					},
				},
				CacheDirs: []string{
					"/abc",
				},
				CachePVCs: []CachePVC{},
				Options:   []string{"cache-dir=/abc"},
			},
			wantErr: false,
		},
		{
			name: "test-cache-conflict",
			args: args{
				JfsSetting: JfsSetting{
					Attr: &PodAttr{
						CacheDirs: []MountPatchCacheDir{
							{
								Type: "HostPath",
								Path: "/abc",
							},
						},
					},
					Options: []string{"cache-dir=/abc"},
				},
			},
			want: JfsSetting{
				Attr: &PodAttr{
					CacheDirs: []MountPatchCacheDir{
						{
							Type: "HostPath",
							Path: "/abc",
						},
					},
				},
				CacheDirs: []string{"/abc"},
				CachePVCs: []CachePVC{},
				Options:   []string{"cache-dir=/abc"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenCacheDirs(&tt.args.JfsSetting, tt.args.volCtx)
			if (err != nil) != tt.wantErr {
				t.Errorf("genCacheDirs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.args.JfsSetting, tt.want) {
				t.Errorf("genCacheDirs() got = %v,\n want %v", tt.args.JfsSetting, tt.want)
			}
		})
	}
}

func Test_genAndValidOptions(t *testing.T) {
	type args struct {
		JfsSetting *JfsSetting
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "test-normal",
			args: args{
				JfsSetting: &JfsSetting{
					Options: []string{"cache-dir=xxx"},
				},
			},
			want:    []string{"cache-dir=xxx"},
			wantErr: false,
		},
		{
			name: "test-space1",
			args: args{
				JfsSetting: &JfsSetting{
					Options: []string{" cache-dir=xxx "},
				},
			},
			want:    []string{"cache-dir=xxx"},
			wantErr: false,
		},
		{
			name: "test-space2",
			args: args{
				JfsSetting: &JfsSetting{
					Options: []string{" cache-dir = xxx "},
				},
			},
			want:    []string{"cache-dir=xxx"},
			wantErr: false,
		},
		{
			name: "test-error",
			args: args{
				JfsSetting: &JfsSetting{
					Options: []string{"cache-dir=xxx cache-size=1024"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-buffersize",
			args: args{
				JfsSetting: &JfsSetting{
					Attr: &PodAttr{
						Resources: corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceMemory: resource.MustParse("1Mi"),
							},
						},
					},
					Options: []string{"buffer-size=1024"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-buffersize-with-unit",
			args: args{
				JfsSetting: &JfsSetting{
					Attr: &PodAttr{
						Resources: corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceMemory: resource.MustParse("1Mi"),
							},
						},
					},
					Options: []string{"buffer-size=1024M"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-buffersize-with-unit",
			args: args{
				JfsSetting: &JfsSetting{
					Options: []string{"buffer-size=10M"},
					Attr: &PodAttr{
						Resources: corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceMemory: resource.MustParse("10Mi"),
							},
						},
					},
				},
			},
			want:    []string{"buffer-size=10M"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := genAndValidOptions(tt.args.JfsSetting)
			if (err != nil) != tt.wantErr {
				t.Errorf("validOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(tt.args.JfsSetting.Options, tt.want) {
				t.Errorf("validOptions() got = %v, want %v", tt.args.JfsSetting.Options, tt.want)
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
			name: "test-nil",
			args: args{
				cpuLimit:      "-1",
				memoryLimit:   "0",
				cpuRequest:    "0",
				memoryRequest: "0",
			},
			want: corev1.ResourceRequirements{
				Limits:   map[corev1.ResourceName]resource.Quantity{},
				Requests: map[corev1.ResourceName]resource.Quantity{},
			},
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
			got, err := ParsePodResources(tt.args.cpuLimit, tt.args.memoryLimit, tt.args.cpuRequest, tt.args.memoryRequest, getDefaultResource())
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

func Test_ParseFormatOptions(t *testing.T) {
	type TestCase struct {
		description, origin, args, stripped string
		stripKeys                           []string
		parseFail                           bool
	}

	testCases := []TestCase{
		{
			description: "test empty",
			origin:      "",
			args:        "",
			stripped:    "",
		},
		{
			description: "test kv",
			origin:      "trash-days=1,block-size=4096",
			args:        "--trash-days=1 --block-size=4096",
			stripped:    "--trash-days=1 --block-size=4096",
		},
		{
			description: "test single key",
			origin:      "format-in-pod,quiet",
			args:        "--format-in-pod --quiet",
			stripped:    "--format-in-pod --quiet",
		},
		{
			description: "test empty item",
			origin:      "format-in-pod,,quiet",
			args:        "--format-in-pod --quiet",
			stripped:    "--format-in-pod --quiet",
		},
		{
			description: "test strip",
			origin:      "trash-days=1,block-size=4096",
			args:        "--trash-days=1 --block-size=4096",
			stripKeys:   []string{"trash-days", "block-size"},
			stripped:    "--trash-days=${trash-days} --block-size=${block-size}",
		},
		{
			description: "test mix",
			origin:      "trash-days=1,block-size=4096,format-in-pod,quiet",
			args:        "--trash-days=1 --block-size=4096 --format-in-pod --quiet",
			stripKeys:   []string{"trash-days", "block-size"},
			stripped:    "--trash-days=${trash-days} --block-size=${block-size} --format-in-pod --quiet",
		},
		{
			description: "test multiple '='",
			origin:      "session-token=xxx=xx=",
			args:        "--session-token=xxx=xx=",
			stripKeys:   []string{"session-token"},
			stripped:    "--session-token=${session-token}",
		},
		{
			description: "test error",
			origin:      "trash-days=",
			parseFail:   true,
		},
	}

	for _, c := range testCases {
		t.Run(c.description, func(t *testing.T) {
			setting := &JfsSetting{FormatOptions: c.origin}
			options, err := setting.ParseFormatOptions()
			if c.parseFail {
				if err == nil {
					t.Errorf("ParseFormatOptions() should fail")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseFormatOptions() should success, but got error: %v", err)
			}

			args := setting.RepresentFormatOptions(options)
			if strings.Join(args, " ") != c.args {
				t.Errorf("RepresentFormatOptions() got %v, want %v", strings.Join(args, " "), c.args)
			}
			stripped := setting.StripFormatOptions(options, c.stripKeys)
			if strings.Join(stripped, " ") != c.stripped {
				t.Errorf("StripFormatOptions() got %v, want %v", strings.Join(stripped, " "), c.stripped)
			}
		})
	}
}
func Test_getPVNameFromTarget(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "valid target",
			target: "/var/lib/kubelet/pods/abc/volumes/kubernetes.io~csi/pv-name/mount",
			want:   "pv-name",
		},
		{
			name:   "invalid target - no csi",
			target: "/var/lib/kubelet/pods/abc/volumes/kubernetes.io~other/pv-name/mount",
			want:   "",
		},
		{
			name:   "invalid target - no pv name",
			target: "/var/lib/kubelet/pods/abc/volumes/kubernetes.io~csi//mount",
			want:   "",
		},
		{
			name:   "invalid target",
			target: "xxxxxx",
			want:   "",
		},
		{
			name:   "empty string",
			target: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPVNameFromTarget(tt.target); got != tt.want {
				t.Errorf("getPVNameFromTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_applyConfigPatch(t *testing.T) {
	type args struct {
		setting *JfsSetting
		patch   MountPodPatch
	}
	tests := []struct {
		name string
		args args
		want *JfsSetting
	}{
		{
			name: "test-merge-options",
			args: args{
				setting: &JfsSetting{
					Attr: &PodAttr{},
					Options: []string{
						"a=c",
						"b=d",
						"c=e",
					},
				},
				patch: MountPodPatch{
					MountOptions: []string{"a=b", "c=d"},
				},
			},
			want: &JfsSetting{
				Attr: &PodAttr{},
				Options: []string{
					"a=b",
					"c=d",
					"b=d",
				},
			},
		},
		{
			name: "test-large-buff-size",
			args: args{
				setting: &JfsSetting{
					Attr: &PodAttr{
						Resources: corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceMemory: resource.MustParse("10Mi"),
							},
						},
					},
				},
				patch: MountPodPatch{
					MountOptions: []string{"buffer-size=1G"},
				},
			},
			want: &JfsSetting{
				Attr: &PodAttr{
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceMemory: resource.MustParse("10Mi"),
						},
					},
				},
				Options: []string{
					"buffer-size=10",
				},
			},
		},
		{
			name: "test-large-buff-size-in-pv",
			args: args{
				setting: &JfsSetting{
					Attr: &PodAttr{
						Resources: corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceMemory: resource.MustParse("10Mi"),
							},
						},
					},
					Options: []string{"buffer-size=1G"},
				},
			},
			want: &JfsSetting{
				Attr: &PodAttr{
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceMemory: resource.MustParse("10Mi"),
						},
					},
				}, Options: []string{
					"buffer-size=10",
				},
			},
		},
		{
			name: "test-empty-limit-resources-buff-size",
			args: args{
				setting: &JfsSetting{
					Attr: &PodAttr{},
				},
				patch: MountPodPatch{
					MountOptions: []string{"buffer-size=1G"},
				},
			},
			want: &JfsSetting{
				Attr: &PodAttr{},
				Options: []string{
					"buffer-size=1G",
				},
			},
		},
	}

	defer GlobalConfig.Reset()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GlobalConfig.MountPodPatch = []MountPodPatch{tt.args.patch}
			applyConfigPatch(tt.args.setting)
			assert.Equal(t, tt.want, tt.args.setting)
		})
	}
}
func TestGenHashOfSetting(t *testing.T) {
	type args struct {
		setting JfsSetting
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				setting: JfsSetting{
					Name: "test",
				},
			},
			want:    "b1881fa7e2b8d42039e294916e4a7751045b4d87ff60309b86cac737c907e78",
			wantErr: false,
		},
		{
			name: "test-list1",
			args: args{
				setting: JfsSetting{
					Name:    "test",
					Options: []string{"f", "a=b", "c=d"},
				},
			},
			want:    "72ebe7ec12a4a0e72f5554c632d98cb81e65265c356ec1be9a5f76023e4f870",
			wantErr: false,
		},
		{
			name: "test-list2",
			args: args{
				setting: JfsSetting{
					Name:    "test",
					Options: []string{"c=d", "f", "a=b"},
				},
			},
			want:    "72ebe7ec12a4a0e72f5554c632d98cb81e65265c356ec1be9a5f76023e4f870",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenHashOfSetting(klog.NewKlogr(), tt.args.setting)
			if got != tt.want {
				t.Errorf("GenHashOfSetting() got = %v, want %v", got, tt.want)
			}
		})
	}
}
