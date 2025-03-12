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
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

func TestObjectMetadata_StringParser(t *testing.T) {
	type fields struct {
		data        map[string]string
		labels      map[string]string
		annotations map[string]string
	}
	type args struct {
		str string
	}
	tests := []struct {
		name string
		pvc  fields
		node fields
		args args
		want string
	}{
		{
			name: "test-pvc-name",
			pvc: fields{
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
			name: "test-pvc-namespace",
			pvc: fields{
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
			name: "test-pvc-label",
			pvc: fields{
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
			name: "test-pvc-annotation",
			pvc: fields{
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
			name: "test-pvc-nil",
			pvc: fields{
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
		{
			name: "test-pvc-lowercase",
			pvc: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
			},
			args: args{
				str: "${.pvc.name}-a",
			},
			want: "test-a",
		},
		{
			name: "test-node-name",
			node: fields{
				data: map[string]string{
					"name": "test",
				},
			},
			args: args{
				str: "${.node.name}-a",
			},
			want: "test-a",
		},
		{
			name: "test-node-cidr",
			node: fields{
				data: map[string]string{
					"name":    "test",
					"podCIDR": "10.244.0.0/24",
				},
			},
			args: args{
				str: "${.node.podCIDR}-a",
			},
			want: "10.244.0.0/24-a",
		},
		{
			name: "test-node-label",
			node: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
				labels: map[string]string{
					"a.a": "b",
				},
			},
			args: args{
				str: "${.node.labels.a.a}-a",
			},
			want: "b-a",
		},
		{
			name: "test-node-annotation",
			node: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
				annotations: map[string]string{
					"a.a": "b",
				},
			},
			args: args{
				str: "${.node.annotations.a.a}-a",
			},
			want: "b-a",
		},
		{
			name: "test-node-nil",
			node: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
			},
			args: args{
				str: "${.node.annotations.a}-a",
			},
			want: "-a",
		},
		{
			name: "test-node-real-annotation",
			node: fields{
				annotations: map[string]string{
					"fs1.juicefs.com/cacheGroup": "region-1",
				},
			},
			args: args{
				str: "${.node.annotations.fs1.juicefs.com/cacheGroup}-a",
			},
			want: "region-1-a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &ObjectMeta{
				pvc: &objectMetadata{
					data:        tt.pvc.data,
					labels:      tt.pvc.labels,
					annotations: tt.pvc.annotations,
				},
				node: &objectMetadata{
					data:        tt.node.data,
					labels:      tt.node.labels,
					annotations: tt.node.annotations,
				},
			}
			if got := meta.StringParser(tt.args.str); got != tt.want {
				t.Errorf("StringParser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckForSubPath(t *testing.T) {
	type args struct {
		volume      *v1.PersistentVolume
		pathPattern string
	}
	tests := []struct {
		name              string
		args              args
		pvs               []v1.PersistentVolume
		wantShouldDeleted bool
		wantErr           bool
	}{
		{
			name: "test",
			args: args{
				volume: &v1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{Name: "test11"},
					Spec: v1.PersistentVolumeSpec{
						PersistentVolumeSource: v1.PersistentVolumeSource{
							CSI: &v1.CSIPersistentVolumeSource{
								Driver: "juicefs.csi.com",
								VolumeAttributes: map[string]string{
									"subPath":     "test",
									"pathPattern": "test",
								},
							},
						},
						StorageClassName: "test-sc",
					},
				},
				pathPattern: "test",
			},
			pvs: []v1.PersistentVolume{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test12"},
					Spec: v1.PersistentVolumeSpec{
						PersistentVolumeSource: v1.PersistentVolumeSource{
							CSI: &v1.CSIPersistentVolumeSource{
								Driver: "juicefs.csi.com",
								VolumeAttributes: map[string]string{
									"subPath":     "test",
									"pathPattern": "test",
								},
							},
						},
						StorageClassName: "test-sc",
					},
				},
			},
			wantShouldDeleted: false,
			wantErr:           false,
		},
		{
			name: "test-no-pathPattern",
			args: args{
				volume: &v1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test21",
					},
					Spec: v1.PersistentVolumeSpec{
						PersistentVolumeSource: v1.PersistentVolumeSource{
							CSI: &v1.CSIPersistentVolumeSource{
								Driver: "juicefs.csi.com",
								VolumeAttributes: map[string]string{
									"subPath": "test-1",
								},
							},
						},
						StorageClassName: "test-sc",
					},
				},
			},
			pvs: []v1.PersistentVolume{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test22",
					},
					Spec: v1.PersistentVolumeSpec{
						PersistentVolumeSource: v1.PersistentVolumeSource{
							CSI: &v1.CSIPersistentVolumeSource{
								Driver: "juicefs.csi.com",
								VolumeAttributes: map[string]string{
									"subPath": "test-2",
								},
							},
						},
						StorageClassName: "test-sc",
					},
				},
			},
			wantShouldDeleted: true,
			wantErr:           false,
		},
		{
			name: "test-no-other-pv",
			args: args{
				volume: &v1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test31",
					},
					Spec: v1.PersistentVolumeSpec{
						PersistentVolumeSource: v1.PersistentVolumeSource{
							CSI: &v1.CSIPersistentVolumeSource{
								Driver: "juicefs.csi.com",
								VolumeAttributes: map[string]string{
									"subPath":     "test",
									"pathPattern": "test",
								},
							},
						},
						StorageClassName: "test-sc",
					},
				},
				pathPattern: "test",
			},
			pvs:               []v1.PersistentVolume{},
			wantShouldDeleted: true,
			wantErr:           false,
		},
		{
			name: "test-root-subPath",
			args: args{
				volume: &v1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test31",
					},
					Spec: v1.PersistentVolumeSpec{
						PersistentVolumeSource: v1.PersistentVolumeSource{
							CSI: &v1.CSIPersistentVolumeSource{
								Driver: "juicefs.csi.com",
								VolumeAttributes: map[string]string{
									"subPath": "/",
								},
							},
						},
						StorageClassName: "test-sc",
					},
				},
				pathPattern: "test",
			},
			pvs:               []v1.PersistentVolume{},
			wantShouldDeleted: false,
			wantErr:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClientSet := fake.NewSimpleClientset()
			client := &k8s.K8sClient{Interface: fakeClientSet}
			for _, pv := range tt.pvs {
				if _, err := fakeClientSet.CoreV1().PersistentVolumes().Create(context.Background(), &pv, metav1.CreateOptions{}); err != nil {
					t.Errorf("CheckForSubPath() create pv err %v", err)
				}
			}
			gotShouldDeleted, err := CheckForSubPath(context.TODO(), client, tt.args.volume, tt.args.pathPattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckForSubPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotShouldDeleted != tt.wantShouldDeleted {
				t.Errorf("CheckForSubPath() gotShouldDeleted = %v, want %v", gotShouldDeleted, tt.wantShouldDeleted)
			}
		})
	}
}

func TestResolveSecret(t *testing.T) {
	type fields struct {
		data        map[string]string
		labels      map[string]string
		annotations map[string]string
	}
	type args struct {
		str    string
		pvname string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "test-pvc-name",
			fields: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
			},
			args: args{
				str:    "${pvc.name}",
				pvname: "pv-a",
			},
			want: "test",
		},
		{
			name: "test-pvc-namespace",
			fields: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
			},
			args: args{
				str:    "${pvc.namespace}",
				pvname: "pv-a",
			},
			want: "default",
		},
		{
			name: "test-pvc-annotation",
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
				str:    "${pvc.annotations['a.a']}",
				pvname: "pv-a",
			},
			want: "b",
		},
		{
			name: "test-pv-name",
			fields: fields{
				data: map[string]string{
					"name":      "test",
					"namespace": "default",
				},
			},
			args: args{
				str:    "${pv.name}",
				pvname: "pv-a",
			},
			want: "pv-a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &ObjectMeta{
				pvc: &objectMetadata{
					data:        tt.fields.data,
					labels:      tt.fields.labels,
					annotations: tt.fields.annotations,
				},
			}
			if got := meta.ResolveSecret(tt.args.str, tt.args.pvname); got != tt.want {
				t.Errorf("ResolveSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}
