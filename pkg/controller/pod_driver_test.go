package controller

import (
	"context"
	"errors"
	. "github.com/agiledragon/gomonkey"
	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/mount"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
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

var readyPod = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-ready",
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
		Client k8sclient.K8sClient
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
				Client: k8sclient.FakeClient,
			},
			args: args{
				pod: nil,
			},
			want: podError,
		},
		{
			name: "ready",
			fields: fields{
				Client: k8sclient.FakeClient,
			},
			args: args{
				pod: readyPod,
			},
			want: podReady,
		},
		{
			name: "error1",
			fields: fields{
				Client: k8sclient.FakeClient,
			},
			args: args{
				pod: errorPod1,
			},
			want: podError,
		},
		{
			name: "error2",
			fields: fields{
				Client: k8sclient.FakeClient,
			},
			args: args{
				pod: errorPod2,
			},
			want: podError,
		},
		{
			name: "error3",
			fields: fields{
				Client: k8sclient.FakeClient,
			},
			args: args{
				pod: errorPod3,
			},
			want: podError,
		},
		{
			name: "pending",
			fields: fields{
				Client: k8sclient.FakeClient,
			},
			args: args{
				pod: pendingPod,
			},
			want: podPending,
		},
		{
			name: "delete",
			fields: fields{
				Client: k8sclient.FakeClient,
			},
			args: args{
				pod: deletedPod,
			},
			want: podDeleted,
		}, {
			name: "running",
			fields: fields{
				Client: k8sclient.FakeClient,
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

func TestPodDriver_podReadyHandler(t *testing.T) {
	Convey("Test pod ready handler", t, FailureContinues, func() {
		Convey("pod ready add need recovery ", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, volErr}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(d.Interface), "Mount",
				func(_ *mount.Mounter, _, _, _ string, _ []string) error {
					return nil
				})
			defer patch2.Reset()
			_, err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("pod ready add don't need recovery ", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, nil}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			_, err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("pod ready add target mntPath not exists ", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, notExistsErr}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			_, err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("pod ready and mount err ", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			outputs := []OutputCell{
				{Values: Params{nil, nil}},
				{Values: Params{nil, volErr}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(d.Interface), "Mount",
				func(_ *mount.Mounter, source string, target string, fstype string, options []string) error {
					return mountErr
				})
			defer patch2.Reset()
			_, err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
		Convey("get nil pod", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			_, err := d.podReadyHandler(context.Background(), nil)
			So(err, ShouldBeNil)
		})
		Convey("pod Annotations is nil", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			_, err := d.podReadyHandler(context.Background(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "juicefs-test-err-pod",
					Annotations: nil,
					Finalizers:  []string{jfsConfig.Finalizer},
				},
			})
			So(err, ShouldBeNil)
		})
		Convey("pod mount cmd <3", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			_, err := d.podReadyHandler(context.Background(), errCmdPod)
			So(err, ShouldBeNil)
		})
		Convey("parse pod mount cmd mntPath err", func() {
			d := NewPodDriver(k8sclient.FakeClient)
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
			_, err := d.podReadyHandler(context.Background(), pod)
			So(err, ShouldBeNil)
		})
		Convey("stat static-pv sourcePath err", func() {
			d := NewPodDriver(k8sclient.FakeClient)
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
						Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/static-pv-xxx"},
					}},
				},
			}
			outputs := []OutputCell{
				{Values: Params{nil, notExistsErr}},
				{Values: Params{nil, volErr}},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			_, err := d.podReadyHandler(context.Background(), pod)
			So(err, ShouldBeNil)
		})
		Convey("stat static-pv sourcePath normal", func() {
			d := NewPodDriver(k8sclient.FakeClient)
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
			outputs := []OutputCell{
				{Values: Params{nil, volErr}},
				{Values: Params{nil, nil}, Times: 2},
			}
			patch1 := ApplyFuncSeq(os.Stat, outputs)
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(d.Interface), "Mount",
				func(_ *mount.Mounter, source string, target string, fstype string, options []string) error {
					return nil
				})
			defer patch2.Reset()
			_, err := d.podReadyHandler(context.Background(), pod)
			So(err, ShouldBeNil)
		})
	})
}

