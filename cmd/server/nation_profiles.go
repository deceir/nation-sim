package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var governmentTypes = map[string]bool{"Parliamentary Democracy": true, "Presidential Republic": true, "Federal Republic": true, "Constitutional Monarchy": true, "Absolute Monarchy": true, "One-Party State": true, "Military Junta": true, "Theocracy": true}
var continents = map[string]bool{"Africa": true, "Asia": true, "Europe": true, "North America": true, "South America": true, "Oceania": true, "Antarctica": true}

type foundingProfile struct{ LeaderName, NationName, Capital, Government, Continent string }

// validRomanName keeps public nation, leader, and Alliance identities readable
// throughout the shared world UI while allowing ordinary Western name punctuation.
func validRomanName(value string) bool {
	value = strings.TrimSpace(value)
	hasLetter := false
	for _, character := range value {
		switch {
		case character >= 'A' && character <= 'Z', character >= 'a' && character <= 'z':
			hasLetter = true
		case strings.ContainsRune(" .,'&()-", character):
		default:
			return false
		}
	}
	return hasLetter
}

func nationUserType(name string) string {
	if strings.EqualFold(strings.TrimSpace(name), "Japan") {
		return "DEV"
	}
	return "PLAYER"
}

func validateFoundingProfile(p foundingProfile) (foundingProfile, bool) {
	p.LeaderName = strings.TrimSpace(p.LeaderName)
	p.NationName = strings.TrimSpace(p.NationName)
	var validCapital bool
	p.Capital, validCapital = normalizeProvinceName(p.Capital)
	if len(p.LeaderName) < 2 || len(p.LeaderName) > 100 || len(p.NationName) < 3 || len(p.NationName) > 100 || !validRomanName(p.LeaderName) || !validRomanName(p.NationName) || !validCapital || !governmentTypes[p.Government] || !continents[p.Continent] {
		return p, false
	}
	return p, true
}

