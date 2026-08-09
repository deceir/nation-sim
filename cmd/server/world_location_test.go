package main

import "testing"

func TestContinentAt(t *testing.T) {
	cases := []struct {
		lat, lng float64
		want     string
	}{{35.7, 139.7, "Asia"}, {48.8, 2.3, "Europe"}, {-33.9, 151.2, "Oceania"}, {-23.5, -46.6, "South America"}, {1.3, 36.8, "Africa"}, {40.7, -74, "North America"}}
	for _, tc := range cases {
		got, ok := continentAt(tc.lat, tc.lng)
		if !ok || got != tc.want {
			t.Fatalf("%.1f,%.1f: expected %s, got %s", tc.lat, tc.lng, tc.want, got)
		}
	}
	if _, ok := continentAt(0, -140); ok {
		t.Fatal("ocean position should be rejected")
	}
}
