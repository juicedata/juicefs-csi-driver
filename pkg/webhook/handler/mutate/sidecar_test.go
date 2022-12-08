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

package mutate

import (
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func TestSidecarMutate_injectVolume(t *testing.T) {
	type fields struct {
		PVC        *corev1.PersistentVolumeClaim
		jfsSetting *config.JfsSetting
	}
	type args struct {
		pod     *corev1.Pod
		volumes []corev1.Volume
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		wantPodVolume []corev1.Volume
	}{
		{
			name: "test-inject-volume",
			fields: fields{
				PVC: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc-1",
					},
				},
				jfsSetting: &config.JfsSetting{VolumeId: "volume-id"},
			},
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "app-volume",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc-1",
									},
								},
							},
						},
					},
				},
				volumes: []corev1.Volume{{
					Name: "mount-volume",
				}},
			},
			wantPodVolume: []corev1.Volume{
				{
					Name: "mount-volume",
				},
				{
					Name: "app-volume",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: filepath.Join(config.MountPointPath, "volume-id"),
						},
					},
				},
			},
		},
		{
			name: "test-not-inject-volume",
			fields: fields{
				PVC: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc-1",
					},
				},
				jfsSetting: &config.JfsSetting{VolumeId: "volume-id"},
			},
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{},
				},
				volumes: []corev1.Volume{{
					Name: "mount-volume",
				}},
			},
			wantPodVolume: []corev1.Volume{{
				Name: "mount-volume",
			}},
		},
		{
			name: "test-inject-volume-subpath",
			fields: fields{
				PVC: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc-3",
					},
				},
				jfsSetting: &config.JfsSetting{VolumeId: "volume-id", SubPath: "subpath"},
			},
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "app-volume",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc-3",
									},
								},
							},
						},
					},
				},
				volumes: []corev1.Volume{{
					Name: "mount-volume",
				}},
			},
			wantPodVolume: []corev1.Volume{
				{
					Name: "mount-volume",
				},
				{
					Name: "app-volume",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: filepath.Join(config.MountPointPath, "volume-id", "subpath"),
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SidecarMutate{
				PVC:        tt.fields.PVC,
				jfsSetting: tt.fields.jfsSetting,
			}
			s.injectVolume(tt.args.pod, tt.args.volumes)
			if len(tt.args.pod.Spec.Volumes) != len(tt.wantPodVolume) {
				t.Errorf("injectVolume() = %v, want %v", tt.args.pod.Spec.Volumes, tt.wantPodVolume)
			}
			podVols := make(map[string]corev1.Volume)
			wantPodVols := make(map[string]corev1.Volume)
			for _, v := range tt.args.pod.Spec.Volumes {
				podVols[v.Name] = v
			}
			for _, v := range tt.wantPodVolume {
				wantPodVols[v.Name] = v
			}
			for name, volume := range podVols {
				wantVolume, ok := wantPodVols[name]
				if !ok {
					t.Errorf("injectVolume() = %v, want %v", tt.args.pod.Spec.Volumes, tt.wantPodVolume)
				}
				if volume.HostPath != nil && wantVolume.HostPath == nil && volume.HostPath.Path != wantVolume.HostPath.Path {
					t.Errorf("injectVolume() = %v, want %v", volume.HostPath.Path, wantVolume.HostPath.Path)
				}
			}
		})
	}
}

func TestSidecarMutate_injectInitContainer(t *testing.T) {
	type args struct {
		pod       *corev1.Pod
		container corev1.Container
	}
	tests := []struct {
		name                 string
		args                 args
		wantInitContainerLen int
	}{
		{
			name: "test inject init container",
			args: args{
				pod: &corev1.Pod{},
				container: corev1.Container{
					Name:  "format",
					Image: "juicedata/mount:latest",
				},
			},
			wantInitContainerLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SidecarMutate{}
			s.injectInitContainer(tt.args.pod, tt.args.container)
			if len(tt.args.pod.Spec.InitContainers) != tt.wantInitContainerLen {
				t.Errorf("injectInitContainer() = %v, want %v", tt.args.pod.Spec.InitContainers, tt.wantInitContainerLen)
			}
		})
	}
}

func TestSidecarMutate_injectContainer(t *testing.T) {
	type args struct {
		pod       *corev1.Pod
		container corev1.Container
	}
	tests := []struct {
		name             string
		args             args
		wantContainerLen int
	}{
		{
			name: "test inject init container",
			args: args{
				pod: &corev1.Pod{},
				container: corev1.Container{
					Name:  "mount",
					Image: "juicedata/mount:latest",
				},
			},
			wantContainerLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SidecarMutate{}
			s.injectContainer(tt.args.pod, tt.args.container)
			if len(tt.args.pod.Spec.Containers) != tt.wantContainerLen {
				t.Errorf("injectContainer() = %v, want %v", tt.args.pod.Spec.Containers, tt.wantContainerLen)
			}
		})
	}
}
