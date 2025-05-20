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

package controller

import (
	ctx "context"
	"fmt"
	"reflect"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

func Test_shouldRequeue(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "no-fuse-label",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			want: false,
		},
		{
			name: "restartAlways",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							common.InjectSidecarDone: common.True,
						},
					},
					Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyAlways},
				},
			},
			want: false,
		},
		{
			name: "no-fuse",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							common.InjectSidecarDone: common.True,
						},
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
				},
			},
			want: false,
		},
		{
			name: "app-cn-not-exit",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test",
						Labels: map[string]string{common.InjectSidecarDone: common.True},
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}, {Name: common.MountContainerName + "-0"}}},
					Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.Time{Time: time.Now()},
								},
							},
						},
						{
							Name: common.MountContainerName + "-0",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.Time{Time: time.Now()},
								},
							},
						},
					}},
				},
			},
			want: false,
		},
		{
			name: "fuse-cn-exit",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test",
						Labels: map[string]string{common.InjectSidecarDone: common.True},
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}, {Name: common.MountContainerName + "-0"}}},
					Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									StartedAt: metav1.Time{Time: time.Now()},
									ExitCode:  0,
								},
							},
						},
						{
							Name: common.MountContainerName + "-0",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									StartedAt: metav1.Time{Time: time.Now()},
									ExitCode:  0,
								},
							},
						},
					}},
				},
			},
			want: false,
		},
		{
			name: "fuse-cn-no-exit",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test",
						Labels: map[string]string{common.InjectSidecarDone: common.True},
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}, {Name: common.MountContainerName + "-0"}}},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "app",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										StartedAt: metav1.Time{Time: time.Now()},
										ExitCode:  0,
									},
								},
							},
							{
								Name: common.MountContainerName + "-0",
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{
										StartedAt: metav1.Time{Time: time.Now()},
									},
								},
							},
						}},
				},
			},
			want: true,
		},
		{
			name: "multi-cn-exit",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test",
						Labels: map[string]string{common.InjectSidecarDone: common.True},
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}, {Name: "app2"}, {Name: common.MountContainerName + "-0"}}},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "app",
								State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
									StartedAt: metav1.Time{Time: time.Now()},
									ExitCode:  0,
								}},
							},
							{
								Name: "app2",
								State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
									StartedAt: metav1.Time{Time: time.Now()},
									ExitCode:  0,
								}},
							},
							{
								Name: common.MountContainerName + "-0",
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{
										StartedAt: metav1.Time{Time: time.Now()},
									},
								},
							}}},
				},
			},
			want: true,
		},
		{
			name: "multi-cn-not-exit",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test",
						Labels: map[string]string{common.InjectSidecarDone: common.True},
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}, {Name: "app2"}, {Name: common.MountContainerName + "-0"}}},
					Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									StartedAt: metav1.Time{Time: time.Now()},
									ExitCode:  0,
								},
							},
						},
						{
							Name: "app2",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.Time{Time: time.Now()},
								},
							},
						},
						{
							Name: common.MountContainerName + "-0",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.Time{Time: time.Now()},
								},
							},
						}}},
				},
			},
			want: false,
		},
		{
			name: "pod-pending",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test",
						Labels: map[string]string{common.InjectSidecarDone: common.True},
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}, {Name: common.MountContainerName + "-0"}}},
					Status: corev1.PodStatus{
						Phase:             corev1.PodPending,
						ContainerStatuses: []corev1.ContainerStatus{}},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldInQueue(tt.args.pod); got != tt.want {
				t.Errorf("shouldReconcile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppController_umountFuseSidecars_normal(t *testing.T) {
	client := &k8sclient.K8sClient{}
	patch1 := ApplyMethod(reflect.TypeOf(client), "ExecuteInContainer", func(_ *k8sclient.K8sClient, c ctx.Context, podName, namespace, containerName string, cmd []string) (stdout string, stderr string, err error) {
		return "", "", nil
	})
	defer patch1.Reset()

	type fields struct {
		Log      logr.Logger
		Recorder record.EventRecorder
	}
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test-no-fuse",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "test"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "test-prestop",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: common.MountContainerName + "-0",
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{Command: []string{"umount"}},
								},
							},
						}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "test-multi-sidecar",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: common.MountContainerName + "-0",
								Lifecycle: &corev1.Lifecycle{
									PreStop: &corev1.LifecycleHandler{
										Exec: &corev1.ExecAction{Command: []string{"umount"}},
									},
								},
							},
							{
								Name: common.MountContainerName + "-1",
								Lifecycle: &corev1.Lifecycle{
									PreStop: &corev1.LifecycleHandler{
										Exec: &corev1.ExecAction{Command: []string{"umount"}},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AppController{
				K8sClient: client,
			}
			if err := a.umountFuseSidecars(ctx.TODO(), tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("umountFuseSidecars() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAppController_umountFuseSidecars_error(t *testing.T) {
	client := &k8sclient.K8sClient{}
	patch1 := ApplyMethod(reflect.TypeOf(client), "ExecuteInContainer", func(_ *k8sclient.K8sClient, c ctx.Context, podName, namespace, containerName string, cmd []string) (stdout string, stderr string, err error) {
		return "", "", fmt.Errorf("exec error")
	})
	defer patch1.Reset()

	type fields struct {
		Log      logr.Logger
		Recorder record.EventRecorder
	}
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test-no-fuse",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "test"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "test-prestop",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: common.MountContainerName + "-0",
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{Command: []string{"umount"}},
								},
							},
						}},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "test-multi-sidecar",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: common.MountContainerName + "-0",
								Lifecycle: &corev1.Lifecycle{
									PreStop: &corev1.LifecycleHandler{
										Exec: &corev1.ExecAction{Command: []string{"umount"}},
									},
								},
							},
							{
								Name: common.MountContainerName + "-1",
								Lifecycle: &corev1.Lifecycle{
									PreStop: &corev1.LifecycleHandler{
										Exec: &corev1.ExecAction{Command: []string{"umount"}},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AppController{
				K8sClient: client,
			}
			if err := a.umountFuseSidecars(ctx.TODO(), tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("umountFuseSidecars() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
