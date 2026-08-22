package main

import (
	"reflect"
	"testing"
)

func TestMilitaryProjectRequirementTestingSwitch(t *testing.T) {
	t.Setenv("MILITARY_PROJECT_REQUIREMENTS", "")
	if militaryProjectRequirementsEnabled() {
		t.Fatal("project requirements should default off during the testing phase")
	}
	t.Setenv("MILITARY_PROJECT_REQUIREMENTS", "true")
	if !militaryProjectRequirementsEnabled() {
		t.Fatal("setting MILITARY_PROJECT_REQUIREMENTS=true should restore project gates")
	}
}

func TestBotMilitaryRegenerationFloors(t *testing.T) {
	want := map[string]int64{"soldiers": 1000, "tanks": 25, "ships": 5, "jets": 25}
	if !reflect.DeepEqual(botMilitaryFloors, want) {
		t.Fatalf("BOT military floors = %#v; want %#v", botMilitaryFloors, want)
	}
	if _, configured := botMilitaryFloors["drones"]; configured {
		t.Fatal("BOT Drones were not requested and should not receive a regeneration floor")
	}
}

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
	wantResources := map[string]float64{"basic_metals": 6, "construction_materials": 8, "energy": 6, "strategic_minerals": 4, "military_equipment": 12}
	if !reflect.DeepEqual(jet.Resources, wantResources) {
		t.Fatalf("fighter jet resource costs=%v, want %v", jet.Resources, wantResources)
	}
	wantOrder := []string{"soldiers", "tanks", "ships", "jets", "drones"}
	if got := militaryUnitKeys(); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("military display order=%v, want %v", got, wantOrder)
	}
}

func TestMilitaryEquipmentConstructionProfiles(t *testing.T) {
	for _, unit := range []string{"tanks", "ships", "jets", "drones"} {
		if militaryUnits[unit].Resources["military_equipment"] <= 0 {
			t.Fatalf("%s construction should consume Military Equipment", unit)
		}
	}
	if militaryUnits["tanks"].Resources["basic_metals"] <= militaryUnits["jets"].Resources["basic_metals"] {
		t.Fatal("tanks should be more Basic Metals-intensive than Fighter Jets")
	}
	if militaryUnits["tanks"].Resources["military_equipment"] >= militaryUnits["jets"].Resources["military_equipment"] {
		t.Fatal("Fighter Jets should be more Military Equipment-intensive than tanks")
	}
	if militaryUnits["drones"].Resources["military_equipment"] < 8 || militaryUnits["drones"].Resources["strategic_minerals"] < 7 {
		t.Fatal("drones should be expensive in both Military Equipment and Strategic Minerals")
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
