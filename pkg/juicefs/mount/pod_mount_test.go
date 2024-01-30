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
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver/mocks"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var testA = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-a",
		Labels: map[string]string{
			jfsConfig.PodUniqueIdLabelKey: "a",
		},
	},
}

var testB = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-b",
		Labels: map[string]string{
			jfsConfig.PodUniqueIdLabelKey: "b",
		},
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
	},
}

var testC = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-c",
		Labels: map[string]string{
			jfsConfig.PodUniqueIdLabelKey: "c",
		},
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
	},
}

var testD = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-d",
		Labels: map[string]string{
			jfsConfig.PodUniqueIdLabelKey: "d",
		},
		Annotations: map[string]string{"a": "b",
			util.GetReferenceKey("/mnt/def"): "/mnt/def"},
	},
}

var testE = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-e",
		Labels: map[string]string{
			jfsConfig.PodUniqueIdLabelKey: "e",
		},
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc",
			util.GetReferenceKey("/mnt/def"): "/mnt/def",
		},
	},
}

var testF = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-f",
		Labels: map[string]string{
			jfsConfig.PodUniqueIdLabelKey: "f",
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

var testG = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-g",
		Labels: map[string]string{
			jfsConfig.PodUniqueIdLabelKey: "g",
		},
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
		Labels: map[string]string{
			jfsConfig.PodUniqueIdLabelKey: "h",
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
				K8sClient:          &k8sclient.K8sClient{Interface: fakeClientSet},
			}
			key := util.GetReferenceKey(tt.args.target)
			_, _ = p.K8sClient.CreatePod(context.TODO(), tt.args.pod)
			old, err := p.K8sClient.GetPod(context.TODO(), tt.args.pod.Name, jfsConfig.Namespace)
			if err != nil {
				t.Errorf("Can't get pod: %v", tt.args.pod.Name)
			}
			if old.Annotations == nil {
				old.Annotations = make(map[string]string)
			}
			old.Annotations[key] = tt.args.target
			if err := p.AddRefOfMount(context.TODO(), tt.args.target, tt.args.pod.Name); (err != nil) != tt.wantErr {
				t.Errorf("AddRefOfMount() error = %v, wantErr %v", err, tt.wantErr)
			}
			newPod, _ := p.K8sClient.GetPod(context.TODO(), tt.args.pod.Name, jfsConfig.Namespace)
			if !reflect.DeepEqual(newPod.Annotations, old.Annotations) {
				t.Errorf("addRefOfMount err, wanted: %v, got: %v", old.Annotations, newPod.Annotations)
			}
		})
	}
}

