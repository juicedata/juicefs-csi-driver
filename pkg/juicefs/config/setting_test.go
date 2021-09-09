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
	"fmt"
	"reflect"
	"testing"
)

func TestParseSecret(t *testing.T) {
	s := map[string]string{"GOOGLE_CLOUD_PROJECT": "/root/.config/gcloud/application_default_credentials.json"}
	ss, _ := json.Marshal(s)
	fmt.Println(string(ss))

	type args struct {
		secrets map[string]string
		volCtx  map[string]string
		usePod  bool
	}
	tests := []struct {
		name    string
		args    args
		want    *JfsSetting
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				secrets: map[string]string{
					"name": "test",
					"envs": "GOOGLE_CLOUD_PROJECT: \"/root/.config/gcloud/application_default_credentials.json\"",
				},
				usePod: true,
			},
			want: &JfsSetting{
				Name:   "test",
				Envs:   s,
				UsePod: true,
			},
			wantErr: false,
		},
		{
			name: "test-storage-nil",
			args: args{
				secrets: map[string]string{
					"name": "test",
				},
				usePod: true,
			},
			want: &JfsSetting{
				Name:   "test",
				Storage: "",
				UsePod: true,
			},
			wantErr: false,
		},
		{
			name: "test-storage",
			args: args{
				secrets: map[string]string{
					"name": "test",
					"storage": "ceph",
				},
				usePod: true,
			},
			want: &JfsSetting{
				Name:   "test",
				Storage: "ceph",
				UsePod: true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSetting(tt.args.secrets, tt.args.volCtx, tt.args.usePod)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSecret() got = %v, want %v", got, tt.want)
			}
		})
	}
}
