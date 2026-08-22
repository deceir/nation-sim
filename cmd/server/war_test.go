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

func TestAutomaticDefenseCommitment(t *testing.T) {
	cases := []struct {
		available int64
		percent   int
		want      int64
	}{
		{1000, 60, 600},
		{7, 60, 4},
		{1000, 0, 0},
		{1000, 100, 1000},
		{1000, -10, 0},
		{1000, 120, 1000},
	}
	for _, tc := range cases {
		if got := automaticDefenseCommitment(tc.available, tc.percent); got != tc.want {
			t.Fatalf("automaticDefenseCommitment(%d,%d)=%d; want %d", tc.available, tc.percent, got, tc.want)
		}
	}
}

func TestInitialMobilizationRoundsDoNotResolveCombat(t *testing.T) {
	cases := []struct {
		round, mobilization int
		pending             bool
	}{
		{1, 1, false},
		{1, 2, true},
		{1, 5, true},
		{4, 5, true},
		{5, 5, false},
	}
	for _, tc := range cases {
		if got := initialMobilizationPending(tc.round, tc.mobilization); got != tc.pending {
			t.Fatalf("initialMobilizationPending(%d,%d)=%v; want %v", tc.round, tc.mobilization, got, tc.pending)
		}
	}
}

func TestWarInfrastructureDamageIsOutcomeScaledAndCapped(t *testing.T) {
	cases := []struct {
		outcome         string
		targeted        bool
		strategicRounds int
		want            float64
	}{
		{"minor", false, 0, .0075},
		{"major", false, 8, .015},
		{"decisive", false, 20, .025},
		{"minor", true, 1, .02},
		{"major", true, 0, .0275},
		{"decisive", true, 6, .06},
	}
	for _, tc := range cases {
		if got := warInfrastructureDamageRate(tc.outcome, tc.targeted, tc.strategicRounds); math.Abs(got-tc.want) > 0.000001 {
			t.Fatalf("damage rate for %s targeted=%v strikes=%d was %.4f; want %.4f", tc.outcome, tc.targeted, tc.strategicRounds, got, tc.want)
		}
	}
}

func TestWarInstitutionDamageIsSeparateAndBounded(t *testing.T) {
	if got := warInstitutionDestructionChance("minor", false, 0); math.Abs(got-.004) > 0.000001 {
		t.Fatalf("minor institution risk = %.4f; want .004", got)
	}
	if got := warInstitutionDestructionChance("decisive", true, 100); math.Abs(got-warInstitutionRiskCap) > 0.000001 {
		t.Fatalf("institution risk = %.4f; want cap %.4f", got, warInstitutionRiskCap)
	}
	if got := deterministicInstitutionLosses("war", "province", "school", 12, 0); got != 0 {
		t.Fatalf("zero-risk damage destroyed %d institutions", got)
	}
	if got := deterministicInstitutionLosses("war", "province", "school", 12, 1); got != 12 {
		t.Fatalf("certain damage destroyed %d institutions; want 12", got)
	}
	first := deterministicWarDamageRoll("war", "province", "school", 0)
	second := deterministicWarDamageRoll("war", "province", "school", 0)
	if first != second || first < 0 || first >= 1 {
		t.Fatalf("deterministic damage roll invalid: %f then %f", first, second)
	}
}
