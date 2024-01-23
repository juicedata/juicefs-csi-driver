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
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	. "github.com/smartystreets/goconvey/convey"
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
			name:    "without init cmd",
			args:    args{cmd: "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
			want:    "/jfs/pvc-xxx",
			want1:   "pvc-xxx",
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
			got, got1, err := ParseMntPath(tt.args.cmd)
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

func TestCutPodLabelKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"normal length of key label", "label_key_1", "label_key_1"},
		{"longest length of key label ", "label_key_test-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "label_key_test-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CutPodLabelKey(tt.key); got != tt.want {
				t.Errorf("CutPodLabelKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
