package main

import (
	"math"
	"testing"
	"time"
)

func TestNextWarRoundUsesThreeHourUTCWindows(t *testing.T) {
	cases := []struct {
		at, want string
	}{
		{"2026-08-22T00:00:00Z", "2026-08-22T03:00:00Z"},
		{"2026-08-22T02:59:00Z", "2026-08-22T03:00:00Z"},
		{"2026-08-22T21:45:00Z", "2026-08-23T00:00:00Z"},
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

func TestTwoFrontTheatersAndFasterMobilization(t *testing.T) {
	home, foreign := warTheatersForNation("attacker", "attacker")
	if home != attackerHomelandTheater || foreign != defenderHomelandTheater {
		t.Fatalf("attacker theaters = %s/%s", home, foreign)
	}
	home, foreign = warTheatersForNation("defender", "attacker")
	if home != defenderHomelandTheater || foreign != attackerHomelandTheater {
		t.Fatalf("defender theaters = %s/%s", home, foreign)
	}
	if warMobilizationRounds(0) != 1 || warMobilizationRounds(9000) != 2 || warMobilizationRounds(20000) != 4 || warMobilizationRounds(40000) != 4 {
		t.Fatal("expeditionary mobilization should scale from one to four rounds")
	}
}

func TestWarObjectivesHaveMechanics(t *testing.T) {
	for key, objective := range warObjectives {
		if objective.Effect == "" {
			t.Fatalf("objective %s has no displayed mechanical effect", key)
		}
	}
	if operationMultiplier("ground_assault", "soldiers", "land", "military_suppression") <= operationMultiplier("ground_assault", "soldiers", "land", "territorial_pressure") {
		t.Fatal("military suppression should improve applicable combat operations")
	}
	if !combinedArms(map[string]int64{"soldiers": 100, "tanks": 2, "jets": 1}) || combinedArms(map[string]int64{"soldiers": 100, "tanks": 2}) {
		t.Fatal("territorial combined-arms requirement should require three fielded unit types")
	}
}

func TestTwoFrontDamageIsEarnedAndBounded(t *testing.T) {
	infra, civic := warAccumulatedDamageRates(0, "")
	if infra != 0 || civic != 0 {
		t.Fatalf("no foreign pressure should cause no damage: %.4f/%.4f", infra, civic)
	}
	infra, civic = warAccumulatedDamageRates(3, "")
	if infra <= 0 || civic <= 0 {
		t.Fatalf("successful foreign pressure should threaten both infrastructure and institutions: %.4f/%.4f", infra, civic)
	}
	infra, civic = warAccumulatedDamageRates(100, "decisive")
	if infra != .06 || civic != warInstitutionRiskCap {
		t.Fatalf("war damage exceeded its recovery-safe caps: %.4f/%.4f", infra, civic)
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

func TestWarInstitutionDamageIsSeparateAndBounded(t *testing.T) {
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

func TestWarDeploymentArrivalUsesScheduledRoundWindows(t *testing.T) {
	next := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	if got := warDeploymentArrival(next, 2, 3); !got.Equal(next) {
		t.Fatalf("next-round deployment arrives at %s; want %s", got, next)
	}
	if got := warDeploymentArrival(next, 2, 5); !got.Equal(next.Add(6 * time.Hour)) {
		t.Fatalf("round-five deployment arrives at %s; want %s", got, next.Add(6*time.Hour))
	}
}
