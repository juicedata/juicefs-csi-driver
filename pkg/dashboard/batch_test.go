/*
 Copyright 2024 Juicedata Inc

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

package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/services/pods"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

func TestNewUpgradeJobIncludesBatchUpgradeTimeoutEnv(t *testing.T) {
	originalNamespace := config.Namespace
	config.Namespace = "kube-system"
	t.Cleanup(func() {
		config.Namespace = originalNamespace
	})

	t.Setenv(batchUpgradeTimeoutEnv, "45")

	job := NewUpgradeJob("job-1")
	envs := job.Spec.Template.Spec.Containers[0].Env

	timeoutEnv := ""
	for _, env := range envs {
		if env.Name == batchUpgradeTimeoutEnv {
			timeoutEnv = env.Value
			break
		}
	}

	assert.Equal(t, "45", timeoutEnv)
}

func TestNewUpgradeJobUsesDashboardImageFromEnvOnly(t *testing.T) {
	originalNamespace := config.Namespace
	config.Namespace = "kube-system"
	t.Cleanup(func() {
		config.Namespace = originalNamespace
	})

	t.Setenv("DASHBOARD_IMAGE", "")

	job := NewUpgradeJob("job-default-image")
	assert.Equal(t, "", job.Spec.Template.Spec.Containers[0].Image)
}

func TestNewUpgradeJobUsesDefaultServiceAccountWhenUnset(t *testing.T) {
	originalNamespace := config.Namespace
	config.Namespace = "kube-system"
	t.Cleanup(func() {
		config.Namespace = originalNamespace
	})

	t.Setenv(common.JuicefsCSIDashboardSAEnv, "")

	job := NewUpgradeJob("job-default-sa")
	assert.Equal(t, common.DefaultDashboardServiceName, job.Spec.Template.Spec.ServiceAccountName)
}

func TestNewUpgradeJobUsesServiceAccountFromEnv(t *testing.T) {
	originalNamespace := config.Namespace
	config.Namespace = "kube-system"
	t.Cleanup(func() {
		config.Namespace = originalNamespace
	})

	t.Setenv(common.JuicefsCSIDashboardSAEnv, "custom-dashboard-sa")

	job := NewUpgradeJob("job-custom-sa")
	assert.Equal(t, "custom-dashboard-sa", job.Spec.Template.Spec.ServiceAccountName)
}

func TestValidateBatchUpgradeTimeoutEnv(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		t.Setenv(batchUpgradeTimeoutEnv, "")
		assert.NoError(t, validateBatchUpgradeTimeoutEnv())
	})

	t.Run("integer", func(t *testing.T) {
		t.Setenv(batchUpgradeTimeoutEnv, "45")
		assert.NoError(t, validateBatchUpgradeTimeoutEnv())
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv(batchUpgradeTimeoutEnv, "45s")
		assert.Error(t, validateBatchUpgradeTimeoutEnv())
	})
}

// Simple mock PodService for testing
type SimpleMockPodService struct {
	listSidecarTargets func(ctx context.Context, namespace string) ([]config.UpgradeTarget, []config.UpgradeTarget, error)
	listUpgradePods    func(c *gin.Context, uniqueId string, nodeName string, recreate bool) ([]corev1.Pod, error)
	listBatchPods      func(c *gin.Context, conf *config.BatchConfig) ([]corev1.Pod, error)
	listCSINodePods    func(ctx context.Context, nodeName string) ([]corev1.Pod, error)
}

func (m *SimpleMockPodService) ListAppPods(ctx *gin.Context) (*pods.ListAppPodResult, error) {
	return nil, nil
}

func (m *SimpleMockPodService) ListSysPods(ctx *gin.Context) (*pods.ListSysPodResult, error) {
	return nil, nil
}

func (m *SimpleMockPodService) ListMountPods(ctx context.Context) ([]corev1.Pod, error) {
	return nil, nil
}

func (m *SimpleMockPodService) ListCSINodePod(ctx context.Context, nodeName string) ([]corev1.Pod, error) {
	if m.listCSINodePods != nil {
		return m.listCSINodePods(ctx, nodeName)
	}
	return nil, nil
}

func (m *SimpleMockPodService) ListPodPVs(ctx context.Context, pod *corev1.Pod) ([]corev1.PersistentVolume, error) {
	return nil, nil
}

func (m *SimpleMockPodService) ListPodPVCs(ctx context.Context, pod *corev1.Pod) ([]corev1.PersistentVolumeClaim, error) {
	return nil, nil
}

func (m *SimpleMockPodService) ListAppPodMountPods(ctx context.Context, pod *corev1.Pod) ([]corev1.Pod, error) {
	return nil, nil
}

func (m *SimpleMockPodService) ListNodeMountPods(ctx context.Context, nodeName string) ([]corev1.Pod, error) {
	return nil, nil
}

func (m *SimpleMockPodService) ListMountPodAppPods(ctx context.Context, mountPod *corev1.Pod) ([]corev1.Pod, error) {
	return nil, nil
}

func (m *SimpleMockPodService) ListBatchPods(c *gin.Context, conf *config.BatchConfig) ([]corev1.Pod, error) {
	if m.listBatchPods != nil {
		return m.listBatchPods(c, conf)
	}
	return nil, nil
}

func (m *SimpleMockPodService) ListUpgradePods(c *gin.Context, uniqueId string, nodeName string, recreate bool) ([]corev1.Pod, error) {
	if m.listUpgradePods != nil {
		return m.listUpgradePods(c, uniqueId, nodeName, recreate)
	}
	return nil, nil
}

func (m *SimpleMockPodService) ListSidecarUpgradeTargets(ctx context.Context, namespace string) ([]config.UpgradeTarget, []config.UpgradeTarget, error) {
	if m.listSidecarTargets != nil {
		return m.listSidecarTargets(ctx, namespace)
	}
	return nil, nil, nil
}

func (m *SimpleMockPodService) ExecPod(c *gin.Context, namespace, name, container string) {
}

func (m *SimpleMockPodService) WatchPodLogs(c *gin.Context, namespace, name, container string) error {
	return nil
}

func (m *SimpleMockPodService) WatchMountPodAccessLog(c *gin.Context, namespace, name, container string) {
}

func (m *SimpleMockPodService) DebugPod(c *gin.Context, namespace, name, container string) {
}

func (m *SimpleMockPodService) WarmupPod(c *gin.Context, namespace, name, container string) {
}

func (m *SimpleMockPodService) StatsPod(c *gin.Context, namespace, name, container string) {
}

func (m *SimpleMockPodService) DownloadDebugFile(c *gin.Context, namespace, name, container string) error {
	return nil
}

// Test Sidecar with recreate method should fail
func TestCreateSidecarUpgradeJobWithRecreateMethodFails(t *testing.T) {
	originalNamespace := config.Namespace
	originalDisable := config.DisableGraceUpgrade
	config.Namespace = "kube-system"
	config.DisableGraceUpgrade = false
	t.Cleanup(func() {
		config.Namespace = originalNamespace
		config.DisableGraceUpgrade = originalDisable
	})

	t.Setenv(batchUpgradeTimeoutEnv, "")

	api := &API{
		client: &k8sclient.K8sClient{
			Interface: fake.NewSimpleClientset(),
		},
		podSvc: &SimpleMockPodService{},
	}

	reqBody := map[string]interface{}{
		"jobName":       "test-job",
		"targetKind":    "sidecar",
		"upgradeMethod": "recreate",
		"namespace":     "default",
		"worker":        5,
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/batch/upgrade/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.createUpgradeJob()(c)

	// Should return 400 error
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Test max concurrency validation
func TestCreateUpgradeJobMaxConcurrencyExceeded(t *testing.T) {
	originalNamespace := config.Namespace
	originalDisable := config.DisableGraceUpgrade
	config.Namespace = "kube-system"
	config.DisableGraceUpgrade = false
	t.Cleanup(func() {
		config.Namespace = originalNamespace
		config.DisableGraceUpgrade = originalDisable
	})

	t.Setenv(batchUpgradeTimeoutEnv, "")

	api := &API{
		client: &k8sclient.K8sClient{
			Interface: fake.NewSimpleClientset(),
		},
		podSvc: &SimpleMockPodService{},
	}

	reqBody := map[string]interface{}{
		"jobName":       "test-job",
		"targetKind":    "sidecar",
		"upgradeMethod": "binary",
		"namespace":     "default",
		"worker":        51, // Exceed max concurrency
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/batch/upgrade/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.createUpgradeJob()(c)

	// Should return 400 error
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Test no eligible targets returns 400
func TestCreateUpgradeJobNoEligibleTargets(t *testing.T) {
	originalNamespace := config.Namespace
	originalDisable := config.DisableGraceUpgrade
	config.Namespace = "kube-system"
	config.DisableGraceUpgrade = false
	t.Cleanup(func() {
		config.Namespace = originalNamespace
		config.DisableGraceUpgrade = originalDisable
	})

	t.Setenv(batchUpgradeTimeoutEnv, "")

	mockPodSvc := &SimpleMockPodService{
		listSidecarTargets: func(ctx context.Context, namespace string) ([]config.UpgradeTarget, []config.UpgradeTarget, error) {
			return []config.UpgradeTarget{}, []config.UpgradeTarget{}, nil
		},
	}

	api := &API{
		client: &k8sclient.K8sClient{
			Interface: fake.NewSimpleClientset(),
		},
		podSvc: mockPodSvc,
	}

	reqBody := map[string]interface{}{
		"jobName":       "test-job",
		"targetKind":    "sidecar",
		"upgradeMethod": "binary",
		"namespace":     "default",
		"worker":        5,
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/batch/upgrade/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.createUpgradeJob()(c)

	// Should return 400 error
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Test Sidecar namespace validation
func TestCreateSidecarUpgradeJobNamespaceRequired(t *testing.T) {
	originalNamespace := config.Namespace
	originalDisable := config.DisableGraceUpgrade
	config.Namespace = "kube-system"
	config.DisableGraceUpgrade = false
	t.Cleanup(func() {
		config.Namespace = originalNamespace
		config.DisableGraceUpgrade = originalDisable
	})

	t.Setenv(batchUpgradeTimeoutEnv, "")

	api := &API{
		client: &k8sclient.K8sClient{
			Interface: fake.NewSimpleClientset(),
		},
		podSvc: &SimpleMockPodService{},
	}

	// Sidecar request without namespace
	reqBody := map[string]interface{}{
		"jobName":       "test-job",
		"targetKind":    "sidecar",
		"upgradeMethod": "binary",
		"worker":        5,
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/batch/upgrade/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.createUpgradeJob()(c)

	// Should return 400 error
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Test defaulting targetKind to mountPod
func TestCreateUpgradeJobDefaultsToMountPod(t *testing.T) {
	originalNamespace := config.Namespace
	originalDisable := config.DisableGraceUpgrade
	config.Namespace = "kube-system"
	config.DisableGraceUpgrade = false
	t.Cleanup(func() {
		config.Namespace = originalNamespace
		config.DisableGraceUpgrade = originalDisable
	})

	t.Setenv(batchUpgradeTimeoutEnv, "")

	mockPodSvc := &SimpleMockPodService{
		listUpgradePods: func(c *gin.Context, uniqueId string, nodeName string, recreate bool) ([]corev1.Pod, error) {
			// Return empty pods to trigger the no candidates error
			// This verifies that it went to the Mount Pod path
			return []corev1.Pod{}, nil
		},
	}

	api := &API{
		client: &k8sclient.K8sClient{
			Interface: fake.NewSimpleClientset(),
		},
		podSvc: mockPodSvc,
	}

	// Don't specify targetKind
	reqBody := map[string]interface{}{
		"jobName":     "test-job",
		"worker":      5,
		"ignoreError": false,
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/batch/upgrade/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.createUpgradeJob()(c)

	// Should get 400 because no mount pods available, but not because of targetKind
	// The fact that listUpgradePods was called proves it defaulted to mountPod
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "mount pods")
}

func TestCreateSidecarUpgradeJobSuccess(t *testing.T) {
	originalNamespace := config.Namespace
	originalDisable := config.DisableGraceUpgrade
	config.Namespace = "kube-system"
	config.DisableGraceUpgrade = false
	t.Cleanup(func() {
		config.Namespace = originalNamespace
		config.DisableGraceUpgrade = originalDisable
	})

	t.Setenv(batchUpgradeTimeoutEnv, "")

	mockPodSvc := &SimpleMockPodService{
		listSidecarTargets: func(ctx context.Context, namespace string) ([]config.UpgradeTarget, []config.UpgradeTarget, error) {
			return []config.UpgradeTarget{
				{
					Namespace:     namespace,
					Name:          "app-pod-1",
					ContainerName: "jfs-mount",
					Node:          "node-1",
				},
			}, nil, nil
		},
	}

	api := &API{
		client: &k8sclient.K8sClient{
			Interface: fake.NewSimpleClientset(),
		},
		podSvc: mockPodSvc,
	}

	reqBody := map[string]interface{}{
		"jobName":       "test-sidecar-job",
		"targetKind":    "sidecar",
		"upgradeMethod": "binary",
		"namespace":     "default",
		"worker":        2,
		"ignoreError":   true,
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/batch/upgrade/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.createUpgradeJob()(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test-sidecar-job")

	job, err := api.client.BatchV1().Jobs(config.Namespace).Get(context.Background(), "test-sidecar-job", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "test-sidecar-job", job.Name)

	cm, err := api.client.CoreV1().ConfigMaps(config.Namespace).Get(context.Background(), "test-sidecar-job-config", metav1.GetOptions{})
	assert.NoError(t, err)
	cfg, err := config.LoadBatchConfig(cm)
	assert.NoError(t, err)
	assert.Equal(t, 2, cfg.Parallel)
	assert.True(t, cfg.IgnoreError)
	assert.Equal(t, config.UpgradeKindSidecar, cfg.Kind)
	assert.Equal(t, "default", cfg.Namespace)
	if assert.Len(t, cfg.Batches, 1) {
		if assert.Len(t, cfg.Batches[0], 1) {
			assert.Equal(t, "app-pod-1/jfs-mount", cfg.Batches[0][0].Key())
			assert.Equal(t, "", cfg.Batches[0][0].CSINodePod)
		}
	}
}

func TestListSidecarUpgradeTargetsSuccess(t *testing.T) {
	originalNamespace := config.Namespace
	config.Namespace = "kube-system"
	t.Cleanup(func() {
		config.Namespace = originalNamespace
	})

	mockPodSvc := &SimpleMockPodService{
		listSidecarTargets: func(ctx context.Context, namespace string) ([]config.UpgradeTarget, []config.UpgradeTarget, error) {
			return []config.UpgradeTarget{
				{
					Namespace:     namespace,
					Name:          "app-pod-1",
					ContainerName: "jfs-mount",
					Node:          "node-1",
				},
			}, nil, nil
		},
	}

	api := &API{
		client: &k8sclient.K8sClient{
			Interface: fake.NewSimpleClientset(),
		},
		podSvc: mockPodSvc,
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/batch/upgrade/sidecar-targets?namespace=default", nil)
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.listSidecarUpgradeTargets()(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Targets []config.UpgradeTarget `json:"targets"`
		Total   int                    `json:"total"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	if assert.Len(t, resp.Targets, 1) {
		assert.Equal(t, "app-pod-1", resp.Targets[0].Name)
		assert.Equal(t, "app-pod-1/jfs-mount", resp.Targets[0].Key())
	}
	assert.Equal(t, 1, resp.Total)
}

func TestGetUpgradeJobSidecarDoesNotRequireMountPodDiffs(t *testing.T) {
	originalNamespace := config.Namespace
	config.Namespace = "kube-system"
	t.Cleanup(func() {
		config.Namespace = originalNamespace
	})

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sidecar-job",
			Namespace: config.Namespace,
			Labels: map[string]string{
				common.JfsUpgradeConfig: "test-sidecar-job-config",
			},
		},
	}
	cfg := &config.BatchConfig{
		Parallel:  1,
		Kind:      config.UpgradeKindSidecar,
		Namespace: "default",
		Batches: [][]config.UpgradeTarget{
			{
				{
					Namespace:     "default",
					Name:          "app-pod-1",
					ContainerName: "jfs-mount",
					Node:          "node-1",
				},
			},
		},
		Status: config.Running,
	}
	cm, err := config.CreateUpgradeConfig(context.Background(), &k8sclient.K8sClient{
		Interface: fake.NewSimpleClientset(job),
	}, "test-sidecar-job-config", cfg)
	assert.NoError(t, err)

	apiClient := &k8sclient.K8sClient{
		Interface: fake.NewSimpleClientset(job, cm),
	}
	api := &API{
		client: apiClient,
		podSvc: &SimpleMockPodService{
			listBatchPods: func(c *gin.Context, conf *config.BatchConfig) ([]corev1.Pod, error) {
				t.Fatal("sidecar job detail should not list mount pods")
				return nil, nil
			},
		},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/batch/upgrade/jobs/test-sidecar-job", nil)
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "jobName", Value: "test-sidecar-job"}}

	api.getUpgradeJob()(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Job    corev1.Pod         `json:"-"`
		Config config.BatchConfig `json:"config"`
		Diffs  []PodDiff          `json:"diffs"`
		Total  int                `json:"total"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
	assert.Len(t, resp.Diffs, 0)
	assert.Len(t, resp.Config.Batches, 1)
	assert.Equal(t, config.UpgradeKindSidecar, resp.Config.Kind)
}

func TestListSidecarUpgradeTargetsEmpty(t *testing.T) {
	originalNamespace := config.Namespace
	config.Namespace = "kube-system"
	t.Cleanup(func() {
		config.Namespace = originalNamespace
	})

	mockPodSvc := &SimpleMockPodService{
		listSidecarTargets: func(ctx context.Context, namespace string) ([]config.UpgradeTarget, []config.UpgradeTarget, error) {
			return []config.UpgradeTarget{}, nil, nil
		},
	}

	api := &API{
		client: &k8sclient.K8sClient{
			Interface: fake.NewSimpleClientset(),
		},
		podSvc: mockPodSvc,
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/batch/upgrade/sidecar-targets?namespace=default", nil)
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	api.listSidecarUpgradeTargets()(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Targets []config.UpgradeTarget `json:"targets"`
		Total   int                    `json:"total"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp.Targets, 0)
	assert.Equal(t, 0, resp.Total)
}
