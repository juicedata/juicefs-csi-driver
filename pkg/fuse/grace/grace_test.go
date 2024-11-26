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
	"reflect"
	"testing"
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
				action:      "recreate",
				name:        "juicefs-xxxx",
				worker:      1,
				ignoreError: false,
			},
		},
		{
			name: "pod",
			args: args{
				message: "juicefs-xxxx",
			},
			want: upgradeRequest{
				action:      noRecreate,
				name:        "juicefs-xxxx",
				worker:      1,
				ignoreError: false,
			},
		},
		{
			name: "batch",
			args: args{
				message: "BATCH NORECREATE worker=3,ignoreError=true",
			},
			want: upgradeRequest{
				action:      noRecreate,
				name:        "BATCH",
				worker:      3,
				ignoreError: true,
			},
		},
		{
			name: "uniqueIds",
			args: args{
				message: "BATCH NORECREATE worker=3,ignoreError=true,uniqueIds=1/2",
			},
			want: upgradeRequest{
				action:      noRecreate,
				name:        "BATCH",
				worker:      3,
				ignoreError: true,
				uniqueIds:   []string{"1", "2"},
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
