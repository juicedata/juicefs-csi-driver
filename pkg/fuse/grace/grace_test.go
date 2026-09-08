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

package grace

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

func Test_parseRequest(t *testing.T) {
	type args struct {
		message string
	}
	tests := []struct {
		name string
		args args
		want upgradeRequest
	}{
		{
			name: "pod",
			args: args{
				message: "juicefs-xxxx recreate",
			},
			want: upgradeRequest{
				action: "recreate",
				name:   "juicefs-xxxx",
			},
		},
		{
			name: "pod",
			args: args{
				message: "juicefs-xxxx",
			},
			want: upgradeRequest{
				action: noRecreate,
				name:   "juicefs-xxxx",
			},
		},
		{
			name: "batch",
			args: args{
				message: fmt.Sprintf("BATCH %s batchConfig=test,batchIndex=1,timeout=45s", recreate),
			},
			want: upgradeRequest{
				action:     recreate,
				name:       "BATCH",
				configName: "test",
				batchIndex: 1,
				timeout:    45 * time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRequest(tt.args.message); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_resolvePpid(t *testing.T) {
	tests := []struct {
		name     string
		ppid     int
		commPath string
		hostPID  bool
		wantPpid int
		wantErr  bool
	}{
		{
			name:     "PPid field is used directly",
			ppid:     3551359,
			commPath: "",
			hostPID:  true,
			wantPpid: 3551359,
		},
		{
			name:     "CommPath suffix parsed when PPid is zero",
			ppid:     0,
			commPath: "/tmp/fuse_fd_comm.3551359",
			hostPID:  true,
			wantPpid: 3551359,
		},
		{
			name:     "CommPath with dotted directory does not mislead parser",
			ppid:     0,
			commPath: "/tmp/dir.99/fuse_fd_comm.1234",
			hostPID:  true,
			wantPpid: 1234,
		},
		{
			name:     "non-HostPID falls back to 1 when both fields are absent",
			ppid:     0,
			commPath: "",
			hostPID:  false,
			wantPpid: 1,
		},
		{
			name:     "non-HostPID falls back to 1 when CommPath has no parseable suffix",
			ppid:     0,
			commPath: "/tmp/fuse_fd_comm",
			hostPID:  false,
			wantPpid: 1,
		},
		{
			name:     "HostPID errors when neither field is parseable",
			ppid:     0,
			commPath: "",
			hostPID:  true,
			wantErr:  true,
		},
		{
			name:     "HostPID errors when CommPath suffix is not a valid number",
			ppid:     0,
			commPath: "/tmp/fuse_fd_comm.abc",
			hostPID:  true,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := &util.JuiceConf{PPid: tt.ppid, CommPath: tt.commPath}
			got, err := resolvePpid(conf, tt.hostPID)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolvePpid() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantPpid {
				t.Errorf("resolvePpid() = %d, want %d", got, tt.wantPpid)
			}
		})
	}
}

type fakeGraceRunner struct {
	failed  bool
	jfsConf *util.JuiceConf
	err     error
}

func (f *fakeGraceRunner) StatusPrefix() string { return "POD" }
func (f *fakeGraceRunner) TargetName() string   { return "demo" }
func (f *fakeGraceRunner) LockKey() string      { return "lock-key" }
func (f *fakeGraceRunner) PrepareShutdown(context.Context) (*util.JuiceConf, error) {
	return f.jfsConf, f.err
}
func (f *fakeGraceRunner) Sighup(context.Context, *util.JuiceConf) error { return nil }
func (f *fakeGraceRunner) OnFail()                                       { f.failed = true }

func TestGraceUpgradeRunGracefulUpgradeCallsOnFail(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	done := make(chan struct{})
	go func() {
		_, _ = bufio.NewReader(serverConn).ReadString('\n')
		close(done)
	}()

	helper := &GraceUpgrade{conn: clientConn}
	runner := &fakeGraceRunner{err: fmt.Errorf("prepare failed")}
	if err := helper.runGracefulUpgrade(context.Background(), runner); err == nil {
		t.Fatal("expected error")
	}
	if !runner.failed {
		t.Fatal("expected OnFail to be called")
	}
	<-done
}

func TestGraceUpgradeRunGracefulUpgradeSkipsEmptySidecarTargetImage(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	message := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(serverConn).ReadString('\n')
		message <- line
	}()

	helper := &GraceUpgrade{conn: clientConn}
	runner := &fakeGraceRunner{
		jfsConf: nil,
	}
	if err := helper.runGracefulUpgrade(context.Background(), runner); err != nil {
		t.Fatalf("expected skip without error, got %v", err)
	}
	select {
	case got := <-message:
		t.Fatalf("unexpected message from generic runner: %q", got)
	default:
	}
}

func TestGraceUpgradeSendMessageFallbackToStdoutWhenConnNil(t *testing.T) {
	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdout = w

	helper := &GraceUpgrade{conn: nil}
	helper.sendMessage("fallback-message")

	_ = w.Close()
	os.Stdout = originalStdout
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(output), "fallback-message") {
		t.Fatalf("expected fallback message in stdout, got %q", string(output))
	}
}

func TestSidecarBinaryTarCommands(t *testing.T) {
	tests := []struct {
		name string
		ce   bool
		want string
	}{
		{
			name: "ce",
			ce:   true,
			want: "tar cf - -C /usr/local/bin juicefs",
		},
		{
			name: "ee",
			ce:   false,
			want: "tar cf - -C /usr/bin juicefs -C /usr/local/juicefs/mount jfsmount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sidecarBinaryTarCommand(tt.ce)
			if got != tt.want {
				t.Fatalf("sidecarBinaryTarCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWaitForCanaryPodRunningTimeoutIncludesPodStatus(t *testing.T) {
	originalNamespace := config.Namespace
	config.Namespace = "kube-system"
	t.Cleanup(func() {
		config.Namespace = originalNamespace
	})

	client := &k8sclient.K8sClient{Interface: fake.NewSimpleClientset(
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "canary-job", Namespace: config.Namespace},
			Status:     batchv1.JobStatus{Active: 1},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "canary-pod",
				Namespace: config.Namespace,
				Labels: map[string]string{
					common.CanaryJobLabelKey: "canary-job",
				},
			},
			Status: corev1.PodStatus{Phase: corev1.PodPending},
		},
	)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := waitForCanaryPodRunning(ctx, client, "canary-job")
	assert.ErrorContains(t, err, "timeout waiting for canary job canary-job to run")
}

func TestCanaryPodErrorUsesResourcePredicate(t *testing.T) {
	pod := corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodUnknown}}
	assert.True(t, resource.IsPodError(&pod))
}

func TestSidecarCanaryJobNameUsesPodCanaryRule(t *testing.T) {
	t.Run("with pod and container", func(t *testing.T) {
		target := SidecarUpgradeTarget{
			PodName:       "app-pod-1",
			ContainerName: "jfs-sidecar",
		}
		got := sidecarCanaryJobName(target)
		assertSidecarCanaryName(t, got)
	})

	t.Run("with only container name", func(t *testing.T) {
		target := SidecarUpgradeTarget{
			ContainerName: "jfs-sidecar",
		}
		got := sidecarCanaryJobName(target)
		assertSidecarCanaryName(t, got)
	})

	t.Run("name should change between upgrades", func(t *testing.T) {
		target := SidecarUpgradeTarget{
			PodName:       "app-pod-1",
			ContainerName: "jfs-sidecar",
		}
		first := sidecarCanaryJobName(target)
		second := sidecarCanaryJobName(target)
		if first == second {
			t.Fatalf("sidecarCanaryJobName() should generate unique name, got same value %q", first)
		}
	})
}

func TestSidecarTargetNameUsesPodName(t *testing.T) {
	runner := &SidecarUpgradeRunner{
		target: SidecarUpgradeTarget{
			PodName:       "app-pod-1",
			ContainerName: "jfs-mount",
		},
	}
	if got, want := runner.TargetName(), "app-pod-1/jfs-mount"; got != want {
		t.Fatalf("TargetName() = %q, want %q", got, want)
	}
}

func assertSidecarCanaryName(t *testing.T, got string) {
	t.Helper()
	if !strings.HasSuffix(got, "-canary") {
		t.Fatalf("sidecarCanaryJobName() = %q, want suffix %q", got, "-canary")
	}
	if strings.Contains(got, "-canary-") {
		t.Fatalf("sidecarCanaryJobName() = %q, random suffix should be in base, not after -canary", got)
	}
	if !strings.HasPrefix(got, "juicefs-") {
		t.Fatalf("sidecarCanaryJobName() = %q, want prefix %q", got, "juicefs-")
	}
}

func TestParseJuiceFSVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "sidecar log line",
			in:   `2026/09/08 07:22:48.490190 juicefs[157] <INFO>: JuiceFS version 5.4.2 (2026-09-08 02e2ef7c8) [mount@mount.go:788]`,
			want: "5.4.2",
		},
		{
			name: "canary version output",
			in:   "juicefs version 5.4.2 (2026-09-08 02e2ef7c8)\n",
			want: "5.4.2",
		},
		{
			name: "ee sidecar log line",
			in:   `2026/09/08 08:32:05.118388 juicefs[586] <INFO>: JuiceFS version 5.1.13 (2025-03-04 381ce32) [mount.go:650]`,
			want: "5.1.13",
		},
		{
			name: "ee version output",
			in:   "juicefs version 5.1.13 (2025-03-04 381ce32)\n",
			want: "5.1.13",
		},
		{
			name: "ce version output with platform suffix",
			in:   "juicefs version 1.4.0+2026-07-06.62bedf3c (linux/amd64)\n",
			want: "1.4.0+2026-07-06.62bedf3c",
		},
		{
			name: "no version",
			in:   "some other output",
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, parseJuiceFSVersion(c.in))
		})
	}
}

