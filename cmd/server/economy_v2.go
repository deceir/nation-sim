package main

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

func (a *app) economyDashboard(w http.ResponseWriter, r *http.Request, u user) {
	n, nid, cash, err := a.loadEconomicNationContext(r.Context(), u.ID)
	if err != nil {
		problem(w, 500, "Economy unavailable.")
		return
	}
	result := calculateEconomy(n)
	if strategy, e := a.loadStrategy(r.Context(), nid); e == nil {
		strategic := calculateStrategy(strategy)
		result.DailyTax *= strategic.IncomeMultiplier
		result.NetDailyCash = result.DailyTax - result.DailyUpkeep
		for i := range result.Cities {
			result.Cities[i].TaxRevenue *= strategic.IncomeMultiplier
		}
		result.Contributors["economicGearIncome"] = strategic.IncomeMultiplier
		result.DailyFoodProduction = strategic.Production["foodstuffs"]
		result.NetDailyFood = result.DailyFoodProduction - result.DailyFoodConsumption
	}
	alliance := map[string]any{"name": "", "taxRate": float64(0), "projectedDailyTax": int64(0)}
	var allianceName string
	var allianceRate float64
	if a.db.QueryRowContext(r.Context(), `SELECT a.name,a.tax_rate FROM alliance_members m JOIN alliances a ON a.id=m.alliance_id WHERE m.nation_id=?`, nid).Scan(&allianceName, &allianceRate) == nil {
		alliance = map[string]any{"name": allianceName, "taxRate": allianceRate, "projectedDailyTax": int64(math.Max(0, result.NetDailyCash) * allianceRate / 100)}
	}
	types := make([]map[string]any, 0, len(buildings))
	keys := make([]string, 0, len(buildings))
	for k := range buildings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := buildings[k]
		types = append(types, map[string]any{"key": k, "name": s.Name, "category": s.Category, "cost": int64(s.Cost), "power": s.Power, "pollution": s.Pollution, "outputResource": s.OutputResource, "output": s.Output, "commerce": s.Commerce, "minTech": s.MinTech})
	}
	projectKeys := make([]string, 0, len(beginnerProjects))
	for k := range beginnerProjects {
		projectKeys = append(projectKeys, k)
	}
	sort.Strings(projectKeys)
	projects := []map[string]any{}
	for _, k := range projectKeys {
		p := beginnerProjects[k]
		projects = append(projects, map[string]any{"key": k, "name": p.Name, "theme": p.Theme, "description": p.Description, "cash": p.Cash, "iron": p.Iron, "steel": p.Steel, "aluminum": p.Aluminum, "coal": p.Coal, "food": p.Food, "completed": n.Projects[k]})
	}
	var totalInfra float64
	a.db.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(infrastructure),0) FROM cities WHERE nation_id=?`, nid).Scan(&totalInfra)
	slots := int(totalInfra / 300)
	write(w, 200, map[string]any{"nation": map[string]any{"taxRate": n.TaxRate, "happiness": n.Happiness, "education": n.Education, "technology": n.Technology, "doctrine": n.Doctrine, "treasury": cash}, "result": result, "alliance": alliance, "buildings": types, "projects": projects, "projectSlots": slots, "projectsCompleted": len(n.Projects), "nextTurnAt": nextHour()})
}

func (a *app) loadEconomicNationContext(ctx context.Context, owner string) (ModelNation, string, int64, error) {
	var n ModelNation
	n.Projects = map[string]bool{}
	var id string
	var cash int64
	err := a.db.QueryRowContext(ctx, `SELECT id,tax_rate,happiness,education,technology,doctrine,treasury FROM nations WHERE owner_id=?`, owner).Scan(&id, &n.TaxRate, &n.Happiness, &n.Education, &n.Technology, &n.Doctrine, &cash)
	if err != nil {
		return n, id, cash, err
	}
	rows, err := a.db.QueryContext(ctx, `SELECT id,name,infrastructure,land,local_population,commerce_percent,pollution,disease_rate,crime_rate FROM cities WHERE nation_id=? ORDER BY created_at`, id)
	if err != nil {
		return n, id, cash, err
	}
	defer rows.Close()
	for rows.Next() {
		var c ModelCity
		c.Buildings = map[string]int{}
		c.Upgrades = map[string]int{}
		if err = rows.Scan(&c.ID, &c.Name, &c.Infra, &c.Land, &c.Population, &c.Commerce, &c.Pollution, &c.Disease, &c.Crime); err != nil {
			return n, id, cash, err
		}
		n.Cities = append(n.Cities, c)
	}
	for i := range n.Cities {
		n.Cities[i].Upgrades = loadProvinceUpgrades(ctx, a.db, n.Cities[i].ID)
		bs, e := a.db.QueryContext(ctx, `SELECT building_type,quantity FROM city_improvements WHERE city_id=?`, n.Cities[i].ID)
		if e != nil {
			return n, id, cash, e
		}
		for bs.Next() {
			var k string
			var q int
			bs.Scan(&k, &q)
			n.Cities[i].Buildings[k] = q
		}
		bs.Close()
	}
	ps, e := a.db.QueryContext(ctx, `SELECT project_type FROM national_projects WHERE nation_id=?`, id)
	if e != nil {
		return n, id, cash, e
	}
	for ps.Next() {
		var k string
		ps.Scan(&k)
		n.Projects[k] = true
	}
	ps.Close()
	return n, id, cash, nil
}

func nextHour() time.Time { return time.Now().UTC().Truncate(time.Hour).Add(time.Hour) }

func (a *app) buyDevelopment(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		CityID, Kind string
		Amount       float64
	}
	if !decode(w, r, &in) || in.Amount <= 0 || in.Amount > 1000 {
		problem(w, 400, "Choose between 1 and 1,000 units.")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		return
	}
	defer tx.Rollback()
	var nid string
	var cash int64
	var infra, land float64
	var tech int
	e = tx.QueryRowContext(r.Context(), `SELECT n.id,n.treasury,n.technology,c.infrastructure,c.land FROM nations n JOIN cities c ON c.nation_id=n.id WHERE n.owner_id=? AND c.id=? FOR UPDATE`, u.ID, in.CityID).Scan(&nid, &cash, &tech, &infra, &land)
	if e != nil {
		problem(w, 404, "City not found.")
		return
	}
	cost := 0.0
	column := ""
	switch in.Kind {
	case "infrastructure":
		cost = infraPurchaseCost(infra, in.Amount, tech)
		var has int
		tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM national_projects WHERE nation_id=? AND project_type='civil_engineering_corps'`, nid).Scan(&has)
		if has > 0 {
			cost *= .94
		}
		column = "infrastructure"
	case "land":
		cost = landPurchaseCost(land, in.Amount, tech)
		column = "land"
	default:
		problem(w, 400, "Unknown development type.")
		return
	}
	if float64(cash) < cost {
		problem(w, 409, "Insufficient treasury.")
		return
	}
	if _, e = tx.ExecContext(r.Context(), `UPDATE cities SET `+column+`=`+column+`+?,total_invested=total_invested+? WHERE id=?`, in.Amount, int64(cost), in.CityID); e != nil {
		return
	}
	tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, int64(cost), nid)
	tx.ExecContext(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'city_development',?,?)`, uuid(), nid, -int64(cost), "Purchased "+in.Kind)
	if e = tx.Commit(); e != nil {
		return
	}
	write(w, 200, map[string]any{"cost": int64(cost)})
}

func (a *app) buildImprovement(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ CityID, Building string }
	if !decode(w, r, &in) {
		return
	}
	spec, ok := buildings[in.Building]
	if !ok {
		problem(w, 400, "Unknown improvement.")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		return
	}
	defer tx.Rollback()
	var nid string
	var cash int64
	var infra float64
	var tech, used int
	e = tx.QueryRowContext(r.Context(), `SELECT n.id,n.treasury,n.technology,c.infrastructure,COALESCE((SELECT SUM(quantity) FROM city_improvements WHERE city_id=c.id),0) FROM nations n JOIN cities c ON c.nation_id=n.id WHERE n.owner_id=? AND c.id=? FOR UPDATE`, u.ID, in.CityID).Scan(&nid, &cash, &tech, &infra, &used)
	if e != nil {
		problem(w, 404, "City not found.")
		return
	}
	if used >= max(1, int(infra/50)) {
		problem(w, 409, "Buy more infrastructure to unlock another improvement slot.")
		return
	}
	if tech < spec.MinTech {
		problem(w, 409, "Technology level is too low for this improvement.")
		return
	}
	if spec.Category == "power" {
		var has int
		tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM national_projects WHERE nation_id=? AND project_type='basic_power_grid'`, nid).Scan(&has)
		if has > 0 {
			spec.Cost *= .90
		}
	}
	if cash < int64(spec.Cost) {
		problem(w, 409, "Insufficient treasury.")
		return
	}
	_, e = tx.ExecContext(r.Context(), `INSERT INTO city_improvements(id,city_id,building_type,quantity) VALUES(?,?,?,1) ON DUPLICATE KEY UPDATE quantity=quantity+1`, uuid(), in.CityID, in.Building)
	if e != nil {
		return
	}
	tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, int64(spec.Cost), nid)
	tx.ExecContext(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'improvement',?,?)`, uuid(), nid, -int64(spec.Cost), "Built "+spec.Name)
	if e = tx.Commit(); e != nil {
		return
	}
	write(w, 201, map[string]any{"ok": true})
}

func (a *app) completeProject(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Project string }
	if !decode(w, r, &in) {
		return
	}
	p, ok := beginnerProjects[in.Project]
	if !ok {
		problem(w, 400, "Unknown National Project.")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		return
	}
	defer tx.Rollback()
	var nid string
	var cash, iron, steel, aluminum, coal, food int64
	var infra float64
	var completed int
	e = tx.QueryRowContext(r.Context(), `SELECT n.id,n.treasury,n.iron,n.steel,n.aluminum,n.coal,n.food,(SELECT COALESCE(SUM(infrastructure),0) FROM cities WHERE nation_id=n.id),(SELECT COUNT(*) FROM national_projects WHERE nation_id=n.id) FROM nations n WHERE owner_id=? FOR UPDATE`, u.ID).Scan(&nid, &cash, &iron, &steel, &aluminum, &coal, &food, &infra, &completed)
	if e != nil {
		return
	}
	if completed >= int(infra/300) {
		problem(w, 409, "Increase total national Infrastructure to unlock another project slot.")
		return
	}
	if cash < p.Cash || iron < p.Iron || steel < p.Steel || aluminum < p.Aluminum || coal < p.Coal || food < p.Food {
		problem(w, 409, "Your nation does not yet have the required cash and resources.")
		return
	}
	_, e = tx.ExecContext(r.Context(), `INSERT INTO national_projects(id,nation_id,project_type) VALUES(?,?,?)`, uuid(), nid, in.Project)
	if e != nil {
		problem(w, 409, "This National Project is already complete.")
		return
	}
	_, e = tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-?,iron=iron-?,steel=steel-?,aluminum=aluminum-?,coal=coal-?,food=food-?,education=LEAST(100,education+?) WHERE id=?`, p.Cash, p.Iron, p.Steel, p.Aluminum, p.Coal, p.Food, map[bool]int{true: 5}[in.Project == "public_education_initiative"], nid)
	if e != nil {
		return
	}
	tx.ExecContext(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'national_project',?,?)`, uuid(), nid, -p.Cash, "Completed "+p.Name)
	if tx.Commit() != nil {
		return
	}
	write(w, 201, map[string]bool{"ok": true})
}

func (a *app) economicPolicy(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		TaxRate  float64
		Doctrine string
	}
	if !decode(w, r, &in) {
		return
	}
	if in.TaxRate < 10 || in.TaxRate > 45 {
		problem(w, 400, "Tax rate must be between 10% and 45%.")
		return
	}
	allowed := map[string]bool{"Balanced": true, "Capitalist": true, "Planned": true, "Green": true}
	in.Doctrine = strings.TrimSpace(in.Doctrine)
	if !allowed[in.Doctrine] {
		problem(w, 400, "Unknown doctrine.")
		return
	}
	_, e := a.db.ExecContext(r.Context(), `UPDATE nations SET tax_rate=?,doctrine=? WHERE owner_id=?`, in.TaxRate, in.Doctrine, u.ID)
	if e != nil {
		problem(w, 500, "Policy could not be saved.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
