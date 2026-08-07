package main

import (
	"database/sql"
	"math"
	"net/http"
	"strings"
	"time"
)

func (a *app) cities(w http.ResponseWriter, r *http.Request, u user) {
	rows, e := a.db.Query(r.Context(), `SELECT c.id,c.name,c.level,c.total_invested,c.improvement_slots,c.population_capacity,(SELECT count(*) FROM city_investments ci WHERE ci.city_id=c.id),COALESCE(GROUP_CONCAT(CONCAT(i.resource,':',i.level)), '') FROM cities c JOIN nations n ON n.id=c.nation_id LEFT JOIN city_industries i ON i.city_id=c.id WHERE n.owner_id=? GROUP BY c.id,c.name,c.level,c.total_invested,c.improvement_slots,c.population_capacity ORDER BY c.created_at`, u.ID)
	if e != nil {
		problem(w, 500, "Cities unavailable.")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, industries string
		var level, slots, used int
		var invested, populationCapacity int64
		rows.Scan(&id, &name, &level, &invested, &slots, &populationCapacity, &used, &industries)
		expandCost := int64(20000) << max(0, slots-2)
		out = append(out, map[string]any{"id": id, "name": name, "level": level, "totalInvested": invested, "improvementSlots": slots, "usedSlots": used, "populationCapacity": populationCapacity, "nextExpansionCost": expandCost, "industries": industries})
	}
	var cityCount int
	var last sql.NullTime
	a.db.QueryRow(r.Context(), `SELECT count(*),max(c.created_at) FROM cities c JOIN nations n ON n.id=c.nation_id WHERE n.owner_id=?`, u.ID).Scan(&cityCount, &last)
	newCityCost := int64(50000) << max(0, cityCount-1)
	var nextCityAt *time.Time
	if cityCount > 1 && last.Valid {
		t := last.Time.Add(7 * 24 * time.Hour)
		nextCityAt = &t
	}
	write(w, 200, map[string]any{"cities": out, "newCityCost": newCityCost, "nextCityAt": nextCityAt})
}

func (a *app) createCity(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Name string }
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 2 {
		problem(w, 400, "Enter a Province name.")
		return
	}
	tx, _ := a.db.Begin(r.Context())
	defer tx.Rollback(r.Context())
	var nid, gear string
	var cash int64
	var count int
	var lastCreated sql.NullTime
	if tx.QueryRow(r.Context(), `SELECT n.id,n.treasury,s.gear,(SELECT count(*) FROM cities WHERE nation_id=n.id),(SELECT max(created_at) FROM cities WHERE nation_id=n.id) FROM nations n JOIN nation_economic_strategy s ON s.nation_id=n.id WHERE n.owner_id=? FOR UPDATE`, u.ID).Scan(&nid, &cash, &gear, &count, &lastCreated) != nil {
		return
	}
	if count < 1 {
		problem(w, 409, "Every nation must found its capital first.")
		return
	}
	if count > 1 && lastCreated.Valid && time.Now().Before(lastCreated.Time.Add(7*24*time.Hour)) {
		problem(w, 429, "A new Province may be founded seven days after the previous Province.")
		return
	}
	policies := map[string]bool{}
	policyRows, _ := tx.QueryContext(r.Context(), `SELECT policy_key FROM social_policy_selections WHERE nation_id=?`, nid)
	if policyRows != nil {
		for policyRows.Next() {
			var key string
			if policyRows.Scan(&key) == nil {
				policies[key] = true
			}
		}
		policyRows.Close()
	}
	cost, constructionCost, happinessStrain := provinceFoundingCosts(count, gear, policies)
	if cash < cost {
		problem(w, 409, "Insufficient treasury to found this Province.")
		return
	}
	var constructionAvailable float64
	tx.QueryRow(r.Context(), `SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity='construction_materials' FOR UPDATE`, nid).Scan(&constructionAvailable)
	if constructionAvailable+0.0001 < constructionCost {
		problem(w, 409, "Insufficient Construction Materials to found this Province.")
		return
	}
	tx.Exec(r.Context(), `UPDATE nations SET treasury=treasury-?,happiness=GREATEST(0,happiness-?) WHERE id=?`, cost, happinessStrain, nid)
	tx.Exec(r.Context(), `UPDATE nation_stockpiles SET amount=GREATEST(0,amount-?) WHERE nation_id=? AND commodity='construction_materials'`, constructionCost, nid)
	provinceID := uuid()
	if _, e := tx.Exec(r.Context(), `INSERT INTO cities(id,nation_id,name) VALUES(?,?,?)`, provinceID, nid, in.Name); e != nil {
		problem(w, 409, "That Province name is unavailable.")
		return
	}
	tx.Exec(r.Context(), `INSERT INTO province_economies(city_id) VALUES(?)`, provinceID)
	for _, resource := range []string{"foodstuffs", "timber", "fibers", "basic_metals", "energy", "strategic_minerals"} {
		richness := .8 + float64(len(in.Name+resource)%8)/10
		tx.Exec(r.Context(), `INSERT INTO province_deposits(city_id,resource,richness) VALUES(?,?,?)`, provinceID, resource, richness)
	}
	tx.Exec(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'province_founding',?,?)`, uuid(), nid, -cost, "Founded Province "+in.Name)
	tx.Exec(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'game','Province founded',?)`, uuid(), nid, "You founded "+in.Name+" and accepted the associated administrative strain.")
	if e := tx.Commit(r.Context()); e != nil {
		problem(w, 500, "Could not complete Province founding.")
		return
	}
	write(w, 201, map[string]any{"ok": true, "cost": cost, "constructionMaterials": math.Round(constructionCost*100) / 100, "happinessStrain": happinessStrain, "nextProvinceAt": time.Now().Add(7 * 24 * time.Hour)})
}

