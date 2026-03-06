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
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver/mocks"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var testA = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-a",
		Labels: map[string]string{
			common.PodUniqueIdLabelKey: "a",
		},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
	},
}

var testB = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-b",
		Labels: map[string]string{
			common.PodUniqueIdLabelKey: "b",
		},
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
	},
}

var testC = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-c",
		Labels: map[string]string{
			common.PodUniqueIdLabelKey: "c",
		},
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
	},
}

var testD = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-d",
		Labels: map[string]string{
			common.PodUniqueIdLabelKey: "d",
		},
		Annotations: map[string]string{"a": "b",
			util.GetReferenceKey("/mnt/def"): "/mnt/def"},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
	},
}

var testE = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-e",
		Labels: map[string]string{
			common.PodUniqueIdLabelKey: "e",
		},
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc",
			util.GetReferenceKey("/mnt/def"): "/mnt/def",
		},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
	},
}

var testF = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-f",
		Labels: map[string]string{
			common.PodUniqueIdLabelKey: "f",
		},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
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
			common.PodUniqueIdLabelKey: "g",
		},
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc",
			util.GetReferenceKey("/mnt/def"): "/mnt/def",
		},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{
			Image:   "juicedata/mount:ce-v1.2.1",
			Command: []string{"sh", "-c", "exec mount.juicefs juicefs-test-node-j /jfs/juicefs-test-node-j"},
		}},
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
			common.PodUniqueIdLabelKey: "h",
		},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{
			Image:   "juicedata/mount:ce-v1.2.1",
			Command: []string{"sh", "-c", "exec mount.juicefs juicefs-test-node-h /jfs/juicefs-test-node-h"},
		}},
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
				log:                klog.NewKlogr(),
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
				log:       klog.NewKlogr(),
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
	defer func() { _ = os.RemoveAll("tmp") }()
	fakeClientSet := fake.NewSimpleClientset()
	passfd.InitTestFds()

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
				log:                klog.NewKlogr(),
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
				log:                klog.NewKlogr(),
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			podName := GenPodNameByUniqueId("ttt", true)
			_, _ = p.K8sClient.CreatePod(context.TODO(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: jfsConfig.Namespace,
					Annotations: map[string]string{
						podName: "/test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
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
				log:                klog.NewKlogr(),
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			podName := GenPodNameByUniqueId("ttt", true)
			_, _ = p.K8sClient.CreatePod(context.TODO(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: jfsConfig.Namespace,
					Annotations: map[string]string{
						util.GetReferenceKey("ttt"): "/test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
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
				log:                klog.NewKlogr(),
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient: &k8sclient.K8sClient{
					Interface: fakeClient,
				},
			}
			podName := GenPodNameByUniqueId("ttt", true)
			_, _ = p.K8sClient.CreatePod(context.TODO(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: jfsConfig.Namespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
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
	Convey("Test UmountTarget", t, func() {
		Convey("umount error", func() {
			tmpCmd := &exec.Cmd{}
			patch1 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte("umount error"), errors.New("umount error")
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(mount.CleanupMountPoint, func(mountPath string, mounter mount.Interface, extensiveMountPointCheck bool) error {
				return nil
			})
			defer patch2.Reset()

			p := NewPodMount(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := p.UmountTarget(ctx, "/test", "ttt")
			So(err, ShouldNotBeNil)
		})
		Convey("success case", func() {
			tmpCmd := &exec.Cmd{}
			patch1 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte("not mounted"), errors.New("not mounted")
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(mount.CleanupMountPoint, func(mountPath string, mounter mount.Interface, extensiveMountPointCheck bool) error {
				return nil
			})
			defer patch2.Reset()

			p := NewPodMount(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := p.UmountTarget(ctx, "/test", "ttt")
			So(err, ShouldBeNil)
		})
		Convey("cleanup error", func() {
			tmpCmd := &exec.Cmd{}
			patch1 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte("not mounted"), errors.New("not mounted")
			})
			defer patch1.Reset()
			patch2 := ApplyFunc(mount.CleanupMountPoint, func(mountPath string, mounter mount.Interface, extensiveMountPointCheck bool) error {
				return errors.New("cleanup error")
			})
			defer patch2.Reset()

			p := NewPodMount(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := p.UmountTarget(ctx, "/test", "ttt")
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
					UniqueId:   "h",
					TargetPath: "/mnt/hhh",
					MountPath:  "/mnt/hhh",
					Attr:       &jfsConfig.PodAttr{},
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
					UniqueId:   "g",
					TargetPath: "/mnt/ggg",
					MountPath:  "/mnt/ggg",
					Attr:       &jfsConfig.PodAttr{},
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
					Attr:       &jfsConfig.PodAttr{},
				},
			},
			pod:     nil,
			wantErr: false,
			wantAnno: map[string]string{
				util.GetReferenceKey("/mnt/iii"): "/mnt/iii",
				common.UniqueId:                  "",
				common.JuiceFSUUID:               "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch := ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
				return nil
			})
			defer patch.Reset()
			p := &PodMount{
				log:                klog.NewKlogr(),
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          &k8sclient.K8sClient{Interface: fakeClientSet},
			}
			if tt.pod != nil {
				hashVal := jfsConfig.GenHashOfSetting(klog.NewKlogr(), *tt.args.jfsSetting)
				tt.args.jfsSetting.HashVal = hashVal
				tt.pod.Labels[common.PodTypeKey] = common.PodTypeValue
				tt.pod.Labels[common.PodUniqueIdLabelKey] = tt.args.jfsSetting.VolumeId
				tt.pod.Labels[common.PodJuiceHashLabelKey] = hashVal
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
			patch := ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
				return nil
			})
			defer patch.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := &PodMount{
				log:                klog.NewKlogr(),
				SafeFormatAndMount: mount.SafeFormatAndMount{},
				K8sClient:          &k8sclient.K8sClient{Interface: fakeClient},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			podName, err := p.genMountPodName(context.TODO(), &jfsConfig.JfsSetting{Storage: "ttt"})
			So(err, ShouldBeNil)
			err = p.createOrAddRef(ctx, podName, &jfsConfig.JfsSetting{Storage: "ttt", Attr: &jfsConfig.PodAttr{}}, nil)
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
			patch := ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
				return nil
			})
			defer patch.Reset()

			fakeClient := fake.NewSimpleClientset()
			p := NewPodMount(&k8sclient.K8sClient{Interface: fakeClient}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := p.JMount(ctx, nil, &jfsConfig.JfsSetting{Storage: "ttt", Attr: &jfsConfig.PodAttr{}})
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
				log: klog.NewKlogr().WithName("pod-mount"),
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

func persistentPVCName(uniqueId, topologyValue string, i int) string {
	hash := sha256.Sum256([]byte(uniqueId))
	return fmt.Sprintf("jfs-cache-%x-%s-%d", hash[:8], topologyValue, i)
}

func newTestPodMount(clientset *fake.Clientset) *PodMount {
	return &PodMount{
		log:                klog.NewKlogr(),
		SafeFormatAndMount: mount.SafeFormatAndMount{},
		K8sClient:          &k8sclient.K8sClient{Interface: clientset},
	}
}

func newTestNode(name string, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func newTestCachePersistent(topologyKey string) *jfsConfig.CachePersistent {
	storageClassName := "standard"
	return &jfsConfig.CachePersistent{
		StorageClassName: &storageClassName,
		Storage:          resource.MustParse("10Gi"),
		AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		TopologyKey:      topologyKey,
		Path:             "/var/jfsCache-persistent-0",
	}
}

func TestEnsurePersistentCachePVCs(t *testing.T) {
	// init() sets jfsConfig.NodeName = "test-node"
	nodeName := jfsConfig.NodeName
	namespace := jfsConfig.Namespace
	uniqueId := "vol-abc123"

	t.Run("no persistent config - no-op", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(newTestNode(nodeName, nil))
		p := newTestPodMount(clientset)
		setting := &jfsConfig.JfsSetting{UniqueId: uniqueId, CachePersistent: nil}

		err := p.ensurePersistentCachePVCs(context.TODO(), setting)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(setting.CachePVCs) != 0 {
			t.Errorf("expected no CachePVCs, got %v", setting.CachePVCs)
		}
		// Verify no PVCs were created
		list, _ := clientset.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{})
		if len(list.Items) != 0 {
			t.Errorf("expected 0 PVCs, got %d", len(list.Items))
		}
	})

	t.Run("PVC does not exist - creates PVC with correct fields", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(
			newTestNode(nodeName, map[string]string{"topology.kubernetes.io/zone": "us-east-1a"}),
		)
		p := newTestPodMount(clientset)
		cache := newTestCachePersistent("")
		setting := &jfsConfig.JfsSetting{
			UniqueId:        uniqueId,
			CachePersistent: []*jfsConfig.CachePersistent{cache},
		}

		err := p.ensurePersistentCachePVCs(context.TODO(), setting)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedName := persistentPVCName(uniqueId, "us-east-1a", 0)
		pvc, getErr := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), expectedName, metav1.GetOptions{})
		if getErr != nil {
			t.Fatalf("PVC not created: %v", getErr)
		}

		// Check labels
		if pvc.Labels["juicefs.com/cache-for"] != uniqueId {
			t.Errorf("label cache-for mismatch: got %q", pvc.Labels["juicefs.com/cache-for"])
		}
		if pvc.Labels["juicefs.com/cache-topology"] != "us-east-1a" {
			t.Errorf("label cache-topology mismatch: got %q", pvc.Labels["juicefs.com/cache-topology"])
		}
		// Check annotation
		if pvc.Annotations["volume.kubernetes.io/selected-node"] != nodeName {
			t.Errorf("selected-node annotation mismatch: got %q", pvc.Annotations["volume.kubernetes.io/selected-node"])
		}
		// Check storage class
		if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "standard" {
			t.Errorf("storageClassName mismatch: got %v", pvc.Spec.StorageClassName)
		}
		// Check storage request
		if pvc.Spec.Resources.Requests[corev1.ResourceStorage] != resource.MustParse("10Gi") {
			t.Errorf("storage request mismatch")
		}
		// Check access modes
		if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
			t.Errorf("access modes mismatch: got %v", pvc.Spec.AccessModes)
		}

		// PVC appended to CachePVCs
		if len(setting.CachePVCs) != 1 || setting.CachePVCs[0].PVCName != expectedName {
			t.Errorf("CachePVCs not populated correctly: %v", setting.CachePVCs)
		}
	})

	t.Run("PVC exists, unbound - appends to CachePVCs", func(t *testing.T) {
		pvcName := persistentPVCName(uniqueId, "us-east-1a", 0)
		existingPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: namespace,
			},
			Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
		}
		clientset := fake.NewSimpleClientset(
			newTestNode(nodeName, map[string]string{"topology.kubernetes.io/zone": "us-east-1a"}),
			existingPVC,
		)
		p := newTestPodMount(clientset)
		cache := newTestCachePersistent("")
		setting := &jfsConfig.JfsSetting{
			UniqueId:        uniqueId,
			CachePersistent: []*jfsConfig.CachePersistent{cache},
		}

		err := p.ensurePersistentCachePVCs(context.TODO(), setting)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(setting.CachePVCs) != 1 || setting.CachePVCs[0].PVCName != pvcName {
			t.Errorf("CachePVCs not populated: %v", setting.CachePVCs)
		}
	})

	t.Run("PVC exists, bound, no active VolumeAttachment - appends (warm cache reuse)", func(t *testing.T) {
		pvcName := persistentPVCName(uniqueId, "us-east-1a", 0)
		existingPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: namespace,
			},
			Spec:   corev1.PersistentVolumeClaimSpec{VolumeName: "pv-abc"},
			Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		}
		clientset := fake.NewSimpleClientset(
			newTestNode(nodeName, map[string]string{"topology.kubernetes.io/zone": "us-east-1a"}),
			existingPVC,
		)
		p := newTestPodMount(clientset)
		cache := newTestCachePersistent("")
		setting := &jfsConfig.JfsSetting{
			UniqueId:        uniqueId,
			CachePersistent: []*jfsConfig.CachePersistent{cache},
		}

		err := p.ensurePersistentCachePVCs(context.TODO(), setting)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(setting.CachePVCs) != 1 || setting.CachePVCs[0].PVCName != pvcName {
			t.Errorf("CachePVCs not populated: %v", setting.CachePVCs)
		}
	})

	t.Run("PVC exists, bound, VolumeAttachment on different node - skipped (graceful degradation)", func(t *testing.T) {
		pvcName := persistentPVCName(uniqueId, "us-east-1a", 0)
		pvName := "pv-abc"
		existingPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: namespace,
			},
			Spec:   corev1.PersistentVolumeClaimSpec{VolumeName: pvName},
			Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		}
		attached := true
		va := &storagev1.VolumeAttachment{
			ObjectMeta: metav1.ObjectMeta{Name: "va-abc"},
			Spec: storagev1.VolumeAttachmentSpec{
				NodeName: "other-node",
				Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName},
			},
			Status: storagev1.VolumeAttachmentStatus{Attached: attached},
		}
		clientset := fake.NewSimpleClientset(
			newTestNode(nodeName, map[string]string{"topology.kubernetes.io/zone": "us-east-1a"}),
			existingPVC,
			va,
		)
		p := newTestPodMount(clientset)
		cache := newTestCachePersistent("")
		setting := &jfsConfig.JfsSetting{
			UniqueId:        uniqueId,
			CachePersistent: []*jfsConfig.CachePersistent{cache},
		}

		err := p.ensurePersistentCachePVCs(context.TODO(), setting)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(setting.CachePVCs) != 0 {
			t.Errorf("expected no CachePVCs for PVC attached to another node, got %v", setting.CachePVCs)
		}
	})

	t.Run("create race - AlreadyExists handled without error, PVC appended", func(t *testing.T) {
		// Simulate the race: GetPVC returns NotFound, CreatePVC returns AlreadyExists (another
		// node won the race), and the subsequent re-fetch finds the PVC free to use.
		pvcName := persistentPVCName(uniqueId, "us-east-1a", 0)
		// Pre-populate the PVC so the re-fetch after AlreadyExists succeeds.
		existingPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
		}
		clientset := fake.NewSimpleClientset(
			newTestNode(nodeName, map[string]string{"topology.kubernetes.io/zone": "us-east-1a"}),
			existingPVC,
		)
		p := newTestPodMount(clientset)

		client := p.K8sClient
		patch := ApplyMethod(reflect.TypeOf(client), "CreatePersistentVolumeClaim",
			func(_ *k8sclient.K8sClient, _ context.Context, pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
				return nil, k8serrors.NewAlreadyExists(corev1.Resource("persistentvolumeclaims"), pvc.Name)
			})
		defer patch.Reset()

		cache := newTestCachePersistent("")
		setting := &jfsConfig.JfsSetting{
			UniqueId:        uniqueId,
			CachePersistent: []*jfsConfig.CachePersistent{cache},
		}

		err := p.ensurePersistentCachePVCs(context.TODO(), setting)
		if err != nil {
			t.Fatalf("unexpected error on AlreadyExists race: %v", err)
		}
		if len(setting.CachePVCs) != 1 || setting.CachePVCs[0].PVCName != pvcName {
			t.Errorf("CachePVCs should be appended even on race: %v", setting.CachePVCs)
		}
	})

	t.Run("topology resolution - zone label present", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(
			newTestNode(nodeName, map[string]string{"topology.kubernetes.io/zone": "eu-west-1b"}),
		)
		p := newTestPodMount(clientset)
		cache := newTestCachePersistent("")
		setting := &jfsConfig.JfsSetting{
			UniqueId:        uniqueId,
			CachePersistent: []*jfsConfig.CachePersistent{cache},
		}

		if err := p.ensurePersistentCachePVCs(context.TODO(), setting); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedName := persistentPVCName(uniqueId, "eu-west-1b", 0)
		if len(setting.CachePVCs) != 1 || setting.CachePVCs[0].PVCName != expectedName {
			t.Errorf("wrong PVC name: %v", setting.CachePVCs)
		}
	})

	t.Run("topology resolution - zone label absent falls back to node name", func(t *testing.T) {
		// Node has no zone label
		clientset := fake.NewSimpleClientset(
			newTestNode(nodeName, map[string]string{}),
		)
		p := newTestPodMount(clientset)
		cache := newTestCachePersistent("")
		setting := &jfsConfig.JfsSetting{
			UniqueId:        uniqueId,
			CachePersistent: []*jfsConfig.CachePersistent{cache},
		}

		if err := p.ensurePersistentCachePVCs(context.TODO(), setting); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedName := persistentPVCName(uniqueId, nodeName, 0)
		if len(setting.CachePVCs) != 1 || setting.CachePVCs[0].PVCName != expectedName {
			t.Errorf("expected fallback to node name in PVC name, got %v", setting.CachePVCs)
		}
	})

	t.Run("custom topologyKey - uses specified label", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(
			newTestNode(nodeName, map[string]string{
				"kubernetes.io/hostname":      nodeName,
				"topology.kubernetes.io/zone": "us-east-1a",
			}),
		)
		p := newTestPodMount(clientset)
		cache := newTestCachePersistent("kubernetes.io/hostname")
		setting := &jfsConfig.JfsSetting{
			UniqueId:        uniqueId,
			CachePersistent: []*jfsConfig.CachePersistent{cache},
		}

		if err := p.ensurePersistentCachePVCs(context.TODO(), setting); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedName := persistentPVCName(uniqueId, nodeName, 0)
		if len(setting.CachePVCs) != 1 || setting.CachePVCs[0].PVCName != expectedName {
			t.Errorf("expected hostname-based PVC name, got %v", setting.CachePVCs)
		}
	})
}
