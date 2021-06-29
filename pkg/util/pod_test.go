package util

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

func TestDeleteResourceOfPod(t *testing.T) {
	podLimit := map[corev1.ResourceName]resource.Quantity{}
	podRequest := map[corev1.ResourceName]resource.Quantity{}
	podLimit[corev1.ResourceCPU] = resource.MustParse("1")
	podLimit[corev1.ResourceMemory] = resource.MustParse("1G")
	podRequest[corev1.ResourceCPU] = resource.MustParse("1")
	podRequest[corev1.ResourceMemory] = resource.MustParse("1G")
	resources := corev1.ResourceRequirements{
		Limits:   podLimit,
		Requests: podRequest,
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test-cn",
				Image: "nginx",
			}},
			NodeName: "test-node",
		},
	}

	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:      "test-cn",
							Image:     "nginx",
							Resources: resources,
						}},
						NodeName: "test-node",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Println(tt.args.pod)
			DeleteResourceOfPod(tt.args.pod)
			fmt.Println(tt.args.pod)
			if !reflect.DeepEqual(pod, tt.args.pod) {
				t.Errorf("deleteResourceOfPod err. got = %v, want = %v", tt.args.pod, pod)
			}
		})
	}
}

func TestIsPodHasResource(t *testing.T) {
	podLimit := map[corev1.ResourceName]resource.Quantity{}
	podRequest := map[corev1.ResourceName]resource.Quantity{}
	podLimit[corev1.ResourceCPU] = resource.MustParse("1")
	podLimit[corev1.ResourceMemory] = resource.MustParse("1G")
	podRequest[corev1.ResourceCPU] = resource.MustParse("1")
	podRequest[corev1.ResourceMemory] = resource.MustParse("1G")
	resources := corev1.ResourceRequirements{
		Limits:   podLimit,
		Requests: podRequest,
	}
	type args struct {
		pod corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test-false",
			args: args{
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:      "test-cn",
							Image:     "nginx",
						}},
						NodeName: "test-node",
					},
				},
			},
			want: false,
		},
		{
			name: "test-true",
			args: args{
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:      "test-cn",
							Image:     "nginx",
							Resources: resources,
						}},
						NodeName: "test-node",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPodHasResource(tt.args.pod); got != tt.want {
				t.Errorf("IsPodHasResource() = %v, want %v", got, tt.want)
			}
		})
	}
}