func (a *app) investCity(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ CityID, Program string }
	if !decode(w, r, &in) {
		return
	}
	programs := map[string]bool{"transit": true, "housing": true, "healthcare": true, "schools": true, "commercial": true}
	if !programs[in.Program] {
		problem(w, 400, "Unknown city development program.")
		return
	}
	tx, _ := a.db.Begin(r.Context())
	defer tx.Rollback(r.Context())
	var nid string
	var cash int64
	var slots, used, daily int
	if tx.QueryRow(r.Context(), `SELECT n.id,n.treasury,c.improvement_slots,(SELECT count(*) FROM city_investments i WHERE i.city_id=c.id),(SELECT count(*) FROM city_investments i WHERE i.nation_id=n.id AND i.created_at>=CURRENT_DATE()) FROM nations n JOIN cities c ON c.nation_id=n.id WHERE n.owner_id=? AND c.id=? FOR UPDATE`, u.ID, in.CityID).Scan(&nid, &cash, &slots, &used, &daily) != nil {
		problem(w, 404, "City not found.")
		return
	}
	if daily >= 5 {
		problem(w, 429, "Your nation has reached its five city investments for today.")
		return
	}
	if used >= slots {
		problem(w, 409, "This city has no open improvement slots. Expand it or found another city.")
		return
	}
	cost := int64(10000 * (used + 1) * (used + 1))
	if cash < cost {
		problem(w, 409, "Insufficient treasury for this program.")
		return
	}
	tx.Exec(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, cost, nid)
	tx.Exec(r.Context(), `UPDATE cities SET total_invested=total_invested+? WHERE id=?`, cost, in.CityID)
	tx.Exec(r.Context(), `INSERT INTO city_investments(id,city_id,nation_id,program,amount) VALUES(?,?,?,?,?)`, uuid(), in.CityID, nid, in.Program, cost)
	tx.Exec(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'city_development',?,?)`, uuid(), nid, -cost, "Municipal program: "+in.Program)
	if e := tx.Commit(r.Context()); e != nil {
		problem(w, 500, "Investment failed.")
		return
	}
	write(w, 200, map[string]any{"ok": true, "cost": cost, "usedSlots": used + 1})
}

func (a *app) expandCity(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ CityID string }
	if !decode(w, r, &in) {
		return
	}
	tx, _ := a.db.Begin(r.Context())
	defer tx.Rollback(r.Context())
	var nid string
	var cash int64
	var slots, level int
	if tx.QueryRow(r.Context(), `SELECT n.id,n.treasury,c.improvement_slots,c.level FROM nations n JOIN cities c ON c.nation_id=n.id WHERE n.owner_id=? AND c.id=? FOR UPDATE`, u.ID, in.CityID).Scan(&nid, &cash, &slots, &level) != nil {
		problem(w, 404, "City not found.")
		return
	}
	if slots >= 12 {
		problem(w, 409, "This city has reached its maximum capacity. Found a new city.")
		return
	}
	cost := int64(20000) << max(0, slots-2)
	if cash < cost {
		problem(w, 409, "Insufficient treasury for this expansion.")
		return
	}
	tx.Exec(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, cost, nid)
	tx.Exec(r.Context(), `UPDATE cities SET improvement_slots=improvement_slots+1,level=level+1,total_invested=total_invested+? WHERE id=?`, cost, in.CityID)
	tx.Exec(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'city_expansion',?,?)`, uuid(), nid, -cost, "Expanded city capacity")
	if e := tx.Commit(r.Context()); e != nil {
		problem(w, 500, "Expansion failed.")
		return
	}
	write(w, 200, map[string]any{"ok": true, "cost": cost, "slots": slots + 1, "level": level + 1})
}

