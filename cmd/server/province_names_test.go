package main

import "testing"

func TestProvinceNamesAreTrimmedAndCappedByCharacters(t *testing.T) {
	name, ok := normalizeProvinceName("  New Kyoto  ")
	if !ok || name != "New Kyoto" {
		t.Fatalf("valid Province name normalized to %q, %v", name, ok)
	}
	if _, ok = normalizeProvinceName("A"); ok {
		t.Fatal("single-character Province name was accepted")
	}
	if _, ok = normalizeProvinceName("1234567890123456789012345678901"); ok {
		t.Fatal("Province name longer than 30 characters was accepted")
	}
	if _, ok = normalizeProvinceName("東京都市東京都市東京都市東京都市東京都市"); !ok {
		t.Fatal("Unicode Province name was measured as bytes instead of characters")
	}
}
