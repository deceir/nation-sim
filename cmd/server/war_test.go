package main

import (
	"math"
	"testing"
	"time"
)

func TestNextWarRoundUsesSixHourUTCWindows(t *testing.T) {
	cases := []struct {
		at, want string
	}{
		{"2026-08-22T00:00:00Z", "2026-08-22T06:00:00Z"},
		{"2026-08-22T05:59:00Z", "2026-08-22T06:00:00Z"},
		{"2026-08-22T18:45:00Z", "2026-08-23T00:00:00Z"},
	}
	for _, tc := range cases {
		at, _ := time.Parse(time.RFC3339, tc.at)
		if got := nextWarRound(at).Format(time.RFC3339); got != tc.want {
			t.Fatalf("nextWarRound(%s) = %s; want %s", tc.at, got, tc.want)
		}
	}
}

func TestMilitaryMobilizationTakesAboutTenDays(t *testing.T) {
	if got := militaryDailyProductionLimit("tanks", 1000); got != 100 {
		t.Fatalf("daily tank limit = %d; want 100", got)
	}
	if got := militaryDailyProductionLimit("ships", 10); got != 2 {
		t.Fatalf("starter ship floor = %d; want 2", got)
	}
	if got := militaryDailyProductionLimit("ships", 1); got != 1 {
		t.Fatalf("daily production may not exceed capacity; got %d", got)
	}
}

func TestHaversineDistance(t *testing.T) {
	// London to Tokyo is approximately 9,560 km. This intentionally accepts a
	// broad band because national pins need only drive strategic game distance.
	got := haversineKM(51.5074, -0.1278, 35.6762, 139.6503)
	if math.Abs(got-9558) > 100 {
		t.Fatalf("unexpected London-Tokyo distance %.1f", got)
	}
}
