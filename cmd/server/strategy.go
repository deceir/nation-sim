package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

type gearSpec struct {
	Name, Description                                               string
	Population, Extraction, Industry, Commerce, Military, Happiness float64
}

var gears = map[string]gearSpec{
	"balanced":    {"Balanced / Mixed", "Broad resilience without major penalties.", 1.03, 1.03, 1.03, 1.03, 1.0, 1.0},
	"agrarian":    {"Agrarian / Expansionist", "Population, food, fibers, and primary growth.", 1.18, 1.15, .82, .90, .85, 1.05},
	"industrial":  {"Industrial", "Infrastructure and secondary commodity production.", .92, 1.0, 1.22, .90, 1.05, .97},
	"commercial":  {"Commercial / Trade", "Citizen income, trade margins, and consumer activity.", 1.0, .82, .95, 1.20, .82, 1.02},
	"militarized": {"Militarized Economy", "Military equipment and strategic production.", .90, .92, 1.05, .82, 1.30, .90},
}

type policySpec struct {
	Name, Description                                                          string
	PoliticalCost                                                              float64
	Population, Extraction, Industry, Commerce, Military, Happiness, Education float64
}

var socialPolicies = map[string]policySpec{
	"family_incentives":      {"Family Incentives", "Faster population growth at a continuing fiscal cost.", 20, 1.10, 1, 1, 1, 1, 1.02, 1},
	"migration_attraction":   {"Migration Attraction", "Attract workers and improve commercial labor supply.", 20, 1.08, 1, 1, 1.05, 1, 1, 1},
	"land_grants":            {"Land Grants", "Accelerate primary-sector settlement.", 15, 1.05, 1.10, 1, 1, 1, 1, 1},
	"market_liberalization":  {"Market Liberalization", "Higher commerce income with a small Morale tradeoff.", 20, 1, 1, 1, 1.10, 1, .97, 1},
	"industrial_subsidies":   {"Industrial Subsidies", "Increase national commodity conversion.", 20, 1, 1, 1.12, 1, 1, .98, 1},
	"worker_training":        {"Worker Training", "Industry and Education reinforce each other.", 20, 1, 1, 1.07, 1, 1, 1, 1.08},
	"extraction_compacts":    {"Extraction Compacts", "Increase deposit utilization.", 15, 1, 1.12, 1, 1, 1, .98, 1},
	"arms_export_incentives": {"Arms Export Incentives", "Increase military-equipment production.", 25, 1, 1, 1, 1, 1.15, .96, 1},
}
var strategicCommodities = []string{"foodstuffs", "timber", "fibers", "basic_metals", "energy", "strategic_minerals", "textiles", "processed_foods", "construction_materials", "basic_goods", "consumer_goods", "military_equipment", "luxury_goods"}
var commodityRecipes = map[string]map[string]float64{"textiles": {"fibers": .8, "energy": .15}, "processed_foods": {"foodstuffs": .9, "energy": .1}, "construction_materials": {"timber": .45, "basic_metals": .45, "energy": .15}, "basic_goods": {"basic_metals": .5, "timber": .2, "energy": .2}, "consumer_goods": {"basic_goods": .55, "fibers": .2, "energy": .2}, "military_equipment": {"basic_metals": .7, "energy": .35, "strategic_minerals": .12}, "luxury_goods": {"consumer_goods": .5, "strategic_minerals": .08, "energy": .15}}

