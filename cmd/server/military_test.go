package main

import "testing"

func TestMilitaryCapacityUsesConfiguredCoefficients(t *testing.T) {
	cases := map[string]int64{
		"soldiers": 17000,
		"tanks":    650,
		"ships":    200,
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
		"drones": {"advanced_ordnance", 85000},
	}
	for unit, want := range cases {
		spec := militaryUnits[unit]
		if spec.Project != want.project || spec.Cash != want.cash || !spec.Tradable {
			t.Fatalf("unexpected %s configuration: %#v", unit, spec)
		}
	}
}
