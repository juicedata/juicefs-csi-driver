/*
 Copyright 2024 Juicedata Inc

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

package grace

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func Test_parseRequest(t *testing.T) {
	type args struct {
		message string
	}
	tests := []struct {
		name string
		args args
		want upgradeRequest
	}{
		{
			name: "pod",
			args: args{
				message: "juicefs-xxxx recreate",
			},
			want: upgradeRequest{
				action: "recreate",
				name:   "juicefs-xxxx",
			},
		},
		{
			name: "pod",
			args: args{
				message: "juicefs-xxxx",
			},
			want: upgradeRequest{
				action: noRecreate,
				name:   "juicefs-xxxx",
			},
		},
		{
			name: "batch",
			args: args{
				message: fmt.Sprintf("BATCH %s batchConfig=test,batchIndex=1", recreate),
			},
			want: upgradeRequest{
				action:     recreate,
				name:       "BATCH",
				configName: "test",
				batchIndex: 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRequest(tt.args.message); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_resolvePpid(t *testing.T) {
	tests := []struct {
		name     string
		ppid     int
		commPath string
		hostPID  bool
		wantPpid int
		wantErr  bool
	}{
		{
			name:     "PPid field is used directly",
			ppid:     3551359,
			commPath: "",
			hostPID:  true,
			wantPpid: 3551359,
		},
		{
			name:     "CommPath suffix parsed when PPid is zero",
			ppid:     0,
			commPath: "/tmp/fuse_fd_comm.3551359",
			hostPID:  true,
			wantPpid: 3551359,
		},
		{
			name:     "CommPath with dotted directory does not mislead parser",
			ppid:     0,
			commPath: "/tmp/dir.99/fuse_fd_comm.1234",
			hostPID:  true,
			wantPpid: 1234,
		},
		{
			name:     "non-HostPID falls back to 1 when both fields are absent",
			ppid:     0,
			commPath: "",
			hostPID:  false,
			wantPpid: 1,
		},
		{
			name:     "non-HostPID falls back to 1 when CommPath has no parseable suffix",
			ppid:     0,
			commPath: "/tmp/fuse_fd_comm",
			hostPID:  false,
			wantPpid: 1,
		},
		{
			name:     "HostPID errors when neither field is parseable",
			ppid:     0,
			commPath: "",
			hostPID:  true,
			wantErr:  true,
		},
		{
			name:     "HostPID errors when CommPath suffix is not a valid number",
			ppid:     0,
			commPath: "/tmp/fuse_fd_comm.abc",
			hostPID:  true,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := &util.JuiceConf{PPid: tt.ppid, CommPath: tt.commPath}
			got, err := resolvePpid(conf, tt.hostPID)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolvePpid() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantPpid {
				t.Errorf("resolvePpid() = %d, want %d", got, tt.wantPpid)
			}
		})
	}
}
