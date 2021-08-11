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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/mount"
	"reflect"
	"testing"
)

var test1 = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-a",
	},
}

var test2 = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-b",
		Annotations: map[string]string{
			getReferenceKey("/mnt/abc"): "/mnt/abc"},
	},
}

var test3 = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-c",
		Annotations: map[string]string{
			getReferenceKey("/mnt/abc"): "/mnt/abc"},
	},
}

var test4 = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-d",
		Annotations: map[string]string{"a": "b",
			getReferenceKey("/mnt/def"): "/mnt/def"},
	},
}

var test5 = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-e",
		Annotations: map[string]string{
			getReferenceKey("/mnt/abc"): "/mnt/abc",
			getReferenceKey("/mnt/def"): "/mnt/def",
		},
	},
}

var test6 = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-f",
	},
}

var test7 = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-g",
		Annotations: map[string]string{
			getReferenceKey("/mnt/abc"): "/mnt/abc",
			getReferenceKey("/mnt/def"): "/mnt/def",
		},
	},
}

func setup() {
	FakeClient.Flush()
	NodeName = "test-node"
	Namespace = "kube-system"
	_, _ = FakeClient.CreatePod(test1)
	_, _ = FakeClient.CreatePod(test2)
	_, _ = FakeClient.CreatePod(test3)
	_, _ = FakeClient.CreatePod(test4)
	_, _ = FakeClient.CreatePod(test5)
	_, _ = FakeClient.CreatePod(test6)
	_, _ = FakeClient.CreatePod(test7)
}

func teardown() {
	FakeClient.Flush()
}

func Test_juicefs_addRefOfMount(t *testing.T) {
	teardown()
	setup()
	type fields struct {
		SafeFormatAndMount mount.SafeFormatAndMount
		K8sClient          K8sClient
	}
	type args struct {
		target string
		pod    *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test-nil",
			fields: fields{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          FakeClient,
			},
			args: args{
				target: "/mnt/abc",
				pod:    test1,
			},
			wantErr: false,
		},
		{
			name: "test2",
			fields: fields{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          FakeClient,
			},
			args: args{
				target: "/mnt/abc",
				pod:    test2,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &juicefs{
				SafeFormatAndMount: tt.fields.SafeFormatAndMount,
				K8sClient:          tt.fields.K8sClient,
			}
			if err := j.addRefOfMount(tt.args.target, tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("addRefOfMount() error = %v, wantErr %v", err, tt.wantErr)
			}
			key := getReferenceKey(tt.args.target)
			wanted := tt.args.pod
			if wanted.Annotations == nil {
				wanted.Annotations = make(map[string]string)
			}
			wanted.Annotations[key] = tt.args.target
			newPod, _ := FakeClient.GetPod(tt.args.pod.Name, tt.args.pod.Namespace)
			if !reflect.DeepEqual(newPod.Annotations, wanted.Annotations) {
				t.Errorf("addRefOfMount err, wanted: %v, got: %v", wanted, tt.args.pod)
			}
		})
	}
}