func TestEvaluateSidecarRestartLog(t *testing.T) {
	busyLog := `2026/09/08 07:22:48 FUSE session is still busy (0 readers, 2 requests, 0 writers) after 10 seconds, give up
2026/09/08 07:22:48.181622 juicefs[57] <WARNING>: FUSE session is busy, don't restart [installHandler@mount_unix.go:1262]`
	restartedLog := `2026/09/08 07:22:48.490190 juicefs[157] <INFO>: JuiceFS version 5.4.2 (2026-09-08 02e2ef7c8) [mount@mount.go:788]`

	t.Run("busy fails", func(t *testing.T) {
		done, err := evaluateSidecarRestartLog(busyLog, "5.4.2")
		assert.False(t, done)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "FUSE session is busy")
	})

	t.Run("matched version succeeds", func(t *testing.T) {
		done, err := evaluateSidecarRestartLog(restartedLog, "5.4.2")
		assert.NoError(t, err)
		assert.True(t, done)
	})

	t.Run("mismatched version keeps waiting", func(t *testing.T) {
		done, err := evaluateSidecarRestartLog(restartedLog, "5.4.3")
		assert.NoError(t, err)
		assert.False(t, done)
	})

	t.Run("empty expected version succeeds on marker", func(t *testing.T) {
		done, err := evaluateSidecarRestartLog(restartedLog, "")
		assert.NoError(t, err)
		assert.True(t, done)
	})

	t.Run("no marker keeps waiting", func(t *testing.T) {
		done, err := evaluateSidecarRestartLog("2026/09/08 07:22:39 try to restart gracefully", "5.4.2")
		assert.NoError(t, err)
		assert.False(t, done)
	})
}

func TestCanaryVersionCommand(t *testing.T) {
	assert.Equal(t, "/usr/local/bin/juicefs --version", canaryVersionCommand(true))
	assert.Equal(t, "/usr/bin/juicefs --version", canaryVersionCommand(false))
}

func TestEvaluateSidecarRestartLogOnlyAfterLastSighup(t *testing.T) {
	logs := `2026/09/08 07:20:00.000000 juicefs[10] <INFO>: JuiceFS version 5.4.2 (2026-09-08 02e2ef7c8) [mount@mount.go:788]
2026/09/08 07:22:38.172540 juicefs[57] <INFO>: received signal hangup [installHandler@mount_unix.go:1255]
2026/09/08 07:22:39 try to restart gracefully`

	done, err := evaluateSidecarRestartLog(logs, "5.4.2")
	assert.NoError(t, err)
	assert.False(t, done, "restart record before the last SIGHUP must be ignored")
}