func commodityName(key string) string {
	parts := strings.Split(key, "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

type provinceStrategy struct {
	ID, Name, Specialization string
	Infra, Development       float64 // Development is retained only for legacy test/data compatibility.
	Deposits                 map[string]float64
	Upgrades                 map[string]int
	UpgradeQuotes            map[string]int64
	InfrastructureQuotes     map[string]int64
	UpgradeCap               int
	Population               float64
}
type strategicInput struct {
	Gear                  string
	Disrupted             bool
	Policies              map[string]bool
	Education, Technology float64
	Provinces             []provinceStrategy
	Quotas                map[string]float64
}
type strategicResult struct {
	Production           map[string]float64            `json:"production"`
	IncomeMultiplier     float64                       `json:"incomeMultiplier"`
	PopulationMultiplier float64                       `json:"populationMultiplier"`
	HappinessMultiplier  float64                       `json:"happinessMultiplier"`
	ExtractionMultiplier float64                       `json:"extractionMultiplier"`
	IndustryMultiplier   float64                       `json:"industryMultiplier"`
	CommerceMultiplier   float64                       `json:"commerceMultiplier"`
	MilitaryMultiplier   float64                       `json:"militaryMultiplier"`
	ProvinceProduction   map[string]map[string]float64 `json:"provinceProduction"`
}

func calculateStrategy(in strategicInput) strategicResult {
	g := gears[in.Gear]
	if g.Name == "" {
		g = gears["balanced"]
	}
	r := strategicResult{Production: map[string]float64{}, ProvinceProduction: map[string]map[string]float64{}, IncomeMultiplier: g.Commerce, PopulationMultiplier: g.Population, HappinessMultiplier: g.Happiness, ExtractionMultiplier: g.Extraction, IndustryMultiplier: g.Industry, CommerceMultiplier: g.Commerce, MilitaryMultiplier: g.Military}
	if in.Disrupted {
		r.IncomeMultiplier *= .75
		r.ExtractionMultiplier *= .75
		r.IndustryMultiplier *= .75
		r.MilitaryMultiplier *= .75
	}
	for k := range in.Policies {
		p := socialPolicies[k]
		r.PopulationMultiplier *= p.Population
		r.ExtractionMultiplier *= p.Extraction
		r.IndustryMultiplier *= p.Industry
		r.CommerceMultiplier *= p.Commerce
		r.MilitaryMultiplier *= p.Military
		r.HappinessMultiplier *= p.Happiness
		r.IncomeMultiplier *= p.Commerce
	}
	knowledge := 1 + in.Education*.0025 + in.Technology*.004
	for _, p := range in.Provinces {
		out := map[string]float64{}
		agriculture := provinceUpgradeEffect(p.Upgrades["agriculture"])
		extraction := provinceUpgradeEffect(p.Upgrades["extraction"])
		light := provinceUpgradeEffect(p.Upgrades["light_industry"])
		heavy := provinceUpgradeEffect(p.Upgrades["heavy_industry"])
		commerce := provinceUpgradeEffect(p.Upgrades["commerce"])
		civil := provinceUpgradeEffect(p.Upgrades["civil"])
		military := provinceUpgradeEffect(p.Upgrades["military_industry"])
		specPrimary, specIndustry := 1.0, 1.0
		if p.Specialization == "extraction" {
			specPrimary = 1.18
		}
		if p.Specialization == "industry" {
			specIndustry = 1.20
		}
		if p.Specialization == "commerce" {
			r.IncomeMultiplier *= 1.015
		}
		if p.Specialization == "military" {
			r.MilitaryMultiplier *= 1.025
		}
		r.IncomeMultiplier *= 1 + commerce*.006
		r.PopulationMultiplier *= 1 + civil*.002
		alignment := 1.0
		if (p.Specialization == "agriculture" && in.Gear == "agrarian") || (p.Specialization == "industry" && in.Gear == "industrial") || (p.Specialization == "commerce" && in.Gear == "commercial") || (p.Specialization == "military" && in.Gear == "militarized") {
			alignment = 1.08
		}
		for resource, richness := range p.Deposits {
			resourceSpec := specPrimary
			if p.Specialization == "agriculture" && (resource == "foodstuffs" || resource == "fibers") {
				resourceSpec *= 1.22
			}
			agricultureBoost := 1.0
			if resource == "foodstuffs" || resource == "fibers" {
				agricultureBoost += agriculture * .03
			}
			v := p.Infra * .075 * richness * r.ExtractionMultiplier * resourceSpec * agricultureBoost * (1 + extraction*.025) * alignment
			out[resource] += v
			r.Production[resource] += v
		}
		capacity := p.Infra * .035 * specIndustry * r.IndustryMultiplier * knowledge * (1 + light*.018 + heavy*.022) * alignment
		quotaTotal := 0.0
		for _, k := range []string{"textiles", "processed_foods", "construction_materials", "basic_goods", "consumer_goods", "military_equipment", "luxury_goods"} {
			quotaTotal += math.Max(0, in.Quotas[k])
		}
		if quotaTotal == 0 {
			quotaTotal = 1
		}
		for _, k := range []string{"textiles", "processed_foods", "construction_materials", "basic_goods", "consumer_goods", "military_equipment", "luxury_goods"} {
			share := math.Max(0, in.Quotas[k]) / quotaTotal
			if in.Quotas[k] == 0 {
				continue
			}
			modifier := 1.0
			if k == "textiles" || k == "processed_foods" || k == "basic_goods" {
				modifier *= 1 + light*.025
			}
			if k == "construction_materials" || k == "consumer_goods" || k == "luxury_goods" {
				modifier *= 1 + heavy*.025
			}
			if k == "military_equipment" {
				modifier *= r.MilitaryMultiplier / r.IndustryMultiplier * (1 + military*.03)
			}
			v := capacity * share * modifier
			out[k] += v
			r.Production[k] += v
		}
		r.ProvinceProduction[p.ID] = out
	}
	return r
}

func (a *app) loadStrategy(ctx context.Context, nid string) (strategicInput, error) {
	in := strategicInput{Policies: map[string]bool{}, Quotas: map[string]float64{}}
	if e := a.db.QueryRowContext(ctx, `SELECT s.gear,(s.disruption_until IS NOT NULL AND s.disruption_until>NOW()),n.education,n.technology FROM nation_economic_strategy s JOIN nations n ON n.id=s.nation_id WHERE s.nation_id=?`, nid).Scan(&in.Gear, &in.Disrupted, &in.Education, &in.Technology); e != nil {
		return in, e
	}
	rows, e := a.db.QueryContext(ctx, `SELECT policy_key FROM social_policy_selections WHERE nation_id=?`, nid)
	if e != nil {
		return in, e
	}
	for rows.Next() {
		var k string
		rows.Scan(&k)
		in.Policies[k] = true
	}
	rows.Close()
	rows, e = a.db.QueryContext(ctx, `SELECT commodity,priority FROM production_quotas WHERE nation_id=?`, nid)
	if e != nil {
		return in, e
	}
	for rows.Next() {
		var k string
		var v float64
		rows.Scan(&k, &v)
		in.Quotas[k] = v
	}
	rows.Close()
	rows, e = a.db.QueryContext(ctx, `SELECT c.id,c.name,p.specialization,c.infrastructure,c.local_population FROM cities c JOIN province_economies p ON p.city_id=c.id WHERE c.nation_id=?`, nid)
	if e != nil {
		return in, e
	}
	for rows.Next() {
		var p provinceStrategy
		p.Deposits = map[string]float64{}
		p.Upgrades = map[string]int{}
		p.UpgradeQuotes = map[string]int64{}
		p.InfrastructureQuotes = map[string]int64{}
		rows.Scan(&p.ID, &p.Name, &p.Specialization, &p.Infra, &p.Population)
		in.Provinces = append(in.Provinces, p)
	}
	rows.Close()
	for i := range in.Provinces {
		in.Provinces[i].Upgrades = loadProvinceUpgrades(ctx, a.db, in.Provinces[i].ID)
		in.Provinces[i].UpgradeCap = provinceUpgradeCap(in.Provinces[i].Infra)
		for key, spec := range provinceUpgradeSpecs {
			in.Provinces[i].UpgradeQuotes[key] = provinceUpgradeCost(spec, in.Provinces[i].Upgrades[key], in.Provinces[i].Infra)
		}
		for _, amount := range []int{10, 50, 100} {
			in.Provinces[i].InfrastructureQuotes[fmt.Sprint(amount)] = int64(infraPurchaseCost(in.Provinces[i].Infra, float64(amount), int(in.Technology)))
		}
		ds, _ := a.db.QueryContext(ctx, `SELECT resource,richness FROM province_deposits WHERE city_id=?`, in.Provinces[i].ID)
		for ds.Next() {
			var k string
			var v float64
			ds.Scan(&k, &v)
			in.Provinces[i].Deposits[k] = v
		}
		ds.Close()
	}
	return in, nil
}

func (a *app) strategyDashboard(w http.ResponseWriter, r *http.Request, u user) {
	nid, e := a.nationID(r.Context(), u.ID)
	if e != nil {
		return
	}
	in, e := a.loadStrategy(r.Context(), nid)
	if e != nil {
		problem(w, 500, "Economic strategy unavailable. Restart migrations if this database predates the redesign.")
		return
	}
	result := calculateStrategy(in)
	var political float64
	var changed, disruption sql.NullTime
	a.db.QueryRowContext(r.Context(), `SELECT political_capital,gear_changed_at,disruption_until FROM nation_economic_strategy WHERE nation_id=?`, nid).Scan(&political, &changed, &disruption)
	gearList := []map[string]any{}
	keys := []string{}
	for k := range gears {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		g := gears[k]
		gearList = append(gearList, map[string]any{"key": k, "name": g.Name, "description": g.Description, "effects": map[string]float64{"population": g.Population, "extraction": g.Extraction, "industry": g.Industry, "commerce": g.Commerce, "military": g.Military, "happiness": g.Happiness}})
	}
	policyList := []map[string]any{}
	keys = []string{}
	for k := range socialPolicies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := socialPolicies[k]
		policyList = append(policyList, map[string]any{"key": k, "name": p.Name, "description": p.Description, "cost": p.PoliticalCost, "active": in.Policies[k], "effects": map[string]float64{"population": p.Population, "extraction": p.Extraction, "industry": p.Industry, "commerce": p.Commerce, "military": p.Military, "happiness": p.Happiness, "education": p.Education}})
	}
	stock := map[string]float64{}
	rows, _ := a.db.QueryContext(r.Context(), `SELECT commodity,amount FROM nation_stockpiles WHERE nation_id=?`, nid)
	for rows.Next() {
		var k string
		var v float64
		rows.Scan(&k, &v)
		stock[k] = v
	}
	rows.Close()
	upgradeList := []map[string]any{}
	keys = keys[:0]
	for key := range provinceUpgradeSpecs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		spec := provinceUpgradeSpecs[key]
		upgradeList = append(upgradeList, map[string]any{"key": key, "name": spec.Name, "description": spec.Description, "baseCost": spec.BaseCost})
	}
	cashCost, materialCost, strain := provinceFoundingCosts(len(in.Provinces), in.Gear, in.Policies)
	var lastProvince time.Time
	var nextProvinceAt any
	if len(in.Provinces) > 1 && a.db.QueryRowContext(r.Context(), `SELECT MAX(created_at) FROM cities WHERE nation_id=?`, nid).Scan(&lastProvince) == nil {
		if next := lastProvince.Add(7 * 24 * time.Hour); next.After(time.Now()) {
			nextProvinceAt = next
		}
	}
	expansion := map[string]any{"provinceCount": len(in.Provinces), "cashCost": cashCost, "constructionMaterials": materialCost, "happinessStrain": strain, "nextProvinceAt": nextProvinceAt, "gearModifier": expansionGearModifier(in.Gear), "policyModifier": expansionPolicyModifier(in.Policies), "formula": "¥200,000 × N^2.6 × Gear × Policy"}
	write(w, 200, map[string]any{"gear": in.Gear, "gears": gearList, "policies": policyList, "politicalCapital": political, "gearChangedAt": changed, "disruptionUntil": disruption, "provinces": in.Provinces, "provinceUpgradeTypes": upgradeList, "expansion": expansion, "quotas": in.Quotas, "recipes": commodityRecipes, "stockpiles": stock, "result": result})
}

