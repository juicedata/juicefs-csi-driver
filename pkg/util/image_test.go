package util

import "testing"

func TestImageSupportSidecarRestartArgs(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  bool
	}{
		{name: "below minimum", image: "juicedata/mount:ee-5.4.2"},
		{name: "minimum", image: "juicedata/mount:ee-5.4.3", want: true},
		{name: "minimum with build suffix", image: "juicedata/mount:ee-5.4.3-abcdef", want: true},
		{name: "higher version", image: "juicedata/mount:ee-5.5.0", want: true},
		{name: "registry port", image: "registry.example.com:5000/juicedata/mount:ee-5.4.3", want: true},
		{name: "digest suffix", image: "juicedata/mount:ee-5.4.3@sha256:0123456789abcdef", want: true},
		{name: "nightly", image: "juicedata/mount:ee-nightly", want: true},
		{name: "development tag", image: "juicedata/mount:2c5ca56", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ImageSupportSidecarRestartArgs(tt.image); got != tt.want {
				t.Fatalf("ImageSupportSidecarRestartArgs(%q) = %v, want %v", tt.image, got, tt.want)
			}
		})
	}
}
