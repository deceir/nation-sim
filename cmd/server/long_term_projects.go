package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
)

type longTermProject struct {
	ID, Name, Category, Description, Target, Exclusivity, Unlock string
	Cash, Turns                                                  int64
	Costs                                                        map[string]float64
	ProductionBoost, IncomeBoost, HappinessTarget                float64
	EducationDaily, TechnologyDaily, EfficiencyBoost             float64
	TaxBoost, InfraDiscount, UpgradeDiscount, CommerceBoost      float64
	DiseaseReduction, CrimeReduction, ShippingDiscount           float64
	PopulationGrowth, EffectivePopulation                        float64
}

func specialization(id, name, target string, cash, turns int64, boost float64, costs map[string]float64) longTermProject {
	return longTermProject{ID: id, Name: name, Category: "specialization", Description: "+" + formatPercent(boost) + " permanent " + commodityName(target) + " production.", Target: target, Exclusivity: target, Cash: cash, Turns: turns, ProductionBoost: boost, Costs: costs}
}
func formatPercent(v float64) string { return fmtFloat(v*100) + "%" }
func fmtFloat(v float64) string {
	if v == math.Trunc(v) {
		return fmtInt(int64(v))
	}
	return fmtDecimal(v)
}

var longTermProjects = map[string]longTermProject{
	"agricultural_expansion":       specialization("agricultural_expansion", "Agricultural Expansion Program", "foodstuffs", 4500000, 72, .75, map[string]float64{"foodstuffs": 8000, "construction_materials": 3000}),
	"forestry_commission":          specialization("forestry_commission", "Forestry Commission", "timber", 4200000, 72, .75, map[string]float64{"timber": 7000, "construction_materials": 2500}),
	"fiber_cultivation":            specialization("fiber_cultivation", "Fiber Cultivation Initiative", "fibers", 4000000, 72, .75, map[string]float64{"fibers": 6500, "construction_materials": 2500}),
	"national_mining_authority":    specialization("national_mining_authority", "National Mining Authority", "basic_metals", 5000000, 84, .75, map[string]float64{"basic_metals": 6000, "construction_materials": 3000}),
	"energy_development":           specialization("energy_development", "Energy Development Act", "energy", 5500000, 84, .75, map[string]float64{"energy": 7000, "construction_materials": 3500}),
	"strategic_resource_bureau":    specialization("strategic_resource_bureau", "Strategic Resource Bureau", "strategic_minerals", 6000000, 96, .75, map[string]float64{"strategic_minerals": 5000, "construction_materials": 4000}),
	"textile_complex":              specialization("textile_complex", "Textile Industrial Complex", "textiles", 4800000, 72, .70, map[string]float64{"textiles": 5000, "fibers": 4000, "construction_materials": 3000}),
	"food_processing_authority":    specialization("food_processing_authority", "Food Processing Authority", "processed_foods", 4600000, 72, .70, map[string]float64{"processed_foods": 5000, "foodstuffs": 4000, "construction_materials": 2500}),
	"construction_materials_board": specialization("construction_materials_board", "Construction Materials Board", "construction_materials", 5200000, 84, .70, map[string]float64{"construction_materials": 6000, "basic_metals": 4000, "timber": 3000}),
	"light_industry_promotion":     specialization("light_industry_promotion", "Light Industry Promotion", "basic_goods", 5000000, 72, .70, map[string]float64{"basic_goods": 5500, "construction_materials": 3500}),
	"consumer_goods_consortium":    specialization("consumer_goods_consortium", "Consumer Goods Consortium", "consumer_goods", 5500000, 84, .65, map[string]float64{"consumer_goods": 5000, "luxury_goods": 3000, "construction_materials": 3000}),
	"luxury_goods_guild":           specialization("luxury_goods_guild", "Luxury Goods Guild", "luxury_goods", 6500000, 96, .65, map[string]float64{"luxury_goods": 4000, "consumer_goods": 3000, "construction_materials": 4000}),
	"military_industrial_complex":  specialization("military_industrial_complex", "Military-Industrial Complex", "military_equipment", 7000000, 96, .65, map[string]float64{"military_equipment": 5000, "basic_metals": 4000, "construction_materials": 4000}),
}

