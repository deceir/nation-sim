package main

import "testing"

func TestValidateFoundingProfile(t *testing.T) {
	valid, ok := validateFoundingProfile(foundingProfile{
		LeaderName: "Aiko Tanaka", NationName: "Sakura Republic", Capital: "Hikari",
		Government: "Parliamentary Democracy", Continent: "Asia",
	})
	if !ok || valid.NationName != "Sakura Republic" {
		t.Fatal("valid founding profile rejected")
	}
	invalid := []foundingProfile{
		{LeaderName: "", NationName: "Sakura Republic", Capital: "Hikari", Government: "Parliamentary Democracy", Continent: "Asia"},
		{LeaderName: "Aiko", NationName: "Sakura Republic", Capital: "Hikari", Government: "Unknown", Continent: "Asia"},
		{LeaderName: "Aiko", NationName: "Sakura Republic", Capital: "Hikari", Government: "Federal Republic", Continent: "Atlantis"},
	}
	for _, profile := range invalid {
		if _, ok := validateFoundingProfile(profile); ok {
			t.Errorf("invalid profile accepted: %+v", profile)
		}
	}
}

func TestNationUserType(t *testing.T) {
	for _, name := range []string{"Japan", "JAPAN", " japan "} {
		if got := nationUserType(name); got != "DEV" {
			t.Errorf("%q got %s", name, got)
		}
	}
	if got := nationUserType("Republic of Japanica"); got != "PLAYER" {
		t.Errorf("unexpected DEV designation: %s", got)
	}
}
