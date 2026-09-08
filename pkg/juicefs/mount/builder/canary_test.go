package builder

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestNewCanaryJobFromSpecServiceAccountName(t *testing.T) {
	job := NewCanaryJobFromSpec(CanaryJobSpec{
		Name:               "test-canary",
		Namespace:          "kube-system",
		Image:              "busybox:latest",
		ServiceAccountName: "juicefs-csi-dashboard-sa",
	})

	got := job.Spec.Template.Spec.ServiceAccountName
	if got != "juicefs-csi-dashboard-sa" {
		t.Fatalf("serviceAccountName = %q, want %q", got, "juicefs-csi-dashboard-sa")
	}
}

func TestNewCanaryJobFromSpecDefaultsToNoopCommand(t *testing.T) {
	job := NewCanaryJobFromSpec(CanaryJobSpec{
		Name:      "test-canary",
		Namespace: "kube-system",
		Image:     "busybox:latest",
	})

	container := job.Spec.Template.Spec.Containers[0]
	if !reflect.DeepEqual(container.Command, []string{"sh", "-c", ""}) {
		t.Fatalf("command = %q, want %q", container.Command, []string{"sh", "-c", ""})
	}
	if job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Fatalf("restartPolicy = %q, want %q", job.Spec.Template.Spec.RestartPolicy, corev1.RestartPolicyNever)
	}
}
