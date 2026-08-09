package main

import "testing"

func TestHourlyHappinessAppliesStrategyMultiplier(t *testing.T) {
	neutral := calculateHourlyHappiness(60, 70, 1)
	positive := calculateHourlyHappiness(60, 70, 1.05)
	negative := calculateHourlyHappiness(60, 70, .90)

	if positive <= neutral {
		t.Fatalf("positive Gear happiness modifier should raise hourly happiness: positive=%v neutral=%v", positive, neutral)
	}
	if negative >= neutral {
		t.Fatalf("negative Gear happiness modifier should lower hourly happiness: negative=%v neutral=%v", negative, neutral)
	}
	if capped := calculateHourlyHappiness(99, 100, 1.50); capped > 100 {
		t.Fatalf("adjusted happiness must remain capped at 100: %v", capped)
	}
}
