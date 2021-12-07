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
	"errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"reflect"
	"testing"

	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var testA = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-a",
	},
}

var testB = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-b",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
	},
}

var testC = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-c",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
	},
}

var testD = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-d",
		Annotations: map[string]string{"a": "b",
			util.GetReferenceKey("/mnt/def"): "/mnt/def"},
	},
}

var testE = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-e",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc",
			util.GetReferenceKey("/mnt/def"): "/mnt/def",
		},
	},
}

var testF = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-f",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   corev1.ContainersReady,
			Status: corev1.ConditionTrue,
		}},
	},
}

var testG = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-g",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc",
			util.GetReferenceKey("/mnt/def"): "/mnt/def",
		},
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   corev1.ContainersReady,
			Status: corev1.ConditionTrue,
		}},
	},
}

var testH = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-h",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   corev1.ContainersReady,
			Status: corev1.ConditionTrue,
		}},
	},
}

func init() {
	jfsConfig.NodeName = "test-node"
}

func TestAddRefOfMount(t *testing.T) {
	fakeClientSet := fake.NewSimpleClientset()
	type fields struct {
		jfsSetting *jfsConfig.JfsSetting
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
				jfsSetting: &jfsConfig.JfsSetting{},
			},
			args: args{
				target: "/mnt/abc",
				pod:    testA,
			},
			wantErr: false,
		},
		{
			name: "test2",
			fields: fields{
				jfsSetting: &jfsConfig.JfsSetting{},
			},
			args: args{
				target: "/mnt/abc",
				pod:    testB,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				jfsSetting:         tt.fields.jfsSetting,
				K8sClient:          &k8sclient.K8sClient{Interface: fakeClientSet},
			}
			key := util.GetReferenceKey(tt.args.target)
			_, _ = p.K8sClient.CreatePod(tt.args.pod)
			old, err := p.K8sClient.GetPod(tt.args.pod.Name, jfsConfig.Namespace)
			if err != nil {
				t.Errorf("Can't get pod: %v", tt.args.pod.Name)
			}
			if old.Annotations == nil {
				old.Annotations = make(map[string]string)
			}
			old.Annotations[key] = tt.args.target
			if err := p.AddRefOfMount(tt.args.target, tt.args.pod.Name); (err != nil) != tt.wantErr {
				t.Errorf("AddRefOfMount() error = %v, wantErr %v", err, tt.wantErr)
			}
			newPod, _ := p.K8sClient.GetPod(tt.args.pod.Name, jfsConfig.Namespace)
			if !reflect.DeepEqual(newPod.Annotations, old.Annotations) {
				t.Errorf("addRefOfMount err, wanted: %v, got: %v", old.Annotations, newPod.Annotations)
			}
		})
	}
}