func (a *app) setGear(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Gear string }
	if !decode(w, r, &in) {
		return
	}
	if gears[in.Gear].Name == "" {
		problem(w, 400, "Unknown economic gear.")
		return
	}
	nid, _ := a.nationID(r.Context(), u.ID)
	var current string
	var capital float64
	var changed sql.NullTime
	a.db.QueryRowContext(r.Context(), `SELECT gear,political_capital,gear_changed_at FROM nation_economic_strategy WHERE nation_id=?`, nid).Scan(&current, &capital, &changed)
	if current == in.Gear {
		write(w, 200, map[string]bool{"ok": true})
		return
	}
	if changed.Valid && time.Now().Before(changed.Time.Add(7*24*time.Hour)) {
		problem(w, 429, "Economic gearing can change once every seven days.")
		return
	}
	if capital < 25 {
		problem(w, 409, "Changing gear requires 25 political capital.")
		return
	}
	tx, _ := a.db.BeginTx(r.Context(), nil)
	defer tx.Rollback()
	tx.ExecContext(r.Context(), `UPDATE nation_economic_strategy SET gear=?,political_capital=political_capital-25,gear_changed_at=NOW(),disruption_until=DATE_ADD(NOW(),INTERVAL 24 HOUR) WHERE nation_id=?`, in.Gear, nid)
	tx.ExecContext(r.Context(), `UPDATE nations SET happiness=GREATEST(0,happiness-5) WHERE id=?`, nid)
	tx.Commit()
	write(w, 200, map[string]bool{"ok": true})
}
func (a *app) setPolicies(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Policies []string }
	if !decode(w, r, &in) || len(in.Policies) > 2 {
		problem(w, 400, "Choose no more than two social policies.")
		return
	}
	nid, _ := a.nationID(r.Context(), u.ID)
	current := map[string]bool{}
	rows, _ := a.db.QueryContext(r.Context(), `SELECT policy_key FROM social_policy_selections WHERE nation_id=?`, nid)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			if rows.Scan(&key) == nil {
				current[key] = true
			}
		}
	}
	cost := 0.0
	seen := map[string]bool{}
	for _, k := range in.Policies {
		p, ok := socialPolicies[k]
		if !ok || seen[k] {
			problem(w, 400, "Invalid social policy selection.")
			return
		}
		seen[k] = true
		if !current[k] {
			cost += p.PoliticalCost
		}
	}
	var capital float64
	a.db.QueryRowContext(r.Context(), `SELECT political_capital FROM nation_economic_strategy WHERE nation_id=?`, nid).Scan(&capital)
	if capital < cost {
		problem(w, 409, "Insufficient political capital.")
		return
	}
	tx, _ := a.db.BeginTx(r.Context(), nil)
	defer tx.Rollback()
	tx.ExecContext(r.Context(), `DELETE FROM social_policy_selections WHERE nation_id=?`, nid)
	for _, k := range in.Policies {
		tx.ExecContext(r.Context(), `INSERT INTO social_policy_selections(nation_id,policy_key) VALUES(?,?)`, nid, k)
	}
	tx.ExecContext(r.Context(), `UPDATE nation_economic_strategy SET political_capital=political_capital-? WHERE nation_id=?`, cost, nid)
	tx.Commit()
	write(w, 200, map[string]bool{"ok": true})
}
func (a *app) setProvinceStrategy(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		ProvinceID, Specialization string
	}
	if !decode(w, r, &in) {
		return
	}
	if !map[string]bool{"mixed": true, "agriculture": true, "extraction": true, "industry": true, "commerce": true, "military": true}[in.Specialization] {
		problem(w, 400, "Unknown Province specialization.")
		return
	}
	nid, _ := a.nationID(r.Context(), u.ID)
	tx, _ := a.db.BeginTx(r.Context(), nil)
	defer tx.Rollback()
	var exists int
	if tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM cities WHERE nation_id=? AND id=?`, nid, in.ProvinceID).Scan(&exists) != nil || exists == 0 {
		problem(w, 404, "Province not found.")
		return
	}
	tx.ExecContext(r.Context(), `UPDATE province_economies SET specialization=? WHERE city_id=?`, in.Specialization, in.ProvinceID)
	tx.Commit()
	write(w, 200, map[string]any{"ok": true})
}
func (a *app) setQuotas(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Quotas map[string]float64 }
	if !decode(w, r, &in) {
		return
	}
	allowed := map[string]bool{"textiles": true, "processed_foods": true, "construction_materials": true, "basic_goods": true, "consumer_goods": true, "military_equipment": true, "luxury_goods": true}
	total := 0.0
	for k, v := range in.Quotas {
		if !allowed[k] || v < 0 {
			problem(w, 400, "Invalid production quota.")
			return
		}
		total += v
	}
	if total > 100.001 {
		problem(w, 400, "Production priorities cannot exceed 100%.")
		return
	}
	nid, _ := a.nationID(r.Context(), u.ID)
	tx, _ := a.db.BeginTx(r.Context(), nil)
	defer tx.Rollback()
	tx.ExecContext(r.Context(), `DELETE FROM production_quotas WHERE nation_id=?`, nid)
	for k, v := range in.Quotas {
		tx.ExecContext(r.Context(), `INSERT INTO production_quotas(nation_id,commodity,priority) VALUES(?,?,?)`, nid, k, v)
	}
	tx.Commit()
	write(w, 200, map[string]bool{"ok": true})
}

func applyStrategicTurn(ctx context.Context, tx *sql.Tx, nid string, in strategicInput, result strategicResult) error {
	for _, commodity := range []string{"foodstuffs", "timber", "fibers", "basic_metals", "energy", "strategic_minerals"} {
		hourly := result.Production[commodity] / 24
		if hourly <= 0 {
			continue
		}
		if _, e := tx.ExecContext(ctx, `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, nid, commodity, hourly); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'economic','Resource production',?)`, uuid(), nid, fmt.Sprintf("You earned %.2f %s last turn.", hourly, commodityName(commodity))); e != nil {
			return e
		}
	}
	for _, commodity := range []string{"textiles", "processed_foods", "construction_materials", "basic_goods", "consumer_goods", "military_equipment", "luxury_goods"} {
		wanted := result.Production[commodity] / 24
		if wanted <= 0 {
			continue
		}
		actual := wanted
		for input, ratio := range commodityRecipes[commodity] {
			var available float64
			tx.QueryRowContext(ctx, `SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity=? FOR UPDATE`, nid, input).Scan(&available)
			actual = math.Min(actual, available/ratio)
		}
		if actual <= 0 {
			continue
		}
		for input, ratio := range commodityRecipes[commodity] {
			if _, e := tx.ExecContext(ctx, `UPDATE nation_stockpiles SET amount=GREATEST(0,amount-?) WHERE nation_id=? AND commodity=?`, actual*ratio, nid, input); e != nil {
				return e
			}
		}
		if _, e := tx.ExecContext(ctx, `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, nid, commodity, actual); e != nil {
			return e
		}
		if _, e := tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'economic','Resource production',?)`, uuid(), nid, fmt.Sprintf("You earned %.2f %s last turn.", actual, commodityName(commodity))); e != nil {
			return e
		}
	}
	_, e := tx.ExecContext(ctx, `UPDATE nation_economic_strategy SET political_capital=LEAST(100,political_capital+.25) WHERE nation_id=?`, nid)
	return e
}