func (a *app) nationDirectory(w http.ResponseWriter, r *http.Request, u user) {
	q := strings.TrimSpace(r.URL.Query().Get("search"))
	q = strings.ReplaceAll(strings.ReplaceAll(q, "\\", "\\\\"), "%", "\\%")
	q = strings.ReplaceAll(q, "_", "\\_")
	orderBy := powerLevelSQL("c", "p", "m") + " DESC"
	switch r.URL.Query().Get("sort") {
	case "population":
		orderBy = "n.population DESC"
	case "provinces":
		orderBy = "c.province_count DESC"
	case "founded":
		orderBy = "n.created_at DESC"
	case "name":
		orderBy = "n.name ASC"
	case "powerLevel", "":
		orderBy = powerLevelSQL("c", "p", "m") + " DESC"
	}
	query := fmt.Sprintf(`SELECT n.id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,n.population,n.created_at,COALESCE(c.province_count,0),COALESCE(a.id,''),COALESCE(a.name,''),n.location_lat,n.location_lng,
		COALESCE(c.total_infrastructure,0),COALESCE(p.project_count,0),COALESCE(m.soldiers,0),COALESCE(m.tanks,0),COALESCE(m.ships,0),COALESCE(m.jets,0),COALESCE(m.drones,0)
		FROM nations n
		LEFT JOIN alliance_members am ON am.nation_id=n.id LEFT JOIN alliances a ON a.id=am.alliance_id
		LEFT JOIN (SELECT nation_id,COUNT(*) province_count,COALESCE(SUM(infrastructure),0) total_infrastructure FROM cities GROUP BY nation_id) c ON c.nation_id=n.id
		LEFT JOIN (SELECT nation_id,COUNT(*) project_count FROM (SELECT nation_id FROM national_projects UNION ALL SELECT nation_id FROM national_long_term_projects) completed_projects GROUP BY nation_id) p ON p.nation_id=n.id
		LEFT JOIN (SELECT nation_id,
			SUM(CASE WHEN unit_type='soldiers' THEN quantity ELSE 0 END) soldiers,SUM(CASE WHEN unit_type='tanks' THEN quantity ELSE 0 END) tanks,
			SUM(CASE WHEN unit_type='ships' THEN quantity ELSE 0 END) ships,SUM(CASE WHEN unit_type='jets' THEN quantity ELSE 0 END) jets,SUM(CASE WHEN unit_type='drones' THEN quantity ELSE 0 END) drones
			FROM (SELECT nation_id,unit_type,quantity FROM military_inventory UNION ALL SELECT nation_id,resource,CAST(escrow_goods AS SIGNED) FROM market_orders WHERE side='sell' AND status IN('open','pending') AND resource IN('tanks','ships','jets','drones')) holdings GROUP BY nation_id) m ON m.nation_id=n.id
		WHERE NOT EXISTS(SELECT 1 FROM user_bans b WHERE b.user_id=n.owner_id AND (b.expires_at IS NULL OR b.expires_at>NOW())) AND (?='' OR n.name LIKE CONCAT('%%',?,'%%') ESCAPE '\\' OR n.leader_name LIKE CONCAT('%%',?,'%%') ESCAPE '\\' OR a.name LIKE CONCAT('%%',?,'%%') ESCAPE '\\')
		ORDER BY %s,n.name LIMIT 100`, orderBy)
	rows, e := a.db.Query(r.Context(), query, q, q, q, q)
	if e != nil {
		problem(w, 500, "Nation directory unavailable.")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, leader, government, continent, motto, userType, allianceID, allianceName string
		var cityCount int
		var population, soldiers, tanks, ships, jets, drones int64
		var totalInfrastructure float64
		var projectCount int
		var createdAt time.Time
		var locationLat, locationLng *float64
		if rows.Scan(&id, &name, &leader, &government, &continent, &motto, &userType, &population, &createdAt, &cityCount, &allianceID, &allianceName, &locationLat, &locationLng, &totalInfrastructure, &projectCount, &soldiers, &tanks, &ships, &jets, &drones) != nil {
			continue
		}
		powerLevel := calculatePowerLevel(powerLevelComponents{Provinces: cityCount, Infrastructure: totalInfrastructure, Projects: projectCount, Soldiers: soldiers, Tanks: tanks, Ships: ships, Jets: jets, Drones: drones}).Total
		out = append(out, map[string]any{"id": id, "name": name, "leaderName": leader, "government": government, "continent": continent, "motto": motto, "userType": userType, "population": population, "powerLevel": powerLevel, "createdAt": createdAt, "cityCount": cityCount, "allianceID": allianceID, "allianceName": allianceName, "locationLat": locationLat, "locationLng": locationLng})
	}
	write(w, 200, out)
}

