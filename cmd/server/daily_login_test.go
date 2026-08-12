package main

import (
	"testing"
	"time"
)

func TestNextLoginStreak(t *testing.T) {
	today := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	if got := nextLoginStreak(today.AddDate(0, 0, -1), 6, today); got != 7 {
		t.Fatalf("consecutive login should continue streak: got %d", got)
	}
	if got := nextLoginStreak(today.AddDate(0, 0, -2), 6, today); got != 1 {
		t.Fatalf("missed day should reset streak: got %d", got)
	}
}

func TestLoginRewardScalesAndCaps(t *testing.T) {
	cases := map[int]int64{1: 2500000, 2: 3000000, 5: 4500000, 6: 5000000, 30: 5000000}
	for streak, expected := range cases {
		if got := loginRewardForStreak(streak); got != expected {
			t.Fatalf("streak %d: expected %d, got %d", streak, expected, got)
		}
	}
}
