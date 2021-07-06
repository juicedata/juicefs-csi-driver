/*

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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"
	"testing"
)

func Test_parsePodResources(t *testing.T) {
	tests := []struct {
		name string
		want v1.ResourceRequirements
	}{
		{
			name: "test",
			want: v1.ResourceRequirements{
				Limits:   map[v1.ResourceName]resource.Quantity{},
				Requests: map[v1.ResourceName]resource.Quantity{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePodResources(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePodResources() = %v, want %v", got, tt.want)
			}
		})
	}
}