func init() {
	add := func(p longTermProject) { longTermProjects[p.ID] = p }
	add(longTermProject{ID: "civil_service_reform", Name: "Civil Service Reform", Category: "national", Description: "Happiness target +8.", Cash: 3800000, Turns: 60, Costs: map[string]float64{"construction_materials": 2500, "basic_goods": 1500}, HappinessTarget: 8})
	add(longTermProject{ID: "national_education_act", Name: "National Education Act", Category: "national", Description: "Education gain +0.15/day and citizen income +4%.", Cash: 4200000, Turns: 72, Costs: map[string]float64{"construction_materials": 3000, "basic_goods": 2000}, EducationDaily: .15, IncomeBoost: .04})
	add(longTermProject{ID: "technological_research_council", Name: "Technological Research Council", Category: "national", Description: "Technology progress +0.12/day and production +6%.", Cash: 5000000, Turns: 84, Costs: map[string]float64{"construction_materials": 3500, "strategic_minerals": 2000}, TechnologyDaily: .12, EfficiencyBoost: .06})
	add(longTermProject{ID: "tax_modernization", Name: "Tax Administration Modernization", Category: "national", Description: "Effective tax revenue +8%.", Cash: 4500000, Turns: 72, Costs: map[string]float64{"construction_materials": 2500, "basic_goods": 2000}, TaxBoost: .08})
	add(longTermProject{ID: "infrastructure_bank", Name: "National Infrastructure Bank", Category: "national", Description: "Infrastructure costs -12%; Province upgrade costs -10%.", Cash: 6000000, Turns: 96, Costs: map[string]float64{"construction_materials": 5000, "basic_metals": 3000}, InfraDiscount: .12, UpgradeDiscount: .10})
	add(longTermProject{ID: "commerce_facilitation", Name: "Commerce Facilitation Act", Category: "national", Description: "Commerce effectiveness +10% and soft ceiling +8%.", Cash: 4800000, Turns: 72, Costs: map[string]float64{"construction_materials": 3000, "consumer_goods": 2500}, CommerceBoost: .10})
	add(longTermProject{ID: "public_health_program", Name: "Public Health & Sanitation Program", Category: "national", Description: "Disease impact -25%; Happiness target +4.", Cash: 3500000, Turns: 60, Costs: map[string]float64{"construction_materials": 2000, "processed_foods": 2500}, DiseaseReduction: .25, HappinessTarget: 4})
	add(longTermProject{ID: "internal_security_reform", Name: "Internal Security Reform", Category: "national", Description: "Crime impact -25%.", Cash: 3600000, Turns: 60, Costs: map[string]float64{"construction_materials": 2000, "basic_goods": 1500}, CrimeReduction: .25})
	add(longTermProject{ID: "logistical_optimization", Name: "Logistical Optimization Bureau", Category: "national", Description: "Production +5%; shipping cost -15%.", Cash: 4000000, Turns: 72, Costs: map[string]float64{"construction_materials": 3000, "energy": 2000}, EfficiencyBoost: .05, ShippingDiscount: .15})
	add(longTermProject{ID: "population_development", Name: "Population Development Initiative", Category: "national", Description: "Population growth +8%; effective population +3%.", Cash: 4500000, Turns: 72, Costs: map[string]float64{"construction_materials": 3000, "foodstuffs": 3000}, PopulationGrowth: .08, EffectivePopulation: .03})
	add(longTermProject{ID: "luxury_market_authority", Name: "Luxury Market Authority", Category: "national", Description: "Unlocks national Luxury Goods consumption and its size-scaled Treasury income.", Cash: 1500000, Turns: 24, Costs: map[string]float64{"construction_materials": 800, "consumer_goods": 600}, Unlock: "luxury_consumption"})
	for _, p := range []longTermProject{
		{ID: "aviation_industry", Name: "Aviation Industry Act", Category: "military", Description: "Unlocks domestic aircraft production.", Cash: 8000000, Turns: 120, Costs: map[string]float64{"construction_materials": 6000, "basic_metals": 4000, "energy": 3000, "strategic_minerals": 2000}, Unlock: "aircraft"},
		{ID: "naval_shipyard", Name: "Naval Shipyard Authority", Category: "military", Description: "Unlocks domestic ship production.", Cash: 8500000, Turns: 120, Costs: map[string]float64{"construction_materials": 7000, "basic_metals": 5000, "energy": 3500, "timber": 2000}, Unlock: "ships"},
		{ID: "armored_vehicle_program", Name: "Armored Vehicle Program", Category: "military", Description: "Unlocks domestic armor production.", Cash: 7500000, Turns: 108, Costs: map[string]float64{"construction_materials": 5500, "basic_metals": 5000, "energy": 3000}, Unlock: "armor"},
		{ID: "advanced_ordnance", Name: "Advanced Ordnance Act", Category: "military", Description: "Unlocks advanced military-equipment production.", Cash: 7000000, Turns: 108, Costs: map[string]float64{"construction_materials": 5000, "basic_metals": 4000, "strategic_minerals": 3000}, Unlock: "advanced_ordnance"},
	} {
		add(p)
	}
	// Commodity requirements should reinforce trade and preparation without
	// eclipsing the multi-million-Yen gate by several months. Specialization
	// projects remain deliberately powerful because limited slots are their
	// primary long-run opportunity cost.
	primary := map[string]bool{"foodstuffs": true, "timber": true, "fibers": true, "basic_metals": true, "energy": true, "strategic_minerals": true}
	advanced := map[string]bool{"consumer_goods": true, "luxury_goods": true, "military_equipment": true}
	for id, p := range longTermProjects {
		p.Cash *= yenScale
		for commodity, amount := range p.Costs {
			p.Costs[commodity] = math.Ceil(amount*.12/10) * 10
		}
		if p.Category == "specialization" {
			switch {
			case primary[p.Target]:
				p.ProductionBoost = 1.00
			case advanced[p.Target]:
				p.ProductionBoost = .80
			default:
				p.ProductionBoost = .90
			}
			p.Description = "+" + formatPercent(p.ProductionBoost) + " permanent " + commodityName(p.Target) + " production."
		}
		longTermProjects[id] = p
	}
}

