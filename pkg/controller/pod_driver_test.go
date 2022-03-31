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
	"errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"os"
	"os/exec"
	"reflect"
	"syscall"
	"testing"
	"time"

	k8sexec "k8s.io/utils/exec"

	. "github.com/agiledragon/gomonkey"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog"
	"k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func init() {
	klog.InitFlags(nil)
}

var (
	volErr       = errors.New("not connected")
	notExistsErr = os.ErrNotExist
	mountErr     = errors.New("target busy")
	podRequest   = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU:    resource.MustParse("3"),
		corev1.ResourceMemory: resource.MustParse("4G"),
	}
	testResources = corev1.ResourceRequirements{
		Requests: podRequest,
	}
)

var target = "/poddir/uid-1/volumes/kubernetes.io~csi/pvn/mount"

var readyPod = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-ready",
		Annotations: map[string]string{
			util.GetReferenceKey(target): target},
		Finalizers: []string{jfsConfig.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:    "test",
			Image:   "juicedata/juicefs-csi-driver",
			Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
		}},
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
			{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
			{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
		},
		ContainerStatuses: []corev1.ContainerStatus{{
			Name: "test",
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{
				StartedAt: metav1.Time{},
			}},
			Ready: true,
		}},
	},
}

var errCmdPod = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-err-mount-cmd-pod",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
		Finalizers: []string{jfsConfig.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:    "test",
			Image:   "juicedata/juicefs-csi-driver",
			Command: []string{"sh", "-c"},
		}},
	},
}

var deletedPod = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-deleted",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
		DeletionTimestamp: &metav1.Time{Time: time.Now()},
		Finalizers:        []string{jfsConfig.Finalizer},
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
			{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
			{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
		},
		ContainerStatuses: []corev1.ContainerStatus{{
			Name: "test",
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{
				StartedAt: metav1.Time{},
			}},
			Ready: true,
		}},
	},
}

// CrashLoopBackOff pod
var errorPod1 = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-error1",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
		Finalizers: []string{jfsConfig.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:      "pvc-node01-xxx",
				Image:     "juicedata/juicefs-csi-driver:v0.10.6",
				Command:   []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
				Resources: testResources,
			},
		},
	},
	Status: corev1.PodStatus{
		Reason: "OutOfCPU",
		Phase:  corev1.PodRunning,
		Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			{Type: corev1.ContainersReady, Status: corev1.ConditionFalse},
			{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
			{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
		},
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "test",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason:  "CrashLoopBackOff",
					Message: "CrashLoopBackOff",
				}},
				Ready: false,
			},
		},
	},
}

// resourceErr pod
var resourceErrPod = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-resourceErr",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
		Finalizers: []string{jfsConfig.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:      "pvc-node01-xxx",
				Image:     "juicedata/juicefs-csi-driver:v0.10.6",
				Command:   []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
				Resources: testResources,
			},
		},
	},
	Status: corev1.PodStatus{
		Reason: "OutOfCPU",
		Phase:  corev1.PodFailed,
		Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			{Type: corev1.ContainersReady, Status: corev1.ConditionFalse},
			{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
			{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
		},
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "test",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason:  "CrashLoopBackOff",
					Message: "CrashLoopBackOff",
				}},
				Ready: false,
			},
		},
	},
}

// terminated error pod
var errorPod2 = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-error2",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
		Finalizers: []string{jfsConfig.Finalizer},
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodFailed,
		Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			{Type: corev1.ContainersReady, Status: corev1.ConditionFalse},
			{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
			{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
		},
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "test",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 2,
					Reason:   "OOMKilled",
				}},
				Ready: false,
			},
		},
	},
}

// unknown error pod
var errorPod3 = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-error4",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
		Finalizers: []string{jfsConfig.Finalizer},
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodUnknown,
		Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			{Type: corev1.ContainersReady, Status: corev1.ConditionFalse},
			{Type: corev1.PodScheduled, Status: corev1.ConditionFalse},
			{Type: corev1.PodInitialized, Status: corev1.ConditionFalse},
		},
		ContainerStatuses: []corev1.ContainerStatus{},
	},
}

// unscheduled pending pod
var pendingPod = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-error3",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
		Finalizers: []string{jfsConfig.Finalizer},
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodPending,
		Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			{Type: corev1.ContainersReady, Status: corev1.ConditionFalse},
			{Type: corev1.PodScheduled, Status: corev1.ConditionFalse},
			{Type: corev1.PodInitialized, Status: corev1.ConditionFalse},
		},
		ContainerStatuses: []corev1.ContainerStatus{},
	},
}

