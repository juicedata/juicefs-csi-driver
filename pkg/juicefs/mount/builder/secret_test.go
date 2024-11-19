/*
 Copyright 2023 Juicedata Inc

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

package builder

import (
	"reflect"
	"testing"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func TestBaseBuilder_GetEnvKey(t *testing.T) {
	type fields struct {
		jfsSetting *config.JfsSetting
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "test",
			fields: fields{
				jfsSetting: &config.JfsSetting{
					MetaUrl:    "redis://",
					SecretKey:  "test_secret",
					SecretKey2: "test_secret2",
					Token:      "test_token",
					Passphrase: "test_passphrase",
					Envs:       map[string]string{"a": "b"},
				},
			},
			want: []string{
				"metaurl", "secretkey", "secretkey2", "token", "passphrase", "a",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &BaseBuilder{
				jfsSetting: tt.fields.jfsSetting,
			}
			if got := r.GetEnvKey(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetEnvKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
