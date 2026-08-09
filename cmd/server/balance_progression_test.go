package main

import "testing"

func TestNewNationEconomyHasRewardingRunway(t *testing.T) {
	n := ModelNation{TaxRate: 25, Happiness: 65, Education: 40, Technology: 20, Projects: map[string]bool{}, LongTermProjects: map[string]bool{}, Cities: []ModelCity{{ID: "capital", Name: "Capital", Infra: 100, Land: 150, Buildings: map[string]int{}, Upgrades: map[string]int{}}}}
	cash := calculateEconomy(n)
	if cash.NetDailyCash < 75000 || cash.NetDailyCash > 175000 {
		t.Fatalf("starter daily cash should fund visible progress without trivializing expansion: %.0f", cash.NetDailyCash)
	}
	in := strategicInput{Gear: "balanced", Education: 40, Technology: 20, Policies: map[string]bool{}, Quotas: map[string]float64{"processed_foods": 35, "construction_materials": 45, "basic_goods": 20}, Provinces: []provinceStrategy{{ID: "capital", Infra: 100, Specialization: "mixed", Deposits: map[string]float64{"foodstuffs": startingDepositRichness("Asia", "foodstuffs"), "timber": startingDepositRichness("Asia", "timber"), "fibers": startingDepositRichness("Asia", "fibers"), "basic_metals": startingDepositRichness("Asia", "basic_metals"), "energy": startingDepositRichness("Asia", "energy"), "strategic_minerals": startingDepositRichness("Asia", "strategic_minerals")}, Upgrades: map[string]int{}}}}
	production := calculateStrategy(in).Production
	if production["foodstuffs"] <= cash.DailyFoodConsumption {
		t.Fatalf("normal starter geography should cover baseline food upkeep: production %.2f consumption %.2f", production["foodstuffs"], cash.DailyFoodConsumption)
	}
	if production["construction_materials"] < 2 || production["processed_foods"] < 2 {
		t.Fatalf("default quotas should demonstrate secondary production: %#v", production)
	}
}

func TestProgressionCostsHaveDistinctTimeHorizons(t *testing.T) {
	secondCash, secondMaterials, _ := provinceFoundingCosts(1, "balanced", map[string]bool{})
	thirdCash, thirdMaterials, _ := provinceFoundingCosts(2, "balanced", map[string]bool{})
	if secondCash != 225000 || secondMaterials != 40 {
		t.Fatalf("unexpected second Province runway: %d / %.0f", secondCash, secondMaterials)
	}
	if thirdCash < secondCash*5 || thirdMaterials < secondMaterials*4 {
		t.Fatalf("later expansion must create a saving and trade milestone: %d / %.0f", thirdCash, thirdMaterials)
	}
	for _, project := range longTermProjects {
		for commodity, cost := range project.Costs {
			if cost > 1000 {
				t.Fatalf("%s still has a commodity gate that overwhelms its cash gate: %s %.0f", project.ID, commodity, cost)
			}
		}
	}
}

func TestGeographyRewardsTradeWithoutZeroingResources(t *testing.T) {
	if startingDepositRichness("Africa", "basic_metals") <= startingDepositRichness("Africa", "timber") {
		t.Fatal("Africa should have a meaningful metals advantage")
	}
	for _, continent := range []string{"Africa", "Asia", "Europe", "North America", "South America", "Oceania", "Antarctica"} {
		for _, resource := range []string{"foodstuffs", "timber", "fibers", "basic_metals", "energy", "strategic_minerals"} {
			if startingDepositRichness(continent, resource) < .4 {
				t.Fatalf("early solo play must remain viable: %s %s", continent, resource)
			}
		}
	}
}
