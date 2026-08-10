package main

import "testing"

func TestProvinceFoundingCostsScaleAndRespectStrategy(t *testing.T) {
	first, materials, _ := provinceFoundingCosts(1, "balanced", map[string]bool{})
	third, moreMaterials, _ := provinceFoundingCosts(3, "balanced", map[string]bool{})
	cheap, _, _ := provinceFoundingCosts(3, "agrarian", map[string]bool{"land_grants": true})
	if first != 25000000 || third <= first || moreMaterials <= materials {
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
	if provinceUpgradeCapacity(100) != 5 || provinceUpgradeCapacity(149.99) != 5 || provinceUpgradeCapacity(150) != 6 || provinceUpgradeCapacity(200) != 7 {
		t.Fatal("Province upgrade capacity must begin at five and rise every 50 Infrastructure")
	}
	if provinceUpgradesUsed(map[string]int{"agriculture": 2, "commerce": 3}) != 5 {
		t.Fatal("only Province upgrade levels should consume upgrade capacity")
	}
}

func TestProvincePriceCurveEncouragesAlternatingDevelopmentAndExpansion(t *testing.T) {
	earlyDevelopment := int64(infraPurchaseCost(100, 50, 20))
	for _, key := range []string{"agriculture", "civil", "extraction", "commerce", "light_industry"} {
		earlyDevelopment += provinceUpgradeCost(provinceUpgradeSpecs[key], 0, 100)
	}
	second, _, _ := provinceFoundingCosts(1, "balanced", map[string]bool{})
	third, _, _ := provinceFoundingCosts(2, "balanced", map[string]bool{})
	if second <= earlyDevelopment || second > earlyDevelopment*3 {
		t.Fatalf("the second Province should follow a meaningful but short initial development phase: development=%d expansion=%d", earlyDevelopment, second)
	}
	if third <= second*5 {
		t.Fatalf("the third Province should require developing the first two while saving: second=%d third=%d", second, third)
	}
	deepDevelopment := int64(infraPurchaseCost(100, 450, 20))
	for level := 0; level < 10; level++ {
		deepDevelopment += provinceUpgradeCost(provinceUpgradeSpecs["commerce"], level, 550)
	}
	sixth, _, _ := provinceFoundingCosts(5, "balanced", map[string]bool{})
	if sixth <= deepDevelopment {
		t.Fatalf("late expansion should cost more than heavily developing an existing Province: development=%d expansion=%d", deepDevelopment, sixth)
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
