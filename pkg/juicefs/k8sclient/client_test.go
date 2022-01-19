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

package k8sclient

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestNewClient(t *testing.T) {
	Convey("Test NewClient", t, func() {
		Convey("test", func() {
			patch1 := ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
				return nil, nil
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(kubernetes.NewForConfig, func(c *rest.Config) (*kubernetes.Clientset, error) {
				return nil, nil
			})
			defer patch2.Reset()
			_, err := NewClient()
			So(err, ShouldNotBeNil)
		})
		Convey("test config error", func() {
			patch1 := ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
				return nil, errors.New("test")
			})
			defer patch1.Reset()
			_, err := NewClient()
			So(err, ShouldNotBeNil)
		})
		Convey("test new error", func() {
			patch1 := ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
				return nil, nil
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(kubernetes.NewForConfig, func(c *rest.Config) (*kubernetes.Clientset, error) {
				return nil, errors.New("test")
			})
			defer patch2.Reset()
			_, err := NewClient()
			So(err, ShouldNotBeNil)
		})
	})
}

func TestK8sClient_CreatePod(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		args    args
		want    *corev1.Pod
		wantErr bool
	}{
		{
			name: "test-nil",
			args: args{
				pod: nil,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "test-create-pod",
			args: args{
				pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
			},
			want:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &K8sClient{
				Interface: fake.NewSimpleClientset(),
			}
			got, err := k.CreatePod(tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreatePod() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestK8sClient_GetPod(t *testing.T) {
	type args struct {
		podName   string
		namespace string
	}
	tests := []struct {
		name    string
		pod     *corev1.Pod
		args    args
		want    *corev1.Pod
		wantErr bool
	}{
		{
			name: "test-nil",
			pod:  nil,
			args: args{
				podName:   "test",
				namespace: "test",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test-get-pod",
			pod:  &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			args: args{
				podName:   "test",
				namespace: "default",
			},
			want:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &K8sClient{
				Interface: fake.NewSimpleClientset(),
			}
			if tt.pod != nil {
				_, _ = k.CreatePod(tt.pod)
			}
			got, err := k.GetPod(tt.args.podName, tt.args.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreatePod() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestK8sClient_PatchPod(t *testing.T) {
	type args struct {
		pod  *corev1.Pod
		data map[string]interface{}
	}
	tests := []struct {
		name    string
		pod     *corev1.Pod
		args    args
		want    *corev1.Pod
		wantErr bool
	}{
		{
			name: "test-nil",
			pod:  nil,
			args: args{
				data: map[string]interface{}{},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "test-patch",
			pod:  &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}},
			args: args{
				pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}},
				data: map[string]interface{}{
					"metadata": map[string]map[string]string{"labels": {"test2": "test2"}},
				},
			},
			want: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
				Labels: map[string]string{
					"test2": "test2",
				},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &K8sClient{
				Interface: fake.NewSimpleClientset(),
			}
			if tt.pod != nil {
				_, _ = k.CreatePod(tt.pod)
			}
			data, err := json.Marshal(tt.args.data)
			if err != nil {
				t.Errorf("Parse json error: %v", err)
				return
			}
			err = k.PatchPod(tt.args.pod, data)
			if (err != nil) != tt.wantErr {
				t.Errorf("PatchPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.pod != nil {
				got, _ := k.GetPod(tt.pod.Name, tt.pod.Namespace)
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("PatchPod() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestK8sClient_UpdatePod(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		pod     *corev1.Pod
		args    args
		want    *corev1.Pod
		wantErr bool
	}{
		{
			name:    "test-nil",
			pod:     nil,
			args:    args{},
			want:    nil,
			wantErr: false,
		},
		{
			name: "test-update",
			pod:  &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}},
			args: args{
				pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Labels: map[string]string{
						"test2": "test2",
					},
				}},
			},
			want: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
				Labels: map[string]string{
					"test2": "test2",
				},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &K8sClient{
				Interface: fake.NewSimpleClientset(),
			}
			if tt.pod != nil {
				_, _ = k.CreatePod(tt.pod)
			}
			err := k.UpdatePod(tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdatePod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.pod != nil {
				got, _ := k.GetPod(tt.pod.Name, tt.pod.Namespace)
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("UpdatePod() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestK8sClient_DeletePod(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		pod     *corev1.Pod
		args    args
		wantErr bool
	}{
		{
			name:    "test-nil",
			pod:     nil,
			args:    args{},
			wantErr: false,
		},
		{
			name: "test-delete",
			pod:  &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}},
			args: args{
				pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &K8sClient{
				Interface: fake.NewSimpleClientset(),
			}
			if tt.pod != nil {
				_, _ = k.CreatePod(tt.pod)
			}
			err := k.DeletePod(tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeletePod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.pod != nil {
				got, err := k.GetPod(tt.pod.Name, tt.pod.Namespace)
				if err == nil || got != nil {
					t.Errorf("DeletePod() error = %v, got %v", err, got)
					return
				}
			}
		})
	}
}
