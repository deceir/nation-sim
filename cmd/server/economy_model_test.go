package main

import (
	"testing"
	"time"
)

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

func TestStartingNationPopulationMatchesEconomicModel(t *testing.T) {
	if got := startingNationPopulation(); got != 10631 {
		t.Fatalf("starting nation population = %d, want 10631", got)
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

func TestStarterProvinceProducesRoughlyTwoMillionDailyTax(t *testing.T) {
	result := calculateEconomy(ModelNation{TaxRate: 25, Happiness: 65, Education: 40, EmploymentRate: 72, Technology: 20, Cities: []ModelCity{{Infra: 100, Land: 150, Buildings: map[string]int{}, Upgrades: map[string]int{}}}})
	if result.DailyTax < 1750000 || result.DailyTax > 2400000 {
		t.Fatalf("starter daily tax = %.0f, want roughly 2 million", result.DailyTax)
	}
}

func TestPopulationGrowthPersistsBeforeInfrastructureExpansion(t *testing.T) {
	base := ModelNation{TaxRate: 25, Happiness: 65, Education: 40, EmploymentRate: 72, Technology: 20, Cities: []ModelCity{{ID: "capital", Infra: 100, Land: 150, Buildings: map[string]int{}, Upgrades: map[string]int{}}}}
	initial := calculateEconomy(base)
	if initial.Cities[0].PopulationCapacity <= initial.Cities[0].EffectivePopulation {
		t.Fatal("starter Province should have room for organic population growth")
	}
	base.Cities[0].Population = initial.Cities[0].EffectivePopulation + 50
	grown := calculateEconomy(base)
	if grown.Cities[0].EffectivePopulation <= initial.Cities[0].EffectivePopulation {
		t.Fatal("stored organic population growth was not retained")
	}
	base.Cities[0].Infra = 150
	expanded := calculateEconomy(base)
	if expanded.Cities[0].PopulationCapacity <= grown.Cities[0].PopulationCapacity || expanded.DailyTax <= grown.DailyTax {
		t.Fatal("Infrastructure should raise population capacity and tax income")
	}
}

func TestStarterPopulationGrowsEveryHourlyTurn(t *testing.T) {
	result := calculateEconomy(ModelNation{TaxRate: 25, Happiness: 65, Education: 40, EmploymentRate: 72, Technology: 20, Cities: []ModelCity{{ID: "capital", Infra: 100, Land: 150, Buildings: map[string]int{}, Upgrades: map[string]int{}}}})
	_, growth := nationalPopulationGrowth("nation", result.Cities, 65, 40, 1, time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC))
	if growth < 5 || growth > 20 {
		t.Fatalf("starter hourly population growth = %d, want the 5-20 floor", growth)
	}
}

func TestManagedProvinceCanReachNinetyPercentEmployment(t *testing.T) {
	result := calculateEconomy(ModelNation{TaxRate: 25, Happiness: 60, Education: 55, EmploymentRate: 72, Cities: []ModelCity{{Infra: 300, Land: 400, Buildings: map[string]int{"marketplace": 3, "transit_authority": 2, "shopping_mall": 2}, Upgrades: map[string]int{}}}})
	if result.Cities[0].EmploymentRate < 90 {
		t.Fatalf("well-managed employment = %.1f%%, want at least 90%%", result.Cities[0].EmploymentRate)
	}
}

func TestIndustrialUpgradesCreateManageableHealthAndSecurityPressure(t *testing.T) {
	base := ModelNation{TaxRate: 25, Happiness: 55, Education: 45, EmploymentRate: 72, Cities: []ModelCity{{Infra: 400, Land: 500, Buildings: map[string]int{}, Upgrades: map[string]int{"extraction": 8, "heavy_industry": 8, "commerce": 8}}}}
	strained := calculateEconomy(base)
	base.Cities[0].Buildings = map[string]int{"hospital": 2, "police_station": 2, "recycling_center": 2}
	managed := calculateEconomy(base)
	if strained.Cities[0].Disease <= 0 || strained.Cities[0].Crime <= 0 {
		t.Fatal("production investment should create disease and crime pressure")
	}
	if managed.Cities[0].Disease >= strained.Cities[0].Disease || managed.Cities[0].Crime >= strained.Cities[0].Crime {
		t.Fatal("health, security, and recovery institutions should mitigate production pressure")
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

func TestInstitutionDeconstructionRefundsSeventyFivePercentOfResources(t *testing.T) {
	spec := buildings["hospital"]
	refunds := institutionResourceRefunds(spec)
	if len(refunds) == 0 {
		t.Fatal("hospital should have resource construction costs to refund")
	}
	for resource, originalCost := range spec.Costs {
		want := originalCost * 0.75
		if got := refunds[resource]; got != want {
			t.Fatalf("%s refund = %v, want %v", resource, got, want)
		}
	}
}