// running pod
var runningPod = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-pod-running",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
		Finalizers: []string{jfsConfig.Finalizer},
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodPending,
		Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			{Type: corev1.ContainersReady, Status: corev1.ConditionFalse},
		},
		ContainerStatuses: []corev1.ContainerStatus{},
	},
}

func TestPodDriver_getPodStatus(t *testing.T) {
	type fields struct {
		Client *k8sclient.K8sClient
	}
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   podStatus
	}{
		{
			name: "error-nil pod",
			fields: fields{
				Client: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
			},
			args: args{
				pod: nil,
			},
			want: podError,
		},
		{
			name: "ready",
			fields: fields{
				Client: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
			},
			args: args{
				pod: readyPod,
			},
			want: podReady,
		},
		{
			name: "error1",
			fields: fields{
				Client: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
			},
			args: args{
				pod: errorPod1,
			},
			want: podError,
		},
		{
			name: "error2",
			fields: fields{
				Client: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
			},
			args: args{
				pod: errorPod2,
			},
			want: podError,
		},
		{
			name: "error3",
			fields: fields{
				Client: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
			},
			args: args{
				pod: errorPod3,
			},
			want: podError,
		},
		{
			name: "pending",
			fields: fields{
				Client: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
			},
			args: args{
				pod: pendingPod,
			},
			want: podPending,
		},
		{
			name: "delete",
			fields: fields{
				Client: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
			},
			args: args{
				pod: deletedPod,
			},
			want: podDeleted,
		}, {
			name: "running",
			fields: fields{
				Client: &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()},
			},
			args: args{
				pod: runningPod,
			},
			want: podPending,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodDriver{
				Client: tt.fields.Client,
			}
			if got := p.getPodStatus(tt.args.pod); got != tt.want {
				t.Errorf("getPodStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func copyPod(oldPod *corev1.Pod) *corev1.Pod {
	var newPod = corev1.Pod{}
	newPod.ObjectMeta = oldPod.ObjectMeta
	newPod.Spec = oldPod.Spec
	newPod.Spec.Containers = make([]corev1.Container, 0)
	if oldPod.Spec.Containers != nil && len(oldPod.Spec.Containers) != 0 {
		for _, v := range oldPod.Spec.Containers {
			newPod.Spec.Containers = append(newPod.Spec.Containers, v)
		}
	}
	newPod.Status = oldPod.Status
	return &newPod
}

func genMountInfos() []mount.MountInfo {
	var mis []mount.MountInfo
	var mi mount.MountInfo
	mi.Root = "/"
	mi.MountPoint = target
	mis = append(mis, mi)
	return mis
}

func TestPodDriver_podReadyHandler(t *testing.T) {
	Convey("Test pod ready handler", t, FailureContinues, func() {
		Convey("pod ready add need recovery ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, os.NewSyscallError("", syscall.ENOTCONN)}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(d.Interface), "Mount",
				func(_ *mount.Mounter, _, _, _ string, _ []string) error {
					return nil
				})
			defer patch2.Reset()
			patch3 := ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
				return genMountInfos(), nil
			})
			defer patch3.Reset()
			err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("pod ready add don't need recovery ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, nil}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("mountinfo parse err", func() {
			patch1 := ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
				return genMountInfos(), errors.New("mountinfo parse fail")
			})
			defer patch1.Reset()
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, nil}},
			}
			patch2 := ApplyFuncSeq(os.Stat, outputs)
			defer patch2.Reset()
			err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("pod ready add target mntPath not exists ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, notExistsErr}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("pod ready and mount err ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, os.NewSyscallError("", syscall.ENOTCONN)}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(d.Interface), "Mount",
				func(_ *mount.Mounter, source string, target string, fstype string, options []string) error {
					return mountErr
				})
			defer patch2.Reset()
			patch3 := ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
				return genMountInfos(), nil
			})
			defer patch3.Reset()
			err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("get nil pod", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			err := d.podReadyHandler(context.Background(), nil)
			So(err, ShouldBeNil)
		})
		Convey("pod Annotations is nil", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			err := d.podReadyHandler(context.Background(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "juicefs-test-err-pod",
					Annotations: nil,
					Finalizers:  []string{jfsConfig.Finalizer},
				},
			})
			So(err, ShouldBeNil)
		})
		Convey("pod mount cmd <3", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			err := d.podReadyHandler(context.Background(), errCmdPod)
			So(err, ShouldBeNil)
		})
		Convey("parse pod mount cmd mntPath err", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "juicefs-test-err-mount-cmd-pod",
					Annotations: map[string]string{
						util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
					Finalizers: []string{jfsConfig.Finalizer},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "test",
						Image:   "juicedata/juicefs-csi-driver",
						Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379/jfs/pvc-xxx"},
					}},
				},
			}
			err := d.podReadyHandler(context.Background(), pod)
			So(err, ShouldBeNil)
		})
		Convey("pod sourcePath err ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			outputs := []OutputCell{
				{Values: Params{nil, volErr}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("pod sourcePath subpath err ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, nil}},
				{Values: Params{nil, nil}},
				{Values: Params{nil, os.NewSyscallError("", syscall.ENOTCONN)}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			patch2 := ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
				mis := genMountInfos()
				mis[0].Root = "/dir"
				return mis, nil
			})
			defer patch2.Reset()
			err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("pod target status unexpected ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, volErr}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			patch2 := ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
				return genMountInfos(), nil
			})
			defer patch2.Reset()
			err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("pod target format invalid ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "juicefs-test-err-mount-cmd-pod",
					Annotations: map[string]string{
						util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
					Finalizers: []string{jfsConfig.Finalizer},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "test",
						Image:   "juicedata/juicefs-csi-driver",
						Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
					}},
				},
			}
			err := d.podReadyHandler(context.Background(), pod)
			So(err, ShouldBeNil)
		})
	})
}

