package main

import "testing"

func TestContinentAt(t *testing.T) {
	cases := []struct {
		lat, lng float64
		want     string
	}{{50.2, 123.7, "Asia"}, {48.8, 2.3, "Europe"}, {-30.4, 153.2, "Oceania"}, {-23, -49.6, "South America"}, {1.8, 34.8, "Africa"}, {44.2, -78, "North America"}}
	for _, tc := range cases {
		got, ok := continentAt(tc.lat, tc.lng)
		if !ok || got != tc.want {
			t.Fatalf("%.1f,%.1f: expected %s, got %s", tc.lat, tc.lng, tc.want, got)
		}
	}
	if _, ok := continentAt(0, -140); ok {
		t.Fatal("ocean position should be rejected")
	}
	for _, ocean := range []geoPoint{{-30, 30}, {135, 20}, {15, 40}} {
		if _, ok := continentAt(ocean.Lat, ocean.Lng); ok {
			t.Fatalf("ocean position inside a former coarse continent polygon should be rejected: %.1f, %.1f", ocean.Lat, ocean.Lng)
		}
	}
}
