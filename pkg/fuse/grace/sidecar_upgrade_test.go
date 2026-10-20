package grace

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
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
	if err := validateRestartedCommand(101, 102, actual, desired, false); err != nil {
		t.Fatalf("validateRestartedCommand() error = %v", err)
	}
	if err := validateRestartedCommand(101, 101, actual, desired, false); err == nil {
		t.Fatal("expected unchanged PID error")
	}
	actual[len(actual)-1] = "foreground"
	if err := validateRestartedCommand(101, 102, actual, desired, false); err == nil {
		t.Fatal("expected argv mismatch error")
	}
}

func TestValidateRestartedCommandLengthMismatchIncludesArgv(t *testing.T) {
	actual := []string{"/usr/local/juicefs/mount/jfsmount", "mount", "volume", "/jfs/path", "-o", "foreground", "--cache-size", "1024"}
	expected := []string{"/sbin/mount.juicefs", "volume", "/jfs/path", "-o", "foreground,cache-size=1024"}

	err := validateRestartedCommand(101, 102, actual, expected, false)
	if err == nil {
		t.Fatal("expected argv length mismatch error")
	}
	want := fmt.Sprintf("mount argv length mismatch: got 8 (actual %q), want 5 (expected %q)", actual, expected)
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

// TestParseProcCmdlineAndValidateRestartedCommand reproduces the production
// failure: the jfs supervisor rewrote /proc/<pid>/cmdline via setproctitle,
// so the old NUL-split parsed it as one space-joined string plus 7 empty
// elements and the strict comparison failed with "got 8, want 5".
func TestParseProcCmdline(t *testing.T) {
	title := "/usr/local/juicefs/mount/jfsmount chaos-test /jfs/9f750d1e-fa8b-429c-b9be-6752a82fb304 -o foreground,no-update,cache-group=cachegroup,no-sharing,cache-size=10,cache-dir=/data/cache,enable-acl,enable-xattr,no-update,subdir=pvc-9f750d1e-fa8b-429c-b9be-6752a"
	argv, rewritten := parseProcCmdline(title + strings.Repeat("\x00", 8))
	if !rewritten {
		t.Fatal("parseProcCmdline() rewritten = false, want true for setproctitle cmdline")
	}
	want := []string{
		"/usr/local/juicefs/mount/jfsmount", "chaos-test", "/jfs/9f750d1e-fa8b-429c-b9be-6752a82fb304", "-o",
		"foreground,no-update,cache-group=cachegroup,no-sharing,cache-size=10,cache-dir=/data/cache,enable-acl,enable-xattr,no-update,subdir=pvc-9f750d1e-fa8b-429c-b9be-6752a",
	}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %#v, want %#v", argv, want)
	}
	if old := strings.Split(strings.TrimSuffix(title+strings.Repeat("\x00", 8), "\x00"), "\x00"); len(old) != 8 {
		t.Fatalf("sanity check failed: old parsing produced %d elements, want 8", len(old))
	}

	argv, rewritten = parseProcCmdline("/sbin/mount.juicefs\x00chaos-test\x00/jfs/mp\x00-o\x00foreground,no-update")
	if rewritten {
		t.Fatal("parseProcCmdline() rewritten = true, want false for NUL-separated cmdline")
	}
	if !reflect.DeepEqual(argv, []string{"/sbin/mount.juicefs", "chaos-test", "/jfs/mp", "-o", "foreground,no-update"}) {
		t.Fatalf("argv = %#v", argv)
	}
}

func TestValidateRestartedCommandRewrittenTitleToleratesTruncation(t *testing.T) {
	expected := []string{
		"/sbin/mount.juicefs", "chaos-test", "/jfs/9f750d1e-fa8b-429c-b9be-6752a82fb304", "-o",
		"foreground,no-update,cache-group=cachegroup,no-sharing,cache-size=10,cache-dir=/data/cache,enable-acl,enable-xattr,no-update,subdir=pvc-9f750d1e-fa8b-429c-b9be-6752a82fb304",
	}
	// The setproctitle title was truncated to the original argv area: the
	// trailing "82fb304" of subdir is cut off.
	truncated := []string{
		"/usr/local/juicefs/mount/jfsmount", "chaos-test", "/jfs/9f750d1e-fa8b-429c-b9be-6752a82fb304", "-o",
		"foreground,no-update,cache-group=cachegroup,no-sharing,cache-size=10,cache-dir=/data/cache,enable-acl,enable-xattr,no-update,subdir=pvc-9f750d1e-fa8b-429c-b9be-6752a",
	}
	if err := validateRestartedCommand(101, 102, truncated, expected, true); err != nil {
		t.Fatalf("truncated setproctitle argv should pass: %v", err)
	}
	if err := validateRestartedCommand(101, 102, expected[:4], expected, true); err != nil {
		t.Fatalf("setproctitle argv with lost trailing args should pass: %v", err)
	}
	if err := validateRestartedCommand(101, 102, truncated, expected, false); err == nil {
		t.Fatal("strict argv comparison should reject truncated argv")
	}
	rolledBack := []string{
		"/usr/local/juicefs/mount/jfsmount", "chaos-test", "/jfs/9f750d1e-fa8b-429c-b9be-6752a82fb304", "-o",
		"foreground,no-update,cache-group=cachegroup,no-sharing,cache-size=1024",
	}
	if err := validateRestartedCommand(101, 102, rolledBack, expected, true); err == nil {
		t.Fatal("setproctitle argv with different args should fail")
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
