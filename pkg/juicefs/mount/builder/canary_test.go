package builder

import "testing"

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
