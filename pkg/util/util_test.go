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
	"testing"

	. "github.com/agiledragon/gomonkey"
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
			args:    args{cmd: "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
			want:    "/jfs/pvc-xxx",
			want1:   "pvc-xxx",
			wantErr: false,
		},
		{
			name:    "err-pod cmd args <3",
			args:    args{cmd: "/bin/mount.juicefs redis://127.0.0.1/6379"},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name:    "err-cmd sourcePath no MountBase prefix",
			args:    args{cmd: "/bin/mount.juicefs redis://127.0.0.1/6379 /err-jfs/pvc-xxx"},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name:    "err-cmd sourcePath length err",
			args:    args{cmd: "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs"},
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
