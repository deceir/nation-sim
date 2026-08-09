package main

import (
	"reflect"
	"testing"
)

func TestMilitaryCapacityUsesConfiguredCoefficients(t *testing.T) {
	cases := map[string]int64{
		"soldiers": 17000,
		"tanks":    650,
		"ships":    200,
		"jets":     265,
		"drones":   710,
	}
	for unit, want := range cases {
		if got := militaryCapacity(militaryUnits[unit], 100000, 2); got != want {
			t.Fatalf("%s capacity=%d, want %d", unit, got, want)
		}
	}
}

func TestMilitaryMarketQuantitiesMustBeWholeUnits(t *testing.T) {
	if _, ok := militaryTradeQuantity(2.5); ok {
		t.Fatal("fractional military equipment should not be tradeable")
	}
	if quantity, ok := militaryTradeQuantity(3); !ok || quantity != 3 {
		t.Fatalf("whole military quantity rejected: %d %v", quantity, ok)
	}
}

func TestMilitaryProductionGatesAndStartingCosts(t *testing.T) {
	if militaryUnits["soldiers"].Project != "" || militaryUnits["soldiers"].Tradable {
		t.Fatal("soldiers must remain universally producible and unavailable on the market")
	}
	cases := map[string]struct {
		project string
		cash    int64
	}{
		"tanks":  {"armored_vehicle_program", 120000},
		"ships":  {"naval_shipyard", 450000},
		"jets":   {"aviation_industry", 350000},
		"drones": {"advanced_ordnance", 85000},
	}
	for unit, want := range cases {
		spec := militaryUnits[unit]
		if spec.Project != want.project || spec.Cash != want.cash || !spec.Tradable {
			t.Fatalf("unexpected %s configuration: %#v", unit, spec)
		}
	}
}

func TestFighterJetConfigurationAndDisplayOrder(t *testing.T) {
	jet := militaryUnits["jets"]
	if jet.Name != "Fighter Jets" || jet.DailyCash != 800 || jet.DailyEnergy != .3 {
		t.Fatalf("unexpected fighter jet upkeep configuration: %#v", jet)
	}
	wantResources := map[string]float64{"basic_metals": 12, "construction_materials": 8, "energy": 6, "strategic_minerals": 4}
	if !reflect.DeepEqual(jet.Resources, wantResources) {
		t.Fatalf("fighter jet resource costs=%v, want %v", jet.Resources, wantResources)
	}
	wantOrder := []string{"soldiers", "tanks", "ships", "jets", "drones"}
	if got := militaryUnitKeys(); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("military display order=%v, want %v", got, wantOrder)
	}
}

func TestMilitaryProductionRequirementsReferenceRealNationalProjects(t *testing.T) {
	wantProjects := map[string]string{
		"tanks":  "armored_vehicle_program",
		"ships":  "naval_shipyard",
		"jets":   "aviation_industry",
		"drones": "advanced_ordnance",
	}
	for unit, projectID := range wantProjects {
		spec := militaryUnits[unit]
		if spec.Project != projectID {
			t.Fatalf("%s requires %q, want %q", unit, spec.Project, projectID)
		}
		project, exists := longTermProjects[projectID]
		if !exists || project.Category != "military" {
			t.Fatalf("%s requirement does not reference a real military National Project: %q", unit, projectID)
		}
	}
	if militaryUnits["soldiers"].Project != "" {
		t.Fatal("soldiers must not require a National Project")
	}
}
