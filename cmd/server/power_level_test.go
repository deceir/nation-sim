package main

import "testing"

func TestPowerLevelUsesEveryPublishedComponent(t *testing.T) {
	got := calculatePowerLevel(powerLevelComponents{Provinces: 2, Infrastructure: 400, Projects: 3, Soldiers: 1000, Tanks: 10, Ships: 2, Jets: 5, Drones: 4})
	if got.Provinces != 1000 || got.Infrastructure != 500 || got.Projects != 900 || got.Military != 102 || got.Total != 2502 {
		t.Fatalf("unexpected Power Level breakdown: %#v", got)
	}
}

func TestPowerLevelInfrastructureHasDiminishingWeight(t *testing.T) {
	first := calculatePowerLevel(powerLevelComponents{Infrastructure: 100}).Infrastructure
	second := calculatePowerLevel(powerLevelComponents{Infrastructure: 400}).Infrastructure
	if first != 250 || second != 500 {
		t.Fatalf("unexpected infrastructure scores: %d, %d", first, second)
	}
}
