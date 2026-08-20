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

package util

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A syscall on an unresponsive mount never returns and its goroutine keeps the
// OS thread, so every retry strands one. These tests check the bound on that.

// blockingCall never returns. It reports starting so tests can count what ran.
func blockingCall(started *atomic.Int64, release <-chan struct{}) func(context.Context) error {
	return func(context.Context) error {
		started.Add(1)
		<-release
		return nil
	}
}

// waitFor polls: the bookkeeping happens on the goroutine running f, not here.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestDoPathWithTimeoutBoundsInflightCalls(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	var started atomic.Int64

	const path = "/jfs/bounds-inflight"
	for i := 0; i < maxInflightPerPath+20; i++ {
		err := DoPathWithTimeout(context.Background(), 10*time.Millisecond, path, blockingCall(&started, release))
		if !errors.Is(err, ErrFunctionTimeout) {
			t.Fatalf("call %d: got %v, want ErrFunctionTimeout", i, err)
		}
	}

	waitFor(t, "the budget to fill", func() bool { return started.Load() >= maxInflightPerPath })
	// A refused call costs no goroutine and no thread.
	time.Sleep(100 * time.Millisecond)
	if got := started.Load(); got != maxInflightPerPath {
		t.Errorf("f ran %d times, want at most %d", got, maxInflightPerPath)
	}
}

func TestDoPathWithTimeoutReleasesBudgetWhenCallsReturn(t *testing.T) {
	release := make(chan struct{})
	var started atomic.Int64

	const path = "/jfs/releases-budget"
	for i := 0; i < maxInflightPerPath; i++ {
		_ = DoPathWithTimeout(context.Background(), 10*time.Millisecond, path, blockingCall(&started, release))
	}
	waitFor(t, "the budget to fill", func() bool { return started.Load() >= maxInflightPerPath })

	err := DoPathWithTimeout(context.Background(), time.Second, path, func(context.Context) error { return nil })
	if !errors.Is(err, ErrFunctionTimeout) {
		t.Fatalf("got %v, want ErrFunctionTimeout while the budget is full", err)
	}

	// Aborting the fuse connection is what makes the blocked syscalls return.
	close(release)
	waitFor(t, "the path to accept calls again", func() bool {
		return DoPathWithTimeout(context.Background(), time.Second, path, func(context.Context) error { return nil }) == nil
	})
}

func TestDoPathWithTimeoutKeepsPathBudgetsSeparate(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	var started atomic.Int64

	for i := 0; i < maxInflightPerPath+5; i++ {
		_ = DoPathWithTimeout(context.Background(), 10*time.Millisecond, "/jfs/separate-stuck", blockingCall(&started, release))
	}
	waitFor(t, "the budget to fill", func() bool { return started.Load() >= maxInflightPerPath })

	err := DoPathWithTimeout(context.Background(), time.Second, "/jfs/separate-healthy", func(context.Context) error { return nil })
	if err != nil {
		t.Errorf("got %v, want one stuck mount not to affect the others", err)
	}
}

func TestDoPathWithTimeoutRefusalReportsParentError(t *testing.T) {
	// WaitUntilMountReady and ceMount retry until the error is their parent's,
	// so a refusal has to report that one or those loops never end.
	release := make(chan struct{})
	defer close(release)
	var started atomic.Int64

	const path = "/jfs/refusal-reports-parent"
	for i := 0; i < maxInflightPerPath; i++ {
		_ = DoPathWithTimeout(context.Background(), 10*time.Millisecond, path, blockingCall(&started, release))
	}
	waitFor(t, "the budget to fill", func() bool { return started.Load() >= maxInflightPerPath })

	parent, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	waitFor(t, "the parent to expire", func() bool { return parent.Err() != nil })

	err := DoPathWithTimeout(parent, time.Second, path, func(context.Context) error { return nil })
	if err != context.DeadlineExceeded {
		t.Errorf("got %v, want context.DeadlineExceeded", err)
	}
}

func TestDoPathWithTimeoutForgetsSettledPaths(t *testing.T) {
	// Every mount point is a key, so keys have to go when nothing is in flight.
	paths := make([]string, 50)
	for i := range paths {
		paths[i] = fmt.Sprintf("/jfs/settled-%d", i)
		if err := DoPathWithTimeout(context.Background(), time.Second, paths[i], func(context.Context) error { return nil }); err != nil {
			t.Fatalf("%s: %v", paths[i], err)
		}
	}

	inflight.Lock()
	defer inflight.Unlock()
	for _, path := range paths {
		if n, ok := inflight.n[path]; ok {
			t.Errorf("%s still tracked with %d in flight", path, n)
		}
	}
}

func TestDoWithTimeoutReturnsErrFunctionTimeout(t *testing.T) {
	// pkg/controller/mountinfo matches this error to decide a target is stuck.
	release := make(chan struct{})
	defer close(release)

	err := DoWithTimeout(context.Background(), 10*time.Millisecond, func(context.Context) error {
		<-release
		return nil
	})
	if !errors.Is(err, ErrFunctionTimeout) {
		t.Errorf("got %v, want ErrFunctionTimeout", err)
	}

	// Operators grep logs for this message; no code compares it any more.
	if got := ErrFunctionTimeout.Error(); got != "function timeout" {
		t.Errorf("message is %q, want the original", got)
	}
}

func TestDoPathWithTimeoutRefusalIsDistinguishable(t *testing.T) {
	// Logs have to tell a timeout apart from a call that never started.
	release := make(chan struct{})
	defer close(release)
	var started atomic.Int64

	const path = "/jfs/refusal-distinguishable"
	for i := 0; i < maxInflightPerPath; i++ {
		_ = DoPathWithTimeout(context.Background(), 10*time.Millisecond, path, blockingCall(&started, release))
	}
	waitFor(t, "the budget to fill", func() bool { return started.Load() >= maxInflightPerPath })

	refused := DoPathWithTimeout(context.Background(), time.Second, path, func(context.Context) error { return nil })
	expired := DoWithTimeout(context.Background(), 10*time.Millisecond, func(context.Context) error {
		<-release
		return nil
	})

	// Both wrap ErrFunctionTimeout so callers branching on it still work.
	if !errors.Is(refused, ErrFunctionTimeout) {
		t.Errorf("refusal: got %v, want it to wrap ErrFunctionTimeout", refused)
	}
	if !errors.Is(expired, ErrFunctionTimeout) {
		t.Errorf("expiry: got %v, want ErrFunctionTimeout", expired)
	}

	if !errors.Is(refused, ErrTooManyInflight) {
		t.Errorf("refusal: got %v, want ErrTooManyInflight", refused)
	}
	if errors.Is(expired, ErrTooManyInflight) {
		t.Errorf("expiry: got %v, want it not to be ErrTooManyInflight", expired)
	}
	if refused.Error() == expired.Error() {
		t.Errorf("both read %q, so the cap is invisible in logs", refused.Error())
	}

	// %w keeps the original message at the front so log greps still match.
	if !strings.HasPrefix(refused.Error(), ErrFunctionTimeout.Error()) {
		t.Errorf("refusal reads %q, want it to start with %q",
			refused.Error(), ErrFunctionTimeout.Error())
	}
}
