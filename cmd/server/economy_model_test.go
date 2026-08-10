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

func TestEconomyIsDeterministicAndPowerConstrainsImprovements(t *testing.T) {
	n := ModelNation{TaxRate: 25, Happiness: 55, Education: 45, Technology: 8, Cities: []ModelCity{{ID: "a", Name: "Capital", Infra: 200, Land: 250, Buildings: map[string]int{"shopping_mall": 2}}}}
	a, b := calculateEconomy(n), calculateEconomy(n)
	if a.NetDailyCash != b.NetDailyCash || a.Cities[0].Commerce != b.Cities[0].Commerce {
		t.Fatal("economic calculation is not deterministic")
	}
	if a.Cities[0].PowerMultiplier != 0 {
		t.Fatalf("unpowered manufacturing should shut down, got %v", a.Cities[0].PowerMultiplier)
	}
	n.Cities[0].Buildings["renewable_plant"] = 1
	powered := calculateEconomy(n)
	if powered.Cities[0].Commerce <= a.Cities[0].Commerce {
		t.Fatal("power generation did not restore powered improvements")
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
	base := ModelNation{TaxRate: 25, Happiness: 50, Education: 40, Projects: map[string]bool{}, Cities: []ModelCity{{Infra: 300, Land: 400, Buildings: map[string]int{"renewable_plant": 1, "bank": 1}}}}
	without := calculateEconomy(base)
	base.Projects = map[string]bool{"resource_survey": true, "commerce_foundation": true, "civil_engineering_corps": true, "public_health_sanitation": true}
	with := calculateEconomy(base)
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

func TestCivicInstitutionsIncreaseEmploymentAndTaxRevenue(t *testing.T) {
	base := ModelNation{TaxRate: 25, Happiness: 55, Education: 45, EmploymentRate: 72, Cities: []ModelCity{{Infra: 100, Land: 150, Buildings: map[string]int{}}}}
	without := calculateEconomy(base)
	base.Cities[0].Buildings = map[string]int{"marketplace": 1, "bank": 1}
	with := calculateEconomy(base)
	if with.Cities[0].EmploymentRate <= without.Cities[0].EmploymentRate {
		t.Fatal("employment-focused Civic Institutions did not improve employment")
	}
	if with.Cities[0].TaxCollectionMultiplier <= without.Cities[0].TaxCollectionMultiplier || with.DailyTax <= without.DailyTax {
		t.Fatal("commerce institutions did not improve tax collection and revenue")
	}
	if with.DailyCivicUpkeep <= 0 || with.NetDailyCash >= with.DailyTax {
		t.Fatal("Civic Institutions must carry visible upkeep")
	}
}

func TestCivicInstitutionCapacityHasProvincialMinimum(t *testing.T) {
	if got := civicInstitutionCapacity(100); got != 5 {
		t.Fatalf("starter Province should have five Civic Institution slots, got %d", got)
	}
	if civicInstitutionCapacity(300) <= civicInstitutionCapacity(100) {
		t.Fatal("Infrastructure should expand Civic Institution capacity")
	}
}
