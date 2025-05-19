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
	"os"
	"os/exec"
	"reflect"
	"syscall"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver/mocks"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

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
		Labels: map[string]string{
			common.PodJuiceHashLabelKey: "e11ef7a140d2e8bac9c75b1c44dcba22954402edc5015a8eae931d389b82db9",
			common.PodUniqueIdLabelKey:  "test",
		},
		Finalizers: []string{common.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:    "test",
			Image:   "juicedata/mount:ce-v1.2.1",
			Command: []string{"sh", "-c", "exec /bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
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
		Labels: map[string]string{
			common.PodJuiceHashLabelKey: "e11ef7a140d2e8bac9c75b1c44dcba22954402edc5015a8eae931d389b82db9",
			common.PodUniqueIdLabelKey:  "test",
		},
		Finalizers: []string{common.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:    "test",
			Image:   "juicedata/mount:ce-v1.2.1",
			Command: []string{"sh", "-c"},
		}},
	},
}

var deletedPod = &corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name: "juicefs-test-node-deleted",
		Annotations: map[string]string{
			util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
		Labels: map[string]string{
			common.PodJuiceHashLabelKey: "e11ef7a140d2e8bac9c75b1c44dcba22954402edc5015a8eae931d389b82db9",
			common.PodUniqueIdLabelKey:  "test",
		},
		DeletionTimestamp: &metav1.Time{Time: time.Now()},
		Finalizers:        []string{common.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
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
		Labels: map[string]string{
			common.PodJuiceHashLabelKey: "e11ef7a140d2e8bac9c75b1c44dcba22954402edc5015a8eae931d389b82db9",
			common.PodUniqueIdLabelKey:  "test",
		},
		Finalizers: []string{common.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:      "pvc-node01-xxx",
				Image:     "juicedata/mount:ce-v1.2.1",
				Command:   []string{"sh", "-c", "exec /bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
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
		Labels: map[string]string{
			common.PodJuiceHashLabelKey: "e11ef7a140d2e8bac9c75b1c44dcba22954402edc5015a8eae931d389b82db9",
			common.PodUniqueIdLabelKey:  "test",
		},
		Finalizers: []string{common.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:      "pvc-node01-xxx",
				Image:     "juicedata/mount:ce-v1.2.1",
				Command:   []string{"sh", "-c", "exec /bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
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
		Labels: map[string]string{
			common.PodJuiceHashLabelKey: "e11ef7a140d2e8bac9c75b1c44dcba22954402edc5015a8eae931d389b82db9",
			common.PodUniqueIdLabelKey:  "test",
		},
		Finalizers: []string{common.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
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
		Labels: map[string]string{
			common.PodJuiceHashLabelKey: "e11ef7a140d2e8bac9c75b1c44dcba22954402edc5015a8eae931d389b82db9",
			common.PodUniqueIdLabelKey:  "test",
		},
		Finalizers: []string{common.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
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
		Labels: map[string]string{
			common.PodJuiceHashLabelKey: "e11ef7a140d2e8bac9c75b1c44dcba22954402edc5015a8eae931d389b82db9",
			common.PodUniqueIdLabelKey:  "test",
		},
		Finalizers: []string{common.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
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
		Labels: map[string]string{
			common.PodJuiceHashLabelKey: "e11ef7a140d2e8bac9c75b1c44dcba22954402edc5015a8eae931d389b82db9",
			common.PodUniqueIdLabelKey:  "test",
		},
		Finalizers: []string{common.Finalizer},
	},
	Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Image: "juicedata/mount:ce-v1.2.1"}},
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
			if got := getPodStatus(tt.args.pod); got != tt.want {
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
	if len(oldPod.Spec.Containers) != 0 {
		newPod.Spec.Containers = append(newPod.Spec.Containers, oldPod.Spec.Containers...)
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

var _ = Describe("pod handler", func() {
	var (
		d *PodDriver
	)
	BeforeEach(func() {
		passfd.InitTestFds()
		d = NewPodDriver(&k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}, mount.SafeFormatAndMount{
			Interface: mount.New(""),
			Exec:      k8sexec.New(),
		}, &corev1.PodList{})
	})

	AfterEach(func() {
		_ = os.RemoveAll("tmp")
	})

	Describe("Test pod ready handler", func() {
		Context("pod ready add need recovery", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, []OutputCell{
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{nil, os.NewSyscallError("", syscall.ENOTCONN)}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
					}),
					ApplyMethod(reflect.TypeOf(d.Interface), "Mount", func(_ *mount.Mounter, _, _, _ string, _ []string) error {
						return nil
					}),
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
						return genMountInfos(), nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := d.podReadyHandler(context.Background(), readyPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("pod ready add don't need recovery", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, []OutputCell{
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
					}),
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
						return genMountInfos(), nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := d.podReadyHandler(context.Background(), readyPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("mountinfo parse err", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
						return genMountInfos(), errors.New("mountinfo parse fail")
					}),
					ApplyFuncSeq(os.Stat, []OutputCell{
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should not succeed", func() {
				_, err := d.podReadyHandler(context.Background(), readyPod)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("pod ready add target mntPath not exists", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, []OutputCell{
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{nil, notExistsErr}},
					}),
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) { return genMountInfos(), nil }),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := d.podReadyHandler(context.Background(), readyPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("pod ready and mount err", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, []OutputCell{
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{nil, os.NewSyscallError("", syscall.ENOTCONN)}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
					}),
					ApplyMethod(reflect.TypeOf(d.Interface), "Mount", func(_ *mount.Mounter, source string, target string, fstype string, options []string) error {
						return mountErr
					}),
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
						return genMountInfos(), nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should not succeed", func() {
				_, err := d.podReadyHandler(context.Background(), readyPod)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("get nil pod", func() {
			It("should succeed", func() {
				_, err := d.podReadyHandler(context.Background(), nil)
				Expect(err).Should(BeNil())
			})
		})
		Context("pod Annotations is nil", func() {
			It("should not succeed", func() {
				_, err := d.podReadyHandler(context.Background(), &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "juicefs-test-err-pod",
						Annotations: nil,
						Finalizers:  []string{common.Finalizer},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Image: "juicedata/mount:ce-v1.2.1",
						}},
					},
				})
				Expect(err).Should(BeNil())
			})
		})
		Context("pod mount cmd <3", func() {
			It("should not succeed", func() {
				_, err := d.podReadyHandler(context.Background(), errCmdPod)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("parse pod mount cmd mntPath err", func() {
			It("should not succeed", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "juicefs-test-err-mount-cmd-pod",
						Annotations: map[string]string{
							util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
						Finalizers: []string{common.Finalizer},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:    "test",
							Image:   "juicedata/mount:ce-v1.2.1",
							Command: []string{"sh", "-c", "exec /bin/mount.juicefs redis://127.0.0.1/6379/jfs/pvc-xxx"},
						}},
					},
				}
				_, err := d.podReadyHandler(context.Background(), pod)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("pod sourcePath err ", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, []OutputCell{
						{Values: Params{nil, volErr}, Times: 121},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}, Times: 4},
					}),
					ApplyFunc(util.UmountPath, func(ctx context.Context, sourcePath string, lazy bool) error {
						return nil
					}),
					ApplyMethod(reflect.TypeOf(d), "DoAbortFuse", func(_ *PodDriver, mountpod *corev1.Pod, devMinor uint32) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should not succeed", func() {
				_, err := d.podReadyHandler(context.Background(), readyPod)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("pod sourcePath subpath err ", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, []OutputCell{
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{nil, os.NewSyscallError("", syscall.ENOTCONN)}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
					}),
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
						mis := genMountInfos()
						mis[0].Root = "/dir"
						return mis, nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should not succeed", func() {
				_, err := d.podReadyHandler(context.Background(), readyPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("pod target status unexpected ", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, []OutputCell{
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{nil, volErr}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
					}),
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
						return genMountInfos(), nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should not succeed", func() {
				_, err := d.podReadyHandler(context.Background(), readyPod)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("pod target format invalid ", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFuncSeq(os.Stat, []OutputCell{
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
						{Values: Params{mocks.FakeFileInfoIno1{}, nil}},
					}),
					ApplyFunc(mount.ParseMountInfo, func(filename string) ([]mount.MountInfo, error) {
						return genMountInfos(), nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "juicefs-test-err-mount-cmd-pod",
						Annotations: map[string]string{
							util.GetReferenceKey("/mnt/abc"): "/mnt/abc"},
						Finalizers: []string{common.Finalizer},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:    "test",
							Image:   "juicedata/juicefs-csi-driver",
							Command: []string{"sh", "-c", "/bin/mount.juicefs redis://127.0.0.1/6379 /jfs/pvc-xxx"},
						}},
					},
				}
				_, err := d.podReadyHandler(context.Background(), pod)
				Expect(err).Should(BeNil())
			})
		})
	})

	Describe("Test pod delete handler", func() {
		Context("umount fail", func() {
			var (
				patches []*Patches
				tmpCmd  = &exec.Cmd{}
				k       = &k8sclient.K8sClient{}
				tmpPod  = copyPod(deletedPod)
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(util.GetMountPathOfPod, func(pod corev1.Pod) (string, string, error) {
						return "/test", "test", nil
					}),
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte(""), errors.New("test")
					}),
					ApplyMethod(reflect.TypeOf(k), "GetPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string) (*corev1.Pod, error) {
						return nil, errors.New("test")
					}),
					ApplyMethod(reflect.TypeOf(k), "PatchPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string, data []byte, pt types.PatchType) error {
						return nil
					}),
					ApplyMethod(reflect.TypeOf(k), "CreatePod", func(_ *k8sclient.K8sClient, _ context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
						return nil, nil
					}),
					ApplyFunc(apierrors.IsNotFound, func(err error) bool {
						return true
					}),
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return true, nil
					}),
					ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
						return mocks.FakeFileInfoIno1{}, nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})

			It("should succeed", func() {
				_, err := d.podDeletedHandler(context.Background(), tmpPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("new pod create", func() {
			var (
				patches []*Patches
				tmpCmd  = &exec.Cmd{}
				k       = &k8sclient.K8sClient{}
				tmpPod  = copyPod(deletedPod)
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(util.GetMountPathOfPod, func(pod corev1.Pod) (string, string, error) {
						return "/test", "test", nil
					}),
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte(""), nil
					}),
					ApplyMethod(reflect.TypeOf(k), "GetPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string) (*corev1.Pod, error) {
						return nil, errors.New("test")
					}),
					ApplyMethod(reflect.TypeOf(k), "PatchPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string, data []byte, pt types.PatchType) error {
						return nil
					}),
					ApplyMethod(reflect.TypeOf(k), "CreatePod", func(_ *k8sclient.K8sClient, _ context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
						return nil, nil
					}),
					ApplyFunc(apierrors.IsNotFound, func(err error) bool {
						return true
					}),
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return true, nil
					}),
					ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
						return mocks.FakeFileInfoIno1{}, nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := d.podDeletedHandler(context.Background(), tmpPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("pod delete success ", func() {
			var (
				patches []*Patches
				tmpCmd  = &exec.Cmd{}
				tmpPod  = copyPod(readyPod)
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(exec.Command, func(name string, args ...string) *exec.Cmd {
						return tmpCmd
					}),
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						err := d.Client.DeletePod(context.TODO(), tmpPod)
						Expect(err).Should(BeNil())
						return []byte(""), nil
					}),
				)
				_, err := d.Client.CreatePod(context.TODO(), tmpPod)
				Expect(err).Should(BeNil())
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := d.podDeletedHandler(context.Background(), tmpPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("get nil pod", func() {
			It("should succeed", func() {
				_, err := d.podDeletedHandler(context.Background(), nil)
				Expect(err).Should(BeNil())
			})
		})
		Context("pod no finalizer", func() {
			It("should succeed", func() {
				tmpPod := copyPod(readyPod)
				tmpPod.Finalizers = nil
				_, err := d.podDeletedHandler(context.Background(), tmpPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("skip delete resource err pod", func() {
			var (
				patches []*Patches
				k       = &k8sclient.K8sClient{}
				tmpPod  = copyPod(resourceErrPod)
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(k), "PatchPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string, data []byte, pt types.PatchType) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := d.podDeletedHandler(context.Background(), tmpPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("remove pod finalizer err ", func() {
			It("should not succeed", func() {
				tmpPod := copyPod(readyPod)
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				_, err := d.podDeletedHandler(ctx, tmpPod)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("pod no Annotations", func() {
			It("should not succeed", func() {
				tmpPod := copyPod(resourceErrPod)
				tmpPod.Annotations = nil
				_, err := d.Client.CreatePod(context.TODO(), tmpPod)
				Expect(err).Should(BeNil())
				_, err = d.podDeletedHandler(context.Background(), tmpPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("can not get mntTarget from pod Annotations", func() {
			It("should succeed", func() {
				tmpPod := copyPod(resourceErrPod)
				tmpPod.Annotations = map[string]string{
					"/var/lib/xxx": "/var/lib/xxx",
				}
				_, err := d.Client.CreatePod(context.TODO(), tmpPod)
				Expect(err).Should(BeNil())
				_, err = d.podDeletedHandler(context.Background(), tmpPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("get sourcePath from pod cmd failed", func() {
			It("should not succeed", func() {
				tmpPod := copyPod(readyPod)
				tmpPod.Spec.Containers[0].Command = []string{}
				_, err := d.Client.CreatePod(context.TODO(), tmpPod)
				Expect(err).Should(BeNil())
				_, err = d.podDeletedHandler(context.Background(), tmpPod)
				Expect(err).ShouldNot(BeNil())
			})
		})
		Context("umount source err and need mount lazy ", func() {
			var (
				patches []*Patches
				tmpPod  = copyPod(readyPod)
				tmpCmd  = &exec.Cmd{}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(exec.Command, func(name string, args ...string) *exec.Cmd {
						return tmpCmd
					}),
					ApplyMethod(reflect.TypeOf(tmpCmd), "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
						return []byte(""), mountErr
					}),
				)
				_, err := d.Client.CreatePod(context.TODO(), tmpPod)
				Expect(err).Should(BeNil())
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				_, err := d.podDeletedHandler(context.Background(), tmpPod)
				Expect(err).Should(BeNil())
			})
		})
	})

	Describe("Test pod error handler", func() {
		Context("get sourcePath from pod cmd failed", func() {
			It("should succeed", func() {
				pod := copyPod(readyPod)
				pod.Spec.Containers = nil
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := d.podErrorHandler(ctx, pod)
				Expect(err).Should(BeNil())
			})
		})
		Context("pod ResourceError but pod no resource", func() {
			It("should succeed", func() {
				errPod := copyPod(resourceErrPod)
				errPod.Spec.Containers[0].Resources = corev1.ResourceRequirements{}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := d.podErrorHandler(ctx, errPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("GetPod error", func() {
			var (
				patches []*Patches
				k       = &k8sclient.K8sClient{}
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyMethod(reflect.TypeOf(k), "GetPod", func(_ *k8sclient.K8sClient, ctx context.Context, podName, namespace string) (*corev1.Pod, error) {
						select {
						case <-ctx.Done():
							return nil, apierrors.NewTimeoutError("timeout", 10)
						default:
							return &corev1.Pod{
								ObjectMeta: metav1.ObjectMeta{
									Finalizers: []string{common.Finalizer},
								},
							}, nil
						}
					}),
					ApplyMethod(reflect.TypeOf(k), "PatchPod", func(_ *k8sclient.K8sClient, _ context.Context, podName, namespace string, data []byte, pt types.PatchType) error {
						return errors.New("test")
					}),
					ApplyMethod(reflect.TypeOf(k), "DeletePod", func(_ *k8sclient.K8sClient, _ context.Context, pod *corev1.Pod) error {
						return nil
					}),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})
			It("should succeed", func() {
				pod := copyPod(resourceErrPod)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := d.podErrorHandler(ctx, pod)
				Expect(err).Should(BeNil())
			})
		})
		Context("pod err add need delete ", func() {
			var patches []*Patches
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return false, notExistsErr
					}),
				)
				_, err := d.Client.CreatePod(context.TODO(), errorPod1)
				Expect(err).Should(BeNil())
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
				_ = d.Client.DeletePod(context.TODO(), errorPod1)
			})
			It("should succeed", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := d.podErrorHandler(ctx, errorPod1)
				Expect(err).Should(BeNil())
			})
		})
		Context("get nil pod", func() {
			It("should succeed", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := d.podErrorHandler(ctx, nil)
				Expect(err).Should(BeNil())
			})
		})
		Context("pod ResourceError", func() {
			var (
				patches []*Patches
				errPod  = copyPod(resourceErrPod)
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return false, notExistsErr
					}),
				)
				_, err := d.Client.CreatePod(context.TODO(), errPod)
				Expect(err).Should(BeNil())
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
				err := d.Client.DeletePod(context.TODO(), errPod)
				Expect(err).Should(BeNil())
			})
			It("should succeed", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := d.podErrorHandler(ctx, errPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("pod ResourceError and remove pod Finalizer err", func() {
			It("should succeed", func() {
				errPod := copyPod(resourceErrPod)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := d.podErrorHandler(ctx, errPod)
				Expect(err).Should(BeNil())
			})
		})
		Context("sourcePath not mount", func() {
			var (
				patches []*Patches
				pod     = copyPod(readyPod)
			)
			BeforeEach(func() {
				patches = append(patches,
					ApplyFunc(mount.PathExists, func(path string) (bool, error) {
						return true, nil
					}),
					ApplyMethod(reflect.TypeOf(d.Interface), "IsLikelyNotMountPoint",
						func(_ *mount.Mounter, file string) (bool, error) {
							return true, nil
						},
					),
				)
			})
			AfterEach(func() {
				for _, patch := range patches {
					patch.Reset()
				}
			})

			It("should succeed", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := d.podErrorHandler(ctx, pod)
				Expect(err).Should(BeNil())
			})
		})
	})
})
