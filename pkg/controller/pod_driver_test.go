package controller

import (
	"context"
	. "github.com/agiledragon/gomonkey"
	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	. "github.com/smartystreets/goconvey/convey"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/mount"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
	"time"
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
	Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
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

// unscheduled error pod
var errorPod3 = &corev1.Pod{
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

// unknown error pod
var errorPod4 = &corev1.Pod{
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

func setup() {
	k8sclient.FakeClient.Flush()
	jfsConfig.NodeName = "test-node"
	jfsConfig.Namespace = "kube-system"
	_, _ = k8sclient.FakeClient.CreatePod(readyPod)
	_, _ = k8sclient.FakeClient.CreatePod(deletedPod)
	_, _ = k8sclient.FakeClient.CreatePod(errorPod1)
	_, _ = k8sclient.FakeClient.CreatePod(errorPod2)
	_, _ = k8sclient.FakeClient.CreatePod(errorPod3)
	_, _ = k8sclient.FakeClient.CreatePod(errorPod4)
}

func teardown() {
	k8sclient.FakeClient.Flush()
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
			name: "error4",
			fields: fields{
				Client: k8sclient.FakeClient,
			},
			args: args{
				pod: errorPod4,
			},
			want: podError,
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

func TestPodDriver_podReadyHandler(t *testing.T) {
	Convey("Test pod ready handler", t, func() {
		Convey("pod ready ", func() {
			d := NewPodDriver(k8sclient.FakeClient)
			patch1 := ApplyFunc(os.Stat,
				func(target string) (os.FileInfo, error) {
					return nil, nil
				})
			defer patch1.Reset()
			patch2 := ApplyMethod(reflect.TypeOf(d.Mounter), "Mount",
				func(_ *mount.Mounter, source string, target string, fstype string, options []string) error {
					return nil
				})
			defer patch2.Reset()

			_, err := d.podReadyHandler(context.Background(), readyPod)
			So(err, ShouldBeNil)
		})
	})
}

func TestPodDriver_podDeletedHandler(t *testing.T) {
	type fields struct {
		Client   k8sclient.K8sClient
		handlers map[podStatus]podHandler
		Mounter  util.MountInter
	}
	type args struct {
		ctx context.Context
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    reconcile.Result
		wantErr bool
	}{
		{
			name:    "test",
			fields:  fields{},
			args:    args{},
			want:    reconcile.Result{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodDriver{
				Client:   tt.fields.Client,
				handlers: tt.fields.handlers,
				Mounter:  tt.fields.Mounter,
			}
			got, err := p.podDeletedHandler(tt.args.ctx, tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("podDeletedHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("podDeletedHandler() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodDriver_podErrorHandler(t *testing.T) {
	type fields struct {
		Client   k8sclient.K8sClient
		handlers map[podStatus]podHandler
		Mounter  util.MountInter
	}
	type args struct {
		ctx context.Context
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    reconcile.Result
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodDriver{
				Client:   tt.fields.Client,
				handlers: tt.fields.handlers,
				Mounter:  tt.fields.Mounter,
			}
			got, err := p.podErrorHandler(tt.args.ctx, tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("podErrorHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("podErrorHandler() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodDriver_podRunningHandler(t *testing.T) {
	type fields struct {
		Client   k8sclient.K8sClient
		handlers map[podStatus]podHandler
		Mounter  util.MountInter
	}
	type args struct {
		ctx context.Context
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    reconcile.Result
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PodDriver{
				Client:   tt.fields.Client,
				handlers: tt.fields.handlers,
				Mounter:  tt.fields.Mounter,
			}
			got, err := p.podRunningHandler(tt.args.ctx, tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("podRunningHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("podRunningHandler() got = %v, want %v", got, tt.want)
			}
		})
	}
}
