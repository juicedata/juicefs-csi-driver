package sidecar

import (
	"reflect"
	"testing"
)

func TestParseRenderedMountCommand(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   []string
	}{
		{
			name:   "mount command",
			script: "exec /sbin/mount.juicefs vol /jfs/premier-static-2-toynca -o foreground,no-update,cache-size=1,fast-mmap,keep-inode",
			want:   []string{"/sbin/mount.juicefs", "vol", "/jfs/premier-static-2-toynca", "-o", "foreground,no-update,cache-size=1,fast-mmap,keep-inode"},
		},
		{
			name:   "last command after init",
			script: "juicefs auth vol --token=x\n\nexec /sbin/mount.juicefs vol /jfs/path -o foreground,no-update",
			want:   []string{"/sbin/mount.juicefs", "vol", "/jfs/path", "-o", "foreground,no-update"},
		},
		{
			name:   "quotes escapes and concatenated segments",
			script: `exec /sbin/mount.juicefs 'volume name' "/jfs/path with space" -o cache-dir=/cache\ dir,"say \"hi\"",foo' bar'`,
			want:   []string{"/sbin/mount.juicefs", "volume name", "/jfs/path with space", "-o", `cache-dir=/cache dir,say "hi",foo bar`},
		},
		{
			name:   "ansi c quote emitted by builder",
			script: `exec /sbin/mount.juicefs $'vol\'ume\\name' /jfs/path -o foreground,no-update`,
			want:   []string{"/sbin/mount.juicefs", `vol'ume\name`, "/jfs/path", "-o", "foreground,no-update"},
		},
		{
			name:   "double quote preserves non special backslash",
			script: `exec /sbin/mount.juicefs "vol\q" /jfs/path -o foreground,no-update`,
			want:   []string{"/sbin/mount.juicefs", `vol\q`, "/jfs/path", "-o", "foreground,no-update"},
		},
		{
			name:   "empty quoted argument",
			script: `exec /sbin/mount.juicefs '' /jfs/path -o foreground,no-update`,
			want:   []string{"/sbin/mount.juicefs", "", "/jfs/path", "-o", "foreground,no-update"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRenderedMountCommand(tt.script)
			if err != nil {
				t.Fatalf("ParseRenderedMountCommand() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseRenderedMountCommand() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseRenderedMountCommandRejectsInvalidScript(t *testing.T) {
	tests := []string{
		"",
		"/sbin/mount.juicefs vol /jfs/path -o foreground",
		"exec /sbin/mount.juicefs vol",
		"exec /sbin/mount.juicefs vol /jfs/path | cat",
		"exec /sbin/mount.juicefs vol /jfs/path > /tmp/log",
		"exec /sbin/mount.juicefs $(echo vol) /jfs/path",
		"exec /sbin/mount.juicefs 'vol /jfs/path",
		"exec /sbin/mount.juicefs vol /jfs/path\\",
		"exec /bin/not-juicefs vol /jfs/path",
	}

	for _, script := range tests {
		t.Run(script, func(t *testing.T) {
			if got, err := ParseRenderedMountCommand(script); err == nil {
				t.Fatalf("ParseRenderedMountCommand(%q) = %#v, want error", script, got)
			}
		})
	}
}
