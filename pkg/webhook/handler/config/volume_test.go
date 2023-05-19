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

package config

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

func TestGetVolumes(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	pvc1 := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-1",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-1"},
	}
	pvc2 := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-2",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-2"},
	}
	pvc3 := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-3",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-3"},
	}
	pvcs := []corev1.PersistentVolumeClaim{pvc1, pvc2, pvc3}
	pv1 := corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{
			CSI: &corev1.CSIPersistentVolumeSource{
				Driver:       "csi.juicefs.com",
				VolumeHandle: "pv-1",
			},
		}},
	}
	pv2 := corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-2"},
		Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/test",
			},
		}},
	}
	pv3 := corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-3"},
		Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{
			CSI: &corev1.CSIPersistentVolumeSource{
				Driver:       "csi.juicefs.com",
				VolumeHandle: "pv-3",
			},
		}},
	}
	pvs := []corev1.PersistentVolume{pv1, pv2, pv3}
	k8sClient := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
	for _, pvc := range pvcs {
		k8sClient.Interface.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(context.TODO(), &pvc, metav1.CreateOptions{})
	}
	for _, pv := range pvs {
		k8sClient.Interface.CoreV1().PersistentVolumes().Create(context.TODO(), &pv, metav1.CreateOptions{})
	}
	tests := []struct {
		name     string
		args     args
		wantUsed bool
		wantPair []PVPair
		wantErr  bool
	}{
		{
			name: "test volume",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{{
							Name: "test",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc-1",
								},
							},
						}},
					},
				},
			},
			wantUsed: true,
			wantPair: []PVPair{{
				PV:  &pv1,
				PVC: &pvc1,
			}},
			wantErr: false,
		},
		{
			name: "test no volume",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-2",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{{
							Name: "test",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc-2",
								},
							},
						}},
					},
				},
			},
			wantUsed: false,
			wantPair: []PVPair{},
			wantErr:  false,
		},
		{
			name: "test-two-volume",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-3",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "test1",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc-1",
									},
								},
							},
							{
								Name: "test3",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc-3",
									},
								},
							},
						},
					},
				},
			},
			wantUsed: true,
			wantPair: []PVPair{
				{
					PV:  &pv1,
					PVC: &pvc1,
				},
				{
					PV:  &pv3,
					PVC: &pvc3,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUsed, gotPair, err := GetVolumes(context.TODO(), k8sClient, tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotUsed != tt.wantUsed {
				t.Errorf("GetVolume() gotUsed = %v, want %v", gotUsed, tt.wantUsed)
			}
			if len(gotPair) != len(tt.wantPair) {
				t.Errorf("GetVolume() gotPair = %v, want %v", gotPair, tt.wantPair)
			}
			for _, p := range gotPair {
				gotPVC := p.PVC
				gotPV := p.PV
				var (
					wantPVC *corev1.PersistentVolumeClaim
					wantPV  *corev1.PersistentVolume
				)
				for _, wp := range tt.wantPair {
					if p.PV.Name == wp.PV.Name && p.PVC.Name == wp.PVC.Name {
						wantPVC = wp.PVC
						wantPV = wp.PV
					}
				}

				if !reflect.DeepEqual(gotPV, wantPV) {
					t.Errorf("GetVolume() gotPvGot = %v, want %v", gotPV, wantPV)
				}
				if !reflect.DeepEqual(gotPVC, wantPVC) {
					t.Errorf("GetVolume() gotPvcGot = %v, want %v", gotPVC, wantPVC)
				}
			}
		})
	}
}
