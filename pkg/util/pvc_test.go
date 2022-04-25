/*
 Copyright 2022 Juicedata Inc

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

import "testing"

func TestPVCMetadata_StringParser(t *testing.T) {
	type fields struct {
		data        map[string]string
		labels      map[string]string
		annotations map[string]string
	}
	type args struct {
		str string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "test-name",
			fields: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
			},
			args: args{
				str: "${.PVC.name}-a",
			},
			want: "test-a",
		},
		{
			name: "test-namespace",
			fields: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
			},
			args: args{
				str: "${.PVC.namespace}-a",
			},
			want: "default-a",
		},
		{
			name: "test-label",
			fields: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
				labels: map[string]string{
					"a.a": "b",
				},
			},
			args: args{
				str: "${.PVC.labels.a.a}-a",
			},
			want: "b-a",
		},
		{
			name: "test-annotation",
			fields: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
				annotations: map[string]string{
					"a.a": "b",
				},
			},
			args: args{
				str: "${.PVC.annotations.a.a}-a",
			},
			want: "b-a",
		},
		{
			name: "test-nil",
			fields: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
			},
			args: args{
				str: "${.PVC.annotations.a}-a",
			},
			want: "-a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &PVCMetadata{
				data:        tt.fields.data,
				labels:      tt.fields.labels,
				annotations: tt.fields.annotations,
			}
			if got := meta.StringParser(tt.args.str); got != tt.want {
				t.Errorf("StringParser() = %v, want %v", got, tt.want)
			}
		})
	}
}