func TestPodDriver_podDeletedHandler(t *testing.T) {
	Convey("Test pod delete handler", t, func() {
		Convey("pod delete success ", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			var tmpCmd = &exec.Cmd{}
			patch1 := ApplyFunc(exec.Command, func(name string, args ...string) *exec.Cmd {
				return tmpCmd
			})
			defer patch1.Reset()
			k8sclient.FakeClient.Flush()
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
			_, err = d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("get nil pod", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			_, err := d.podDeletedHandler(context.Background(), nil)
			So(err, ShouldBeNil)
		})
		Convey("pod no finalizer", func() {
			tmpPod := copyPod(readyPod)
			tmpPod.Finalizers = nil
			d := NewPodDriver(k8sclient.FakeClient)
			_, err := d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("skip delete resource err pod", func() {
			tmpPod := copyPod(resourceErrPod)
			d := NewPodDriver(k8sclient.FakeClient)
			_, err := d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("remove pod finalizer err ", func() {
			k8sclient.FakeClient.Flush()
			tmpPod := copyPod(readyPod)
			d := NewPodDriver(k8sclient.FakeClient)
			_, err := d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeError)
		})
		Convey("pod no Annotations", func() {
			tmpPod := copyPod(resourceErrPod)
			tmpPod.Annotations = nil
			_, err := k8sclient.FakeClient.CreatePod(tmpPod)
			if err != nil {
				t.Fatal(err)
			}
			defer k8sclient.FakeClient.Flush()
			d := NewPodDriver(k8sclient.FakeClient)
			_, err = d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("can not get mntTarget from pod Annotations", func() {
			tmpPod := copyPod(resourceErrPod)
			tmpPod.Annotations = map[string]string{
				"/var/lib/xxx": "/var/lib/xxx",
			}
			_, err := k8sclient.FakeClient.CreatePod(tmpPod)
			if err != nil {
				t.Fatal(err)
			}
			defer k8sclient.FakeClient.Flush()
			d := NewPodDriver(k8sclient.FakeClient)
			_, err = d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("get sourcePath from pod cmd failed", func() {
			k8sclient.FakeClient.Flush()
			tmpPod := copyPod(readyPod)
			tmpPod.Spec.Containers = nil
			_, err := k8sclient.FakeClient.CreatePod(tmpPod)
			if err != nil {
				t.Fatal(err)
			}
			defer k8sclient.FakeClient.Flush()
			d := NewPodDriver(k8sclient.FakeClient)
			_, err = d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
		Convey("umount source err and need mount lazy ", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			var tmpCmd = &exec.Cmd{}
			patch1 := ApplyFunc(exec.Command, func(name string, args ...string) *exec.Cmd {
				return tmpCmd
			})
			defer patch1.Reset()
			k8sclient.FakeClient.Flush()
			tmpPod := copyPod(readyPod)
			_, err := d.Client.CreatePod(tmpPod)
			if err != nil {
				t.Fatal(err)
			}
			patch2 := ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput",
				func(_ *exec.Cmd) ([]byte, error) {
					k8sclient.FakeClient.Flush()
					return []byte(""), mountErr
				})
			defer patch2.Reset()
			_, err = d.podDeletedHandler(context.Background(), tmpPod)
			So(err, ShouldBeNil)
		})
	})
}

func TestPodDriver_podErrorHandler(t *testing.T) {
	Convey("Test pod err handler", t, func() {
		Convey("pod err add need delete ", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, notExistsErr
			})
			defer patch1.Reset()
			_, err := d.Client.CreatePod(errorPod1)
			defer d.Client.DeletePod(errorPod1)
			if err != nil {
				t.Fatal(err)
			}
			_, err = d.podErrorHandler(context.Background(), errorPod1)
			So(err, ShouldBeNil)
		})
		Convey("get nil pod", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			_, err := d.podErrorHandler(context.Background(), nil)
			So(err, ShouldBeNil)
		})
		Convey("pod ResourceError", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return false, notExistsErr
			})
			defer patch1.Reset()
			errPod := copyPod(resourceErrPod)
			k8sclient.FakeClient.PodMap[errPod.Name] = errPod
			_, err := d.podErrorHandler(context.Background(), errPod)
			So(err, ShouldBeNil)
		})
		Convey("pod ResourceError and remove pod Finalizer err", func() {
			k8sclient.FakeClient.Flush()
			d := NewPodDriver(k8sclient.FakeClient)
			errPod := copyPod(resourceErrPod)
			_, err := d.podErrorHandler(context.Background(), errPod)
			So(err, ShouldBeNil)
		})
		Convey("pod ResourceError but pod no resource", func() {
			k8sclient.FakeClient.Flush()
			d := NewPodDriver(k8sclient.FakeClient)
			errPod := copyPod(resourceErrPod)
			errPod.Spec.Containers[0].Resources = corev1.ResourceRequirements{}
			_, err := d.podErrorHandler(context.Background(), errPod)
			So(err, ShouldBeNil)
		})
		Convey("get sourcePath from pod cmd failed", func() {
			k8sclient.FakeClient.Flush()
			d := NewPodDriver(k8sclient.FakeClient)
			Pod := copyPod(readyPod)
			Pod.Spec.Containers = nil
			_, err := d.podErrorHandler(context.Background(), Pod)
			So(err, ShouldBeError)
		})
		Convey("sourcePath not mount", func() {
			k8sclient.FakeClient.Flush()
			d := NewPodDriver(k8sclient.FakeClient)
			Pod := copyPod(readyPod)
			patch1 := ApplyFunc(mount.PathExists, func(path string) (bool, error) {
				return true, nil
			})
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(d.Interface), "IsLikelyNotMountPoint",
				func(_ *mount.Mounter, file string) (bool, error) {
					k8sclient.FakeClient.Flush()
					return true, nil
				},
			)
			defer patch2.Reset()
			_, err := d.podErrorHandler(context.Background(), Pod)
			So(err, ShouldBeNil)
		})
	})
}
