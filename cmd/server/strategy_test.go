package main

import (
	"math"
	"testing"
)

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

func TestSpecializedGearBonusesUseStrongerBalancePass(t *testing.T) {
	tests := []struct {
		gear  string
		field float64
		want  float64
	}{
		{"agrarian extraction", gears["agrarian"].Extraction, 1.22},
		{"industrial production", gears["industrial"].Industry, 1.28},
		{"commercial income", gears["commercial"].Commerce, 1.25},
	}
	for _, test := range tests {
		if math.Abs(test.field-test.want) > .0001 {
			t.Errorf("%s multiplier = %.2f, want %.2f", test.gear, test.field, test.want)
		}
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

func TestWorkerTrainingEducationEffectIsWired(t *testing.T) {
	in := sampleStrategy()
	in.Policies = map[string]bool{"worker_training": true}
	result := calculateStrategy(in)
	if math.Abs(result.EducationMultiplier-1.10) > .0001 {
		t.Fatalf("Worker Training Education multiplier = %.2f, want 1.10", result.EducationMultiplier)
	}
	if got := policyAdjustedEducationChange(2, result.EducationMultiplier); math.Abs(got-2.2) > .0001 {
		t.Fatalf("positive Education change = %.2f, want 2.20", got)
	}
	if got := policyAdjustedEducationChange(-.5, result.EducationMultiplier); got != -.5 {
		t.Fatalf("Education policy should not accelerate decay, got %.2f", got)
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

func TestZeroCommodityAllocationFallsBackToEvenProduction(t *testing.T) {
	quotas := defaultProductionQuotas()
	if total := productionQuotaTotal(quotas); math.Abs(total-100) > .001 {
		t.Fatalf("default production allocation totals %.2f, want 100", total)
	}
	in := sampleStrategy()
	in.Quotas = map[string]float64{}
	result := calculateStrategy(in)
	producing := 0
	for commodity := range commodityRecipes {
		if result.Production[commodity] > 0 {
			producing++
		}
	}
	if producing == 0 {
		t.Fatal("zero-allocation nation should receive productive default quotas")
	}
}
