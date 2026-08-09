package main

import (
	"net/url"
	"testing"
)

func TestLeaderboardParametersValidateAndNormalizeFilters(t *testing.T) {
	values := url.Values{"metric": {"jets"}, "order": {"asc"}, "continent": {"Asia"}, "minProvinces": {"12"}, "maxProvinces": {"4"}, "page": {"2"}, "pageSize": {"25"}}
	got := leaderboardParameters(values)
	if got.Metric != "jets" || got.Order != "asc" || got.Continent != "Asia" || got.MinProvinces != 4 || got.MaxProvinces != 12 || got.Page != 2 || got.PageSize != 25 {
		t.Fatalf("unexpected normalized leaderboard filters: %#v", got)
	}
}

func TestLeaderboardParametersUseSafeDefaults(t *testing.T) {
	got := leaderboardParameters(url.Values{"metric": {"treasury"}, "order": {"sideways"}, "continent": {"Atlantis"}, "pageSize": {"500"}})
	if got.Metric != "population" || got.Order != "desc" || got.Continent != "" || got.Page != 1 || got.PageSize != 10 {
		t.Fatalf("unexpected leaderboard defaults: %#v", got)
	}
}
