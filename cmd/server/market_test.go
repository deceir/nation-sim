package main

import "testing"

func TestDistanceTableSpecifiedPairs(t *testing.T) {
	tests := []struct {
		from, to string
		want     float64
	}{
		{"Africa", "Asia", 1.35}, {"Asia", "South America", 1.90}, {"Europe", "North America", 1.55},
		{"North America", "South America", 1.40}, {"Antarctica", "North America", 1.95}, {"Africa", "Africa", 1},
	}
	for _, tt := range tests {
		if got := distanceModifier(tt.from, tt.to); got != tt.want {
			t.Errorf("%s to %s: got %v want %v", tt.from, tt.to, got, tt.want)
		}
		if got := distanceModifier(tt.to, tt.from); got != tt.want {
			t.Errorf("distance must be symmetric: %s to %s got %v", tt.to, tt.from, got)
		}
	}
}

func TestShipmentTerms(t *testing.T) {
	distance, turns, fee, risk := shipmentTerms("Africa", "Africa", 100, 100000)
	if distance != 1 || turns != 3 || fee != 1000 || risk != .5 {
		t.Fatalf("same-continent terms incorrect: %v %v %v %v", distance, turns, fee, risk)
	}
	_, bulkTurns, _, _ := shipmentTerms("Asia", "South America", 20000, 100000)
	if bulkTurns != 8 {
		t.Fatalf("expected 8 bulk turns, got %d", bulkTurns)
	}
}

func TestMarketNotificationQuantityRoundsUpToTenth(t *testing.T) {
	cases := map[float64]string{1: "1.0", 1.01: "1.1", 1.10: "1.1", 9.999: "10.0"}
	for input, expected := range cases {
		if actual := marketNotificationQuantity(input); actual != expected {
			t.Fatalf("quantity %v displayed as %s; expected %s", input, actual, expected)
		}
	}
}
