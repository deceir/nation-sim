package main

import "testing"

func TestBuildNationalDetailsWeightsConditionsAndAggregatesInstitutions(t *testing.T) {
	nation := ModelNation{Happiness: 70, Education: 55, Cities: []ModelCity{
		{Buildings: map[string]int{"hospital": 2, "police_station": 1}},
		{Buildings: map[string]int{"hospital": 1}},
	}}
	result := NationResult{
		EffectiveEmploymentRate:          91,
		EffectiveTaxCollectionMultiplier: 1.08,
		DailyCivicUpkeep:                 6400,
		Cities: []CityResult{
			{EffectivePopulation: 1000, Disease: .02, Crime: .04, Pollution: 10},
			{EffectivePopulation: 3000, Disease: .06, Crime: .08, Pollution: 30},
		},
	}
	details := buildNationalDetails(nation, result)
	if details.DiseaseRate != .05 || details.CrimeRate != .07 || details.Pollution != 25 {
		t.Fatalf("unexpected weighted conditions: disease=%v crime=%v pollution=%v", details.DiseaseRate, details.CrimeRate, details.Pollution)
	}
	if details.InstitutionCount != 4 {
		t.Fatalf("institution count = %d, want 4", details.InstitutionCount)
	}
	counts := map[string]int{}
	for _, item := range details.Institutions {
		counts[item.Key] = item.Count
	}
	if counts["hospital"] != 3 || counts["police_station"] != 1 {
		t.Fatalf("unexpected institution totals: %#v", counts)
	}
}