func TestJUmount(t *testing.T) {
	fakeClientSet := fake.NewSimpleClientset()

	type fields struct {
		jfsSetting *jfsConfig.JfsSetting
	}
	type args struct {
		volumeId string
		target   string
	}
	var tests = []struct {
		name            string
		fields          fields
		args            args
		pod             *corev1.Pod
		wantErr         bool
		wantPodDeleted  bool
		wantAnnotations map[string]string
	}{
		{
			name:   "test-delete",
			fields: fields{},
			args: args{
				volumeId: "c",
				target:   "/mnt/abc",
			},
			pod:             testC,
			wantErr:         false,
			wantPodDeleted:  true,
			wantAnnotations: nil,
		},
		{
			name:   "test-delete2",
			fields: fields{},
			args: args{
				volumeId: "d",
				target:   "/mnt/def",
			},
			pod:             testD,
			wantErr:         false,
			wantPodDeleted:  true,
			wantAnnotations: nil,
		},
		{
			name:   "test-true",
			fields: fields{},
			args: args{
				volumeId: "e",
				target:   "/mnt/def",
			},
			pod:            testE,
			wantErr:        false,
			wantPodDeleted: false,
			wantAnnotations: map[string]string{
				util.GetReferenceKey("/mnt/abc"): "/mnt/abc",
			},
		},
		{
			name:   "test-delete3",
			fields: fields{},
			args: args{
				volumeId: "f",
				target:   "/mnt/def",
			},
			pod:             testF,
			wantErr:         false,
			wantPodDeleted:  true,
			wantAnnotations: nil,
		},
		{
			name:   "test-nil",
			fields: fields{},
			args: args{
				volumeId: "x",
				target:   "/mnt/def",
			},
			pod:             nil,
			wantErr:         false,
			wantPodDeleted:  true,
			wantAnnotations: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				jfsSetting:         tt.fields.jfsSetting,
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClientSet,
				},
			}
			if tt.pod != nil {
				_, _ = p.K8sClient.CreatePod(tt.pod)
			}
			if err := p.JUmount(tt.args.volumeId, tt.args.target); (err != nil) != tt.wantErr {
				t.Errorf("JUmount() error = %v, wantErr %v", err, tt.wantErr)
			}
			got, _ := p.K8sClient.GetPod(GeneratePodNameByVolumeId(tt.args.volumeId), jfsConfig.Namespace)
			if tt.wantPodDeleted && got != nil {
				t.Errorf("DelRefOfMountPod() got: %v, wanted pod deleted: %v", got, tt.wantPodDeleted)
			}
			if !tt.wantPodDeleted && !reflect.DeepEqual(got.Annotations, tt.wantAnnotations) {
				t.Errorf("DelRefOfMountPod() got: %v, wanted: %v", got.Annotations, tt.wantAnnotations)
			}
		})
	}
}