func fmtInt(v int64) string       { return strconv.FormatInt(v, 10) }
func fmtDecimal(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
func longTermProjectSlots(infra float64, provinces int) int {
	return 2 + int(infra/1200) + max(0, provinces-1)/2
}
func longTermProjectUsesSlot(projectID string) bool {
	project, exists := longTermProjects[projectID]
	return !exists || project.Category != "military"
}

func (a *app) longTermProjectsDashboard(w http.ResponseWriter, r *http.Request, u user) {
	var nid string
	var infra float64
	var provinces int
	if a.db.QueryRowContext(r.Context(), `SELECT n.id,(SELECT COALESCE(SUM(infrastructure),0) FROM cities WHERE nation_id=n.id),(SELECT COUNT(*) FROM cities WHERE nation_id=n.id) FROM nations n WHERE owner_id=?`, u.ID).Scan(&nid, &infra, &provinces) != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	projects, slots, used := a.longTermProjectCatalog(r.Context(), nid, infra, provinces)
	write(w, 200, map[string]any{"projects": projects, "slots": slots, "used": used, "infrastructure": infra, "provinces": provinces})
}

func (a *app) longTermProjectCatalog(ctx context.Context, nid string, infra float64, provinces int) ([]map[string]any, int, int) {
	completed := map[string]bool{}
	rows, _ := a.db.QueryContext(ctx, `SELECT project_type FROM national_long_term_projects WHERE nation_id=?`, nid)
	if rows != nil {
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				completed[id] = true
			}
		}
		rows.Close()
	}
	building := map[string]map[string]any{}
	rows, _ = a.db.QueryContext(ctx, `SELECT project_type,turns_total,turns_remaining FROM national_project_construction WHERE nation_id=? AND status='building'`, nid)
	if rows != nil {
		for rows.Next() {
			var id string
			var total, remaining int
			if rows.Scan(&id, &total, &remaining) == nil {
				building[id] = map[string]any{"total": total, "remaining": remaining}
			}
		}
		rows.Close()
	}
	keys := make([]string, 0, len(longTermProjects))
	for id := range longTermProjects {
		keys = append(keys, id)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := longTermProjects[keys[i]], longTermProjects[keys[j]]
		if a.Category == b.Category {
			return a.Name < b.Name
		}
		return a.Category < b.Category
	})
	out := []map[string]any{}
	for _, id := range keys {
		p := longTermProjects[id]
		out = append(out, map[string]any{"id": p.ID, "name": p.Name, "category": p.Category, "description": p.Description, "target": p.Target, "exclusivity": p.Exclusivity, "unlock": p.Unlock, "cash": p.Cash, "turns": p.Turns, "costs": p.Costs, "completed": completed[id], "construction": building[id], "usesSlot": longTermProjectUsesSlot(id)})
	}
	used := 0
	for id := range completed {
		if longTermProjectUsesSlot(id) {
			used++
		}
	}
	for id := range building {
		if longTermProjectUsesSlot(id) {
			used++
		}
	}
	return out, longTermProjectSlots(infra, provinces), used
}

