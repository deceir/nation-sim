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
	"agrarian":    {"Agrarian / Expansionist", "Population, food, fibers, and primary growth.", 1.18, 1.22, .82, .90, .85, 1.05},
	"industrial":  {"Industrial", "Infrastructure and secondary commodity production.", .92, 1.0, 1.28, .90, 1.05, .97},
	"commercial":  {"Commercial / Trade", "Citizen income, trade margins, and consumer activity.", 1.0, .82, .95, 1.25, .82, 1.02},
	"militarized": {"Militarized Economy", "Military equipment and strategic production.", .90, .92, 1.05, .82, 1.30, .90},
}

type policySpec struct {
	Name, Description                                                          string
	PoliticalCost                                                              float64
	Population, Extraction, Industry, Commerce, Military, Happiness, Education float64
	Disease, Crime, Employment, FoodConsumption, InfrastructureUpkeep          float64
}

var socialPolicies = map[string]policySpec{
	"family_incentives":       {"Family Incentives", "Support household formation and long-term population growth.", 20, 1.15, 1, 1, 1, 1, 1.03, 1, 1, 1, 1, 1, 1},
	"migration_attraction":    {"Migration Attraction", "Attract workers and expand commercial labor supply.", 20, 1.12, 1, 1, 1.08, 1, 1, 1, 1, 1, 1, 1, 1},
	"land_grants":             {"Land Grants", "Accelerate primary-sector settlement and extraction.", 15, 1.08, 1.15, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	"market_liberalization":   {"Market Liberalization", "Higher commerce income with a small Satisfaction tradeoff.", 20, 1, 1, 1, 1.10, 1, .97, 1, 1, 1, 1, 1, 1},
	"industrial_subsidies":    {"Industrial Subsidies", "Increase national commodity conversion.", 20, 1, 1, 1.12, 1, 1, .98, 1, 1, 1, 1, 1, 1},
	"worker_training":         {"Worker Training", "Improve industrial output and the rate of Education development.", 20, 1, 1, 1.08, 1, 1, 1, 1.10, 1, 1, 1, 1, 1},
	"extraction_compacts":     {"Extraction Compacts", "Increase deposit utilization.", 15, 1, 1.12, 1, 1, 1, .98, 1, 1, 1, 1, 1, 1},
	"arms_export_incentives":  {"Arms Export Incentives", "Increase military-equipment production.", 25, 1, 1, 1, 1, 1.15, .96, 1, 1, 1, 1, 1, 1},
	"universal_education":     {"Universal Education Program", "Accelerate Education development and improve workforce participation.", 25, 1, 1, 1, .98, 1, 1.01, 1.15, 1, 1, 1.02, 1, 1},
	"national_health_service": {"National Health Service", "Reduce disease nationwide through sustained public healthcare funding.", 25, 1, 1, 1, .96, 1, 1.02, 1, .72, 1, 1, 1, 1},
	"community_policing":      {"Community Policing Initiative", "Reduce crime through local prevention and professional public security.", 20, 1, 1, 1, .97, 1, 1, 1, 1, .72, 1, 1, 1},
	"food_security":           {"Food Security Program", "Reduce population food consumption through storage, distribution, and waste controls.", 20, 1, 1, 1, .97, 1, 1.02, 1, 1, 1, 1, .88, 1},
	"public_works":            {"Public Works Program", "Create employment while reducing national Infrastructure upkeep.", 25, 1, 1, 1, .96, 1, 1.01, 1, 1, 1, 1.03, 1, .90},
}
var strategicCommodities = []string{"foodstuffs", "timber", "fibers", "basic_metals", "energy", "strategic_minerals", "textiles", "processed_foods", "construction_materials", "basic_goods", "consumer_goods", "military_equipment", "luxury_goods"}
var commodityRecipes = map[string]map[string]float64{"textiles": {"fibers": .8, "energy": .15}, "processed_foods": {"foodstuffs": .9, "energy": .1}, "construction_materials": {"timber": .45, "basic_metals": .45, "energy": .15}, "basic_goods": {"basic_metals": .5, "timber": .2, "energy": .2}, "consumer_goods": {"basic_goods": .55, "fibers": .2, "energy": .2}, "military_equipment": {"basic_metals": .7, "energy": .35, "strategic_minerals": .12}, "luxury_goods": {"consumer_goods": .5, "strategic_minerals": .08, "energy": .15}}

func defaultProductionQuotas() map[string]float64 {
	return map[string]float64{"textiles": 14.29, "processed_foods": 14.29, "construction_materials": 14.29, "basic_goods": 14.29, "consumer_goods": 14.28, "military_equipment": 14.28, "luxury_goods": 14.28}
}

func startingDepositRichness(continent, resource string) float64 {
	profiles := map[string]map[string]float64{
		"Africa":        {"foodstuffs": 1.05, "timber": .75, "fibers": 1.10, "basic_metals": 1.35, "energy": .90, "strategic_minerals": 1.30},
		"Asia":          {"foodstuffs": 1.05, "timber": .70, "fibers": 1.35, "basic_metals": .90, "energy": 1.25, "strategic_minerals": .85},
		"Europe":        {"foodstuffs": 1.10, "timber": .90, "fibers": .90, "basic_metals": 1.00, "energy": .70, "strategic_minerals": .70},
		"North America": {"foodstuffs": 1.10, "timber": 1.35, "fibers": .75, "basic_metals": .90, "energy": 1.25, "strategic_minerals": 1.00},
		"South America": {"foodstuffs": 1.25, "timber": 1.30, "fibers": 1.15, "basic_metals": 1.20, "energy": .70, "strategic_minerals": .85},
		"Oceania":       {"foodstuffs": .90, "timber": .85, "fibers": .75, "basic_metals": 1.20, "energy": 1.05, "strategic_minerals": 1.35},
		"Antarctica":    {"foodstuffs": .45, "timber": .45, "fibers": .45, "basic_metals": .65, "energy": 1.10, "strategic_minerals": 1.50},
	}
	if value := profiles[continent][resource]; value > 0 {
		return value
	}
	return 1
}

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
	ID, Name, Specialization       string
	Infra, Development             float64 // Development is retained only for legacy test/data compatibility.
	Deposits                       map[string]float64
	Upgrades                       map[string]int
	UpgradeQuotes                  map[string]int64
	CurrentUpgradeBenefits         map[string]map[string]float64
	NextUpgradeBenefits            map[string]map[string]float64
	InfrastructureQuotes           map[string]int64
	UpgradeCap                     int // Per-category hard cap retained for API compatibility.
	UpgradeCapacity                int
	UpgradesUsed                   int
	NextUpgradeCapacityAt          int
	CivicCapacity                  int
	CivicUsed                      int
	Institutions                   map[string]int
	IsCapital                      bool
	Population                     float64
	EmploymentRate, Disease, Crime float64
}
type strategicInput struct {
	Gear                  string
	Disrupted             bool
	Policies              map[string]bool
	Education, Technology float64
	Provinces             []provinceStrategy
	Quotas                map[string]float64
	Projects              map[string]bool
	LongTermProjects      map[string]bool
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
	EducationMultiplier  float64                       `json:"educationMultiplier"`
	ProvinceProduction   map[string]map[string]float64 `json:"provinceProduction"`
	ProvinceFactors      map[string]map[string]float64 `json:"provinceFactors"`
}

