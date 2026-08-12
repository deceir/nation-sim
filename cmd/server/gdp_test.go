package main

import "testing"

func TestAnnualizedGDP(t *testing.T) {
	if got := annualizedGDP(2_000_000); got != 730_000_000 {
		t.Fatalf("annualized GDP = %d, want 730000000", got)
	}
	if got := annualizedGDP(1.999); got != 729 {
		t.Fatalf("fractional annualized GDP = %d, want 729", got)
	}
}