func (a *app) nationProfile(w http.ResponseWriter, r *http.Request, u user) {
	var id, ownerID, name, leader, government, continent, motto, capital, userType, allianceID, allianceName, allianceRole string
	var cityCount int
	var population int64
	var created time.Time
	var lastActive, guardianUntil *time.Time
	var locationLat, locationLng *float64
	e := a.db.QueryRow(r.Context(), `SELECT n.id,n.owner_id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,n.population,n.created_at,count(DISTINCT c.id),COALESCE((SELECT name FROM cities WHERE id=n.capital_city_id AND nation_id=n.id),(SELECT name FROM cities WHERE nation_id=n.id ORDER BY created_at ASC,id ASC LIMIT 1),''),COALESCE(a.id,''),COALESCE(a.name,''),COALESCE(ar.title,''),(SELECT MAX(s.last_action_at) FROM sessions s WHERE s.user_id=n.owner_id),(SELECT MAX(g.expires_at) FROM guardian_grants g WHERE g.nation_id=n.id AND g.revoked_at IS NULL AND g.starts_at<=NOW() AND g.expires_at>NOW()),n.location_lat,n.location_lng FROM nations n LEFT JOIN cities c ON c.nation_id=n.id LEFT JOIN alliance_members am ON am.nation_id=n.id LEFT JOIN alliances a ON a.id=am.alliance_id LEFT JOIN alliance_roles ar ON ar.id=am.role_id WHERE n.id=? GROUP BY n.id,n.name,n.leader_name,n.government_type,n.continent,n.motto,n.user_type,n.population,n.created_at,n.owner_id,n.capital_city_id,n.location_lat,n.location_lng,a.id,a.name,ar.title`, r.PathValue("id")).Scan(&id, &ownerID, &name, &leader, &government, &continent, &motto, &userType, &population, &created, &cityCount, &capital, &allianceID, &allianceName, &allianceRole, &lastActive, &guardianUntil, &locationLat, &locationLng)
	if e != nil {
		var bannedReason string
		var bannedUntil sql.NullTime
		if a.db.QueryRowContext(r.Context(), `SELECT b.reason,b.expires_at FROM nations n JOIN user_bans b ON b.user_id=n.owner_id WHERE n.id=? AND (b.expires_at IS NULL OR b.expires_at>NOW())`, r.PathValue("id")).Scan(&bannedReason, &bannedUntil) == nil {
			until := "indefinitely"
			if bannedUntil.Valid {
				until = bannedUntil.Time.Format("2006-01-02")
			}
			problem(w, http.StatusGone, "This nation is unavailable because its user is banned until "+until+".")
			return
		}
		problem(w, 404, "Nation not found.")
		return
	}
	gear := "balanced"
	a.db.QueryRowContext(r.Context(), `SELECT gear FROM nation_economic_strategy WHERE nation_id=?`, id).Scan(&gear)
	provinceSetup := []map[string]any{}
	rows, rowsErr := a.db.QueryContext(r.Context(), `SELECT c.name,c.infrastructure,p.specialization,COALESCE(c.id=n.capital_city_id,0) FROM cities c JOIN province_economies p ON p.city_id=c.id JOIN nations n ON n.id=c.nation_id WHERE c.nation_id=? ORDER BY COALESCE(c.id=n.capital_city_id,0) DESC,c.created_at ASC,c.id ASC`, id)
	if rowsErr == nil {
		defer rows.Close()
		for rows.Next() {
			var provinceName, specialization string
			var infrastructure float64
			var isCapital bool
			if rows.Scan(&provinceName, &infrastructure, &specialization, &isCapital) == nil {
				provinceSetup = append(provinceSetup, map[string]any{"name": provinceName, "infrastructure": infrastructure, "specialization": specialization, "isCapital": isCapital})
			}
		}
	}
	military := loadMilitaryOverview(r.Context(), a.db, id)
	powerLevel, _ := loadNationPowerLevel(r.Context(), a.db, id)
	var details nationalDetails
	if economicNation, _, _, detailsErr := a.loadEconomicNationContext(r.Context(), ownerID); detailsErr == nil {
		details = buildNationalDetails(economicNation, calculateEconomy(economicNation))
	}
	gdp, _, gdpErr := a.projectedGDPForOwner(r.Context(), ownerID)
	if gdpErr == nil {
		a.db.ExecContext(r.Context(), `UPDATE nations SET gdp=? WHERE id=?`, gdp, id)
	}
	write(w, 200, map[string]any{"id": id, "name": name, "leaderName": leader, "government": government, "continent": continent, "motto": motto, "userType": userType, "population": population, "powerLevel": powerLevel.Total, "powerLevelBreakdown": powerLevel, "gdp": gdp, "capital": capital, "cityCount": cityCount, "createdAt": created, "lastActiveAt": lastActive, "guardianUntil": guardianUntil, "economicGear": gear, "provinceSetup": provinceSetup, "military": military, "nationalDetails": details, "allianceID": allianceID, "allianceName": allianceName, "allianceRole": allianceRole, "locationLat": locationLat, "locationLng": locationLng})
}
