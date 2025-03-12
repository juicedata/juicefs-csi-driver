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

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
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

func TestGetUniqueId(t *testing.T) {
	type args struct {
		pod corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test",
			args: args{
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "juicefs-test1-123-3456-6789-123",
					},
					Spec: corev1.PodSpec{
						NodeName: "test1",
					},
				},
			},
			want: "123-3456-6789-123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetUniqueId(tt.args.pod); got != tt.want {
				t.Errorf("GetUniqueId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeEnvs(t *testing.T) {
	type args struct {
		pod *corev1.Pod
		env []corev1.EnvVar
	}
	tests := []struct {
		name string
		args args
		want []corev1.EnvVar
	}{
		{
			name: "test-merge-envs",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-cn",
							Image: "nginx",
							Env: []corev1.EnvVar{
								{Name: "JFS_FOREGROUND", Value: "true"},
								{Name: "EXISTING_ENV", Value: "existing_value"},
							},
						}},
						NodeName: "test-node",
					},
				},
				env: []corev1.EnvVar{
					{Name: "NEW_ENV", Value: "new_value"},
				},
			},
			want: []corev1.EnvVar{
				{Name: "JFS_FOREGROUND", Value: "true"},
				{Name: "NEW_ENV", Value: "new_value"},
			},
		},
		{
			name: "test-merge-envs-duplicates",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-cn",
							Image: "nginx",
							Env: []corev1.EnvVar{
								{Name: "JFS_NO_UPDATE_CONFIG", Value: "true"},
								{Name: "EXISTING_ENV", Value: "existing_value"},
							},
						}},
					},
				},
				env: []corev1.EnvVar{
					{Name: "JFS_NO_UPDATE_CONFIG", Value: "false"},
					{Name: "EXISTING_ENV", Value: "new_value"},
				},
			},
			want: []corev1.EnvVar{
				{Name: "JFS_NO_UPDATE_CONFIG", Value: "false"},
				{Name: "EXISTING_ENV", Value: "new_value"},
			},
		},
		{
			name: "test-merge-envs-empty",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-cn",
							Image: "nginx",
							Env:   []corev1.EnvVar{},
						}},
					},
				},
				env: []corev1.EnvVar{
					{Name: "NEW_ENV", Value: "new_value"},
				},
			},
			want: []corev1.EnvVar{
				{Name: "NEW_ENV", Value: "new_value"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeEnvs(tt.args.pod, tt.args.env)
			assert.Equal(t, tt.want, tt.args.pod.Spec.Containers[0].Env)
		})
	}
}

func TestMergeVolumes(t *testing.T) {
	type args struct {
		pod        *corev1.Pod
		jfsSetting *config.JfsSetting
	}
	tests := []struct {
		name string
		args args
		want *corev1.Pod
	}{
		{
			name: "test-append-extra-volumes",
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "test-init-config",
									MountPath: "/test-init-config",
								},
							},
						}},
						Volumes: []corev1.Volume{
							{
								Name: "test-init-config",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/init-config",
										Type: func() *corev1.HostPathType {
											hostPathType := corev1.HostPathDirectoryOrCreate
											return &hostPathType
										}(),
									},
								},
							},
						},
					},
				},
				jfsSetting: &config.JfsSetting{
					Attr: &config.PodAttr{
						Volumes: []corev1.Volume{
							{
								Name: "extra-volume1",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "extra-volume1",
								MountPath: "/mnt/volume1",
							},
						},
						VolumeDevices: []corev1.VolumeDevice{
							{
								Name:       "extra-device1",
								DevicePath: "/dev/device1",
							},
						},
					},
				},
			},
			want: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "test-init-config",
								MountPath: "/test-init-config",
							},
							{
								Name:      "extra-volume1",
								MountPath: "/mnt/volume1",
							},
						},
						VolumeDevices: []corev1.VolumeDevice{
							{
								Name:       "extra-device1",
								DevicePath: "/dev/device1",
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "test-init-config",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/init-config",
									Type: func() *corev1.HostPathType {
										hostPathType := corev1.HostPathDirectoryOrCreate
										return &hostPathType
									}(),
								},
							},
						},
						{
							Name: "extra-volume1",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
		{
			name: "test-append-cachedir-volumes",
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{}}},
				},
				jfsSetting: &config.JfsSetting{
					CacheDirs: []string{"/cache1"},
					CachePVCs: []config.CachePVC{
						{PVCName: "pvc1", Path: "/pvc1"},
					},
				},
			},
			want: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "cachedir-0",
								MountPath: "/cache1",
							},
							{
								Name:      "cachedir-pvc-0",
								MountPath: "/pvc1",
							},
						},
						VolumeDevices: []corev1.VolumeDevice{},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "cachedir-0",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/cache1",
									Type: func() *corev1.HostPathType {
										hostPathType := corev1.HostPathDirectoryOrCreate
										return &hostPathType
									}(),
								},
							},
						},
						{
							Name: "cachedir-pvc-0",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc1",
									ReadOnly:  false,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "test-overwrite-cachedir-volumes",
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "cachedir-0",
									MountPath: "/cache1",
								},
							},
						}},
						Volumes: []corev1.Volume{
							{
								Name: "cachedir-0",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/cache1",
										Type: func() *corev1.HostPathType {
											hostPathType := corev1.HostPathDirectoryOrCreate
											return &hostPathType
										}(),
									},
								},
							},
						},
					},
				},
				jfsSetting: &config.JfsSetting{},
			},
			want: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						VolumeMounts:  []corev1.VolumeMount{},
						VolumeDevices: []corev1.VolumeDevice{},
					}},
					Volumes: []corev1.Volume{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeVolumes(tt.args.pod, tt.args.jfsSetting)
			assert.Equal(t, tt.want, tt.args.pod)
		})
	}
}

