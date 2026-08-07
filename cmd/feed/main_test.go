package main

import (
	"strings"
	"testing"
)

func TestBotNationNames(t *testing.T) {
	if len(bots) != 5 {
		t.Fatalf("expected five bots, got %d", len(bots))
	}
	seen := map[string]bool{}
	for _, bot := range bots {
		if bot.Name != strings.ToUpper(bot.Name) {
			t.Errorf("name is not uppercase: %s", bot.Name)
		}
		if strings.Contains(strings.ToUpper(bot.Name), "JAPAN") {
			t.Errorf("reserved Japan name used: %s", bot.Name)
		}
		if seen[bot.Name] {
			t.Errorf("duplicate bot: %s", bot.Name)
		}
		seen[bot.Name] = true
	}
}