func TestJUmountWithMock(t *testing.T) {
	Convey("Test JUmount", t, func() {
		Convey("pod notfound", func() {
			patch1 := ApplyFunc(k8serrors.IsNotFound, func(err error) bool {
				return false
			})
			defer patch1.Reset()
			client := &k8sclient.K8sClient{}
			patch2 := ApplyMethod(reflect.TypeOf(client), "GetPod", func(_ *k8sclient.K8sClient, podName, namespace string) (*corev1.Pod, error) {
				return nil, errors.New("test")
			})
			defer patch2.Reset()

			p := NewPodMount(nil, &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()})
			err := p.JUmount("ttt", "/test")
			So(err, ShouldNotBeNil)
		})
		Convey("pod hasRef", func() {
			patch1 := ApplyFunc(hasRef, func(pod *corev1.Pod) bool {
				return true
			})
			defer patch1.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				jfsSetting:         nil,
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			p.K8sClient.CreatePod(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GeneratePodNameByVolumeId("ttt"),
					Namespace: jfsConfig.Namespace,
					Annotations: map[string]string{
						GeneratePodNameByVolumeId("ttt"): "/test",
					},
				},
			})
			err := p.JUmount("ttt", "/test")
			So(err, ShouldBeNil)
		})
		Convey("pod conflict", func() {
			patch1 := ApplyFunc(k8serrors.IsConflict, func(err error) bool {
				return true
			})
			defer patch1.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				jfsSetting:         nil,
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			p.K8sClient.CreatePod(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GeneratePodNameByVolumeId("ttt"),
					Namespace: jfsConfig.Namespace,
					Annotations: map[string]string{
						util.GetReferenceKey("ttt"): "/test",
					},
				},
			})
			err := p.JUmount("ttt", "/test")
			So(err, ShouldBeNil)
		})
		Convey("pod update error", func() {
			client := &k8sclient.K8sClient{}
			patch1 := ApplyMethod(reflect.TypeOf(client), "UpdatePod", func(_ *k8sclient.K8sClient, pod *corev1.Pod) error {
				return errors.New("test")
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(k8serrors.IsConflict, func(err error) bool {
				return false
			})
			defer patch2.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				jfsSetting:         nil,
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			p.K8sClient.CreatePod(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GeneratePodNameByVolumeId("aaa"),
					Namespace: jfsConfig.Namespace,
					Annotations: map[string]string{
						util.GetReferenceKey("/test"): "/test",
					},
				},
			})
			err := p.JUmount("aaa", "/test")
			So(err, ShouldNotBeNil)
		})
		Convey("pod delete error", func() {
			client := &k8sclient.K8sClient{}
			patch1 := ApplyMethod(reflect.TypeOf(client), "DeletePod", func(_ *k8sclient.K8sClient, pod *corev1.Pod) error {
				return errors.New("test")
			})
			defer patch1.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				jfsSetting:         nil,
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			p.K8sClient.CreatePod(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      GeneratePodNameByVolumeId("ttt"),
					Namespace: jfsConfig.Namespace,
				},
			})
			err := p.JUmount("ttt", "/test")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestWaitUntilMount(t *testing.T) {
	fakeClientSet := fake.NewSimpleClientset()
	type fields struct {
		SafeFormatAndMount mount.SafeFormatAndMount
		jfsSetting         *jfsConfig.JfsSetting
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
		pod      *corev1.Pod
		wantErr  bool
		wantAnno map[string]string
	}{
		{
			name: "test-new",
			fields: fields{
				jfsSetting: &jfsConfig.JfsSetting{},
			},
			args: args{
				volumeId:   "h",
				target:     "/mnt/hhh",
				mountPath:  "/mnt/hhh",
				cmd:        "/local/bin/juicefs.mount test",
				jfsSetting: nil,
			},
			pod:      testH,
			wantErr:  false,
			wantAnno: map[string]string{util.GetReferenceKey("/mnt/hhh"): "/mnt/hhh"},
		},
		{
			name: "test-exists",
			fields: fields{
				jfsSetting: &jfsConfig.JfsSetting{},
			},
			args: args{
				volumeId:   "g",
				target:     "/mnt/ggg",
				mountPath:  "/mnt/ggg",
				cmd:        "/local/bin/juicefs.mount test",
				jfsSetting: nil,
			},
			pod:     testG,
			wantErr: false,
			wantAnno: map[string]string{
				util.GetReferenceKey("/mnt/abc"): "/mnt/abc",
				util.GetReferenceKey("/mnt/def"): "/mnt/def",
				util.GetReferenceKey("/mnt/ggg"): "/mnt/ggg",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				jfsSetting:         tt.fields.jfsSetting,
				K8sClient:          &k8sclient.K8sClient{Interface: fakeClientSet},
			}
			_, _ = p.K8sClient.CreatePod(tt.pod)
			if err := p.waitUntilMount(tt.args.volumeId, tt.args.target, tt.args.mountPath, tt.args.cmd); (err != nil) != tt.wantErr {
				t.Errorf("waitUntilMount() error = %v, wantErr %v", err, tt.wantErr)
			}
			newPod, _ := p.K8sClient.GetPod(GeneratePodNameByVolumeId(tt.args.volumeId), jfsConfig.Namespace)
			if newPod == nil || !reflect.DeepEqual(newPod.Annotations, tt.wantAnno) {
				t.Errorf("waitUntilMount() got = %v, wantAnnotation = %v", newPod, tt.wantAnno)
			}
		})
	}
}

func TestNewPodMount(t *testing.T) {
	type args struct {
		setting *jfsConfig.JfsSetting
		client  *k8sclient.K8sClient
	}
	tests := []struct {
		name string
		args args
		want Interface
	}{
		{
			name: "test",
			args: args{
				setting: nil,
				client:  nil,
			},
			want: &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				},
				jfsSetting: nil,
				K8sClient:  nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewPodMount(tt.args.setting, tt.args.client); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPodMount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodMount_JMount(t *testing.T) {
	type args struct {
		storage   string
		volumeId  string
		mountPath string
		target    string
		options   []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test-mount",
			args:    args{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodMount{}
			if err := p.JMount(tt.args.storage, tt.args.volumeId, tt.args.mountPath, tt.args.target, tt.args.options); (err != nil) != tt.wantErr {
				t.Errorf("JMount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
