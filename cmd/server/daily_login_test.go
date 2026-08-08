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
	cases := map[int]int64{1: 25000, 2: 30000, 5: 45000, 6: 50000, 30: 50000}
	for streak, expected := range cases {
		if got := loginRewardForStreak(streak); got != expected {
			t.Fatalf("streak %d: expected %d, got %d", streak, expected, got)
		}
	}
}