func Test_juicefs_DelRefOfMountPod(t *testing.T) {
	teardown()
	setup()
	type fields struct {
		SafeFormatAndMount mount.SafeFormatAndMount
		K8sClient          K8sClient
	}
	type args struct {
		volumeId string
		target   string
	}
	var tests = []struct {
		name            string
		fields          fields
		args            args
		wantErr         bool
		wantPodDeleted  bool
		wantAnnotations map[string]string
	}{
		{
			name: "test-delete",
			fields: fields{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          FakeClient,
			},
			args: args{
				volumeId: "c",
				target:   "/mnt/abc",
			},
			wantErr:         false,
			wantPodDeleted:  true,
			wantAnnotations: nil,
		},
		{
			name: "test-delete2",
			fields: fields{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          FakeClient,
			},
			args: args{
				volumeId: "d",
				target:   "/mnt/def",
			},
			wantErr:         false,
			wantPodDeleted:  true,
			wantAnnotations: nil,
		},
		{
			name: "test-true",
			fields: fields{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          FakeClient,
			},
			args: args{
				volumeId: "e",
				target:   "/mnt/def",
			},
			wantErr:        false,
			wantPodDeleted: false,
			wantAnnotations: map[string]string{
				getReferenceKey("/mnt/abc"): "/mnt/abc",
			},
		},
		{
			name: "test-delete3",
			fields: fields{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          FakeClient,
			},
			args: args{
				volumeId: "f",
				target:   "/mnt/def",
			},
			wantErr:         false,
			wantPodDeleted:  true,
			wantAnnotations: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &juicefs{
				SafeFormatAndMount: tt.fields.SafeFormatAndMount,
				K8sClient:          tt.fields.K8sClient,
			}
			if err := j.DelRefOfMountPod(tt.args.volumeId, tt.args.target); (err != nil) != tt.wantErr {
				t.Errorf("DelRefOfMountPod() error = %v, wantErr %v", err, tt.wantErr)
			}
			got, _ := FakeClient.GetPod(GeneratePodNameByVolumeId(tt.args.volumeId), Namespace)
			if tt.wantPodDeleted && got != nil {
				t.Errorf("DelRefOfMountPod() got: %v, wanted pod deleted: %v", got, tt.wantPodDeleted)
			}
			if !tt.wantPodDeleted && !reflect.DeepEqual(got.Annotations, tt.wantAnnotations) {
				t.Errorf("DelRefOfMountPod() got: %v, wanted: %v", got.Annotations, tt.wantAnnotations)
			}
		})
	}
}

func Test_juicefs_waitUntilMount(t *testing.T) {
	type fields struct {
		SafeFormatAndMount mount.SafeFormatAndMount
		K8sClient          K8sClient
	}
	type args struct {
		volumeId   string
		target     string
		mountPath  string
		cmd        string
		jfsSetting *JfsSetting
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantErr  bool
		wantAnno map[string]string
	}{
		{
			name: "test-new",
			fields: fields{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          FakeClient,
			},
			args: args{
				volumeId:   "h",
				target:     "/mnt/hhh",
				mountPath:  "/mnt/hhh",
				cmd:        "/local/bin/juicefs.mount test",
				jfsSetting: nil,
			},
			wantErr:  false,
			wantAnno: map[string]string{getReferenceKey("/mnt/hhh"): "/mnt/hhh"},
		},
		{
			name: "test-exists",
			fields: fields{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          FakeClient,
			},
			args: args{
				volumeId:   "g",
				target:     "/mnt/ggg",
				mountPath:  "/mnt/ggg",
				cmd:        "/local/bin/juicefs.mount test",
				jfsSetting: nil,
			},
			wantErr: false,
			wantAnno: map[string]string{
				getReferenceKey("/mnt/abc"): "/mnt/abc",
				getReferenceKey("/mnt/def"): "/mnt/def",
				getReferenceKey("/mnt/ggg"): "/mnt/ggg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &juicefs{
				SafeFormatAndMount: tt.fields.SafeFormatAndMount,
				K8sClient:          tt.fields.K8sClient,
			}
			if err := j.waitUntilMount(tt.args.volumeId, tt.args.target, tt.args.mountPath, tt.args.cmd, tt.args.jfsSetting); (err != nil) != tt.wantErr {
				t.Errorf("waitUntilMount() error = %v, wantErr %v", err, tt.wantErr)
			}

			newPod, _ := FakeClient.GetPod(GeneratePodNameByVolumeId(tt.args.volumeId), Namespace)
			if newPod == nil || !reflect.DeepEqual(newPod.Annotations, tt.wantAnno) {
				t.Errorf("waitUntilMount() got = %v, wantAnnotation = %v", newPod, tt.wantAnno)
			}
		})
	}
}