func TestPodDriver_podDeletedHandler(t *testing.T) {
	Convey("Test pod delete handler", t, func() {
		Convey("umount fail", func() {
			var tmpCmd = &exec.Cmd{}
			patch1 := ApplyFunc(util.GetMountPathOfPod, func(pod corev1.Pod) (string, string, error) {
				return "/test", "test", nil
			})
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), errors.New("test")
			})
			defer patch2.Reset()
			k := &k8sclient.K8sClient{}
			patch3 := ApplyMethod(reflect.TypeOf(k), "GetPod", func(_ *k8sclient.K8sClient, podName, namespace string) (*corev1.Pod, error) {
				return nil, errors.New("test")
			})
			defer patch3.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(k), "PatchPod", func(_ *k8sclient.K8sClient, pod *corev1.Pod, data []byte) error {
				return nil
			})
			defer patch4.Reset()
			patch5 := ApplyMethod(reflect.TypeOf(k), "CreatePod", func(_ *k8sclient.K8sClient, pod *corev1.Pod) (*corev1.Pod, error) {
				return nil, nil
			})
			defer patch5.Reset()
			patch6 := ApplyFunc(apierrors.IsNotFound, func(err error) bool {
				return true
			})
			defer patch6.Reset()
			patch7 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, nil
			})
			defer patch7.Reset()

			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			tmpPod := copyPod(deletedPod)
			err := d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("new pod create", func() {
			var tmpCmd = &exec.Cmd{}
			patch1 := ApplyFunc(util.GetMountPathOfPod, func(pod corev1.Pod) (string, string, error) {
				return "/test", "test", nil
			})
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
				return []byte(""), nil
			})
			defer patch2.Reset()
			k := &k8sclient.K8sClient{}
			patch3 := ApplyMethod(reflect.TypeOf(k), "GetPod", func(_ *k8sclient.K8sClient, podName, namespace string) (*corev1.Pod, error) {
				return nil, errors.New("test")
			})
			defer patch3.Reset()
			patch4 := ApplyMethod(reflect.TypeOf(k), "PatchPod", func(_ *k8sclient.K8sClient, pod *corev1.Pod, data []byte) error {
				return nil
			})
			defer patch4.Reset()
			patch5 := ApplyMethod(reflect.TypeOf(k), "CreatePod", func(_ *k8sclient.K8sClient, pod *corev1.Pod) (*corev1.Pod, error) {
				return nil, nil
			})
			defer patch5.Reset()
			patch6 := ApplyFunc(apierrors.IsNotFound, func(err error) bool {
				return true
			})
			defer patch6.Reset()
			patch7 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, nil
			})
			defer patch7.Reset()

			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			tmpPod := copyPod(deletedPod)
			err := d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("pod delete success ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			var tmpCmd = &exec.Cmd{}
			patch1 := ApplyFunc(exec.Command, func(name string, args ...string) *exec.Cmd {
				return tmpCmd
			})
			defer patch1.Reset()
			tmpPod := copyPod(readyPod)
			_, err := d.Client.CreatePod(tmpPod)
			if err != nil {
				t.Fatal(err)
			}
			patch2 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput",
				func(_ *exec.Cmd) ([]byte, error) {
					err := d.Client.DeletePod(tmpPod)
					if err != nil {
						t.Fatal(err)
					}
					return []byte(""), nil
				})
			defer patch2.Reset()
			err = d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("get nil pod", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			err := d.podDeletedHandler(context.Background(), nil)
			So(err, ShouldBeNil)
		})
		Convey("pod no finalizer", func() {
			tmpPod := copyPod(readyPod)
			tmpPod.Finalizers = nil
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			err := d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("skip delete resource err pod", func() {
			tmpPod := copyPod(resourceErrPod)
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			err := d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("remove pod finalizer err ", func() {
			tmpPod := copyPod(readyPod)
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			err := d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeError)
		})
		Convey("pod no Annotations", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			tmpPod := copyPod(resourceErrPod)
			tmpPod.Annotations = nil
			_, err := d.Client.CreatePod(tmpPod)
			if err != nil {
				t.Fatal(err)
			}
			err = d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("can not get mntTarget from pod Annotations", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			tmpPod := copyPod(resourceErrPod)
			tmpPod.Annotations = map[string]string{
				"/var/lib/xxx": "/var/lib/xxx",
			}
			_, err := d.Client.CreatePod(tmpPod)
			if err != nil {
				t.Fatal(err)
			}
			err = d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("get sourcePath from pod cmd failed", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			tmpPod := copyPod(readyPod)
			tmpPod.Spec.Containers = nil
			_, err := d.Client.CreatePod(tmpPod)
			if err != nil {
				t.Fatal(err)
			}
			err = d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("umount source err and need mount lazy ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			var tmpCmd = &exec.Cmd{}
			patch1 := ApplyFunc(exec.Command, func(name string, args ...string) *exec.Cmd {
				return tmpCmd
			})
			defer patch1.Reset()
			tmpPod := copyPod(readyPod)
			_, err := d.Client.CreatePod(tmpPod)
			if err != nil {
				t.Fatal(err)
			}
			patch2 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput",
				func(_ *exec.Cmd) ([]byte, error) {
					return []byte(""), mountErr
				})
			defer patch2.Reset()
			err = d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
	})
}

func TestPodDriver_podErrorHandler(t *testing.T) {
	Convey("Test pod err handler", t, func() {
		Convey("get sourcePath from pod cmd failed", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			Pod := copyPod(readyPod)
			Pod.Spec.Containers = nil
			err := d.podErrorHandler(context.Background(), Pod)
			So(err, ShouldBeNil)
		})
		Convey("pod ResourceError but pod no resource", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			errPod := copyPod(resourceErrPod)
			errPod.Spec.Containers[0].Resources = corev1.ResourceRequirements{}
			err := d.podErrorHandler(context.Background(), errPod)
			So(err, ShouldBeNil)
		})
		Convey("GetPod error", func() {
			k := &k8sclient.K8sClient{}
			patch1 := ApplyMethod(reflect.TypeOf(k), "GetPod", func(_ *k8sclient.K8sClient, podName, namespace string) (*corev1.Pod, error) {
				return &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{jfsConfig.Finalizer},
					},
				}, nil
			})
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(k), "PatchPod", func(_ *k8sclient.K8sClient, pod *corev1.Pod, data []byte) error {
				return errors.New("test")
			})
			defer patch2.Reset()
			patch3 := ApplyMethod(reflect.TypeOf(k), "DeletePod", func(_ *k8sclient.K8sClient, pod *corev1.Pod) error {
				return nil
			})
			defer patch3.Reset()
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			Pod := copyPod(resourceErrPod)
			err := d.podErrorHandler(context.Background(), Pod)
			So(err, ShouldBeNil)
		})
		Convey("pod err add need delete ", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, notExistsErr
			})
			defer patch1.Reset()
			_, err := d.Client.CreatePod(errorPod1)
			defer d.Client.DeletePod(errorPod1)
			if err != nil {
				t.Fatal(err)
			}
			err = d.podErrorHandler(context.Background(), errorPod1)
			So(err, ShouldBeNil)
		})
		Convey("get nil pod", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			err := d.podErrorHandler(context.Background(), nil)
			So(err, ShouldBeNil)
		})
		Convey("pod ResourceError", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, notExistsErr
			})
			defer patch1.Reset()
			errPod := copyPod(resourceErrPod)
			d.Client.CreatePod(errPod)
			err := d.podErrorHandler(context.Background(), errPod)
			So(err, ShouldBeNil)
		})
		Convey("pod ResourceError and remove pod Finalizer err", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			errPod := copyPod(resourceErrPod)
			err := d.podErrorHandler(context.Background(), errPod)
			So(err, ShouldBeNil)
		})
		Convey("sourcePath not mount", func() {
			d := NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      k8sexec.New(),
			})
			Pod := copyPod(readyPod)
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, nil
			})
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(d.Interface), "IsLikelyNotMountPoint",
				func(_ *mount.Mounter, file string) (bool, error) {
					return true, nil
				},
			)
			defer patch2.Reset()
			err := d.podErrorHandler(context.Background(), Pod)
			So(err, ShouldBeNil)
		})
	})
}
