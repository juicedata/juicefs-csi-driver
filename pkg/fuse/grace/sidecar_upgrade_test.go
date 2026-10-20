package grace

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestSupportsSidecarRestartArgs(t *testing.T) {
	for _, tt := range []struct {
		version string
		want    bool
	}{
		{version: "5.4.2"},
		{version: "5.4.3", want: true},
		{version: "5.4.3 (2026-09-09 abcdef)", want: true},
		{version: "5.4.4", want: true},
		{version: "6.0.0", want: true},
		{version: "unknown"},
		{version: ""},
	} {
		t.Run(tt.version, func(t *testing.T) {
			if got := supportsSidecarRestartArgs(tt.version); got != tt.want {
				t.Fatalf("supportsSidecarRestartArgs(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestSupportsSidecarRestartArgsRequiresTargetAndSupervisor(t *testing.T) {
	for _, tt := range []struct {
		name               string
		target, supervisor string
		want               bool
	}{
		{name: "both supported", target: "5.4.3", supervisor: "5.4.3", want: true},
		{name: "old target", target: "5.4.2", supervisor: "5.4.3"},
		{name: "old supervisor after in-place binary replacement", target: "5.4.3", supervisor: "5.3.13"},
		{name: "unknown supervisor", target: "5.4.3"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportsSidecarRestartArgsForProcesses(tt.target, tt.supervisor); got != tt.want {
				t.Fatalf("supportsSidecarRestartArgsForProcesses(%q, %q) = %v, want %v", tt.target, tt.supervisor, got, tt.want)
			}
		})
	}
}

func TestMarshalSidecarRestartArgsPreservesOrderedArgv(t *testing.T) {
	argv := []string{"/sbin/mount.juicefs", "volume name", "/jfs/path", "-o", "cache-dir=/cache dir", ""}
	payload, err := marshalSidecarRestartArgs(argv)
	if err != nil {
		t.Fatalf("marshalSidecarRestartArgs() error = %v", err)
	}
	var got []string
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, argv) {
		t.Fatalf("args = %#v, want %#v", got, argv)
	}
}

func TestValidateRestartedCommand(t *testing.T) {
	desired := []string{"/usr/bin/juicefs", "volume", "/jfs/path", "-o", "foreground,no-update"}
	actual := append([]string{"/usr/local/juicefs/mount/jfsmount"}, desired[1:]...)
	if err := validateRestartedCommand(101, 102, actual, desired); err != nil {
		t.Fatalf("validateRestartedCommand() error = %v", err)
	}
	if err := validateRestartedCommand(101, 101, actual, desired); err == nil {
		t.Fatal("expected unchanged PID error")
	}
	actual[len(actual)-1] = "foreground"
	if err := validateRestartedCommand(101, 102, actual, desired); err == nil {
		t.Fatal("expected argv mismatch error")
	}
}

func TestValidateRestartedCommandLengthMismatchIncludesArgv(t *testing.T) {
	actual := []string{"/usr/local/juicefs/mount/jfsmount", "mount", "volume", "/jfs/path", "-o", "foreground", "--cache-size", "1024"}
	expected := []string{"/sbin/mount.juicefs", "volume", "/jfs/path", "-o", "foreground,cache-size=1024"}

	err := validateRestartedCommand(101, 102, actual, expected)
	if err == nil {
		t.Fatal("expected argv length mismatch error")
	}
	want := fmt.Sprintf("mount argv length mismatch: got 8 (actual %q), want 5 (expected %q)", actual, expected)
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestSidecarRestartArgsPathUsesSupervisorPID(t *testing.T) {
	if got := sidecarRestartArgsPath(42); got != "/tmp/jfs_restart_args.42" {
		t.Fatalf("path = %q", got)
	}
}

func TestRestartArgsWriteCommandIsAtomicAndPrivate(t *testing.T) {
	want := `set -e; umask 077; tmp=/tmp/jfs_restart_args.42.tmp.$$; trap 'rm -f "$tmp"' EXIT; cat >"$tmp"; chmod 600 "$tmp"; mv -f "$tmp" /tmp/jfs_restart_args.42; trap - EXIT`
	if got := restartArgsWriteCommand(42); got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}
