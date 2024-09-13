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
	"strings"
	"testing"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func TestBaseBuilder_overwriteSubdirWithSubPath(t *testing.T) {
	type fields struct {
		jfsSetting *common.JfsSetting
	}
	tests := []struct {
		name       string
		fields     fields
		wantSubdir string
	}{
		{
			name: "test1",
			fields: fields{
				jfsSetting: &common.JfsSetting{
					Options: []string{
						"subdir=abc",
					},
					SubPath: "def",
				},
			},
			wantSubdir: "abc/def",
		},
		{
			name: "test2",
			fields: fields{
				jfsSetting: &common.JfsSetting{
					Options: []string{},
					SubPath: "def",
				},
			},
			wantSubdir: "def",
		},
		{
			name: "test3",
			fields: fields{
				jfsSetting: &common.JfsSetting{
					Options: []string{},
					SubPath: "",
				},
			},
			wantSubdir: "",
		},
		{
			name: "test4",
			fields: fields{
				jfsSetting: &common.JfsSetting{
					Options: []string{
						"subdir=abc",
					},
					SubPath: "",
				},
			},
			wantSubdir: "abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &BaseBuilder{
				jfsSetting: tt.fields.jfsSetting,
			}
			r.overwriteSubdirWithSubPath()
			var subdir string
			for _, option := range r.jfsSetting.Options {
				if strings.HasPrefix(option, "subdir=") {
					s := strings.Split(option, "=")
					if len(s) != 2 {
						t.Error("overwriteSubdirWithSubPath() error")
					}
					if s[0] == "subdir" {
						subdir = s[1]
					}
				}
			}

			if subdir != tt.wantSubdir {
				t.Errorf("overwriteSubdirWithSubPath() got=%s, want=%s", subdir, tt.wantSubdir)
			}
		})
	}
}
