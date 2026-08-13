package main

import "testing"

func TestWorldDataSeriesIncludesPopulationAndResources(t *testing.T) {
	series := worldDataSeries()
	if len(series) != len(strategicCommodities)+1 || series[0] != worldPopulationSeries {
		t.Fatalf("world data series should begin with population and include every resource: %v", series)
	}
	seen := map[string]bool{}
	for _, value := range series {
		if seen[value] {
			t.Fatalf("duplicate world data series %q", value)
		}
		seen[value] = true
	}
	for _, resource := range strategicCommodities {
		if !seen[resource] {
			t.Fatalf("missing resource series %q", resource)
		}
	}
}
