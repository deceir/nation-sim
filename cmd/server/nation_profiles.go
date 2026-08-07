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
	rows, e := a.db.Query(r.Context(), `SELECT n.id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,count(DISTINCT c.id),COALESCE(a.id,''),COALESCE(a.name,'') FROM nations n LEFT JOIN cities c ON c.nation_id=n.id LEFT JOIN alliance_members am ON am.nation_id=n.id LEFT JOIN alliances a ON a.id=am.alliance_id WHERE (?='' OR n.name LIKE CONCAT('%',?,'%') ESCAPE '\\' OR n.leader_name LIKE CONCAT('%',?,'%') ESCAPE '\\' OR a.name LIKE CONCAT('%',?,'%') ESCAPE '\\') GROUP BY n.id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,a.id,a.name ORDER BY n.name LIMIT 100`, q, q, q, q)
	if e != nil {
		problem(w, 500, "Nation directory unavailable.")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, leader, government, continent, motto, userType, allianceID, allianceName string
		var cityCount int
		rows.Scan(&id, &name, &leader, &government, &continent, &motto, &userType, &cityCount, &allianceID, &allianceName)
		out = append(out, map[string]any{"id": id, "name": name, "leaderName": leader, "government": government, "continent": continent, "motto": motto, "userType": userType, "cityCount": cityCount, "allianceID": allianceID, "allianceName": allianceName})
	}
	write(w, 200, out)
}

func (a *app) nationProfile(w http.ResponseWriter, r *http.Request, u user) {
	var id, name, leader, government, continent, motto, capital, userType, allianceID, allianceName string
	var cityCount int
	var created time.Time
	var lastActive *time.Time
	e := a.db.QueryRow(r.Context(), `SELECT n.id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,n.created_at,count(DISTINCT c.id),COALESCE((SELECT name FROM cities WHERE nation_id=n.id ORDER BY created_at LIMIT 1),''),COALESCE(a.id,''),COALESCE(a.name,''),(SELECT MAX(s.last_action_at) FROM sessions s WHERE s.user_id=n.owner_id) FROM nations n LEFT JOIN cities c ON c.nation_id=n.id LEFT JOIN alliance_members am ON am.nation_id=n.id LEFT JOIN alliances a ON a.id=am.alliance_id WHERE n.id=? GROUP BY n.id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,n.created_at,n.owner_id,a.id,a.name`, r.PathValue("id")).Scan(&id, &name, &leader, &government, &continent, &motto, &userType, &created, &cityCount, &capital, &allianceID, &allianceName, &lastActive)
	if e != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	write(w, 200, map[string]any{"id": id, "name": name, "leaderName": leader, "government": government, "continent": continent, "motto": motto, "userType": userType, "capital": capital, "cityCount": cityCount, "createdAt": created, "lastActiveAt": lastActive, "allianceID": allianceID, "allianceName": allianceName})
}
