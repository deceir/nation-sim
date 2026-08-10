package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type leaderboardQuery struct {
	Metric, Order, Continent, Search string
	MinProvinces, MaxProvinces       int
	Page, PageSize                   int
}

var leaderboardMetrics = map[string]string{
	"population": "n.population",
	"gdp":        "n.gdp",
	"provinces":  "province_count",
	"soldiers":   "soldiers",
	"tanks":      "tanks",
	"ships":      "ships",
	"jets":       "jets",
	"drones":     "drones",
}

func leaderboardParameters(values url.Values) leaderboardQuery {
	result := leaderboardQuery{Metric: values.Get("metric"), Order: strings.ToLower(values.Get("order")), Continent: values.Get("continent"), Search: strings.TrimSpace(values.Get("search")), Page: 1, PageSize: 10}
	if _, ok := leaderboardMetrics[result.Metric]; !ok {
		result.Metric = "population"
	}
	if result.Order != "asc" {
		result.Order = "desc"
	}
	if result.Continent != "" && !continents[result.Continent] {
		result.Continent = ""
	}
	result.MinProvinces, _ = strconv.Atoi(values.Get("minProvinces"))
	result.MaxProvinces, _ = strconv.Atoi(values.Get("maxProvinces"))
	result.Page, _ = strconv.Atoi(values.Get("page"))
	result.PageSize, _ = strconv.Atoi(values.Get("pageSize"))
	if result.MinProvinces < 0 {
		result.MinProvinces = 0
	}
	if result.MaxProvinces < 0 || result.MaxProvinces > 1000000 {
		result.MaxProvinces = 0
	}
	if result.MaxProvinces > 0 && result.MinProvinces > result.MaxProvinces {
		result.MinProvinces, result.MaxProvinces = result.MaxProvinces, result.MinProvinces
	}
	if result.Page < 1 {
		result.Page = 1
	}
	if result.PageSize < 1 || result.PageSize > 50 {
		result.PageSize = 10
	}
	return result
}

func (a *app) leaderboards(w http.ResponseWriter, r *http.Request, _ user) {
	filters := leaderboardParameters(r.URL.Query())
	continent, search := filters.Continent, "%"+filters.Search+"%"
	var total int
	countQuery := `SELECT COUNT(*) FROM nations n WHERE (?='' OR n.continent=?) AND (?='' OR n.name LIKE ?) AND (SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id)>=? AND (?=0 OR (SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id)<=?)`
	if err := a.db.QueryRowContext(r.Context(), countQuery, continent, continent, filters.Search, search, filters.MinProvinces, filters.MaxProvinces, filters.MaxProvinces).Scan(&total); err != nil {
		problem(w, 500, "Leaderboards are temporarily unavailable.")
		return
	}
	orderColumn := leaderboardMetrics[filters.Metric]
	query := fmt.Sprintf(`SELECT n.id,n.name,n.leader_name,n.continent,n.population,n.gdp,(SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id) province_count,COALESCE(a.id,''),COALESCE(a.name,''),COALESCE(m.soldiers,0),COALESCE(m.tanks,0),COALESCE(m.ships,0),COALESCE(m.jets,0),COALESCE(m.drones,0)
		FROM nations n
		LEFT JOIN alliance_members am ON am.nation_id=n.id
		LEFT JOIN alliances a ON a.id=am.alliance_id
		LEFT JOIN (
			SELECT nation_id,
			SUM(CASE WHEN unit_type='soldiers' THEN quantity ELSE 0 END) soldiers,
			SUM(CASE WHEN unit_type='tanks' THEN quantity ELSE 0 END) tanks,
			SUM(CASE WHEN unit_type='ships' THEN quantity ELSE 0 END) ships,
			SUM(CASE WHEN unit_type='jets' THEN quantity ELSE 0 END) jets,
			SUM(CASE WHEN unit_type='drones' THEN quantity ELSE 0 END) drones
			FROM (
				SELECT nation_id,unit_type,quantity FROM military_inventory
				UNION ALL
				SELECT nation_id,resource,CAST(escrow_goods AS SIGNED) FROM market_orders WHERE side='sell' AND status IN('open','pending') AND resource IN('tanks','ships','jets','drones')
			) holdings GROUP BY nation_id
		) m ON m.nation_id=n.id
		WHERE (?='' OR n.continent=?) AND (?='' OR n.name LIKE ?) AND (SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id)>=? AND (?=0 OR (SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id)<=?)
		ORDER BY %s %s,n.name ASC LIMIT ? OFFSET ?`, orderColumn, strings.ToUpper(filters.Order))
	rows, err := a.db.QueryContext(r.Context(), query, continent, continent, filters.Search, search, filters.MinProvinces, filters.MaxProvinces, filters.MaxProvinces, filters.PageSize, (filters.Page-1)*filters.PageSize)
	if err != nil {
		problem(w, 500, "Leaderboards are temporarily unavailable.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	position := (filters.Page-1)*filters.PageSize + 1
	for rows.Next() {
		var id, name, leader, continentName, allianceID, allianceName string
		var population, gdp, soldiers, tanks, ships, jets, drones int64
		var provinces int
		if rows.Scan(&id, &name, &leader, &continentName, &population, &gdp, &provinces, &allianceID, &allianceName, &soldiers, &tanks, &ships, &jets, &drones) != nil {
			continue
		}
		items = append(items, map[string]any{"rank": position, "id": id, "name": name, "leaderName": leader, "continent": continentName, "population": population, "gdp": gdp, "provinces": provinces, "allianceID": allianceID, "allianceName": allianceName, "soldiers": soldiers, "tanks": tanks, "ships": ships, "jets": jets, "drones": drones})
		position++
	}
	write(w, 200, map[string]any{"metric": filters.Metric, "order": filters.Order, "page": filters.Page, "pageSize": filters.PageSize, "total": total, "items": items})
}
