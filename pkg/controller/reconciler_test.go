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

package controller

import (
	"context"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func TestReconcile(t *testing.T) {
	Convey("Test Reconcile", t, func() {
		Convey("test normal", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
					Labels:    map[string]string{config.PodTypeKey: config.PodTypeValue},
				},
				Spec: corev1.PodSpec{NodeName: "test"},
			}
			config.NodeName = "test"

			kc := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
			_, _ = kc.CreatePod(pod)

			podReconciler := PodReconciler{mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			}, kc}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			}

			_, err := podReconciler.Reconcile(context.TODO(), req)
			So(err, ShouldBeNil)
		})
		Convey("pod not found", func() {
			kc := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}

			podReconciler := PodReconciler{mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			}, kc}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "test",
					Name:      "test",
				},
			}

			_, err := podReconciler.Reconcile(context.TODO(), req)
			So(err, ShouldNotBeNil)
		})
		Convey("pod with no label", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			}

			kc := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
			_, _ = kc.CreatePod(pod)

			podReconciler := PodReconciler{mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			}, kc}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			}

			rsp, err := podReconciler.Reconcile(context.TODO(), req)
			So(err, ShouldBeNil)
			So(rsp.Requeue, ShouldBeTrue)
		})
		Convey("test not node", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
					Labels:    map[string]string{config.PodTypeKey: config.PodTypeValue},
				},
				Spec: corev1.PodSpec{NodeName: "test"},
			}
			config.NodeName = "test2"

			kc := &k8s.K8sClient{Interface: fake.NewSimpleClientset()}
			_, _ = kc.CreatePod(pod)

			podReconciler := PodReconciler{mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			}, kc}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			}

			rsp, err := podReconciler.Reconcile(context.TODO(), req)
			So(err, ShouldBeNil)
			So(rsp.Requeue, ShouldBeTrue)
		})
	})
}

func TestPodReconciler_fetchPod(t *testing.T) {
	type args struct {
		name types.NamespacedName
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantPod *corev1.Pod
		wantErr bool
	}{
		{
			name: "test-right",
			args: args{
				name: types.NamespacedName{
					Namespace: "test",
					Name:      "test",
				},
			},
			want: false,
			wantPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"},
			},
			wantErr: false,
		},
		{
			name: "test-wrong",
			args: args{
				name: types.NamespacedName{
					Namespace: "test",
					Name:      "test",
				},
			},
			want:    true,
			wantPod: nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kc := k8s.K8sClient{Interface: fake.NewSimpleClientset()}
			if tt.wantPod != nil {
				_, _ = kc.CreatePod(tt.wantPod)
			}
			p := &PodReconciler{
				K8sClient: &kc,
			}
			got, gotPod, err := p.fetchPod(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("fetchPod() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(gotPod, tt.wantPod) {
				t.Errorf("fetchPod() gotPod = %v, want %v", gotPod, tt.wantPod)
			}
		})
	}
}
