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
