package main

import "testing"

func TestConflictPageValue(t *testing.T) {
	cases := []struct {
		raw               string
		fallback, maximum int
		want              int
	}{
		{"", 10, 50, 10},
		{"0", 10, 50, 10},
		{"12", 10, 50, 12},
		{"500", 10, 50, 50},
		{"invalid", 10, 50, 10},
	}
	for _, tc := range cases {
		if got := conflictPageValue(tc.raw, tc.fallback, tc.maximum); got != tc.want {
			t.Fatalf("conflictPageValue(%q,%d,%d)=%d; want %d", tc.raw, tc.fallback, tc.maximum, got, tc.want)
		}
	}
}
