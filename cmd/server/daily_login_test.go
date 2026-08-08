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
