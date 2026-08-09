package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestCrisisCatalogContainsTwoHundredFiftyValidTemplates(t *testing.T) {
	var catalog crisisCatalog
	if err := json.Unmarshal(crisisCatalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Templates) != 250 {
		t.Fatalf("templates = %d, want 250", len(catalog.Templates))
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
		options := template.Options
		if len(options) != 3 {
			t.Fatalf("template %q has %d options", template.ID, len(options))
		}
		for _, option := range options {
			if option.Label == "" || option.EffectText == "" || len(option.Effects) == 0 {
				t.Fatalf("template %q has invalid option %#v", template.ID, option)
			}
			for _, effect := range option.Effects {
				if !validCrisisEffects[effect.Type] {
					t.Fatalf("template %q has invalid effect %#v", template.ID, effect)
				}
			}
		}
		if options[2].Effects[0].Type != "none" {
			t.Fatalf("template %q third option is not a neutral resolution", template.ID)
		}
		if strings.Contains(strings.ToLower(template.Briefing+options[0].EffectText+options[1].EffectText+options[2].EffectText), "no penalt") {
			t.Fatalf("template %q contains retired no-penalty phrasing", template.ID)
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
