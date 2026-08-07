package main

import "testing"

func TestProvinceFoundingCostsScaleAndRespectStrategy(t *testing.T) {
	first, materials, _ := provinceFoundingCosts(1, "balanced", map[string]bool{})
	third, moreMaterials, _ := provinceFoundingCosts(3, "balanced", map[string]bool{})
	cheap, _, _ := provinceFoundingCosts(3, "agrarian", map[string]bool{"land_grants": true})
	if first != 200000 || third <= first || moreMaterials <= materials {
		t.Fatalf("expected escalating founding costs, got %d/%d and %.2f/%.2f", first, third, materials, moreMaterials)
	}
	if cheap >= third {
		t.Fatalf("expected expansion strategy to reduce cost: %d >= %d", cheap, third)
	}
}

func TestProvinceUpgradesCostMoreAndDiminish(t *testing.T) {
	spec := provinceUpgradeSpecs["commerce"]
	if provinceUpgradeCost(spec, 8, 500) <= provinceUpgradeCost(spec, 1, 500) {
		t.Fatal("upgrade cost did not rise with level")
	}
	early := provinceUpgradeEffect(8) - provinceUpgradeEffect(7)
	late := provinceUpgradeEffect(12) - provinceUpgradeEffect(11)
	if late >= early {
		t.Fatalf("expected diminishing marginal effect, got early %.3f late %.3f", early, late)
	}
	if provinceUpgradeCap(100) != 4 || provinceUpgradeCap(5000) != 15 {
		t.Fatal("infrastructure upgrade caps are incorrect")
	}
}

func TestProvincialInvestmentIncreasesStrategicOutput(t *testing.T) {
	baseProvince := provinceStrategy{ID: "p", Infra: 300, Specialization: "mixed", Deposits: map[string]float64{"foodstuffs": 1}, Upgrades: map[string]int{}}
	input := strategicInput{Gear: "balanced", Policies: map[string]bool{}, Education: 40, Technology: 5, Provinces: []provinceStrategy{baseProvince}, Quotas: map[string]float64{"processed_foods": 100}}
	base := calculateStrategy(input)
	input.Provinces[0].Upgrades = map[string]int{"agriculture": 6, "extraction": 6, "light_industry": 6}
	upgraded := calculateStrategy(input)
	if upgraded.Production["foodstuffs"] <= base.Production["foodstuffs"] || upgraded.Production["processed_foods"] <= base.Production["processed_foods"] {
		t.Fatal("provincial upgrades did not increase the relevant outputs")
	}
}
