package main

import (
	"testing"
	"time"
)

func TestWorldNewsSlotUsesThreeHourUTCWindows(t *testing.T) {
	at := time.Date(2026, 9, 24, 8, 47, 0, 0, time.FixedZone("JST", 9*60*60))
	want := time.Date(2026, 9, 23, 21, 0, 0, 0, time.UTC)
	if got := worldNewsSlot(at); !got.Equal(want) {
		t.Fatalf("worldNewsSlot(%v) = %v, want %v", at, got, want)
	}
}

func TestMarketNewsRequiresMeaningfulMovement(t *testing.T) {
	slot := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	previous := worldNewsMetrics{Prices: map[string]float64{"energy": 100}}
	quiet := worldNewsMetrics{Prices: map[string]float64{"energy": 107}}
	if items := marketNewsCandidates(quiet, previous, slot); len(items) != 0 {
		t.Fatalf("7%% movement produced %d stories, want none", len(items))
	}
	moving := worldNewsMetrics{Prices: map[string]float64{"energy": 112}}
	if items := marketNewsCandidates(moving, previous, slot); len(items) != 1 {
		t.Fatalf("12%% movement produced %d stories, want one", len(items))
	}
}
