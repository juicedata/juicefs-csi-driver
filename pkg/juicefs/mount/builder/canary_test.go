package builder

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

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

func TestNewCanaryJobFromSpecCopiesNodeSelector(t *testing.T) {
	nodeSelector := map[string]string{
		"serverless": "true",
		"zone":       "cn-hangzhou-a",
	}
	job := NewCanaryJobFromSpec(CanaryJobSpec{
		Name:         "test-canary",
		Namespace:    "kube-system",
		Image:        "busybox:latest",
		NodeSelector: nodeSelector,
	})

	if got := job.Spec.Template.Spec.NodeSelector["zone"]; got != "cn-hangzhou-a" {
		t.Fatalf("nodeSelector zone = %q, want %q", got, "cn-hangzhou-a")
	}
}

func TestNewCanaryJobFromSpecCopiesTolerations(t *testing.T) {
	tolerations := []corev1.Toleration{
		{Key: "node.kubernetes.io/serverless", Operator: corev1.TolerationOpExists},
	}
	job := NewCanaryJobFromSpec(CanaryJobSpec{
		Name:        "test-canary",
		Namespace:   "kube-system",
		Image:       "busybox:latest",
		Tolerations: tolerations,
	})

	if !reflect.DeepEqual(job.Spec.Template.Spec.Tolerations, tolerations) {
		t.Fatalf("tolerations = %v, want %v", job.Spec.Template.Spec.Tolerations, tolerations)
	}
}

func TestNewCanaryJobFromSpecSetsAnnotations(t *testing.T) {
	job := NewCanaryJobFromSpec(CanaryJobSpec{
		Name:        "test-canary",
		Namespace:   "kube-system",
		Image:       "busybox:latest",
		Annotations: map[string]string{"vke.volcengine.com/burst-to-vci": "enforce"},
	})

	if got := job.Spec.Template.Annotations["vke.volcengine.com/burst-to-vci"]; got != "enforce" {
		t.Fatalf("annotation = %q, want %q", got, "enforce")
	}
}
