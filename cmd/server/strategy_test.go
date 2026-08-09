package main

import "testing"

func sampleStrategy() strategicInput {
	return strategicInput{Gear: "balanced", Education: 40, Technology: 10, Policies: map[string]bool{}, Quotas: map[string]float64{"textiles": 100}, Provinces: []provinceStrategy{{ID: "p", Infra: 200, Development: 1, Specialization: "agriculture", Deposits: map[string]float64{"foodstuffs": 1.2, "fibers": 1.1}}}}
}
func TestEconomicGearsProduceDistinctStrategies(t *testing.T) {
	in := sampleStrategy()
	balanced := calculateStrategy(in)
	in.Gear = "agrarian"
	agrarian := calculateStrategy(in)
	if agrarian.Production["foodstuffs"] <= balanced.Production["foodstuffs"] {
		t.Fatal("agrarian gear should increase primary production")
	}
	in.Gear = "commercial"
	if calculateStrategy(in).IncomeMultiplier <= balanced.IncomeMultiplier {
		t.Fatal("commercial gear should increase citizen income")
	}
}
func TestGearDisruptionReducesOutput(t *testing.T) {
	in := sampleStrategy()
	in.Gear = "industrial"
	normal := calculateStrategy(in)
	in.Disrupted = true
	disrupted := calculateStrategy(in)
	if disrupted.Production["textiles"] >= normal.Production["textiles"] || disrupted.IncomeMultiplier >= normal.IncomeMultiplier {
		t.Fatal("gear-change disruption must reduce efficiency")
	}
}
func TestPolicySynergyCompoundsWithGear(t *testing.T) {
	in := sampleStrategy()
	in.Gear = "agrarian"
	without := calculateStrategy(in)
	in.Policies = map[string]bool{"land_grants": true}
	with := calculateStrategy(in)
	if with.ExtractionMultiplier <= without.ExtractionMultiplier || with.PopulationMultiplier <= without.PopulationMultiplier {
		t.Fatal("land grants should reinforce agrarian growth")
	}
}

func TestResourceSurveyBoostsCurrentPrimaryResources(t *testing.T) {
	in := sampleStrategy()
	without := calculateStrategy(in)
	in.Projects = map[string]bool{"resource_survey": true}
	with := calculateStrategy(in)
	if with.Production["foodstuffs"] <= without.Production["foodstuffs"] {
		t.Fatal("resource survey should boost current primary resource production")
	}
}
