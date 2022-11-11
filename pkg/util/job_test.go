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

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestIsJobCompleted(t *testing.T) {
	type args struct {
		job *batchv1.Job
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-complete",
			args: args{
				job: &batchv1.Job{
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						}},
					},
				},
			},
			want: true,
		},
		{
			name: "test-fail",
			args: args{
				job: &batchv1.Job{
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						}},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsJobCompleted(tt.args.job); got != tt.want {
				t.Errorf("IsJobCompleted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsJobFailed(t *testing.T) {
	type args struct {
		job *batchv1.Job
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-complete",
			args: args{
				job: &batchv1.Job{
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						}},
					},
				},
			},
			want: false,
		},
		{
			name: "test-fail",
			args: args{
				job: &batchv1.Job{
					Status: batchv1.JobStatus{
						Conditions: []batchv1.JobCondition{{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						}},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsJobFailed(tt.args.job); got != tt.want {
				t.Errorf("IsJobFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}
