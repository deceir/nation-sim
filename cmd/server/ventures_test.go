package main

import "testing"

func TestVentureCancellationRefund(t *testing.T) {
	if got := ventureCancellationRefund(1_000_000); got != 750_000 {
		t.Fatalf("refund = %d, want 750000", got)
	}
	if got := ventureCancellationRefund(101); got != 75 {
		t.Fatalf("whole-Yen refund = %d, want 75", got)
	}
}

func TestVentureOutcomeStaysWithinAdvertisedRiskRange(t *testing.T) {
	ranges := map[string][2]int{"low": {-1000, 800}, "medium": {-1200, 4000}, "high": {-2000, 7000}}
	for risk, bounds := range ranges {
		for i := 0; i < 500; i++ {
			got := ventureOutcome(risk)
			if got < bounds[0] || got > bounds[1] {
				t.Fatalf("%s outcome %d outside [%d,%d]", risk, got, bounds[0], bounds[1])
			}
		}
	}
}
