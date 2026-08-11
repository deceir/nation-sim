package main

import "sort"

type nationalInstitutionCount struct {
	Key, Name, Category string
	Count               int
}

type nationalDetails struct {
	EmploymentRate, DiseaseRate, CrimeRate, Pollution, TaxCollectionMultiplier float64
	Satisfaction, Education                                                    float64
	DailyCivicUpkeep                                                           float64
	InstitutionCount                                                           int
	Institutions                                                               []nationalInstitutionCount
}

func buildNationalDetails(n ModelNation, result NationResult) nationalDetails {
	details := nationalDetails{
		EmploymentRate:          result.EffectiveEmploymentRate,
		TaxCollectionMultiplier: result.EffectiveTaxCollectionMultiplier,
		Satisfaction:            n.Happiness,
		Education:               n.Education,
		DailyCivicUpkeep:        result.DailyCivicUpkeep,
		Institutions:            []nationalInstitutionCount{},
	}
	weightedPopulation := 0.0
	for _, city := range result.Cities {
		population := city.EffectivePopulation
		weightedPopulation += population
		details.DiseaseRate += city.Disease * population
		details.CrimeRate += city.Crime * population
		details.Pollution += city.Pollution * population
	}
	if weightedPopulation > 0 {
		details.DiseaseRate /= weightedPopulation
		details.CrimeRate /= weightedPopulation
		details.Pollution /= weightedPopulation
	}
	counts := map[string]int{}
	for _, city := range n.Cities {
		for key, count := range city.Buildings {
			if _, isInstitution := buildings[key]; isInstitution && count > 0 {
				counts[key] += count
				details.InstitutionCount += count
			}
		}
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, right := buildings[keys[i]], buildings[keys[j]]
		if left.Category == right.Category {
			return left.Name < right.Name
		}
		return left.Category < right.Category
	})
	for _, key := range keys {
		spec := buildings[key]
		details.Institutions = append(details.Institutions, nationalInstitutionCount{Key: key, Name: spec.Name, Category: spec.Category, Count: counts[key]})
	}
	return details
}
