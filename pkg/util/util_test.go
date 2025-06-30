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
	"errors"
	"math"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestContainsString(t *testing.T) {
	type args struct {
		slice []string
		s     string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-true",
			args: args{
				slice: []string{"a", "b"},
				s:     "a",
			},
			want: true,
		},
		{
			name: "test-false",
			args: args{
				slice: []string{"a", "b"},
				s:     "c",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsString(tt.args.slice, tt.args.s); got != tt.want {
				t.Errorf("ContainsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseEndpoint(t *testing.T) {
	type args struct {
		endpoint string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				endpoint: "unix://tmp/csi.sock",
			},
			want:    "unix",
			want1:   "/tmp/csi.sock",
			wantErr: false,
		},
		{
			name: "test-error",
			args: args{
				endpoint: "http://test",
			},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name: "test-nil",
			args: args{
				endpoint: "",
			},
			want:    "",
			want1:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := ParseEndpoint(tt.args.endpoint)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseEndpoint() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ParseEndpoint() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestParseEndpointError(t *testing.T) {
	Convey("Test ParseEndpoint", t, func() {
		Convey("parse error", func() {
			patch1 := ApplyFunc(url.Parse, func(rawURL string) (*url.URL, error) {
				return nil, errors.New("test")
			})
			defer patch1.Reset()
			_, _, err := ParseEndpoint("unix://tmp/csi.sock")
			So(err, ShouldNotBeNil)
		})
		Convey("not exist", func() {
			patch1 := ApplyFunc(os.IsNotExist, func(err error) bool {
				return false
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(os.Remove, func(addr string) error {
				return errors.New("test")
			})
			defer patch2.Reset()
			_, _, err := ParseEndpoint("unix://tmp/csi.sock")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestGetReferenceKey(t *testing.T) {
	type args struct {
		target string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				target: "test",
			},
			want: "juicefs-9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c1",
		},
		{
			name: "test-nil",
			args: args{
				target: "",
			},
			want: "juicefs-e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetReferenceKey(tt.args.target); got != tt.want {
				t.Errorf("GetReferenceKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTimeAfterDelay(t *testing.T) {
	now := time.Now()
	type args struct {
		delayStr string
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
				delayStr: "1h",
			},
			want:    now.Add(1 * time.Hour).Format("2006-01-02 15:04:05"),
			wantErr: false,
		},
		{
			name: "test-err",
			args: args{
				delayStr: "1hour",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTimeAfterDelay(tt.args.delayStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTimeAfterDelay() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetTimeAfterDelay() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTime(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name    string
		args    args
		want    time.Time
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				str: "2006-01-02 15:04:05",
			},
			want:    time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTime(tt.args.str)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTime() got = %v, want %v", got, tt.want)
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
			if got := QuoteForShell(tt.args.cmd); got != tt.want {
				t.Errorf("transformCmd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripPasswd(t *testing.T) {
	type args struct {
		uri string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test1",
			args: args{
				uri: "redis://:abc@127.0.0.1:6379/0",
			},
			want: "redis://:****@127.0.0.1:6379/0",
		},
		{
			name: "test2",
			args: args{
				uri: "redis://127.0.0.1:6379/0",
			},
			want: "redis://127.0.0.1:6379/0",
		},
		{
			name: "test3",
			args: args{
				uri: "redis://abc:abc@127.0.0.1:6379/0",
			},
			want: "redis://abc:****@127.0.0.1:6379/0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripPasswd(tt.args.uri); got != tt.want {
				t.Errorf("StripPasswd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckDynamicPV(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "test-true",
			args: args{
				name: "pvc-95aba554-3fe4-4433-9d25-d2a63a114367",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "test-false",
			args: args{
				name: "test",
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckDynamicPV(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckDynamicPV() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CheckDynamicPV() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsPrefix(t *testing.T) {
	type args struct {
		slice []string
		s     string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-true",
			args: args{
				slice: []string{"metrics=0.0.0.0:9567", "subdir=/metrics"},
				s:     "metrics=",
			},
			want: true,
		},
		{
			name: "test-false",
			args: args{
				slice: []string{"subdir=/metrics"},
				s:     "metrics=",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsPrefix(tt.args.slice, tt.args.s); got != tt.want {
				t.Errorf("ContainsPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripReadonlyOption(t *testing.T) {
	type args struct {
		options []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test-none",
			args: args{
				options: []string{},
			},
			want: []string{},
		},
		{
			name: "test-ro",
			args: args{
				options: []string{"ro"},
			},
			want: []string{},
		},
		{
			name: "test-read-only",
			args: args{
				options: []string{"read-only"},
			},
			want: []string{},
		},
		{
			name: "test-no-ro",
			args: args{
				options: []string{"subdir=/metrics"},
			},
			want: []string{"subdir=/metrics"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripReadonlyOption(tt.args.options); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StripReadonlyOption() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckExpectValue(t *testing.T) {
	type args struct {
		m           map[string]string
		key         string
		targetValue string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-true",
			args: args{
				m: map[string]string{
					"a": "b",
					"c": "d",
				},
				key:         "a",
				targetValue: "b",
			},
			want: true,
		},
		{
			name: "test-false",
			args: args{
				m: map[string]string{
					"a": "b",
					"c": "d",
				},
				key:         "a",
				targetValue: "c",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckExpectValue(tt.args.m, tt.args.key, tt.args.targetValue); got != tt.want {
				t.Errorf("CheckExpectValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageResol(t *testing.T) {
	type args struct {
		image string
	}
	tests := []struct {
		name      string
		args      args
		wantHasCE bool
		wantHasEE bool
	}{
		{
			name: "test-latest",
			args: args{
				image: "juicedata/mount:latest",
			},
			wantHasCE: true,
			wantHasEE: false,
		},
		{
			name: "test-ce",
			args: args{
				image: "juicedata/mount:ce-1.0.0",
			},
			wantHasCE: true,
			wantHasEE: false,
		},
		{
			name: "test-ee",
			args: args{
				image: "juicedata/mount:ee-4.9.0",
			},
			wantHasCE: false,
			wantHasEE: true,
		},
		{
			name: "test-both",
			args: args{
				image: "juicedata/mount:v1.0.0-4.9.0",
			},
			wantHasCE: true,
			wantHasEE: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHasCE, gotHasEE := ImageResol(tt.args.image)
			if gotHasCE != tt.wantHasCE {
				t.Errorf("ImageResol() gotHasCE = %v, want %v", gotHasCE, tt.wantHasCE)
			}
			if gotHasEE != tt.wantHasEE {
				t.Errorf("ImageResol() gotHasEE = %v, want %v", gotHasEE, tt.wantHasEE)
			}
		})
	}
}

func TestParseToBytes(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    uint64
		wantErr bool
	}{
		{
			name: "test-normal",
			args: "1",
			want: 1 << 20,
		},
		{
			name: "test-has-uint",
			args: "1M",
			want: 1 << 20,
		},
		{
			name: "test-has-uint-2",
			args: "1G",
			want: 1 << 30,
		},
		{
			name:    "test-invalid",
			args:    "1d",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseToBytes(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseBytes() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseClientVersion(t *testing.T) {
	type args struct {
		image string
	}
	tests := []struct {
		name string
		args args
		want ClientVersion
	}{
		{
			name: "ce-v1.1.1",
			args: args{
				image: "juicedata/mount:ce-v1.1.1",
			},
			want: ClientVersion{
				IsCe:  true,
				Dev:   false,
				Major: 1,
				Minor: 1,
				Patch: 1,
			},
		},
		{
			name: "ce-nightly",
			args: args{
				image: "juicedata/mount:ce-nightly",
			},
			want: ClientVersion{
				IsCe:    true,
				Dev:     true,
				Nightly: true,
			},
		},
		{
			name: "ce-latest",
			args: args{
				image: "juicedata/mount",
			},
			want: ClientVersion{
				IsCe:  true,
				Dev:   false,
				Major: math.MaxInt32,
			},
		},
		{
			name: "ee-5.0.18-43a7d32",
			args: args{
				image: "juicedata/mount:ee-5.0.18-43a7d32",
			},
			want: ClientVersion{
				IsCe:  false,
				Dev:   false,
				Major: 5,
				Minor: 0,
				Patch: 18,
			},
		},
		{
			name: "ee-nightly",
			args: args{
				image: "juicedata/mount:ee-nightly",
			},
			want: ClientVersion{
				IsCe:    false,
				Dev:     true,
				Nightly: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseClientVersionFromImage(tt.args.image); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseClientVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientVersion_SupportFusePass(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  bool
	}{
		{
			name:  "dev",
			image: "juicedata/mount:v1.2.3-dev",
			want:  false,
		},
		{
			name:  "ce-1.2.1",
			image: "juicedata/mount:ce-v1.2.1",
			want:  true,
		},
		{
			name:  "ce-1.3.0",
			image: "juicedata/mount:ce-v1.3.0",
			want:  true,
		},
		{
			name:  "ce-2.0.0",
			image: "juicedata/mount:ce-v2.0.0",
			want:  true,
		},
		{
			name:  "ee-5.1.0",
			image: "juicedata/mount:ee-5.1.0-xxx",
			want:  true,
		},
		{
			name:  "ee-6.1.0",
			image: "juicedata/mount:ee-6.1.0-xxx",
			want:  true,
		},
		{
			name:  "ce-nightly",
			image: "juicedata/mount:ce-nightly",
			want:  true,
		},
		{
			name:  "ee-nightly",
			image: "juicedata/mount:ee-nightly",
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportFusePass(tt.image); got != tt.want {
				t.Errorf("SupportFusePass() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMountPathOfPod(t *testing.T) {
	type args struct {
		pod corev1.Pod
	}
	var normalPod = corev1.Pod{Spec: corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:    "pvc-node01-xxx",
				Image:   "juicedata/juicefs-csi-driver:v0.10.6",
				Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
			},
		},
	}}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name:    "get mntPath from pod cmd success",
			args:    args{pod: normalPod},
			want:    "/jfs/pvc-xxx",
			want1:   "pvc-xxx",
			wantErr: false,
		},
		{
			name:    "nil pod ",
			args:    args{pod: corev1.Pod{}},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name: "err-pod cmd <3",
			//args:    args{cmd: "/bin/mount.juicefs redis://127.0.0.1/6379"},
			args: args{pod: corev1.Pod{Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "pvc-node01-xxx",
						Image:   "juicedata/juicefs-csi-driver:v0.10.6",
						Command: []string{"sh", "-c"},
					},
				}}}},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name: "err-cmd sourcePath no MountBase prefix",
			args: args{pod: corev1.Pod{Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "pvc-node01-xxx",
						Image:   "juicedata/juicefs-csi-driver:v0.10.6",
						Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /err-jfs/pvc-xxx}"},
					},
				}}}},
			want:    "",
			want1:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := GetMountPathOfPod(tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMntPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseMntPath() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ParseMntPath() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestParseMntPath(t *testing.T) {
	type args struct {
		cmd string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name:    "get sourcePath from pod cmd success",
			args:    args{cmd: "/usr/local/bin/juicefs format --storage=s3 --bucket=http://juicefs-bucket.minio.default.svc.cluster.local:9000 --access-key=minioadmin --secret-key=${secretkey} ${metaurl} ce-secret\n/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
			want:    "/jfs/pvc-xxx",
			want1:   "pvc-xxx",
			wantErr: false,
		},
		{
			name:    "get sourcePath from pod cmd with exec success",
			args:    args{cmd: "/usr/local/bin/juicefs format --storage=s3 --bucket=http://juicefs-bucket.minio.default.svc.cluster.local:9000 --access-key=minioadmin --secret-key=${secretkey} ${metaurl} ce-secret\nexec /bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
			want:    "/jfs/pvc-xxx",
			want1:   "pvc-xxx",
			wantErr: false,
		},
		{
			name:    "without init cmd",
			args:    args{cmd: "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
			want:    "/jfs/pvc-xxx",
			want1:   "pvc-xxx",
			wantErr: false,
		},
		{
			name:    "without mnt jfs",
			args:    args{cmd: "/bin/mount.juicefs redis://127.0.0.1/6379 /mnt/jfs"},
			want:    "/mnt/jfs",
			want1:   "jfs",
			wantErr: false,
		},
		{
			name: "with create subpath",
			args: args{cmd: "/usr/local/bin/juicefs format --storage=s3 --bucket=http://juicefs-bucket.minio.default.svc.cluster.local:9000 --access-key=minioadmin --secret-key=${secretkey} ${metaurl} ce-secret\n" +
				"/bin/mount.juicefs ${metaurl} /mnt/jfs -o buffer-size=300,cache-size=100,enable-xattr\n" +
				"if [ ! -d /mnt/jfs/pvc-fb2ec20c-474f-4804-9504-966da4af9b73 ]; then mkdir -m 777 /mnt/jfs/pvc-fb2ec20c-474f-4804-9504-966da4af9b73; fi;\n" +
				"umount /mnt/jfs -l\n" +
				"/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
			want:    "/jfs/pvc-xxx",
			want1:   "pvc-xxx",
			wantErr: false,
		},
		{
			name:    "err-pod cmd args <3",
			args:    args{cmd: "/usr/local/bin/juicefs format --storage=s3 --bucket=http://juicefs-bucket.minio.default.svc.cluster.local:9000 --access-key=minioadmin --secret-key=${secretkey} ${metaurl} ce-secret\n/bin/mount.juicefs redis://127.0.0.1/6379"},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name:    "err-cmd sourcePath no MountBase prefix",
			args:    args{cmd: "/usr/local/bin/juicefs format --storage=s3 --bucket=http://juicefs-bucket.minio.default.svc.cluster.local:9000 --access-key=minioadmin --secret-key=${secretkey} ${metaurl} ce-secret\n/bin/mount.juicefs redis://127.0.0.1/6379 /err-jfs/pvc-xxx"},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name:    "err-cmd sourcePath length err",
			args:    args{cmd: "/usr/local/bin/juicefs format --storage=s3 --bucket=http://juicefs-bucket.minio.default.svc.cluster.local:9000 --access-key=minioadmin --secret-key=${secretkey} ${metaurl} ce-secret\n/bin/mount.juicefs redis://127.0.0.1/6379 /jfs"},
			want:    "",
			want1:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := parseMntPath(tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMntPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseMntPath() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("parseMntPath() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestSupportUpgradeRecreate(t *testing.T) {
	type args struct {
		ce      bool
		version string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-5.1",
			args: args{
				ce:      false,
				version: "juicefs version 5.1.0 (2024-09-09 5a1303e2)",
			},
			want: true,
		},
		{
			name: "test-5.0",
			args: args{
				ce:      false,
				version: "juicefs version 5.0.0 (2024-09-09 5a1303e2)",
			},
			want: false,
		},
		{
			name: "test-4.9",
			args: args{
				ce:      false,
				version: "JuiceFS version 4.9.0 (2023-03-28 bfeaf6a)",
			},
			want: false,
		},
		{
			name: "test-1.2.0",
			args: args{
				ce:      true,
				version: "juicefs version 1.2.0+2024-06-18.873c47b9",
			},
			want: false,
		},
		{
			name: "test-1.1.0",
			args: args{
				ce:      true,
				version: "juicefs version 1.1.0+2023-09-04.08c4ae62",
			},
			want: false,
		},
		{
			name: "test-dev",
			args: args{
				ce:      true,
				version: "juicefs version 1.3.0-dev+2024-08-23.f4e98bd3",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportUpgradeRecreate(tt.args.ce, tt.args.version); got != tt.want {
				t.Errorf("SupportUpgradeRecreate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportUpgradeBinary(t *testing.T) {
	type args struct {
		ce      bool
		version string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-5.1",
			args: args{
				ce:      false,
				version: "juicefs version 5.1.0 (2024-09-09 5a1303e2)",
			},
			want: true,
		},
		{
			name: "test-5.0",
			args: args{
				ce:      false,
				version: "juicefs version 5.0.0 (2024-09-09 5a1303e2)",
			},
			want: true,
		},
		{
			name: "test-4.9",
			args: args{
				ce:      false,
				version: "JuiceFS version 4.9.0 (2023-03-28 bfeaf6a)",
			},
			want: false,
		},
		{
			name: "test-1.2.0",
			args: args{
				ce:      true,
				version: "juicefs version 1.2.0+2024-06-18.873c47b9",
			},
			want: true,
		},
		{
			name: "test-1.1.0",
			args: args{
				ce:      true,
				version: "juicefs version 1.1.0+2023-09-04.08c4ae62",
			},
			want: false,
		},
		{
			name: "test-dev",
			args: args{
				ce:      true,
				version: "juicefs version 1.3.0-dev+2024-08-23.f4e98bd3",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportUpgradeBinary(tt.args.ce, tt.args.version); got != tt.want {
				t.Errorf("SupportUpgradeBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMountOptionsOfPod(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want []string
	}{
		{
			name: "test-valid-options",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Command: []string{"sh", "-c", "exec /sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update"},
						},
					},
				},
			},
			want: []string{"foreground", "no-update"},
		},
		{
			name: "test-with-cp",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Command: []string{"sh", "-c", "cp test.config /root/test.config\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update"},
						},
					},
				},
			},
			want: []string{"foreground", "no-update"},
		},
		{
			name: "test-without-exec",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Command: []string{"sh", "-c", "/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update"},
						},
					},
				},
			},
			want: []string{"foreground", "no-update"},
		},
		{
			name: "test-no-options",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Command: []string{"sh", "-c", "exec /sbin/mount.juicefs test /jfs/mntPath"},
						},
					},
				},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetMountOptionsOfPod(tt.pod); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMountOptionsOfPod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortBy(t *testing.T) {
	ss := []string{"a", "b", "d", "e", "c"}
	wantSS := []string{"a", "b", "c", "d", "e"}
	SortBy(ss, func(i, j int) bool {
		return strings.Compare(ss[i], ss[j]) < 0
	})
	if !reflect.DeepEqual(ss, wantSS) {
		t.Errorf("SortBy() = %v, want %v", ss, wantSS)
	}

	sc := []corev1.EnvVar{
		{Name: "d", Value: "4"},
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}
	wantSC := []corev1.EnvVar{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
		{Name: "d", Value: "4"},
	}
	SortBy(sc, func(i, j int) bool {
		return strings.Compare(sc[i].Name, sc[j].Name) < 0
	})
	if !reflect.DeepEqual(sc, wantSC) {
		t.Errorf("SortBy() = %v, want %v", sc, wantSC)
	}
}

func TestMergeMap(t *testing.T) {
	type args struct {
		s map[string]string
		d map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "test1",
			args: args{
				s: map[string]string{"a": "1", "b": "2"},
				d: map[string]string{"c": "3", "d": "4"},
			},
			want: map[string]string{"a": "1", "b": "2", "c": "3", "d": "4"},
		},
		{
			name: "test2",
			args: args{
				s: nil,
				d: map[string]string{"c": "3", "d": "4"},
			},
			want: map[string]string{"c": "3", "d": "4"},
		},
		{
			name: "test3",
			args: args{s: map[string]string{"a": "1", "b": "2"}, d: nil},
			want: map[string]string{"a": "1", "b": "2"},
		},
		{
			name: "test4",
			args: args{s: nil, d: nil},
			want: nil,
		},
		{
			name: "test5",
			args: args{s: map[string]string{"a": "1", "b": "2"}, d: map[string]string{"a": "3", "d": "4"}},
			want: map[string]string{"a": "1", "b": "2", "d": "4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergeMap(tt.args.s, tt.args.d); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeDuplicate(t *testing.T) {
	type args struct {
		target []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test1",
			args: args{
				target: []string{"a", "b", "c", "a", "b"},
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "test2",
			args: args{
				target: nil,
			},
			want: nil,
		},
		{
			name: "test3",
			args: args{
				target: []string{"a", "b", "c"},
			},
			want: []string{"a", "b", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeDuplicate(tt.args.target); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeDuplicate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMountPathOfSidecar(t *testing.T) {
	type args struct {
		pod           corev1.Pod
		containerName string
	}

	var podNoSidecar = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-without-sidecar",
		},
	}

	var podWithSidecar = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-with-sidecar",
			Labels: map[string]string{
				common.InjectSidecarDone: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "app-container",
				},
			},
		},
	}

	var podWithShortCmd = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-with-short-cmd",
			Labels: map[string]string{
				common.InjectSidecarDone: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "mount-sidecar",
					Command: []string{"sh", "-c"},
					// Command too short
				},
			},
		},
	}

	var podWithValidSidecar = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-with-valid-sidecar",
			Labels: map[string]string{
				common.InjectSidecarDone: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "mount-sidecar",
					Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
				},
			},
		},
	}

	var podWithInitSidecar = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-with-init-sidecar",
			Labels: map[string]string{
				common.InjectSidecarDone: "true",
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:    "init-mount-sidecar",
					Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-yyy"},
				},
			},
		},
	}

	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name: "pod has no sidecar",
			args: args{
				pod:           podNoSidecar,
				containerName: "mount-sidecar",
			},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name: "pod has sidecar but container not found",
			args: args{
				pod:           podWithSidecar,
				containerName: "mount-sidecar",
			},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name: "pod has sidecar but command invalid",
			args: args{
				pod:           podWithShortCmd,
				containerName: "mount-sidecar",
			},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name: "pod has sidecar with valid command",
			args: args{
				pod:           podWithValidSidecar,
				containerName: "mount-sidecar",
			},
			want:    "/jfs/pvc-xxx",
			want1:   "pvc-xxx",
			wantErr: false,
		},
		{
			name: "pod has init container with valid command",
			args: args{
				pod:           podWithInitSidecar,
				containerName: "init-mount-sidecar",
			},
			want:    "/jfs/pvc-yyy",
			want1:   "pvc-yyy",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := GetMountPathOfSidecar(tt.args.pod, tt.args.containerName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMountPathOfSidecar() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetMountPathOfSidecar() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetMountPathOfSidecar() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
