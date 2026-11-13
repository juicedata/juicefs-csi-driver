/*
 Copyright 2026 Juicedata Inc

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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

// wsCall is one call that reached the pod service from a websocket endpoint.
type wsCall struct {
	method, namespace, name, container string
}

// recordingPodService records the websocket calls that reach the pod service, instead
// of streaming logs or running a command in a container.
type recordingPodService struct {
	SimpleMockPodService
	calls []wsCall
}

func (r *recordingPodService) record(method, namespace, name, container string) {
	r.calls = append(r.calls, wsCall{method, namespace, name, container})
}

func (r *recordingPodService) ExecPod(_ *gin.Context, namespace, name, container string) {
	r.record("exec", namespace, name, container)
}

func (r *recordingPodService) WatchPodLogs(_ *gin.Context, namespace, name, container string) error {
	r.record("logs", namespace, name, container)
	return nil
}

func (r *recordingPodService) WatchMountPodAccessLog(_ *gin.Context, namespace, name, container string) {
	r.record("accesslog", namespace, name, container)
}

func (r *recordingPodService) DebugPod(_ *gin.Context, namespace, name, container string) {
	r.record("debug", namespace, name, container)
}

func (r *recordingPodService) WarmupPod(_ *gin.Context, namespace, name, container string) {
	r.record("warmup", namespace, name, container)
}

func (r *recordingPodService) StatsPod(_ *gin.Context, namespace, name, container string) {
	r.record("stats", namespace, name, container)
}

func podWithContainer(namespace, name, container string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: container}}},
	}
}

// newWebsocketTestRouter serves the dashboard API over a fake cluster that holds a
// mount pod, a CSI node pod with an init container, an application pod with a JuiceFS volume, and an
// unrelated pod in another namespace.
func newWebsocketTestRouter(t *testing.T) (*gin.Engine, *recordingPodService) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	appPod := podWithContainer("tenant", "app-with-juicefs", "app", nil)
	appPod.Spec.Volumes = []corev1.Volume{{
		Name:         "data",
		VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"}},
	}}
	csiNode := podWithContainer("kube-system", "juicefs-csi-node-abc", "juicefs-plugin",
		map[string]string{"app.kubernetes.io/name": "juicefs-csi-driver", "app": "juicefs-csi-node"})
	csiNode.Spec.InitContainers = []corev1.Container{{Name: "init"}}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		podWithContainer("kube-system", "juicefs-mount-pod", "jfs-mount",
			map[string]string{"app.kubernetes.io/name": "juicefs-mount"}),
		podWithContainer("kube-system", "jfs-cg-worker-0", "juicefs-cg-worker",
			map[string]string{"app.kubernetes.io/name": "juicefs-cache-group-worker"}),
		podWithContainer("kube-system", "juicefs-csi-controller-0", "juicefs-plugin",
			map[string]string{"app.kubernetes.io/name": "juicefs-csi-driver", "app": "juicefs-csi-controller"}),
		podWithContainer("kube-system", "juicefs-csi-dashboard-xyz", "dashboard",
			map[string]string{"app.kubernetes.io/name": "juicefs-csi-driver", "app.kubernetes.io/component": "dashboard"}),
		csiNode,
		appPod,
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Namespace: "tenant", Name: "data"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-data"},
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-data"},
			Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{Driver: config.DriverName, VolumeHandle: "pv-data"},
			}},
		},
		podWithContainer("other-team", "unrelated", "app", nil),
	).Build()

	gin.SetMode(gin.TestMode)
	svc := &recordingPodService{}
	router := gin.New()
	(&API{cachedReader: reader, mgrClient: reader, podSvc: svc}).Handle(router.Group("/api/v1"))
	return router, svc
}

func TestWebsocketPodEndpointsScope(t *testing.T) {
	// logs stay available on every JuiceFS pod the dashboard lists; the exec-style
	// endpoints are limited to the pods those features operate on.
	logsEndpoints := []string{"logs"}
	execEndpoints := []string{"exec", "accesslog", "debug", "warmup", "stats"}

	requests := []struct {
		name      string
		path      string // namespace/name/container
		container string
		logsOK    bool
		execOK    bool
	}{
		{name: "mount pod", path: "kube-system/juicefs-mount-pod/jfs-mount", container: "jfs-mount", logsOK: true, execOK: true},
		{name: "cache group worker", path: "kube-system/jfs-cg-worker-0/juicefs-cg-worker", container: "juicefs-cg-worker", logsOK: true, execOK: true},
		{name: "application pod with a JuiceFS volume", path: "tenant/app-with-juicefs/app", container: "app", logsOK: true, execOK: true},
		{name: "CSI node pod", path: "kube-system/juicefs-csi-node-abc/juicefs-plugin", container: "juicefs-plugin", logsOK: true, execOK: false},
		{name: "CSI controller pod", path: "kube-system/juicefs-csi-controller-0/juicefs-plugin", container: "juicefs-plugin", logsOK: true, execOK: false},
		{name: "dashboard pod", path: "kube-system/juicefs-csi-dashboard-xyz/dashboard", container: "dashboard", logsOK: true, execOK: false},
		{name: "pod outside JuiceFS", path: "other-team/unrelated/app", container: "app", logsOK: false, execOK: false},
		{name: "missing pod", path: "other-team/does-not-exist/app", container: "app", logsOK: false, execOK: false},
	}

	run := func(t *testing.T, endpoint, path, container string, wantAccept bool) {
		t.Helper()
		router, svc := newWebsocketTestRouter(t)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ws/pod/"+path+"/"+endpoint, nil))

		if !wantAccept {
			if w.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404, body = %q", w.Code, w.Body.String())
			}
			if len(svc.calls) != 0 {
				t.Fatalf("calls = %+v, want none (request refused)", svc.calls)
			}
			return
		}
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body = %q", w.Code, w.Body.String())
		}
		parts := strings.Split(path, "/")
		want := wsCall{method: endpoint, namespace: parts[0], name: parts[1], container: container}
		if len(svc.calls) != 1 || svc.calls[0] != want {
			t.Fatalf("calls = %+v, want [%+v]", svc.calls, want)
		}
	}

	for _, req := range requests {
		for _, endpoint := range logsEndpoints {
			t.Run(endpoint+"/"+req.name, func(t *testing.T) { run(t, endpoint, req.path, req.container, req.logsOK) })
		}
		for _, endpoint := range execEndpoints {
			t.Run(endpoint+"/"+req.name, func(t *testing.T) { run(t, endpoint, req.path, req.container, req.execOK) })
		}
	}
}

// An exec-style endpoint with a container the pod does not have is refused, after
// the pod itself is accepted as an exec target.
func TestWebsocketExecUnknownContainerRefused(t *testing.T) {
	for _, endpoint := range []string{"exec", "debug", "warmup", "stats", "accesslog"} {
		t.Run(endpoint, func(t *testing.T) {
			router, svc := newWebsocketTestRouter(t)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ws/pod/kube-system/juicefs-mount-pod/other/"+endpoint, nil))
			if w.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404, body = %q", w.Code, w.Body.String())
			}
			if len(svc.calls) != 0 {
				t.Fatalf("calls = %+v, want none", svc.calls)
			}
		})
	}
}

// The smooth upgrade endpoint takes no container, and its handler talks to the API
// server directly, so a refused pod must stop the request before the handler. A pod
// outside JuiceFS and a JuiceFS system pod that is not an exec target (here a CSI
// node pod) are both refused.
func TestWebsocketUpgradeRefusesNonExecTargets(t *testing.T) {
	for _, path := range []string{"other-team/unrelated", "kube-system/juicefs-csi-node-abc"} {
		t.Run(path, func(t *testing.T) {
			router, _ := newWebsocketTestRouter(t)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ws/pod/"+path+"/upgrade", nil))
			if w.Code != http.StatusNotFound || w.Body.String() != "not found" {
				t.Fatalf("status = %d, body = %q, want 404 \"not found\"", w.Code, w.Body.String())
			}
		})
	}
}

// The pod check stops the request, so a refused pod gets one answer, not one from the
// check and another from the handler.
func TestPodMiddlewareAbortsForUnlistedPod(t *testing.T) {
	router, _ := newWebsocketTestRouter(t)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/pod/other-team/unrelated/", nil))
	if w.Code != http.StatusNotFound || w.Body.String() != "not found" {
		t.Fatalf("status = %d, body = %q, want 404 \"not found\"", w.Code, w.Body.String())
	}
}
