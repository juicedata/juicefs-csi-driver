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

package mount

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
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
	k8sclient.FakeClient.Flush()
	jfsConfig.NodeName = "test-node"
	jfsConfig.Namespace = "kube-system"
	_, _ = k8sclient.FakeClient.CreatePod(test1)
	_, _ = k8sclient.FakeClient.CreatePod(test2)
	_, _ = k8sclient.FakeClient.CreatePod(test3)
	_, _ = k8sclient.FakeClient.CreatePod(test4)
	_, _ = k8sclient.FakeClient.CreatePod(test5)
	_, _ = k8sclient.FakeClient.CreatePod(test6)
	_, _ = k8sclient.FakeClient.CreatePod(test7)
}

func teardown() {
	k8sclient.FakeClient.Flush()
}

func Test_juicefs_addRefOfMount(t *testing.T) {
	teardown()
	setup()
	type fields struct {
		SafeFormatAndMount mount.SafeFormatAndMount
		jfsSetting         *jfsConfig.JfsSetting
		K8sClient          k8sclient.K8sClient
	}
	type args struct {
		target  string
		podName string
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
				jfsSetting:         &jfsConfig.JfsSetting{},
				K8sClient:          k8sclient.FakeClient,
			},
			args: args{
				target:  "/mnt/abc",
				podName: test1.Name,
			},
			wantErr: false,
		},
		{
			name: "test2",
			fields: fields{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				jfsSetting:         &jfsConfig.JfsSetting{},
				K8sClient:          k8sclient.FakeClient,
			},
			args: args{
				target:  "/mnt/abc",
				podName: test2.Name,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := getReferenceKey(tt.args.target)
			old, err := k8sclient.FakeClient.GetPod(tt.args.podName, jfsConfig.Namespace)
			if err != nil {
				t.Errorf("Can't get pod: %v", tt.args.podName)
			}
			if old.Annotations == nil {
				old.Annotations = make(map[string]string)
			}
			old.Annotations[key] = tt.args.target
			p := &PodMount{
				SafeFormatAndMount: tt.fields.SafeFormatAndMount,
				jfsSetting:         tt.fields.jfsSetting,
				K8sClient:          tt.fields.K8sClient,
			}
			if err := p.AddRefOfMount(tt.args.target, tt.args.podName); (err != nil) != tt.wantErr {
				t.Errorf("AddRefOfMount() error = %v, wantErr %v", err, tt.wantErr)
			}
			newPod, _ := k8sclient.FakeClient.GetPod(tt.args.podName, jfsConfig.Namespace)
			if !reflect.DeepEqual(newPod.Annotations, old.Annotations) {
				t.Errorf("addRefOfMount err, wanted: %v, got: %v", old.Annotations, newPod.Annotations)
			}
		})
	}
}

func Test_juicefs_JUmount(t *testing.T) {
	teardown()
	setup()
	type fields struct {
		SafeFormatAndMount mount.SafeFormatAndMount
		jfsSetting         *jfsConfig.JfsSetting
		K8sClient          k8sclient.K8sClient
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
				K8sClient:          k8sclient.FakeClient,
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
				K8sClient:          k8sclient.FakeClient,
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
				K8sClient:          k8sclient.FakeClient,
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
				K8sClient:          k8sclient.FakeClient,
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
			p := &PodMount{
				SafeFormatAndMount: tt.fields.SafeFormatAndMount,
				jfsSetting:         tt.fields.jfsSetting,
				K8sClient:          tt.fields.K8sClient,
			}
			if err := p.JUmount(tt.args.volumeId, tt.args.target); (err != nil) != tt.wantErr {
				t.Errorf("JUmount() error = %v, wantErr %v", err, tt.wantErr)
			}
			got, _ := k8sclient.FakeClient.GetPod(GeneratePodNameByVolumeId(tt.args.volumeId), jfsConfig.Namespace)
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
		jfsSetting         *jfsConfig.JfsSetting
		K8sClient          k8sclient.K8sClient
	}
	type args struct {
		volumeId   string
		target     string
		mountPath  string
		cmd        string
		jfsSetting *jfsConfig.JfsSetting
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
				jfsSetting:         &jfsConfig.JfsSetting{},
				K8sClient:          k8sclient.FakeClient,
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
				jfsSetting:         &jfsConfig.JfsSetting{},
				K8sClient:          k8sclient.FakeClient,
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
			p := &PodMount{
				SafeFormatAndMount: tt.fields.SafeFormatAndMount,
				jfsSetting:         tt.fields.jfsSetting,
				K8sClient:          tt.fields.K8sClient,
			}
			if err := p.waitUntilMount(tt.args.volumeId, tt.args.target, tt.args.mountPath, tt.args.cmd); (err != nil) != tt.wantErr {
				t.Errorf("waitUntilMount() error = %v, wantErr %v", err, tt.wantErr)
			}
			newPod, _ := k8sclient.FakeClient.GetPod(GeneratePodNameByVolumeId(tt.args.volumeId), jfsConfig.Namespace)
			if newPod == nil || !reflect.DeepEqual(newPod.Annotations, tt.wantAnno) {
				t.Errorf("waitUntilMount() got = %v, wantAnnotation = %v", newPod, tt.wantAnno)
			}
		})
	}
}
