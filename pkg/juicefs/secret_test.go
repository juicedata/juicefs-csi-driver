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

package juicefs

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
	}
	tests := []struct {
		name    string
		args    args
		want    *JfsSecret
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				secrets: map[string]string{
					"name": "test",
					"envs": "GOOGLE_CLOUD_PROJECT: \"/root/.config/gcloud/application_default_credentials.json\"",
				},
			},
			want: &JfsSecret{
				Name: "test",
				Envs: s,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSecret(tt.args.secrets, tt.args.volCtx)
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
