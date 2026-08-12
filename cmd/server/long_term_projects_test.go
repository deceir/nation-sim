package main

import "testing"

func TestMilitaryProjectsDoNotUseStandardProjectSlots(t *testing.T) {
	military := []string{"aviation_industry", "naval_shipyard", "armored_vehicle_program", "advanced_ordnance"}
	for _, id := range military {
		project, exists := longTermProjects[id]
		if !exists || project.Category != "military" {
			t.Fatalf("military project %q is missing or miscategorized", id)
		}
		if longTermProjectUsesSlot(id) {
			t.Fatalf("military project %q consumes a standard slot", id)
		}
	}
	for _, id := range []string{"agricultural_expansion", "civil_service_reform"} {
		if !longTermProjectUsesSlot(id) {
			t.Fatalf("standard project %q unexpectedly bypasses slot capacity", id)
		}
	}
}

func TestLongTermProjectSlotProgression(t *testing.T) {
	cases := []struct {
		infra     float64
		provinces int
		want      int
	}{
		{100, 1, 3},
		{1000, 1, 4},
		{1000, 3, 5},
		{2500, 5, 7},
	}
	for _, test := range cases {
		if got := longTermProjectSlots(test.infra, test.provinces); got != test.want {
			t.Fatalf("slots(%.0f Infrastructure, %d Provinces) = %d, want %d", test.infra, test.provinces, got, test.want)
		}
	}
}

func TestLongTermProjectTreasuryCostsUseFortyPercentReduction(t *testing.T) {
	want := int64(4500000) * yenScale * longTermProjectCashPercent / 100
	if got := longTermProjects["agricultural_expansion"].Cash; got != want {
		t.Fatalf("Agricultural Expansion Treasury cost = %d, want %d", got, want)
	}
	if got := longTermProjects["agricultural_expansion"].Costs["foodstuffs"]; got != 960 {
		t.Fatalf("resource cost changed unexpectedly: %.0f Foodstuffs", got)
	}
}

func TestSpecializationProjectsProvideMajorTargetedBoosts(t *testing.T) {
	for id, project := range longTermProjects {
		if project.Category != "specialization" {
			continue
		}
		want := 1.50
		if id == "luxury_goods_guild" {
			want = .80
		}
		if project.ProductionBoost != want {
			t.Fatalf("%s production boost = %.2f, want %.2f", id, project.ProductionBoost, want)
		}
	}
}