func TestMergeMountOptions(t *testing.T) {
	type args struct {
		pod        *corev1.Pod
		jfsSetting *config.JfsSetting
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test-with-cp",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-cn",
							Image: "nginx",
							Command: []string{
								"sh",
								"-c",
								"cp test.config /root/test.config\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update",
							},
						}},
					},
				},
				jfsSetting: &config.JfsSetting{
					Options: []string{"opt3", "opt4"},
				},
			},
			want: []string{
				"sh",
				"-c",
				"cp test.config /root/test.config\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update,opt3,opt4",
			},
		},
		{
			name: "test-with-auth",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-cn",
							Image: "nginx",
							Command: []string{
								"sh",
								"-c",
								"cp test.config /root/test.config\n/usr/bin/juicefs auth jfs-test --access-key=ceph --token=${token} --secret-key=${secretkey} --conf-dir=/root/.juicefs\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update",
							},
						}},
					},
				},
				jfsSetting: &config.JfsSetting{
					Options: []string{"opt3", "opt4"},
				}},
			want: []string{
				"sh",
				"-c",
				"cp test.config /root/test.config\n/usr/bin/juicefs auth jfs-test --access-key=ceph --token=${token} --secret-key=${secretkey} --conf-dir=/root/.juicefs\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update,opt3,opt4",
			},
		},
		{
			name: "test-with-cachedir",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-cn",
							Image: "nginx",
							Command: []string{
								"sh",
								"-c",
								"cp test.config /root/test.config\n/usr/bin/juicefs auth jfs-test --access-key=ceph --token=${token} --secret-key=${secretkey} --conf-dir=/root/.juicefs\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update,cache-dir=/cache1",
							},
						}},
					},
				},
				jfsSetting: &config.JfsSetting{
					Options: []string{"opt3", "opt4"},
				}},
			want: []string{
				"sh",
				"-c",
				"cp test.config /root/test.config\n/usr/bin/juicefs auth jfs-test --access-key=ceph --token=${token} --secret-key=${secretkey} --conf-dir=/root/.juicefs\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update,opt3,opt4",
			},
		},
		{
			name: "test-remove-option",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-cn",
							Image: "nginx",
							Command: []string{
								"sh",
								"-c",
								"cp test.config /root/test.config\n/usr/bin/juicefs auth jfs-test --access-key=ceph --token=${token} --secret-key=${secretkey} --conf-dir=/root/.juicefs\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update,verbose",
							},
						}},
					},
				},
				jfsSetting: &config.JfsSetting{
					Options: []string{},
				}},
			want: []string{
				"sh",
				"-c",
				"cp test.config /root/test.config\n/usr/bin/juicefs auth jfs-test --access-key=ceph --token=${token} --secret-key=${secretkey} --conf-dir=/root/.juicefs\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update",
			},
		},
		{
			name: "test-overwrite-inter-option",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-cn",
							Image: "nginx",
							Command: []string{
								"sh",
								"-c",
								"cp test.config /root/test.config\n/usr/bin/juicefs auth jfs-test --access-key=ceph --token=${token} --secret-key=${secretkey} --conf-dir=/root/.juicefs\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update,metrics=0.0.0.0:8080,cache-size=10240",
							},
						}},
					},
				},
				jfsSetting: &config.JfsSetting{
					Options: []string{
						"metrics=0.0.0.0:8081",
						"cache-size=10241",
					},
				}},
			want: []string{
				"sh",
				"-c",
				"cp test.config /root/test.config\n/usr/bin/juicefs auth jfs-test --access-key=ceph --token=${token} --secret-key=${secretkey} --conf-dir=/root/.juicefs\n/sbin/mount.juicefs test /jfs/mntPath -o foreground,no-update,metrics=0.0.0.0:8081,cache-size=10241",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeMountOptions(tt.args.pod, tt.args.jfsSetting)
			assert.Equal(t, tt.want, tt.args.pod.Spec.Containers[0].Command)
		})
	}
}

func TestFilterVars(t *testing.T) {
	t.Run("test env", func(t *testing.T) {
		if got := FilterVars([]corev1.EnvVar{
			{
				Name:  "test1",
				Value: "value1",
			},
			{
				Name:  "abc",
				Value: "value1",
			},
		}, "abc", func(envVar corev1.EnvVar) string {
			return envVar.Name
		}); !reflect.DeepEqual(got, []corev1.EnvVar{
			{
				Name:  "test1",
				Value: "value1",
			},
		}) {
			t.Errorf("FilterVars env error, got: %v", got)
		}
	})

	t.Run("test volume", func(t *testing.T) {
		if got := FilterVars([]corev1.Volume{
			{
				Name: "test1",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "abc",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}, "abc", func(volume corev1.Volume) string {
			return volume.Name
		}); !reflect.DeepEqual(got, []corev1.Volume{
			{
				Name: "test1",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}) {
			t.Errorf("FilterVars volume error, got: %v", got)
		}
	})
	t.Run("test volumeMount", func(t *testing.T) {
		if got := FilterVars([]corev1.VolumeMount{
			{
				Name:      "test1",
				MountPath: "/tmp",
			},
			{
				Name:      "abc",
				MountPath: "/tmp",
			},
		}, "abc", func(volume corev1.VolumeMount) string {
			return volume.Name
		}); !reflect.DeepEqual(got, []corev1.VolumeMount{
			{
				Name:      "test1",
				MountPath: "/tmp",
			},
		}) {
			t.Errorf("FilterVars volumeMount error, got: %v", got)
		}
	})
}
