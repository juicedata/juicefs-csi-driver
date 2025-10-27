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
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sMount "k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func TestDaemonSetMount_JMount(t *testing.T) {
	// Use a context with timeout for tests
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Set required global variables
	jfsConfig.NodeName = "test-node"
	jfsConfig.Namespace = "test-ns"
	
	tests := []struct {
		name         string
		jfsSetting   *jfsConfig.JfsSetting
		existingDS   *appsv1.DaemonSet
		existingPod  *corev1.Pod
		wantErr      bool
		errContains  string
	}{
		{
			name: "create new daemonset",
			jfsSetting: &jfsConfig.JfsSetting{
				UniqueId:   "test-unique-id",
				TargetPath: "/var/lib/kubelet/pods/test-pod/volumes/test-volume",
				VolumeId:   "test-volume",
				Name:       "test-name",
				MetaUrl:    "redis://localhost:6379/1",
				Source:     "test-source",
				PV: &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pv",
					},
				},
				MountMode: string(jfsConfig.MountModeDaemonSet),
				Attr: &jfsConfig.PodAttr{
					Image: "juicedata/mount:latest",
				},
			},
			existingDS: nil,
			existingPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "juicefs-test-unique-id-mount-ds-12345",
					Namespace: "test-ns",
					Labels: map[string]string{
						common.PodTypeKey:          common.PodTypeValue,
						common.PodUniqueIdLabelKey: "test-unique-id",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "test-node",
					Containers: []corev1.Container{
						{
							Name: "jfs-mount",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "jfs-dir",
									MountPath: "/data/test-unique-id",
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "update existing daemonset with different hash",
			jfsSetting: &jfsConfig.JfsSetting{
				UniqueId:   "test-unique-id",
				TargetPath: "/var/lib/kubelet/pods/test-pod/volumes/test-volume",
				VolumeId:   "test-volume",
				Name:       "test-name",
				MetaUrl:    "redis://localhost:6379/1",
				Source:     "test-source",
				PV: &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pv",
					},
				},
				MountMode: string(jfsConfig.MountModeDaemonSet),
				Attr: &jfsConfig.PodAttr{
					Image: "juicedata/mount:latest",
				},
			},
			existingDS: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "juicefs-test-unique-id-mount-ds",
					Namespace: "test-ns",
					Labels: map[string]string{
						common.PodJuiceHashLabelKey: "old-hash",
					},
					Annotations: map[string]string{},
				},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.PodUniqueIdLabelKey: "test-unique-id",
						},
					},
				},
			},
			existingPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "juicefs-test-unique-id-mount-ds-12345",
					Namespace: "test-ns",
					Labels: map[string]string{
						common.PodTypeKey:          common.PodTypeValue,
						common.PodUniqueIdLabelKey: "test-unique-id",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "test-node",
					Containers: []corev1.Container{
						{
							Name: "jfs-mount",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "jfs-dir",
									MountPath: "/data/test-unique-id",
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "add reference to existing daemonset with same hash",
			jfsSetting: &jfsConfig.JfsSetting{
				UniqueId:   "test-unique-id",
				TargetPath: "/var/lib/kubelet/pods/test-pod/volumes/test-volume",
				HashVal:    "test-hash",
				VolumeId:   "test-volume",
				Name:       "test-name",
				MetaUrl:    "redis://localhost:6379/1",
				Source:     "test-source",
				PV: &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pv",
					},
				},
				MountMode: string(jfsConfig.MountModeDaemonSet),
				Attr: &jfsConfig.PodAttr{
					Image: "juicedata/mount:latest",
				},
			},
			existingDS: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "juicefs-test-unique-id-mount-ds",
					Namespace: "test-ns",
					Labels: map[string]string{
						common.PodJuiceHashLabelKey: "test-hash",
					},
					Annotations: map[string]string{
						util.GetReferenceKey("/existing/path"): "/existing/path",
					},
				},
				Spec: appsv1.DaemonSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.PodUniqueIdLabelKey: "test-unique-id",
						},
					},
				},
			},
			existingPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "juicefs-test-unique-id-mount-ds-12345",
					Namespace: "test-ns",
					Labels: map[string]string{
						common.PodTypeKey:          common.PodTypeValue,
						common.PodUniqueIdLabelKey: "test-unique-id",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: "test-node",
					Containers: []corev1.Container{
						{
							Name: "jfs-mount",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "jfs-dir",
									MountPath: "/data/test-unique-id",
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake k8s client
			var objects []runtime.Object
			if tt.existingDS != nil {
				objects = append(objects, tt.existingDS)
			}
			if tt.existingPod != nil {
				objects = append(objects, tt.existingPod)
			}
			
			fakeClient := fake.NewSimpleClientset(objects...)
			k8sClient := &k8sclient.K8sClient{}
			k8sClient.Interface = fakeClient
			
			// Create DaemonSetMount
			mounter := &k8sMount.FakeMounter{}
			d := NewDaemonSetMount(k8sClient, k8sMount.SafeFormatAndMount{
				Interface: mounter,
				Exec:      nil,
			})
			
			// Call JMount
			appInfo := &jfsConfig.AppInfo{}
			err := d.JMount(ctx, appInfo, tt.jfsSetting)
			
			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("JMount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if err != nil && tt.errContains != "" && !contains(err.Error(), tt.errContains) {
				t.Errorf("JMount() error = %v, want error containing %v", err, tt.errContains)
			}
		})
	}
}

