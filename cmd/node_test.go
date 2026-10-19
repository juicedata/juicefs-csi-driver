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

package main

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

const testPodName, testPodNamespace = "juicefs-csi-node-abcde", "juicefs-system"

// clientTimeout is what client-go returns when the apiserver doesn't send headers within rest.Config.Timeout (#1669).
var clientTimeout = &url.Error{Op: "Get", URL: "https://172.20.0.1:443/api/v1/namespaces/" + testPodNamespace + "/pods/" + testPodName, Err: context.DeadlineExceeded}

// newPodClient returns a client whose n-th GetPod call fails with getErr(n) when that is non-nil,
// and a counter of the GetPod calls.
func newPodClient(getErr func(n int) error) (*k8sclient.K8sClient, *int) {
	clientset := fake.NewSimpleClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: testPodName, Namespace: testPodNamespace}})
	calls := new(int)
	clientset.PrependReactor("get", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		*calls++
		if err := getErr(*calls); err != nil {
			return true, nil, err
		}
		return false, nil, nil
	})
	return &k8sclient.K8sClient{Interface: clientset}, calls
}

func TestGetOwnPodWithRetry(t *testing.T) {
	backoff := wait.Backoff{Duration: time.Millisecond, Factor: 1, Steps: 3}
	isDeadlineExceeded := func(err error) bool { return errors.Is(err, context.DeadlineExceeded) }

	tests := []struct {
		name      string
		failures  int
		err       error
		wantErr   func(error) bool // nil: no error expected
		wantCalls int
	}{
		{name: "first attempt succeeds", wantCalls: 1},
		{name: "client timeout is retried", failures: 2, err: clientTimeout, wantCalls: 3},
		{name: "503 is retried", failures: 2, err: k8serrors.NewServiceUnavailable("apiserver overloaded"), wantCalls: 3},
		{name: "gives up after the backoff steps with the last error", failures: 5, err: clientTimeout, wantErr: isDeadlineExceeded, wantCalls: 3},
		{name: "not found is not retried", failures: 5, err: k8serrors.NewNotFound(corev1.Resource("pods"), testPodName), wantErr: k8serrors.IsNotFound, wantCalls: 1},
		{name: "forbidden is not retried", failures: 5, err: k8serrors.NewForbidden(corev1.Resource("pods"), testPodName, errors.New("rbac")), wantErr: k8serrors.IsForbidden, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, calls := newPodClient(func(n int) error {
				if n <= tt.failures {
					return tt.err
				}
				return nil
			})

			pod, err := getOwnPodWithRetry(context.Background(), client, testPodName, testPodNamespace, backoff)
			if tt.wantErr == nil {
				if err != nil || pod == nil || pod.Name != testPodName {
					t.Fatalf("pod = %v, err = %v, want pod %s", pod, err, testPodName)
				}
			} else if !tt.wantErr(err) {
				t.Fatalf("err = %v, not the expected error", err)
			}
			if *calls != tt.wantCalls {
				t.Fatalf("GetPod calls = %d, want %d", *calls, tt.wantCalls)
			}
		})
	}
}

// On SIGTERM nodeRun's ctx is canceled, and the retries must stop instead of running until SIGKILL.
func TestGetOwnPodWithRetryStopsOnCancel(t *testing.T) {
	backoff := wait.Backoff{Duration: 5 * time.Second, Factor: 1, Steps: 2}

	tests := []struct {
		name   string
		getErr func(cancel context.CancelFunc) error
	}{
		{name: "canceled during an attempt", getErr: func(cancel context.CancelFunc) error {
			cancel()
			return &url.Error{Op: "Get", URL: clientTimeout.URL, Err: context.Canceled}
		}},
		{name: "canceled during the backoff sleep", getErr: func(cancel context.CancelFunc) error {
			time.AfterFunc(10*time.Millisecond, cancel)
			return clientTimeout
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client, calls := newPodClient(func(int) error { return tt.getErr(cancel) })

			start := time.Now()
			_, err := getOwnPodWithRetry(ctx, client, testPodName, testPodNamespace, backoff)
			if elapsed := time.Since(start); elapsed >= backoff.Duration {
				t.Fatalf("returned after %v, want before the %v backoff sleep ends", elapsed, backoff.Duration)
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", err)
			}
			if *calls != 1 {
				t.Fatalf("GetPod calls = %d, want 1", *calls)
			}
		})
	}
}

func TestStartupGetPodBackoff(t *testing.T) {
	// Same loop as wait.ExponentialBackoffWithContext: one attempt per step, with a sleep
	// between attempts but not after the last one. A Cap would end the steps early.
	b := startupGetPodBackoff
	b.Jitter = 0
	attempts, slept := 0, time.Duration(0)
	for b.Steps > 0 {
		attempts++
		if b.Steps == 1 {
			break
		}
		slept += b.Step()
	}
	if attempts != 7 || slept != 63*time.Second {
		t.Fatalf("startupGetPodBackoff gives %d attempts and %v of sleep, want 7 and 63s", attempts, slept)
	}
}
