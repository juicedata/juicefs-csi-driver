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

package k8sclient

import (
	"errors"
	"sync"
	"testing"
)

// TestCheckAccessErrPerClientIsolation locks in that the failure streak is
// per-client. Before the fix a single package-global counter was shared by the
// reconciler client and the mount client, so one client's success reset the
// other's accumulated failures (and vice versa). Here client A must reach its
// own limit even though client B reports a success in between, and B must never
// exit because of A's failures.
func TestCheckAccessErrPerClientIsolation(t *testing.T) {
	fail := errors.New("kubelet unreachable")
	var aExits, bExits int
	a := &KubeletClient{exitFunc: func(int) { aExits++ }}
	b := &KubeletClient{exitFunc: func(int) { bExits++ }}

	// A accumulates failures up to one short of the limit.
	for i := 0; i < kubeletAccessErrMax-1; i++ {
		a.checkAccessErr(fail)
	}
	// A success on B must not touch A's streak.
	b.checkAccessErr(nil)
	if aExits != 0 {
		t.Fatalf("A exited before reaching the limit: aExits=%d", aExits)
	}

	// A's next failure crosses its own limit and exits A only.
	a.checkAccessErr(fail)
	if aExits == 0 {
		t.Fatalf("A did not exit after %d consecutive failures", kubeletAccessErrMax)
	}
	if bExits != 0 {
		t.Fatalf("B exited because of A's failures: bExits=%d", bExits)
	}
}

// TestCheckAccessErrResetsOnSuccess documents that a success clears the streak,
// so a single client below the limit never exits no matter how many isolated
// failures it has seen between successes.
func TestCheckAccessErrResetsOnSuccess(t *testing.T) {
	fail := errors.New("kubelet unreachable")
	var exits int
	kc := &KubeletClient{exitFunc: func(int) { exits++ }}

	for i := 0; i < kubeletAccessErrMax*3; i++ {
		kc.checkAccessErr(fail) // one failure...
		kc.checkAccessErr(nil)  // ...immediately cleared
	}
	if exits != 0 {
		t.Fatalf("client exited despite every failure being reset: exits=%d", exits)
	}
}

// TestCheckAccessErrDataRace drives one client's counter from several goroutines
// at once, mirroring the concurrent CSI mount handlers that share a single
// KubeletClient. exitFunc is a no-op so the probe never terminates the process.
// With the per-client atomic counter this passes cleanly under `go test -race`;
// reverting the counter to a plain int makes the race detector fail here.
func TestCheckAccessErrDataRace(t *testing.T) {
	fail := errors.New("kubelet unreachable")
	kc := &KubeletClient{exitFunc: func(int) {}}

	const goroutines = 4
	const iterations = 2000

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				kc.checkAccessErr(fail) // failure branch: increment
				kc.checkAccessErr(nil)  // success branch: reset
			}
		}()
	}
	wg.Wait()
}