func calculateStrategy(in strategicInput) strategicResult {
	g := gears[in.Gear]
	if g.Name == "" {
		g = gears["balanced"]
	}
	r := strategicResult{Production: map[string]float64{}, ProvinceProduction: map[string]map[string]float64{}, ProvinceFactors: map[string]map[string]float64{}, IncomeMultiplier: g.Commerce, PopulationMultiplier: g.Population, HappinessMultiplier: g.Happiness, ExtractionMultiplier: g.Extraction, IndustryMultiplier: g.Industry, CommerceMultiplier: g.Commerce, MilitaryMultiplier: g.Military, EducationMultiplier: 1}
	if in.Projects["resource_survey"] {
		r.ExtractionMultiplier *= 1.12
	}
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
		r.EducationMultiplier *= p.Education
		r.IncomeMultiplier *= p.Commerce
	}
	knowledge := 1 + in.Education*.0025 + in.Technology*.004
	for _, p := range in.Provinces {
		out := map[string]float64{}
		employment := p.EmploymentRate
		if employment <= 0 {
			employment = 72
		}
		workforceFactor := clamp(1+(employment-72)*.006, .72, 1.12)
		healthFactor := clamp(1-p.Disease*.90, .70, 1)
		securityFactor := clamp(1-p.Crime*.55, .78, 1)
		operationalFactor := clamp(workforceFactor*healthFactor*securityFactor, .60, 1.12)
		r.ProvinceFactors[p.ID] = map[string]float64{"employmentRate": employment, "workforceFactor": workforceFactor, "diseaseRate": p.Disease, "healthFactor": healthFactor, "crimeRate": p.Crime, "securityFactor": securityFactor, "operationalFactor": operationalFactor}
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
		r.PopulationMultiplier *= 1 + civil*.003
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
				agricultureBoost += agriculture * .045
			}
			v := p.Infra * .10 * richness * r.ExtractionMultiplier * resourceSpec * agricultureBoost * (1 + extraction*.04) * alignment * operationalFactor
			out[resource] += v
			r.Production[resource] += v
		}
		capacity := p.Infra * .055 * specIndustry * r.IndustryMultiplier * knowledge * (1 + light*.025 + heavy*.03) * alignment * operationalFactor
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
				modifier *= 1 + light*.035
			}
			if k == "construction_materials" || k == "consumer_goods" || k == "luxury_goods" {
				modifier *= 1 + heavy*.035
			}
			if k == "military_equipment" {
				modifier *= r.MilitaryMultiplier / r.IndustryMultiplier * (1 + military*.04)
			}
			v := capacity * share * modifier
			out[k] += v
			r.Production[k] += v
		}
		r.ProvinceProduction[p.ID] = out
	}
	efficiency := 1.0
	if in.LongTermProjects["technological_research_council"] {
		efficiency *= 1.06
	}
	if in.LongTermProjects["logistical_optimization"] {
		efficiency *= 1.05
	}
	for id := range in.LongTermProjects {
		p, ok := longTermProjects[id]
		if !ok || p.Target == "" {
			continue
		}
		multiplier := 1 + p.ProductionBoost
		r.Production[p.Target] *= multiplier
		for province := range r.ProvinceProduction {
			r.ProvinceProduction[province][p.Target] *= multiplier
		}
	}
	for resource := range r.Production {
		r.Production[resource] *= efficiency
	}
	for province := range r.ProvinceProduction {
		for resource := range r.ProvinceProduction[province] {
			r.ProvinceProduction[province][resource] *= efficiency
		}
	}
	if in.LongTermProjects["population_development"] {
		r.PopulationMultiplier *= 1.08
	}
	return r
}

