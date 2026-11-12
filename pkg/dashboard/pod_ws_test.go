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

func TestWebsocketPodEndpointsRefuseUnlistedPods(t *testing.T) {
	endpoints := []string{"logs", "exec", "accesslog", "debug", "warmup", "stats"}
	requests := []struct {
		name           string
		path           string // namespace/name/container
		wantStatus     int
		wantCall       bool
		wantCallTarget wsCall
	}{
		{name: "pod outside JuiceFS", path: "other-team/unrelated/app", wantStatus: http.StatusNotFound},
		{name: "missing pod", path: "other-team/does-not-exist/app", wantStatus: http.StatusNotFound},
		{name: "mount pod, unknown container", path: "kube-system/juicefs-mount-pod/other", wantStatus: http.StatusNotFound},
		{name: "mount pod", path: "kube-system/juicefs-mount-pod/jfs-mount", wantStatus: http.StatusOK, wantCall: true,
			wantCallTarget: wsCall{namespace: "kube-system", name: "juicefs-mount-pod", container: "jfs-mount"}},
		{name: "CSI node pod", path: "kube-system/juicefs-csi-node-abc/juicefs-plugin", wantStatus: http.StatusOK, wantCall: true,
			wantCallTarget: wsCall{namespace: "kube-system", name: "juicefs-csi-node-abc", container: "juicefs-plugin"}},
		{name: "CSI node pod, init container", path: "kube-system/juicefs-csi-node-abc/init", wantStatus: http.StatusOK, wantCall: true,
			wantCallTarget: wsCall{namespace: "kube-system", name: "juicefs-csi-node-abc", container: "init"}},
		{name: "application pod with a JuiceFS volume", path: "tenant/app-with-juicefs/app", wantStatus: http.StatusOK, wantCall: true,
			wantCallTarget: wsCall{namespace: "tenant", name: "app-with-juicefs", container: "app"}},
	}

	for _, endpoint := range endpoints {
		for _, req := range requests {
			t.Run(endpoint+"/"+req.name, func(t *testing.T) {
				router, svc := newWebsocketTestRouter(t)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ws/pod/"+req.path+"/"+endpoint, nil))

				if w.Code != req.wantStatus {
					t.Fatalf("status = %d, want %d, body = %q", w.Code, req.wantStatus, w.Body.String())
				}
				if !req.wantCall {
					if len(svc.calls) != 0 {
						t.Fatalf("calls = %+v, want none", svc.calls)
					}
					return
				}
				want := req.wantCallTarget
				want.method = endpoint
				if len(svc.calls) != 1 || svc.calls[0] != want {
					t.Fatalf("calls = %+v, want [%+v]", svc.calls, want)
				}
			})
		}
	}
}

// The smooth upgrade endpoint takes no container, and its handler talks to the API
// server directly, so a refused pod must stop the request before the handler.
func TestWebsocketUpgradeRefusesUnlistedPod(t *testing.T) {
	router, _ := newWebsocketTestRouter(t)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ws/pod/other-team/unrelated/upgrade", nil))
	if w.Code != http.StatusNotFound || w.Body.String() != "not found" {
		t.Fatalf("status = %d, body = %q, want 404 \"not found\"", w.Code, w.Body.String())
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
