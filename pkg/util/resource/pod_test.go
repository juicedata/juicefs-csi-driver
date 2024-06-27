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

package resource

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	podLimit = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("1G"),
	}
	podRequest = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("1G"),
	}
	testResources = corev1.ResourceRequirements{
		Limits:   podLimit,
		Requests: podRequest,
	}
)

func TestDeleteResourceOfPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-cn",
				Image: "nginx",
			}},
			NodeName: "test-node",
		},
	}

	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:      "test-cn",
							Image:     "nginx",
							Resources: testResources,
						}},
						NodeName: "test-node",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DeleteResourceOfPod(tt.args.pod)
			if !reflect.DeepEqual(pod, tt.args.pod) {
				t.Errorf("deleteResourceOfPod err. got = %v, want = %v", tt.args.pod, pod)
			}
		})
	}
}

func TestIsPodHasResource(t *testing.T) {
	type args struct {
		pod corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-false",
			args: args{
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-cn",
							Image: "nginx",
						}},
						NodeName: "test-node",
					},
				},
			},
			want: false,
		},
		{
			name: "test-true",
			args: args{
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:      "test-cn",
							Image:     "nginx",
							Resources: testResources,
						}},
						NodeName: "test-node",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPodHasResource(tt.args.pod); got != tt.want {
				t.Errorf("IsPodHasResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPodReady(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-true",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.ContainersReady,
								Status: corev1.ConditionTrue,
							},
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test-false",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.ContainersReady,
								Status: corev1.ConditionFalse,
							},
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPodReady(tt.args.pod); got != tt.want {
				t.Errorf("IsPodReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPodError(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-true",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.ContainersReady,
								Status: corev1.ConditionFalse,
							},
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test-true: pod-unknown-status",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodUnknown,
					},
				},
			},
			want: true,
		},
		{
			name: "test-true: waiting reason != ContainerCreating",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Waiting: &corev1.ContainerStateWaiting{
										Reason:  "CrashLoopBackoff",
										Message: "",
									},
									Running:    nil,
									Terminated: nil,
								},
							},
						},
					},
				},
			},
			want: true,
		}, {
			name: "test-true: container State is Terminated and Terminated.ExitCode != 0",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Waiting: nil,
									Running: nil,
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode:    1,
										Signal:      0,
										Reason:      "",
										Message:     "",
										StartedAt:   metav1.Time{},
										FinishedAt:  metav1.Time{},
										ContainerID: "",
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test-false: container Terminated and Terminated.ExitCode is 0",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Waiting: nil,
									Running: nil,
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode:    0,
										Signal:      0,
										Reason:      "",
										Message:     "",
										StartedAt:   metav1.Time{},
										FinishedAt:  metav1.Time{},
										ContainerID: "",
									},
								},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "test-false",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.ContainersReady,
								Status: corev1.ConditionTrue,
							},
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				},
			},
			want: false,
		}, {
			name: "test-false- waiting reason is ContainerCreating",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Waiting: &corev1.ContainerStateWaiting{
										Reason:  "ContainerCreating",
										Message: "",
									},
									Running:    nil,
									Terminated: nil,
								},
							},
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPodError(tt.args.pod); got != tt.want {
				t.Errorf("IsPodError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPodResourceError(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-true",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase:  corev1.PodFailed,
						Reason: "OutOfCpu",
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.ContainersReady,
								Status: corev1.ConditionFalse,
							},
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test-true2",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:    corev1.PodScheduled,
								Status:  corev1.ConditionFalse,
								Reason:  corev1.PodReasonUnschedulable,
								Message: "Insufficient cpu",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test-resource-error",
			args: args{
				pod: &corev1.Pod{
					Status: corev1.PodStatus{
						Phase:   corev1.PodFailed,
						Reason:  "UnexpectedAdmissionError",
						Message: "Fail to reclaim resources",
					},
				},
			},
			want: true,
		},
		{
			name: "test-false",
			args: args{
				pod: &corev1.Pod{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{{
							Type:   corev1.PodReady,
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
			if got := IsPodResourceError(tt.args.pod); got != tt.want {
				t.Errorf("IsPodResourceError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMountPathOfPod(t *testing.T) {
	type args struct {
		pod corev1.Pod
	}
	var normalPod = corev1.Pod{Spec: corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:    "pvc-node01-xxx",
				Image:   "juicedata/juicefs-csi-driver:v0.10.6",
				Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
			},
		},
	}}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name:    "get mntPath from pod cmd success",
			args:    args{pod: normalPod},
			want:    "/jfs/pvc-xxx",
			want1:   "pvc-xxx",
			wantErr: false,
		},
		{
			name:    "nil pod ",
			args:    args{pod: corev1.Pod{}},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name: "err-pod cmd <3",
			//args:    args{cmd: "/bin/mount.juicefs redis://127.0.0.1/6379"},
			args: args{pod: corev1.Pod{Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "pvc-node01-xxx",
						Image:   "juicedata/juicefs-csi-driver:v0.10.6",
						Command: []string{"sh", "-c"},
					},
				}}}},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name: "err-cmd sourcePath no MountBase prefix",
			args: args{pod: corev1.Pod{Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "pvc-node01-xxx",
						Image:   "juicedata/juicefs-csi-driver:v0.10.6",
						Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /err-jfs/pvc-xxx}"},
					},
				}}}},
			want:    "",
			want1:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := GetMountPathOfPod(tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMntPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseMntPath() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ParseMntPath() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGetAllRefKeys(t *testing.T) {
	type args struct {
		pod corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "test-1",
			args: args{
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"juicefs-8d156faf0f66234b8d78c5efa19acaec04d40fdf9629fad3f975d9a": "/var/lib/kubelet/pods/147fef36-241a-4148-b5f8-8ac41f1719e5/volumes/kubernetes.io~csi/pvc-4256b212-6581-4873-8d4c-d1481bcbd305/mount",
						},
					},
				},
			},
			want: map[string]string{"juicefs-8d156faf0f66234b8d78c5efa19acaec04d40fdf9629fad3f975d9a": "/var/lib/kubelet/pods/147fef36-241a-4148-b5f8-8ac41f1719e5/volumes/kubernetes.io~csi/pvc-4256b212-6581-4873-8d4c-d1481bcbd305/mount"},
		},
		{
			name: "test-1",
			args: args{
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"juicefs-8d156faf0f66234b8d78c5efa19acaec04d40fdf9629fad3f975d9a": "/var/lib/kubelet/pods/147fef36-241a-4148-b5f8-8ac41f1719e5/volumes/kubernetes.io~csi/pvc-4256b212-6581-4873-8d4c-d1481bcbd305/mount",
							"juicefs-abc": "abc",
						},
					},
				},
			},
			want: map[string]string{"juicefs-8d156faf0f66234b8d78c5efa19acaec04d40fdf9629fad3f975d9a": "/var/lib/kubelet/pods/147fef36-241a-4148-b5f8-8ac41f1719e5/volumes/kubernetes.io~csi/pvc-4256b212-6581-4873-8d4c-d1481bcbd305/mount"},
		},
		{
			name: "test-2",
			args: args{
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"juicefs-8d156faf0f66234b8d78c5efa19acaec04d40fdf9629fad3f975d9a": "/var/lib/kubelet/pods/147fef36-241a-4148-b5f8-8ac41f1719e5/volumes/kubernetes.io~csi/pvc-4256b212-6581-4873-8d4c-d1481bcbd305/mount",
							"juicefs-633ac2eb9e3f1a969cd64e240c09a84cf23e727745f63ba67c93ccc": "/var/lib/kubelet/pods/6468b90b-c255-4dc9-9dd7-1ddcc1b9bc66/volumes/kubernetes.io~csi/pvc-4256b212-6581-4873-8d4c-d1481bcbd305/mount",
							"juicefs-abc": "abc",
						},
					},
				},
			},
			want: map[string]string{
				"juicefs-8d156faf0f66234b8d78c5efa19acaec04d40fdf9629fad3f975d9a": "/var/lib/kubelet/pods/147fef36-241a-4148-b5f8-8ac41f1719e5/volumes/kubernetes.io~csi/pvc-4256b212-6581-4873-8d4c-d1481bcbd305/mount",
				"juicefs-633ac2eb9e3f1a969cd64e240c09a84cf23e727745f63ba67c93ccc": "/var/lib/kubelet/pods/6468b90b-c255-4dc9-9dd7-1ddcc1b9bc66/volumes/kubernetes.io~csi/pvc-4256b212-6581-4873-8d4c-d1481bcbd305/mount",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAllRefKeys(tt.args.pod); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAllRefKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}