func applyProvincialOperatingConditions(in *strategicInput, economy NationResult) {
	byID := make(map[string]CityResult, len(economy.Cities))
	for _, city := range economy.Cities {
		byID[city.ID] = city
	}
	for index := range in.Provinces {
		if city, ok := byID[in.Provinces[index].ID]; ok {
			in.Provinces[index].EmploymentRate = city.EmploymentRate
			in.Provinces[index].Disease = city.Disease
			in.Provinces[index].Crime = city.Crime
		}
	}
}

func (a *app) loadStrategy(ctx context.Context, nid string) (strategicInput, error) {
	in := strategicInput{Policies: map[string]bool{}, Quotas: map[string]float64{}, Projects: map[string]bool{}, LongTermProjects: map[string]bool{}}
	if e := a.db.QueryRowContext(ctx, `SELECT s.gear,(s.disruption_until IS NOT NULL AND s.disruption_until>NOW()),n.education,n.technology FROM nation_economic_strategy s JOIN nations n ON n.id=s.nation_id WHERE s.nation_id=?`, nid).Scan(&in.Gear, &in.Disrupted, &in.Education, &in.Technology); e != nil {
		return in, e
	}
	rows, e := a.db.QueryContext(ctx, `SELECT policy_key FROM social_policy_selections WHERE nation_id=?`, nid)
	if e != nil {
		return in, e
	}
	in.LongTermProjects = loadLongTermProjectSet(ctx, a.db, nid)
	projectRows, projectErr := a.db.QueryContext(ctx, `SELECT project_type FROM national_projects WHERE nation_id=?`, nid)
	if projectErr != nil {
		rows.Close()
		return in, projectErr
	}
	for projectRows.Next() {
		var key string
		if projectRows.Scan(&key) == nil {
			in.Projects[key] = true
		}
	}
	projectRows.Close()
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
	rows, e = a.db.QueryContext(ctx, `SELECT c.id,c.name,p.specialization,c.infrastructure,c.local_population,COALESCE(c.id=n.capital_city_id,0) FROM cities c JOIN province_economies p ON p.city_id=c.id JOIN nations n ON n.id=c.nation_id WHERE c.nation_id=? ORDER BY COALESCE(c.id=n.capital_city_id,0) DESC,c.created_at ASC,c.id ASC`, nid)
	if e != nil {
		return in, e
	}
	for rows.Next() {
		var p provinceStrategy
		p.Deposits = map[string]float64{}
		p.Upgrades = map[string]int{}
		p.Institutions = map[string]int{}
		p.UpgradeQuotes = map[string]int64{}
		p.CurrentUpgradeBenefits = map[string]map[string]float64{}
		p.NextUpgradeBenefits = map[string]map[string]float64{}
		p.InfrastructureQuotes = map[string]int64{}
		rows.Scan(&p.ID, &p.Name, &p.Specialization, &p.Infra, &p.Population, &p.IsCapital)
		in.Provinces = append(in.Provinces, p)
	}
	rows.Close()
	for i := range in.Provinces {
		in.Provinces[i].Upgrades = loadProvinceUpgrades(ctx, a.db, in.Provinces[i].ID)
		in.Provinces[i].UpgradeCap = provinceUpgradeLevelHardCap
		in.Provinces[i].UpgradeCapacity = provinceUpgradeCapacity(in.Provinces[i].Infra)
		in.Provinces[i].UpgradesUsed = provinceUpgradesUsed(in.Provinces[i].Upgrades)
		in.Provinces[i].NextUpgradeCapacityAt = nextProvinceUpgradeCapacityAt(in.Provinces[i].Infra)
		in.Provinces[i].CivicCapacity = civicInstitutionCapacity(in.Provinces[i].Infra)
		for key, spec := range provinceUpgradeSpecs {
			level := in.Provinces[i].Upgrades[key]
			in.Provinces[i].UpgradeQuotes[key] = provinceUpgradeCost(spec, level, in.Provinces[i].Infra)
			in.Provinces[i].CurrentUpgradeBenefits[key] = provinceUpgradeBenefits(key, level)
			in.Provinces[i].NextUpgradeBenefits[key] = provinceUpgradeBenefits(key, min(level+1, spec.HardCap))
			if in.LongTermProjects["infrastructure_bank"] {
				in.Provinces[i].UpgradeQuotes[key] = int64(math.Ceil(float64(in.Provinces[i].UpgradeQuotes[key]) * .90))
			}
		}
		for _, amount := range []int{10, 50, 100} {
			in.Provinces[i].InfrastructureQuotes[fmt.Sprint(amount)] = int64(infraPurchaseCost(in.Provinces[i].Infra, float64(amount), int(in.Technology)))
			if in.LongTermProjects["infrastructure_bank"] {
				in.Provinces[i].InfrastructureQuotes[fmt.Sprint(amount)] = int64(math.Ceil(float64(in.Provinces[i].InfrastructureQuotes[fmt.Sprint(amount)]) * .88))
			}
		}
		ds, _ := a.db.QueryContext(ctx, `SELECT resource,richness FROM province_deposits WHERE city_id=?`, in.Provinces[i].ID)
		for ds.Next() {
			var k string
			var v float64
			ds.Scan(&k, &v)
			in.Provinces[i].Deposits[k] = v
		}
		ds.Close()
		institutionRows, _ := a.db.QueryContext(ctx, `SELECT building_type,quantity FROM city_improvements WHERE city_id=?`, in.Provinces[i].ID)
		for institutionRows.Next() {
			var key string
			var quantity int
			if institutionRows.Scan(&key, &quantity) == nil {
				in.Provinces[i].Institutions[key] = quantity
				in.Provinces[i].CivicUsed += quantity
			}
		}
		institutionRows.Close()
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
	var economicResult NationResult
	if economicNation, _, _, economicErr := a.loadEconomicNationContext(r.Context(), u.ID); economicErr == nil {
		economicResult = calculateEconomy(economicNation)
		applyProvincialOperatingConditions(&in, economicResult)
	}
	result := calculateStrategy(in)
	crisisModifiers := a.loadCrisisModifiers(r.Context(), nid)
	applyCrisisTurnModifiers(&result, crisisModifiers)
	militaryCashUpkeep, _ := militaryUpkeepProjection(r.Context(), a.db, nid)
	militaryFoodUpkeep := militaryFoodUpkeepProjection(r.Context(), a.db, nid)
	distress := assessEconomicDistress(r.Context(), a.db, nid,
		(economicResult.DailyCivilianFoodConsumption+militaryFoodUpkeep)/balance.TurnsPerDay,
		result.Production["foodstuffs"]/balance.TurnsPerDay,
		economicResult.DailyTax*result.IncomeMultiplier/balance.TurnsPerDay,
		(economicResult.DailyUpkeep*(1-crisisModifiers.UpkeepReductionPct/100)+militaryCashUpkeep)/balance.TurnsPerDay,
	)
	applyEconomicDistress(&result, distress)
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
		policyList = append(policyList, map[string]any{"key": k, "name": p.Name, "description": p.Description, "cost": p.PoliticalCost, "active": in.Policies[k], "effects": map[string]float64{"population": p.Population, "extraction": p.Extraction, "industry": p.Industry, "commerce": p.Commerce, "military": p.Military, "happiness": p.Happiness, "education": p.Education, "disease": p.Disease, "crime": p.Crime, "employment": p.Employment, "foodConsumption": p.FoodConsumption, "infrastructureUpkeep": p.InfrastructureUpkeep}})
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
		upgradeList = append(upgradeList, map[string]any{"key": key, "name": spec.Name, "description": spec.Description, "baseCost": spec.BaseCost, "hardCap": spec.HardCap})
	}
	institutionList := []map[string]any{}
	keys = keys[:0]
	for key := range buildings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		spec := buildings[key]
		institutionList = append(institutionList, map[string]any{"key": key, "name": spec.Name, "category": spec.Category, "description": spec.Description, "cashCost": int64(spec.Cost), "resourceCosts": spec.Costs, "dailyUpkeep": spec.DailyUpkeep, "commerce": spec.Commerce, "education": spec.Education, "happiness": spec.Happiness, "pollution": spec.Pollution, "crimeReduction": spec.CrimeReduction, "diseaseReduction": spec.DiseaseReduction, "employment": spec.Employment, "taxCollection": spec.TaxCollection, "minTech": spec.MinTech, "maxPerProvince": spec.MaxPerProvince})
	}
	cashCost, materialCost, strain := provinceFoundingCosts(len(in.Provinces), in.Gear, in.Policies)
	var lastProvince time.Time
	var nextProvinceAt any
	if len(in.Provinces) > 1 && a.db.QueryRowContext(r.Context(), `SELECT MAX(created_at) FROM cities WHERE nation_id=?`, nid).Scan(&lastProvince) == nil {
		if next := lastProvince.Add(7 * 24 * time.Hour); next.After(time.Now()) {
			nextProvinceAt = next
		}
	}
	expansion := map[string]any{"provinceCount": len(in.Provinces), "cashCost": cashCost, "constructionMaterials": materialCost, "happinessStrain": strain, "nextProvinceAt": nextProvinceAt, "gearModifier": expansionGearModifier(in.Gear), "policyModifier": expansionPolicyModifier(in.Policies), "formula": "¥25,000,000 × N^2.55 × Gear × Policy"}
	provinceCivicMetrics := map[string]any{}
	if len(economicResult.Cities) > 0 {
		for _, city := range economicResult.Cities {
			provinceCivicMetrics[city.ID] = map[string]any{"employmentRate": city.EmploymentRate, "taxCollectionMultiplier": city.TaxCollectionMultiplier, "disease": city.Disease, "crime": city.Crime, "dailyUpkeep": city.CivicUpkeep}
		}
	}
	write(w, 200, map[string]any{"gear": in.Gear, "gears": gearList, "policies": policyList, "politicalCapital": political, "technology": in.Technology, "gearChangedAt": changed, "disruptionUntil": disruption, "distress": distress, "provinces": in.Provinces, "provinceUpgradeTypes": upgradeList, "civicInstitutions": institutionList, "provinceCivicMetrics": provinceCivicMetrics, "expansion": expansion, "quotas": in.Quotas, "recipes": commodityRecipes, "stockpiles": stock, "result": result})
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
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not save production allocation.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM production_quotas WHERE nation_id=?`, nid); err != nil {
		problem(w, 500, "Could not save production allocation.")
		return
	}
	for k, v := range in.Quotas {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO production_quotas(nation_id,commodity,priority) VALUES(?,?,?)`, nid, k, v); err != nil {
			problem(w, 500, "Could not save production allocation.")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Could not save production allocation.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func applyStrategicTurn(ctx context.Context, tx *sql.Tx, nid string, in strategicInput, result strategicResult, civilianFoodNeed, militaryFoodNeed float64) error {
	produced := []string{}
	producedAmounts := map[string]float64{}
	for _, commodity := range []string{"foodstuffs", "timber", "fibers", "basic_metals", "energy", "strategic_minerals"} {
		hourly := result.Production[commodity] / 24
		if hourly <= 0 {
			continue
		}
		if _, e := tx.ExecContext(ctx, `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, nid, commodity, hourly); e != nil {
			return e
		}
		produced = append(produced, fmt.Sprintf("%.2f %s", hourly, commodityName(commodity)))
		producedAmounts[commodity] += hourly
	}
	// Population has first claim on food before industry converts primary inputs.
	if _, e := tx.ExecContext(ctx, `INSERT IGNORE INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,'foodstuffs',0)`, nid); e != nil {
		return e
	}
	var foodAvailable float64
	if e := tx.QueryRowContext(ctx, `SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity='foodstuffs' FOR UPDATE`, nid).Scan(&foodAvailable); e != nil {
		return e
	}
	civilianFoodConsumed := math.Min(civilianFoodNeed, foodAvailable)
	militaryFoodConsumed := math.Min(militaryFoodNeed, math.Max(0, foodAvailable-civilianFoodConsumed))
	foodConsumed := civilianFoodConsumed + militaryFoodConsumed
	if _, e := tx.ExecContext(ctx, `UPDATE nation_stockpiles SET amount=amount-? WHERE nation_id=? AND commodity='foodstuffs'`, foodConsumed, nid); e != nil {
		return e
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
		produced = append(produced, fmt.Sprintf("%.2f %s", actual, commodityName(commodity)))
		producedAmounts[commodity] += actual
	}
	allianceID, allianceName, _, resourceTaxRate := applicableAllianceTax(ctx, tx, nid)
	if allianceID != "" && resourceTaxRate > 0 {
		for commodity, gross := range producedAmounts {
			tax := gross * resourceTaxRate / 100
			if tax <= 0 {
				continue
			}
			if _, e := tx.ExecContext(ctx, `UPDATE nation_stockpiles SET amount=GREATEST(0,amount-?) WHERE nation_id=? AND commodity=?`, tax, nid, commodity); e != nil {
				return e
			}
			if _, e := tx.ExecContext(ctx, `INSERT INTO alliance_stockpiles(alliance_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, allianceID, commodity, tax); e != nil {
				return e
			}
			tx.ExecContext(ctx, `INSERT INTO alliance_bank_transactions(id,alliance_id,actor_nation_id,kind,resource,amount,memo) VALUES(?,?,?,'tax',?,?,?)`, uuid(), allianceID, nid, commodity, int64(math.Ceil(tax)), fmt.Sprintf("%.2f%% resource tax for %s", resourceTaxRate, allianceName))
		}
	}
	var turnRevenueNotifications bool
	tx.QueryRowContext(ctx, `SELECT u.turn_revenue_notifications FROM users u JOIN nations n ON n.owner_id=u.id WHERE n.id=?`, nid).Scan(&turnRevenueNotifications)
	foodNeed := civilianFoodNeed + militaryFoodNeed
	if turnRevenueNotifications && (len(produced) > 0 || foodNeed > 0) {
		message := fmt.Sprintf("Domestic demand consumed %.2f Foodstuffs (%.2f civilian, %.2f standing military).", foodConsumed, civilianFoodConsumed, militaryFoodConsumed)
		if len(produced) > 0 {
			message = "Last turn you produced: " + strings.Join(produced, ", ") + ". " + message
		}
		if foodConsumed+0.0001 < foodNeed {
			message += fmt.Sprintf(" Food shortage: %.2f Foodstuffs unmet.", foodNeed-foodConsumed)
		}
		if _, e := tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'economic','Turn production summary',?)`, uuid(), nid, message); e != nil {
			return e
		}
	}
	_, e := tx.ExecContext(ctx, `UPDATE nation_economic_strategy SET political_capital=LEAST(100,political_capital+.25) WHERE nation_id=?`, nid)
	return e
}
