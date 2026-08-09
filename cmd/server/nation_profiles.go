package main

import (
	"net/http"
	"strings"
	"time"
)

var governmentTypes = map[string]bool{"Parliamentary Democracy": true, "Presidential Republic": true, "Federal Republic": true, "Constitutional Monarchy": true, "Absolute Monarchy": true, "One-Party State": true, "Military Junta": true, "Theocracy": true}
var continents = map[string]bool{"Africa": true, "Asia": true, "Europe": true, "North America": true, "South America": true, "Oceania": true, "Antarctica": true}

type foundingProfile struct{ LeaderName, NationName, Capital, Government, Continent string }

func nationUserType(name string) string {
	if strings.EqualFold(strings.TrimSpace(name), "Japan") {
		return "DEV"
	}
	return "PLAYER"
}

func validateFoundingProfile(p foundingProfile) (foundingProfile, bool) {
	p.LeaderName = strings.TrimSpace(p.LeaderName)
	p.NationName = strings.TrimSpace(p.NationName)
	p.Capital = strings.TrimSpace(p.Capital)
	if len(p.LeaderName) < 2 || len(p.LeaderName) > 100 || len(p.NationName) < 3 || len(p.NationName) > 100 || len(p.Capital) < 2 || len(p.Capital) > 100 || !governmentTypes[p.Government] || !continents[p.Continent] {
		return p, false
	}
	return p, true
}

func (a *app) nationDirectory(w http.ResponseWriter, r *http.Request, u user) {
	q := strings.TrimSpace(r.URL.Query().Get("search"))
	q = strings.ReplaceAll(strings.ReplaceAll(q, "\\", "\\\\"), "%", "\\%")
	q = strings.ReplaceAll(q, "_", "\\_")
	rows, e := a.db.Query(r.Context(), `SELECT n.id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,n.population,count(DISTINCT c.id),COALESCE(a.id,''),COALESCE(a.name,'') FROM nations n LEFT JOIN cities c ON c.nation_id=n.id LEFT JOIN alliance_members am ON am.nation_id=n.id LEFT JOIN alliances a ON a.id=am.alliance_id WHERE (?='' OR n.name LIKE CONCAT('%',?,'%') ESCAPE '\\' OR n.leader_name LIKE CONCAT('%',?,'%') ESCAPE '\\' OR a.name LIKE CONCAT('%',?,'%') ESCAPE '\\') GROUP BY n.id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,n.population,a.id,a.name ORDER BY n.population DESC,n.name LIMIT 100`, q, q, q, q)
	if e != nil {
		problem(w, 500, "Nation directory unavailable.")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, leader, government, continent, motto, userType, allianceID, allianceName string
		var cityCount int
		var population int64
		rows.Scan(&id, &name, &leader, &government, &continent, &motto, &userType, &population, &cityCount, &allianceID, &allianceName)
		out = append(out, map[string]any{"id": id, "name": name, "leaderName": leader, "government": government, "continent": continent, "motto": motto, "userType": userType, "population": population, "cityCount": cityCount, "allianceID": allianceID, "allianceName": allianceName})
	}
	write(w, 200, out)
}

func (a *app) nationProfile(w http.ResponseWriter, r *http.Request, u user) {
	var id, name, leader, government, continent, motto, capital, userType, allianceID, allianceName string
	var cityCount int
	var population int64
	var created time.Time
	var lastActive, guardianUntil *time.Time
	var locationLat, locationLng *float64
	e := a.db.QueryRow(r.Context(), `SELECT n.id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,n.population,n.created_at,count(DISTINCT c.id),COALESCE((SELECT name FROM cities WHERE nation_id=n.id ORDER BY created_at LIMIT 1),''),COALESCE(a.id,''),COALESCE(a.name,''),(SELECT MAX(s.last_action_at) FROM sessions s WHERE s.user_id=n.owner_id),(SELECT MAX(g.expires_at) FROM guardian_grants g WHERE g.nation_id=n.id AND g.revoked_at IS NULL AND g.starts_at<=NOW() AND g.expires_at>NOW()),n.location_lat,n.location_lng FROM nations n LEFT JOIN cities c ON c.nation_id=n.id LEFT JOIN alliance_members am ON am.nation_id=n.id LEFT JOIN alliances a ON a.id=am.alliance_id WHERE n.id=? GROUP BY n.id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,n.population,n.created_at,n.owner_id,n.location_lat,n.location_lng,a.id,a.name`, r.PathValue("id")).Scan(&id, &name, &leader, &government, &continent, &motto, &userType, &population, &created, &cityCount, &capital, &allianceID, &allianceName, &lastActive, &guardianUntil, &locationLat, &locationLng)
	if e != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	gear := "balanced"
	a.db.QueryRowContext(r.Context(), `SELECT gear FROM nation_economic_strategy WHERE nation_id=?`, id).Scan(&gear)
	provinceSetup := []map[string]any{}
	rows, rowsErr := a.db.QueryContext(r.Context(), `SELECT c.name,c.infrastructure,p.specialization FROM cities c JOIN province_economies p ON p.city_id=c.id WHERE c.nation_id=? ORDER BY c.created_at`, id)
	if rowsErr == nil {
		defer rows.Close()
		for rows.Next() {
			var provinceName, specialization string
			var infrastructure float64
			if rows.Scan(&provinceName, &infrastructure, &specialization) == nil {
				provinceSetup = append(provinceSetup, map[string]any{"name": provinceName, "infrastructure": infrastructure, "specialization": specialization})
			}
		}
	}
	military := loadMilitaryOverview(r.Context(), a.db, id)
	write(w, 200, map[string]any{"id": id, "name": name, "leaderName": leader, "government": government, "continent": continent, "motto": motto, "userType": userType, "population": population, "capital": capital, "cityCount": cityCount, "createdAt": created, "lastActiveAt": lastActive, "guardianUntil": guardianUntil, "economicGear": gear, "provinceSetup": provinceSetup, "military": military, "allianceID": allianceID, "allianceName": allianceName, "locationLat": locationLat, "locationLng": locationLng})
}
