package main

import "testing"

func TestMilitaryProjectsDoNotUseStandardProjectSlots(t *testing.T) {
	military := []string{"aviation_industry", "naval_shipyard", "armored_vehicle_program", "advanced_ordnance"}
	for _, id := range military {
		project, exists := longTermProjects[id]
		if !exists || project.Category != "military" {
			t.Fatalf("military project %q is missing or miscategorized", id)
		}
		if longTermProjectUsesSlot(id) {
			t.Fatalf("military project %q consumes a standard slot", id)
		}
	}
	for _, id := range []string{"agricultural_expansion", "civil_service_reform"} {
		if !longTermProjectUsesSlot(id) {
			t.Fatalf("standard project %q unexpectedly bypasses slot capacity", id)
		}
	}
}
