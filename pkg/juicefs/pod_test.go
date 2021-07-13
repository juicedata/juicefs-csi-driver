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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"
	"testing"
)

var (
	podLimit = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("2G"),
	}
	podRequest = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU:    resource.MustParse("3"),
		corev1.ResourceMemory: resource.MustParse("4G"),
	}
	testResources = corev1.ResourceRequirements{
		Limits:   podLimit,
		Requests: podRequest,
	}
)

func Test_parsePodResources(t *testing.T) {
	type args struct {
		MountPodCpuLimit   string
		MountPodMemLimit   string
		MountPodCpuRequest string
		MountPodMemRequest string
	}
	tests := []struct {
		name string
		args args
		want corev1.ResourceRequirements
	}{
		{
			name: "test",
			args: args{
				MountPodCpuLimit:   "1",
				MountPodMemLimit:   "2G",
				MountPodCpuRequest: "3",
				MountPodMemRequest: "4G",
			},
			want: testResources,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePodResources(tt.args.MountPodCpuLimit, tt.args.MountPodMemLimit, tt.args.MountPodCpuRequest, tt.args.MountPodMemRequest); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePodResources() = %v, want %v", got, tt.want)
			}
		})
	}
}