func (a *app) investIndustry(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ CityID, Resource string }
	if !decode(w, r, &in) {
		return
	}
	required := map[string]int{"food": 2, "coal": 3, "steel": 5}[in.Resource]
	if required == 0 {
		problem(w, 400, "Unknown industry.")
		return
	}
	tx, _ := a.db.Begin(r.Context())
	defer tx.Rollback(r.Context())
	var nid string
	var cash int64
	var cityLevel, current, slots, used int
	if tx.QueryRow(r.Context(), `SELECT n.id,n.treasury,c.level,COALESCE((SELECT level FROM city_industries WHERE city_id=c.id AND resource=?),0),c.improvement_slots,(SELECT count(*) FROM city_investments WHERE city_id=c.id) FROM nations n JOIN cities c ON c.nation_id=n.id WHERE n.owner_id=? AND c.id=? FOR UPDATE`, in.Resource, u.ID, in.CityID).Scan(&nid, &cash, &cityLevel, &current, &slots, &used) != nil {
		problem(w, 404, "City not found.")
		return
	}
	if cityLevel < required {
		problem(w, 409, "Develop this city further before establishing that industry.")
		return
	}
	if used >= slots {
		problem(w, 409, "This city has no open improvement slots.")
		return
	}
	cost := int64(20000 * (current + 1) * (current + 1))
	if cash < cost {
		problem(w, 409, "Insufficient treasury.")
		return
	}
	tx.Exec(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, cost, nid)
	tx.Exec(r.Context(), `INSERT INTO city_industries(id,city_id,resource,level,total_invested) VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE level=level+1,total_invested=total_invested+VALUES(total_invested)`, uuid(), in.CityID, in.Resource, 1, cost)
	tx.Exec(r.Context(), `INSERT INTO city_investments(id,city_id,nation_id,program,amount) VALUES(?,?,?,?,?)`, uuid(), in.CityID, nid, "industry_"+in.Resource, cost)
	tx.Exec(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'industry',?,?)`, uuid(), nid, -cost, "Expanded "+in.Resource+" production")
	tx.Commit(r.Context())
	write(w, 200, map[string]any{"ok": true, "cost": cost})
}

func (a *app) income(w http.ResponseWriter, r *http.Request, u user) {
	var pop, treasury int64
	var employment, education, satisfaction, technology float64
	var nid string
	if a.db.QueryRow(r.Context(), `SELECT id,population,treasury,employment_rate,education,happiness,technology FROM nations WHERE owner_id=?`, u.ID).Scan(&nid, &pop, &treasury, &employment, &education, &satisfaction, &technology) != nil {
		return
	}
	var food, coal, steel int64
	a.db.QueryRow(r.Context(), `SELECT COALESCE(sum(CASE resource WHEN 'food' THEN level*5 ELSE 0 END),0),COALESCE(sum(CASE resource WHEN 'coal' THEN level*2 ELSE 0 END),0),COALESCE(sum(CASE resource WHEN 'steel' THEN level ELSE 0 END),0) FROM city_industries i JOIN cities c ON c.id=i.city_id WHERE c.nation_id=?`, nid).Scan(&food, &coal, &steel)
	baseDaily := float64(pop) * 0.02
	employmentFactor := employment / 100
	educationFactor := 0.5 + education/200
	satisfactionFactor := 0.5 + satisfaction/200
	productivityFactor := 1 + technology/500
	dailyCash := int64(baseDaily * employmentFactor * educationFactor * satisfactionFactor * productivityFactor)
	hourlyCash := dailyCash / 24
	var populationCapacity int64
	a.db.QueryRow(r.Context(), `SELECT COALESCE(sum(population_capacity),0) FROM cities WHERE nation_id=?`, nid).Scan(&populationCapacity)
	dailyPopulationGrowth := int64(float64(pop) * 0.002 * satisfactionFactor)
	hourlyPopulationGrowth := dailyPopulationGrowth / 24
	if remaining := populationCapacity - pop; remaining < hourlyPopulationGrowth {
		hourlyPopulationGrowth = max(int64(0), remaining)
	}
	now := time.Now().UTC()
	next := now.Truncate(time.Hour).Add(time.Hour)
	write(w, 200, map[string]any{"hourlyCash": hourlyCash, "dailyCash": hourlyCash * 24, "baseTaxCapacityDaily": int64(baseDaily), "factors": map[string]any{"employment": employmentFactor, "education": educationFactor, "satisfaction": satisfactionFactor, "productivity": productivityFactor}, "hourlyResources": map[string]int64{"food": food, "coal": coal, "steel": steel}, "dailyResources": map[string]int64{"food": food * 24, "coal": coal * 24, "steel": steel * 24}, "population": pop, "populationCapacity": populationCapacity, "hourlyPopulationGrowth": hourlyPopulationGrowth, "dailyPopulationGrowth": hourlyPopulationGrowth * 24, "treasury": treasury, "nextTurnAt": next})
}

func (a *app) worldStatus(w http.ResponseWriter, r *http.Request, u user) {
	var active int
	a.db.QueryRow(r.Context(), `SELECT count(DISTINCT user_id) FROM sessions WHERE last_action_at>=DATE_SUB(NOW(),INTERVAL 5 MINUTE)`).Scan(&active)
	now := time.Now().UTC()
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	game := epoch.Add(now.Sub(epoch) * 4)
	write(w, 200, map[string]any{"realTime": now, "gameTime": game, "gameSpeed": 4, "activePlayers": active})
}