func TestAddRefOfMountWithMock(t *testing.T) {
	Convey("Test AddRefOfMount", t, func() {
		Convey("get pod error", func() {
			client := &k8sclient.K8sClient{}
			patch1 := ApplyMethod(reflect.TypeOf(client), "GetPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string) (*corev1.Pod, error) {
				return nil, errors.New("test")
			})
			defer patch1.Reset()
			p := &PodMount{
				K8sClient: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := p.AddRefOfMount(ctx, "test-target", "test-pod")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestJUmount(t *testing.T) {
	fakeClientSet := fake.NewSimpleClientset()

	type args struct {
		podName string
		target  string
	}
	var tests = []struct {
		name           string
		args           args
		pod            *corev1.Pod
		wantErr        bool
		wantPodDeleted bool
	}{
		{
			name: "test-delete",
			args: args{
				podName: testC.Name,
				target:  "/mnt/abc",
			},
			pod:            testC,
			wantErr:        false,
			wantPodDeleted: false,
		},
		{
			name: "test-delete2",
			args: args{
				podName: testD.Name,
				target:  "/mnt/def",
			},
			pod:            testD,
			wantErr:        false,
			wantPodDeleted: false,
		},
		{
			name: "test-true",
			args: args{
				podName: testE.Name,
				target:  "/mnt/def",
			},
			pod:            testE,
			wantErr:        false,
			wantPodDeleted: false,
		},
		{
			name: "test-delete3",
			args: args{
				podName: testF.Name,
				target:  "/mnt/def",
			},
			pod:            testF,
			wantErr:        false,
			wantPodDeleted: true,
		},
		{
			name: "test-nil",
			args: args{
				podName: "x",
				target:  "/mnt/def",
			},
			pod:            nil,
			wantErr:        false,
			wantPodDeleted: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClientSet,
				},
			}
			if tt.pod != nil {
				_, _ = p.K8sClient.CreatePod(context.TODO(), tt.pod)
			}
			if err := p.JUmount(context.TODO(), tt.args.target, tt.args.podName); (err != nil) != tt.wantErr {
				t.Errorf("JUmount() error = %v, wantErr %v", err, tt.wantErr)
			}
			got, _ := p.K8sClient.GetPod(context.TODO(), tt.args.podName, jfsConfig.Namespace)
			if tt.wantPodDeleted && got != nil {
				t.Errorf("JUmount() got: %v, wanted pod deleted: %v", got, tt.wantPodDeleted)
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
			patch2 := ApplyMethod(reflect.TypeOf(client), "GetPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string) (*corev1.Pod, error) {
				return nil, errors.New("test")
			})
			defer patch2.Reset()

			p := NewPodMount(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := p.JUmount(ctx, "/test", "ttt")
			So(err, ShouldNotBeNil)
		})
		Convey("pod hasRef", func() {
			patch1 := ApplyFunc(GetRef, func(pod *corev1.Pod) int {
				return 1
			})
			defer patch1.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			podName := GenPodNameByUniqueId("ttt", true)
			p.K8sClient.CreatePod(context.TODO(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: jfsConfig.Namespace,
					Annotations: map[string]string{
						podName: "/test",
					},
				},
			})
			err := p.JUmount(context.TODO(), "/test", podName)
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
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			podName := GenPodNameByUniqueId("ttt", true)
			p.K8sClient.CreatePod(context.TODO(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: jfsConfig.Namespace,
					Annotations: map[string]string{
						util.GetReferenceKey("ttt"): "/test",
					},
				},
			})
			err := p.JUmount(context.TODO(), "/test", podName)
			So(err, ShouldBeNil)
		})
		Convey("pod delete error", func() {
			client := &k8sclient.K8sClient{}
			patch1 := ApplyMethod(reflect.TypeOf(client), "DeletePod", func(_ *k8sclient.K8sClient, _ context.Context, pod *corev1.Pod) error {
				return errors.New("test")
			})
			defer patch1.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			podName := GenPodNameByUniqueId("ttt", true)
			p.K8sClient.CreatePod(context.TODO(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: jfsConfig.Namespace,
				},
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := p.JUmount(ctx, "/test", podName)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestUmountTarget(t *testing.T) {
	Convey("Test JUmount", t, func() {
		Convey("pod notfound", func() {
			patch1 := ApplyFunc(k8serrors.IsNotFound, func(err error) bool {
				return false
			})
			defer patch1.Reset()
			client := &k8sclient.K8sClient{}
			patch2 := ApplyMethod(reflect.TypeOf(client), "GetPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string) (*corev1.Pod, error) {
				return nil, errors.New("test")
			})
			defer patch2.Reset()
			tmpCmd := &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte("not mounted"), errors.New("not mounted")
			})
			defer patch3.Reset()
			patch4 := ApplyFunc(mount.CleanupMountPoint, func(mountPath string, mounter mount.Interface, extensiveMountPointCheck bool) error {
				return nil
			})
			defer patch4.Reset()

			p := NewPodMount(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := p.UmountTarget(ctx, "/test", "ttt")
			So(err, ShouldNotBeNil)
		})
		Convey("pod conflict", func() {
			patch1 := ApplyFunc(k8serrors.IsConflict, func(err error) bool {
				return true
			})
			defer patch1.Reset()
			tmpCmd := &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte("not mounted"), errors.New("not mounted")
			})
			defer patch3.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			t.Logf("PodMount %T %v", p, p)
			podName := GenPodNameByUniqueId("ttt", true)
			p.K8sClient.CreatePod(context.TODO(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: jfsConfig.Namespace,
					Annotations: map[string]string{
						util.GetReferenceKey("ttt"): "/test",
					},
				},
			})
			err := p.UmountTarget(context.TODO(), "/test", podName)
			So(err, ShouldBeNil)
		})
		Convey("pod update error", func() {
			client := &k8sclient.K8sClient{}
			patch1 := ApplyMethod(reflect.TypeOf(client), "PatchPod", func(_ *k8sclient.K8sClient, _ context.Context, pod *corev1.Pod, data []byte, pt types.PatchType) error {
				return errors.New("test")
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(k8serrors.IsConflict, func(err error) bool {
				return false
			})
			defer patch2.Reset()
			tmpCmd := &exec.Cmd{}
			patch3 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte("not mounted"), errors.New("not mounted")
			})
			defer patch3.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			podName := GenPodNameByUniqueId("aaa", true)
			p.K8sClient.CreatePod(context.TODO(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: jfsConfig.Namespace,
					Annotations: map[string]string{
						util.GetReferenceKey("/test"): "/test",
					},
				},
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := p.UmountTarget(ctx, "/test", podName)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestWaitUntilMount(t *testing.T) {
	fakeClientSet := fake.NewSimpleClientset()
	type args struct {
		cmd        string
		jfsSetting *jfsConfig.JfsSetting
	}
	tests := []struct {
		name     string
		args     args
		pod      *corev1.Pod
		wantErr  bool
		wantAnno map[string]string
	}{
		{
			name: "test-new",
			args: args{
				cmd: "/local/bin/juicefs.mount test",
				jfsSetting: &jfsConfig.JfsSetting{
					VolumeId:   "h",
					TargetPath: "/mnt/hhh",
					MountPath:  "/mnt/hhh",
				},
			},
			pod:      testH,
			wantErr:  false,
			wantAnno: map[string]string{util.GetReferenceKey("/mnt/hhh"): "/mnt/hhh"},
		},
		{
			name: "test-exists",
			args: args{
				jfsSetting: &jfsConfig.JfsSetting{
					VolumeId:   "g",
					TargetPath: "/mnt/ggg",
					MountPath:  "/mnt/ggg",
				},
			},
			pod:     testG,
			wantErr: false,
			wantAnno: map[string]string{
				util.GetReferenceKey("/mnt/abc"): "/mnt/abc",
				util.GetReferenceKey("/mnt/def"): "/mnt/def",
				util.GetReferenceKey("/mnt/ggg"): "/mnt/ggg",
			},
		},
		{
			name: "test-not-found",
			args: args{
				cmd: "/local/bin/juicefs.mount test",
				jfsSetting: &jfsConfig.JfsSetting{
					VolumeId:   "i",
					TargetPath: "/mnt/iii",
					MountPath:  "/mnt/iii",
				},
			},
			pod:     nil,
			wantErr: false,
			wantAnno: map[string]string{
				util.GetReferenceKey("/mnt/iii"): "/mnt/iii",
				jfsConfig.UniqueId:               "",
				jfsConfig.JuiceFSUUID:            "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          &k8sclient.K8sClient{Interface: fakeClientSet},
			}
			if tt.pod != nil {
				hashVal, _ := GenHashOfSetting(*tt.args.jfsSetting)
				tt.pod.Labels = map[string]string{
					jfsConfig.PodTypeKey:           jfsConfig.PodTypeValue,
					jfsConfig.PodUniqueIdLabelKey:  tt.args.jfsSetting.UniqueId,
					jfsConfig.PodJuiceHashLabelKey: hashVal,
				}
				tt.pod.Spec.NodeName = jfsConfig.NodeName
				_, _ = p.K8sClient.CreatePod(context.TODO(), tt.pod)
			}
			podName, err := p.genMountPodName(context.TODO(), tt.args.jfsSetting)
			if (err != nil) != tt.wantErr {
				t.Errorf("createOrAddRef() error = %v, wantErr %v", err, tt.wantErr)
			}
			err = p.createOrAddRef(context.TODO(), podName, tt.args.jfsSetting, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("createOrAddRef() error = %v, wantErr %v", err, tt.wantErr)
			}
			newPod, _ := p.K8sClient.GetPod(context.TODO(), podName, jfsConfig.Namespace)
			if newPod == nil || !reflect.DeepEqual(newPod.Annotations, tt.wantAnno) {
				t.Errorf("waitUntilMount() got = %v, wantAnnotation = %v", newPod.Annotations, tt.wantAnno)
			}
		})
	}
}

func TestWaitUntilMountWithMock(t *testing.T) {
	Convey("Test WaitUntilMount mock", t, func() {
		Convey("waitUntilMount pod notfound", func() {
			client := &k8sclient.K8sClient{}
			patch1 := ApplyFunc(k8serrors.IsNotFound, func(err error) bool {
				return true
			})
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(client), "GetPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string) (*corev1.Pod, error) {
				return nil, errors.New("test")
			})
			defer patch2.Reset()
			patch3 := ApplyMethod(reflect.TypeOf(client), "CreatePod", func(_ *k8sclient.K8sClient, _ context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
				return nil, errors.New("test")
			})
			defer patch3.Reset()
			patch4 := ApplyFunc(k8serrors.IsAlreadyExists, func(err error) bool {
				return true
			})
			defer patch4.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := &PodMount{
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          &k8sclient.K8sClient{Interface: fakeClient},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			podName, err := p.genMountPodName(context.TODO(), &jfsConfig.JfsSetting{Storage: "ttt"})
			So(err, ShouldBeNil)
			err = p.createOrAddRef(ctx, podName, &jfsConfig.JfsSetting{Storage: "ttt"}, nil)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestJMount(t *testing.T) {
	Convey("Test JMount mock", t, func() {
		Convey("JMount", func() {
			client := &k8sclient.K8sClient{}
			patch1 := ApplyFunc(k8serrors.IsNotFound, func(err error) bool {
				return false
			})
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(client), "GetPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string) (*corev1.Pod, error) {
				return nil, errors.New("test")
			})
			defer patch2.Reset()
			patch5 := ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
				return mocks.FakeFileInfoIno1{}, nil
			})
			defer patch5.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := NewPodMount(&k8sclient.K8sClient{Interface: fakeClient}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := p.JMount(ctx, nil, &jfsConfig.JfsSetting{Storage: "ttt"})
			So(err, ShouldNotBeNil)
		})
	})
}

func TestNewPodMount(t *testing.T) {
	type args struct {
		setting *jfsConfig.JfsSetting
		client  *k8sclient.K8sClient
	}
	tests := []struct {
		name string
		args args
		want MntInterface
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
				K8sClient: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewPodMount(tt.args.client, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			}); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPodMount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRef(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "test-true",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Annotations: map[string]string{"a": "b", util.GetReferenceKey("aa"): "aa"},
					},
				},
			},
			want: 1,
		},
		{
			name: "test-false",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Annotations: map[string]string{"a": "b"},
					},
				},
			},
			want: 0,
		},
		{
			name: "test-null",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Annotations: nil,
					},
				},
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRef(tt.args.pod); got != tt.want {
				t.Errorf("HasRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenHashOfSetting(t *testing.T) {
	type args struct {
		setting jfsConfig.JfsSetting
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				setting: jfsConfig.JfsSetting{
					Name: "test",
				},
			},
			want:    "60fff489f3675b6761479ecbf61850b2489c73f9a540428e7ba16c19422fc07",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenHashOfSetting(tt.args.setting)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenHashOfSetting() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GenHashOfSetting() got = %v, want %v", got, tt.want)
			}
		})
	}
}
