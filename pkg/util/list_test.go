package util

import "testing"

func TestEqualStringLists(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{name: "identical", a: []string{"a", "b"}, b: []string{"a", "b"}, want: true},
		{name: "reordered", a: []string{"a", "b", "c"}, b: []string{"c", "a", "b"}, want: true},
		{name: "different length", a: []string{"a", "b"}, b: []string{"a"}, want: false},
		{name: "different elements", a: []string{"a", "b"}, b: []string{"a", "c"}, want: false},
		{name: "duplicate count matters", a: []string{"a", "a", "b"}, b: []string{"a", "b", "b"}, want: false},
		{name: "same duplicates", a: []string{"a", "a", "b"}, b: []string{"b", "a", "a"}, want: true},
		{name: "both empty", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EqualStringLists(tt.a, tt.b); got != tt.want {
				t.Fatalf("EqualStringLists(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
