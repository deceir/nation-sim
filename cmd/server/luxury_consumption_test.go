package main

import "testing"

func TestLuxuryConsumptionRewardsNationScale(t *testing.T) {
	small := luxurySizeEfficiency(20_000, 1)
	large := luxurySizeEfficiency(2_000_000, 10)
	if small != luxuryBalance.MinEfficiency {
		t.Fatalf("small nation efficiency=%v, want floor %v", small, luxuryBalance.MinEfficiency)
	}
	if large <= 1 || large <= small {
		t.Fatalf("large nation efficiency=%v should materially exceed small nation efficiency=%v", large, small)
	}
}

func TestLuxuryConsumptionCapsScaleWithNationSize(t *testing.T) {
	small := luxuryMaxConsumptionRate(20_000, 1)
	large := luxuryMaxConsumptionRate(2_000_000, 10)
	if large <= small*3 {
		t.Fatalf("large nation cap=%v should substantially exceed small nation cap=%v", large, small)
	}
}

func TestLuxuryProjectionUsesStockpileAndConfiguredValue(t *testing.T) {
	consumed, efficiency, income := projectedLuxuryConsumption(100, 40, 20_000, 1)
	if consumed != 40 || efficiency != luxuryBalance.MinEfficiency {
		t.Fatalf("unexpected projection: consumed=%v efficiency=%v", consumed, efficiency)
	}
	want := int64(40 * luxuryBalance.BaseValue * luxuryBalance.MinEfficiency)
	if income != want {
		t.Fatalf("income=%d, want %d", income, want)
	}
}

func TestLuxuryConsumptionRequiresNationalProject(t *testing.T) {
	project, ok := longTermProjects["luxury_market_authority"]
	if !ok || project.Category != "national" || project.Unlock != "luxury_consumption" {
		t.Fatalf("Luxury Market Authority is not wired as a national unlock: %#v", project)
	}
	if project.Cash > 2_000_000*yenScale || project.Turns > 24 {
		t.Fatalf("Luxury Market Authority should remain an accessible safeguard project: %#v", project)
	}
}
