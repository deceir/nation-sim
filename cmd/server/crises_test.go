package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestCrisisCatalogContainsOneHundredValidTemplates(t *testing.T) {
	var catalog crisisCatalog
	if err := json.Unmarshal(crisisCatalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Templates) != 100 {
		t.Fatalf("templates = %d, want 100", len(catalog.Templates))
	}
	seen := map[string]bool{}
	for _, template := range catalog.Templates {
		if template.ID == "" || template.Title == "" || template.Briefing == "" {
			t.Fatalf("incomplete template: %#v", template)
		}
		if seen[template.ID] {
			t.Fatalf("duplicate template ID %q", template.ID)
		}
		seen[template.ID] = true
		options, ok := catalog.Profiles[template.Profile]
		if !ok || len(options) < 2 || len(options) > 3 {
			t.Fatalf("template %q has invalid profile %q", template.ID, template.Profile)
		}
		for _, option := range options {
			if option.Label == "" || option.EffectText == "" || !validCrisisEffects[option.EffectType] {
				t.Fatalf("template %q has invalid option %#v", template.ID, option)
			}
		}
	}
}

func TestDailyCrisisCountRange(t *testing.T) {
	for i := 0; i < 500; i++ {
		count := secureInt(4)
		if count < 0 || count > 3 {
			t.Fatalf("daily Crisis count %d outside 0-3", count)
		}
	}
}

func TestApplyCrisisTurnModifiers(t *testing.T) {
	result := strategicResult{IncomeMultiplier: 1, HappinessMultiplier: 1, PopulationMultiplier: 1, Production: map[string]float64{"foodstuffs": 100, "energy": 80}}
	applyCrisisTurnModifiers(&result, crisisTurnModifiers{CashIncomePct: 3, HappinessPct: 4, PopulationGrowthPct: 2, Production: map[string]float64{"all": 2, "foodstuffs": 5}})
	if result.IncomeMultiplier != 1.03 || result.HappinessMultiplier != 1.04 || result.PopulationMultiplier != 1.02 {
		t.Fatalf("national modifiers not applied: %#v", result)
	}
	if math.Abs(result.Production["energy"]-81.6) > .0001 || math.Abs(result.Production["foodstuffs"]-107.1) > .0001 {
		t.Fatalf("production modifiers not stacked correctly: %#v", result.Production)
	}
}
