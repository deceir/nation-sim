package main

import "testing"

func TestInfrastructurePricingCompoundsAndBulkDiscounts(t *testing.T) {
	low := infraPurchaseCost(100, 50, 0)
	high := infraPurchaseCost(1000, 50, 0)
	if high <= low {
		t.Fatalf("expected developed city infrastructure to cost more: low=%v high=%v", low, high)
	}
	individual := infraPurchaseCost(100, 99, 0) / 99
	bulk := infraPurchaseCost(100, 100, 0) / 100
	if bulk >= individual {
		t.Fatalf("expected 100-unit purchase discount: individual=%v bulk=%v", individual, bulk)
	}
}

func TestEconomyIsDeterministicAndPowerConstrainsOutput(t *testing.T) {
	n := ModelNation{TaxRate: 25, Happiness: 55, Education: 45, Technology: 8, Cities: []ModelCity{{ID: "a", Name: "Capital", Infra: 200, Land: 250, Buildings: map[string]int{"steel_mill": 2, "iron_mine": 2, "coal_mine": 2}}}}
	a, b := calculateEconomy(n), calculateEconomy(n)
	if a.NetDailyCash != b.NetDailyCash || a.Production["steel"] != b.Production["steel"] {
		t.Fatal("economic calculation is not deterministic")
	}
	if a.Cities[0].PowerMultiplier != 0 {
		t.Fatalf("unpowered manufacturing should shut down, got %v", a.Cities[0].PowerMultiplier)
	}
	n.Cities[0].Buildings["coal_plant"] = 1
	powered := calculateEconomy(n)
	if powered.Production["steel"] <= a.Production["steel"] {
		t.Fatal("power plant did not restore manufacturing")
	}
}

func TestHappinessRespondsToTaxAndPollution(t *testing.T) {
	base := ModelNation{TaxRate: 20, Happiness: 50, Education: 40, Cities: []ModelCity{{Infra: 100, Land: 150, Buildings: map[string]int{}}}}
	good := calculateEconomy(base)
	base.TaxRate = 45
	base.Cities[0].Pollution = 30
	bad := calculateEconomy(base)
	if bad.HappinessTarget >= good.HappinessTarget {
		t.Fatalf("tax and pollution should reduce happiness target: good=%v bad=%v", good.HappinessTarget, bad.HappinessTarget)
	}
}

func TestBeginnerProjectsApplyPermanentEconomicEffects(t *testing.T) {
	base := ModelNation{TaxRate: 25, Happiness: 50, Education: 40, Projects: map[string]bool{}, Cities: []ModelCity{{Infra: 300, Land: 400, Buildings: map[string]int{"coal_plant": 1, "farm": 1, "bank": 1}}}}
	without := calculateEconomy(base)
	base.Projects = map[string]bool{"resource_survey": true, "commerce_foundation": true, "civil_engineering_corps": true, "public_health_sanitation": true}
	with := calculateEconomy(base)
	if with.Production["food"] <= without.Production["food"] {
		t.Fatal("Resource Survey did not increase extraction")
	}
	if with.Cities[0].Commerce <= without.Cities[0].Commerce {
		t.Fatal("Commerce Foundation did not increase city commerce")
	}
	if with.DailyUpkeep >= without.DailyUpkeep {
		t.Fatal("Civil Engineering Corps did not reduce upkeep")
	}
	if with.Cities[0].Disease >= without.Cities[0].Disease {
		t.Fatal("Public Health project did not reduce disease")
	}
}
