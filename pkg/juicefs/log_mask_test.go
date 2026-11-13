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

package juicefs

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"k8s.io/klog/v2"
	k8sexec "k8s.io/utils/exec"
	testingexec "k8s.io/utils/exec/testing"
	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

// captureLog sends klog's output to a buffer for the rest of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	state := klog.CaptureState()
	t.Cleanup(state.Restore)
	klog.LogToStderr(false)
	klog.SetOutput(&buf)
	return &buf
}

func assertMasked(t *testing.T, line string, kept []string, hidden []string) {
	t.Helper()
	for _, s := range kept {
		if !strings.Contains(line, s) {
			t.Errorf("log line %q lacks %q", line, s)
		}
	}
	for _, s := range hidden {
		if strings.Contains(line, s) {
			t.Errorf("log line %q shows %q", line, s)
		}
	}
}

func logLine(t *testing.T, buf *bytes.Buffer, msg string) string {
	t.Helper()
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, msg) {
			return line
		}
	}
	t.Fatalf("no %q line in the log:\n%s", msg, buf.String())
	return ""
}

func TestSettingsLogsFormatCommandWithoutCredentials(t *testing.T) {
	tests := []struct {
		name    string
		secrets map[string]string
		kept    []string
		hidden  []string
	}{
		{
			name: "community edition",
			secrets: map[string]string{
				"name":           "vol",
				"metaurl":        "redis://:metapass@redis.example.com:6379/1",
				"storage":        "s3",
				"bucket":         "https://user:bucket@pass@bucket.example.com",
				"access-key":     "AKEXAMPLE",
				"secret-key":     "SKEXAMPLE",
				"format-options": "session-token=STEXAMPLE,trash-days=1",
			},
			kept:   []string{"--access-key=***", "--secret-key=${secretkey}", "--bucket=https://***@bucket.example.com", "${metaurl}", "--trash-days=1"},
			hidden: []string{"AKEXAMPLE", "SKEXAMPLE", "bucket@pass", "metapass", "STEXAMPLE"},
		},
		{
			name: "enterprise edition",
			secrets: map[string]string{
				"name":           "vol",
				"token":          "TKEXAMPLE",
				"bucket":         "https://bucket.example.com",
				"access-key":     "AKEXAMPLE",
				"secret-key":     "SKEXAMPLE",
				"format-options": "passphrase=PPEXAMPLE,session-token2=ST2EXAMPLE",
			},
			kept:   []string{"auth vol", "--access-key=***", "--token=${token}", "--secret-key=${secretkey}", "--passphrase=***", "--session-token2=***"},
			hidden: []string{"AKEXAMPLE", "SKEXAMPLE", "TKEXAMPLE", "PPEXAMPLE", "ST2EXAMPLE"},
		},
	}
	// Settings only builds the command: process mode runs it.
	byProcess := config.ByProcess
	config.ByProcess = false
	t.Cleanup(func() { config.ByProcess = byProcess })
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLog(t)
			j := &juicefs{}
			if _, err := j.Settings(context.TODO(), "vol", "vol", "vol-uuid", tt.secrets, map[string]string{}, nil); err != nil {
				t.Fatalf("Settings() error = %v", err)
			}
			klog.Flush()
			assertMasked(t, logLine(t, buf, "Format/Auth command"), tt.kept, tt.hidden)
		})
	}
}

func TestProcessModeLogsFormatAndAuthCommandsWithoutCredentials(t *testing.T) {
	byProcess := config.ByProcess
	config.ByProcess = true
	t.Cleanup(func() { config.ByProcess = byProcess })
	// With JFS_NO_UPDATE_CONFIG=enabled, auth also needs a bucket.
	t.Setenv("JFS_NO_UPDATE_CONFIG", "")
	// Each command the test runs succeeds without running anything.
	succeed := func(cmd string, args ...string) k8sexec.Cmd {
		return testingexec.InitFakeCmd(&testingexec.FakeCmd{CombinedOutputScript: []testingexec.FakeAction{
			func() ([]byte, []byte, error) { return nil, nil, nil },
		}}, cmd, args...)
	}
	j := &juicefs{SafeFormatAndMount: mount.SafeFormatAndMount{Exec: &testingexec.FakeExec{
		CommandScript: []testingexec.FakeCommandAction{succeed, succeed},
	}}}

	t.Run("community edition format", func(t *testing.T) {
		buf := captureLog(t)
		secrets := map[string]string{"name": "vol", "metaurl": "redis://redis.example.com:6379/1", "storage": "s3",
			"bucket": "https://bucket.example.com", "access-key": "AKEXAMPLE", "secret-key": "SKEXAMPLE"}
		if _, err := j.ceFormat(context.TODO(), secrets, false, &config.JfsSetting{}); err != nil {
			t.Fatalf("ceFormat() error = %v", err)
		}
		klog.Flush()
		assertMasked(t, logLine(t, buf, "ce format cmd"), []string{"--access-key=***", "--secret-key=${secretkey}"},
			[]string{"AKEXAMPLE", "SKEXAMPLE"})
	})

	t.Run("enterprise edition auth", func(t *testing.T) {
		buf := captureLog(t)
		secrets := map[string]string{"name": "vol", "token": "TKEXAMPLE", "access-key": "AKEXAMPLE", "secret-key": "SKEXAMPLE"}
		if _, err := j.AuthFs(context.TODO(), secrets, &config.JfsSetting{}, true); err != nil {
			t.Fatalf("AuthFs() error = %v", err)
		}
		klog.Flush()
		assertMasked(t, logLine(t, buf, "AuthFs cmd"), []string{"--access-key=***", "--token=${token}"},
			[]string{"AKEXAMPLE", "SKEXAMPLE", "TKEXAMPLE"})
	})
}