func (a *app) startLongTermProject(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Project string }
	if !decode(w, r, &in) {
		return
	}
	p, ok := longTermProjects[in.Project]
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
	var cash int64
	var infra float64
	var provinces, used int
	if e = tx.QueryRowContext(r.Context(), `SELECT n.id,n.treasury,(SELECT COALESCE(SUM(infrastructure),0) FROM cities WHERE nation_id=n.id),(SELECT COUNT(*) FROM cities WHERE nation_id=n.id) FROM nations n WHERE owner_id=? FOR UPDATE`, u.ID).Scan(&nid, &cash, &infra, &provinces); e != nil {
		return
	}
	if p.Category != "military" {
		rows, queryErr := tx.QueryContext(r.Context(), `SELECT project_type FROM national_long_term_projects WHERE nation_id=? UNION ALL SELECT project_type FROM national_project_construction WHERE nation_id=? AND status='building'`, nid, nid)
		if queryErr != nil {
			problem(w, 500, "Could not verify National Project capacity.")
			return
		}
		for rows.Next() {
			var projectID string
			if rows.Scan(&projectID) == nil && longTermProjectUsesSlot(projectID) {
				used++
			}
		}
		rows.Close()
	}
	if p.Category != "military" && used >= longTermProjectSlots(infra, provinces) {
		problem(w, 409, "No long-term Project Slot is available.")
		return
	}
	var exists int
	tx.QueryRowContext(r.Context(), `SELECT (SELECT COUNT(*) FROM national_long_term_projects WHERE nation_id=? AND project_type=?)+(SELECT COUNT(*) FROM national_project_construction WHERE nation_id=? AND project_type=? AND status='building')`, nid, p.ID, nid, p.ID).Scan(&exists)
	if exists > 0 {
		problem(w, 409, "This National Project is already owned or under construction.")
		return
	}
	if p.Exclusivity != "" {
		rows, _ := tx.QueryContext(r.Context(), `SELECT project_type FROM national_long_term_projects WHERE nation_id=? UNION ALL SELECT project_type FROM national_project_construction WHERE nation_id=? AND status='building'`, nid, nid)
		if rows != nil {
			for rows.Next() {
				var id string
				rows.Scan(&id)
				if prior, found := longTermProjects[id]; found && prior.Exclusivity == p.Exclusivity {
					rows.Close()
					problem(w, 409, "That commodity specialization is already occupied.")
					return
				}
			}
			rows.Close()
		}
	}
	if cash < p.Cash {
		problem(w, 409, "Insufficient Treasury.")
		return
	}
	for commodity, cost := range p.Costs {
		var available float64
		tx.QueryRowContext(r.Context(), `SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity=? FOR UPDATE`, nid, commodity).Scan(&available)
		if available+.0001 < cost {
			problem(w, 409, "Insufficient "+commodityName(commodity)+".")
			return
		}
	}
	tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, p.Cash, nid)
	for commodity, cost := range p.Costs {
		tx.ExecContext(r.Context(), `UPDATE nation_stockpiles SET amount=amount-? WHERE nation_id=? AND commodity=?`, cost, nid, commodity)
	}
	locked, _ := json.Marshal(p.Costs)
	if _, e = tx.ExecContext(r.Context(), `INSERT INTO national_project_construction(id,nation_id,project_type,turns_total,turns_remaining,cash_locked,commodities_locked,status) VALUES(?,?,?,?,?,?,?,'building')`, uuid(), nid, p.ID, p.Turns, p.Turns, p.Cash, locked); e != nil {
		problem(w, 500, "Could not begin construction.")
		return
	}
	tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'game','National Project construction started',?)`, uuid(), nid, p.Name+" will complete in "+fmtInt(p.Turns)+" turns.")
	if tx.Commit() != nil {
		return
	}
	write(w, 201, map[string]any{"ok": true, "turns": p.Turns})
}

func (a *app) demolishLongTermProject(w http.ResponseWriter, r *http.Request, u user) {
	p, ok := longTermProjects[r.PathValue("id")]
	if !ok {
		problem(w, 404, "Project not found.")
		return
	}
	result, e := a.db.ExecContext(r.Context(), `DELETE p FROM national_long_term_projects p JOIN nations n ON n.id=p.nation_id WHERE n.owner_id=? AND p.project_type=?`, u.ID, p.ID)
	if e != nil || affected(result) != 1 {
		problem(w, 404, "Completed project not found.")
		return
	}
	nid, _ := a.nationID(r.Context(), u.ID)
	a.db.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'game','National Project demolished',?)`, uuid(), nid, p.Name+" was permanently demolished. No resources were refunded.")
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) processLongTermProjects(ctx context.Context) {
	rows, e := a.db.QueryContext(ctx, `SELECT id,nation_id,project_type,turns_remaining FROM national_project_construction WHERE status='building'`)
	if e != nil {
		return
	}
	type job struct {
		id, nid, project string
		remaining        int
	}
	jobs := []job{}
	for rows.Next() {
		var j job
		if rows.Scan(&j.id, &j.nid, &j.project, &j.remaining) == nil {
			jobs = append(jobs, j)
		}
	}
	rows.Close()
	for _, j := range jobs {
		tx, e := a.db.BeginTx(ctx, nil)
		if e != nil {
			continue
		}
		if j.remaining > 1 {
			tx.ExecContext(ctx, `UPDATE national_project_construction SET turns_remaining=turns_remaining-1 WHERE id=? AND status='building'`, j.id)
		} else {
			tx.ExecContext(ctx, `INSERT IGNORE INTO national_long_term_projects(id,nation_id,project_type) VALUES(?,?,?)`, uuid(), j.nid, j.project)
			tx.ExecContext(ctx, `UPDATE national_project_construction SET turns_remaining=0,status='complete',completed_at=NOW() WHERE id=?`, j.id)
			name := j.project
			if p, ok := longTermProjects[j.project]; ok {
				name = p.Name
			}
			tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'game','National Project completed',?)`, uuid(), j.nid, name+" is now operational.")
		}
		tx.Commit()
	}
}

func loadLongTermProjectSet(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, nid string) map[string]bool {
	out := map[string]bool{}
	rows, e := q.QueryContext(ctx, `SELECT project_type FROM national_long_term_projects WHERE nation_id=?`, nid)
	if e == nil {
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				out[id] = true
			}
		}
		rows.Close()
	}
	return out
}