func TestDaemonSetMount_GetMountRef(t *testing.T) {
	ctx := context.Background()
	jfsConfig.Namespace = "test-ns"
	
	tests := []struct {
		name      string
		dsName    string
		target    string
		existingDS *appsv1.DaemonSet
		wantRefs  int
		wantErr   bool
	}{
		{
			name:   "daemonset not found",
			dsName: "non-existent-ds",
			target: "/test/target",
			existingDS: nil,
			wantRefs: 0,
			wantErr: false,
		},
		{
			name:   "daemonset with no references",
			dsName: "test-ds",
			target: "/test/target",
			existingDS: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ds",
					Namespace: "test-ns",
					Annotations: map[string]string{},
				},
			},
			wantRefs: 0,
			wantErr: false,
		},
		{
			name:   "daemonset with multiple references",
			dsName: "test-ds",
			target: "/test/target",
			existingDS: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ds",
					Namespace: "test-ns",
					Annotations: map[string]string{
						util.GetReferenceKey("/path1"): "/path1",
						util.GetReferenceKey("/path2"): "/path2",
						util.GetReferenceKey("/path3"): "/path3",
						"other-annotation": "value",
					},
				},
			},
			wantRefs: 3,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake k8s client
			var objects []runtime.Object
			if tt.existingDS != nil {
				objects = append(objects, tt.existingDS)
			}
			
			fakeClient := fake.NewSimpleClientset(objects...)
			k8sClient := &k8sclient.K8sClient{}
			k8sClient.Interface = fakeClient
			
			// Create DaemonSetMount
			mounter := &k8sMount.FakeMounter{}
			d := NewDaemonSetMount(k8sClient, k8sMount.SafeFormatAndMount{
				Interface: mounter,
				Exec:      nil,
			})
			
			// Get mount references
			refs, err := d.GetMountRef(ctx, tt.target, tt.dsName)
			
			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMountRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			// Check reference count
			if refs != tt.wantRefs {
				t.Errorf("GetMountRef() refs = %v, want %v", refs, tt.wantRefs)
			}
		})
	}
}

func TestDaemonSetMount_UmountTarget(t *testing.T) {
	ctx := context.Background()
	jfsConfig.Namespace = "test-ns"
	
	tests := []struct {
		name         string
		dsName       string
		target       string
		existingDS   *appsv1.DaemonSet
		expectDelete bool
		wantErr      bool
	}{
		{
			name:   "remove last reference and delete daemonset",
			dsName: "test-ds",
			target: "/test/target",
			existingDS: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ds",
					Namespace: "test-ns",
					Annotations: map[string]string{
						util.GetReferenceKey("/test/target"): "/test/target",
					},
				},
			},
			expectDelete: true,
			wantErr:      false,
		},
		{
			name:   "remove reference but keep daemonset",
			dsName: "test-ds",
			target: "/test/target",
			existingDS: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ds",
					Namespace: "test-ns",
					Annotations: map[string]string{
						util.GetReferenceKey("/test/target"): "/test/target",
						util.GetReferenceKey("/other/path"):  "/other/path",
					},
				},
			},
			expectDelete: false,
			wantErr:      false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake k8s client
			var objects []runtime.Object
			if tt.existingDS != nil {
				objects = append(objects, tt.existingDS)
			}
			
			fakeClient := fake.NewSimpleClientset(objects...)
			k8sClient := &k8sclient.K8sClient{}
			k8sClient.Interface = fakeClient
			
			// Create DaemonSetMount
			mounter := &k8sMount.FakeMounter{}
			d := NewDaemonSetMount(k8sClient, k8sMount.SafeFormatAndMount{
				Interface: mounter,
				Exec:      nil,
			})
			
			// Unmount target
			err := d.UmountTarget(ctx, tt.target, tt.dsName)
			
			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("UmountTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			// Check if DaemonSet was deleted
			_, err = k8sClient.GetDaemonSet(ctx, tt.dsName, jfsConfig.Namespace)
			deleted := err != nil
			if deleted != tt.expectDelete {
				t.Errorf("DaemonSet deleted = %v, want %v", deleted, tt.expectDelete)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}