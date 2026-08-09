package main

import "math"

// EconomyConfig is the single balance surface for the economic simulation.
// Values are daily; the hourly turn resolver divides flows by TurnsPerDay.
type EconomyConfig struct {
	TurnsPerDay, PopulationPerInfra, InfraPerSlot, BaseCitizenIncome float64
	FoodPerCitizen, HappinessIncomePerPoint, PopulationGrowthRate    float64
	EducationIncomeMax, TechnologyIncomePerLevel, InfraUpkeepBase    float64
}

var balance = EconomyConfig{24, 90, 50, 28, .001, .018, .0012, .55, .0025, .32}

type BuildingSpec struct {
	Name, Category, InputResource, OutputResource          string
	Cost, Power, Pollution, Output, Commerce               float64
	Education, Happiness, CrimeReduction, DiseaseReduction float64
	MinTech                                                int
}

var buildings = map[string]BuildingSpec{
	"coal_plant":        {Name: "Coal power plant", Category: "power", InputResource: "coal", Cost: 18000, Power: -120, Pollution: 8},
	"renewable_plant":   {Name: "Renewable power park", Category: "power", Cost: 42000, Power: -90, Pollution: .5, MinTech: 8},
	"farm":              {Name: "Industrial farm", Category: "extraction", OutputResource: "food", Cost: 8000, Output: 180, Power: 2, Pollution: 1},
	"coal_mine":         {Name: "Coal mine", Category: "extraction", OutputResource: "coal", Cost: 11000, Output: 55, Power: 3, Pollution: 4},
	"iron_mine":         {Name: "Iron mine", Category: "extraction", OutputResource: "iron", Cost: 13000, Output: 45, Power: 3, Pollution: 4},
	"oil_well":          {Name: "Oil well", Category: "extraction", OutputResource: "oil", Cost: 18000, Output: 35, Power: 4, Pollution: 5},
	"bauxite_mine":      {Name: "Bauxite mine", Category: "extraction", OutputResource: "bauxite", Cost: 15000, Output: 40, Power: 3, Pollution: 4},
	"steel_mill":        {Name: "Steel mill", Category: "manufacturing", InputResource: "iron+coal", OutputResource: "steel", Cost: 30000, Output: 24, Power: 18, Pollution: 7, MinTech: 3},
	"aluminum_refinery": {Name: "Aluminum refinery", Category: "manufacturing", InputResource: "bauxite", OutputResource: "aluminum", Cost: 34000, Output: 22, Power: 20, Pollution: 6, MinTech: 5},
	"oil_refinery":      {Name: "Oil refinery", Category: "manufacturing", InputResource: "oil", OutputResource: "gasoline", Cost: 36000, Output: 25, Power: 16, Pollution: 7, MinTech: 5},
	"bank":              {Name: "Commercial bank", Category: "commerce", Cost: 22000, Power: 5, Commerce: 8},
	"shopping_mall":     {Name: "Shopping mall", Category: "commerce", Cost: 28000, Power: 8, Commerce: 11, Pollution: 1},
	"hospital":          {Name: "Hospital", Category: "civil", Cost: 26000, Power: 8, Happiness: 1.5, DiseaseReduction: .012},
	"police_station":    {Name: "Police station", Category: "civil", Cost: 17000, Power: 4, CrimeReduction: .012},
	"school":            {Name: "School system", Category: "civil", Cost: 16000, Power: 3, Education: .7, Happiness: .4},
	"university":        {Name: "University", Category: "civil", Cost: 38000, Power: 9, Education: 1.6, MinTech: 4},
	"recycling_center":  {Name: "Recycling center", Category: "civil", Cost: 21000, Power: 5, Pollution: -5, Happiness: .5},
	"park":              {Name: "Public park", Category: "civil", Cost: 12000, Happiness: 1.1, Pollution: -1},
}

type ProjectSpec struct {
	Name, Theme, Description                string
	Cash, Iron, Steel, Aluminum, Coal, Food int64
}

var beginnerProjects = map[string]ProjectSpec{
	"civil_engineering_corps":     {"Civil Engineering Corps", "Infrastructure growth", "Infrastructure costs −6% and upkeep −4%.", 150000, 400, 150, 0, 0, 0},
	"public_education_initiative": {"Public Education Initiative", "Education", "Education +5 immediately, stronger passive Education, and Happiness support.", 120000, 0, 100, 0, 0, 0},
	"commerce_foundation":         {"Commerce Foundation", "Commerce", "Commerce cap rises to 110% and every city gains 3 Commerce.", 140000, 0, 150, 50, 0, 0},
	"resource_survey":             {"Resource Survey & Extraction Boost", "Extraction", "Farm, mine, and well output increases by 12%.", 100000, 250, 50, 0, 0, 0},
	"basic_power_grid":            {"Basic Power Grid", "Power", "Power capacity rises by 10% and power improvements cost less.", 130000, 0, 100, 0, 300, 0},
	"public_health_sanitation":    {"Public Health & Sanitation", "Stability", "Lower disease and pollution pressure with better Happiness recovery.", 115000, 0, 80, 0, 0, 500},
}

type ModelCity struct {
	ID, Name                                                     string
	Infra, Land, Population, Commerce, Pollution, Disease, Crime float64
	Buildings                                                    map[string]int
	Upgrades                                                     map[string]int
}

type ModelNation struct {
	TaxRate, Happiness, Education float64
	Technology                    int
	Doctrine                      string
	Projects                      map[string]bool
	LongTermProjects              map[string]bool
	Cities                        []ModelCity
}

type CityResult struct {
	ID, Name            string             `json:"id"`
	Slots, Used         int                `json:"slots"`
	BasePopulation      float64            `json:"basePopulation"`
	EffectivePopulation float64            `json:"effectivePopulation"`
	DensityMultiplier   float64            `json:"densityMultiplier"`
	Commerce            float64            `json:"commerce"`
	PowerCapacity       float64            `json:"powerCapacity"`
	PowerUsage          float64            `json:"powerUsage"`
	PowerMultiplier     float64            `json:"powerMultiplier"`
	Pollution           float64            `json:"pollution"`
	Disease             float64            `json:"disease"`
	Crime               float64            `json:"crime"`
	CitizenIncome       float64            `json:"citizenIncome"`
	TaxRevenue          float64            `json:"taxRevenue"`
	Upkeep              float64            `json:"upkeep"`
	Production          map[string]float64 `json:"production"`
	Contributors        map[string]float64 `json:"contributors"`
}

type NationResult struct {
	Cities                []CityResult       `json:"cities"`
	DailyTax              float64            `json:"dailyTax"`
	DailyUpkeep           float64            `json:"dailyUpkeep"`
	NetDailyCash          float64            `json:"netDailyCash"`
	Population            float64            `json:"population"`
	HappinessTarget       float64            `json:"happinessTarget"`
	EducationChange       float64            `json:"educationChange"`
	Production            map[string]float64 `json:"production"`
	Contributors          map[string]float64 `json:"contributors"`
	DailyFoodConsumption  float64            `json:"dailyFoodConsumption"`
	HourlyFoodConsumption float64            `json:"hourlyFoodConsumption"`
	DailyFoodProduction   float64            `json:"dailyFoodProduction"`
	NetDailyFood          float64            `json:"netDailyFood"`
}

func infraUnitCost(current float64, tech int) float64 {
	x := math.Max(0, current-5)
	discount := math.Max(.72, 1-.004*float64(tech))
	return (475 + math.Pow(x, 2.15)/760) * discount
}

func infraPurchaseCost(current, amount float64, tech int) float64 {
	if amount <= 0 {
		return 0
	}
	// Integral approximation makes bulk quotes deterministic while charging the curve.
	steps := math.Ceil(amount)
	total := 0.0
	for i := 0.0; i < steps; i++ {
		total += infraUnitCost(current+i, tech)
	}
	if amount >= 100 {
		total *= .97
	} else if amount >= 50 {
		total *= .98
	}
	return math.Ceil(total)
}

func landPurchaseCost(current, amount float64, tech int) float64 {
	if amount <= 0 {
		return 0
	}
	total := 0.0
	for i := 0.0; i < math.Ceil(amount); i++ {
		total += (90 + math.Pow(math.Max(0, current+i), 1.72)/520) * math.Max(.78, 1-.003*float64(tech))
	}
	if amount >= 100 {
		total *= .98
	}
	return math.Ceil(total)
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func calculateEconomy(n ModelNation) NationResult {
	out := NationResult{Production: map[string]float64{}, Contributors: map[string]float64{}}
	weightedLocal, totalBase, educationBuildings := 0.0, 0.0, 0.0
	doctrineProduction, doctrineIncome, doctrineHappiness, commerceCap := 1.0, 1.0, 0.0, 100.0
	switch n.Doctrine {
	case "Capitalist":
		doctrineIncome, commerceCap, doctrineHappiness = 1.06, 115, -2
	case "Planned":
		doctrineProduction, doctrineIncome = 1.08, .97
	case "Green":
		doctrineProduction, doctrineHappiness = .97, 3
	}
	if n.Projects["commerce_foundation"] {
		commerceCap = math.Max(commerceCap, 110)
	}
	for _, c := range n.Cities {
		r := CityResult{ID: c.ID, Name: c.Name, Production: map[string]float64{}, Contributors: map[string]float64{}}
		r.Slots = int(math.Floor(c.Infra / balance.InfraPerSlot))
		if r.Slots < 1 {
			r.Slots = 1
		}
		for _, q := range c.Buildings {
			r.Used += q
		}
		agriculture := provinceUpgradeEffect(c.Upgrades["agriculture"])
		commerceUpgrade := provinceUpgradeEffect(c.Upgrades["commerce"])
		civil := provinceUpgradeEffect(c.Upgrades["civil"])
		r.BasePopulation = c.Infra * balance.PopulationPerInfra * (1 + agriculture*.006 + civil*.009)
		density := r.BasePopulation / math.Max(1, c.Land*75)
		r.DensityMultiplier = 1
		if density > 1 {
			r.DensityMultiplier = clamp(1-(density-1)*.08, .70, 1)
		}
		powerCapacity, powerUse, commerce, pollution := 0.0, 0.0, 0.0, c.Pollution
		diseaseReduction, crimeReduction, localHappiness := 0.0, 0.0, civil*.55
		educationBuildings += civil * .12
		for key, q := range c.Buildings {
			s, ok := buildings[key]
			if !ok {
				continue
			}
			v := float64(q)
			if s.Power < 0 {
				powerCapacity += -s.Power * v
			} else {
				powerUse += s.Power * v
			}
			commerce += s.Commerce * v
			pollution += s.Pollution * v
			educationBuildings += s.Education * v
			diseaseReduction += s.DiseaseReduction * v
			crimeReduction += s.CrimeReduction * v
			localHappiness += s.Happiness * v
		}
		r.PowerCapacity, r.PowerUsage = powerCapacity, powerUse
		r.PowerMultiplier = 1
		if powerUse > powerCapacity && powerUse > 0 {
			r.PowerMultiplier = clamp(powerCapacity/powerUse, 0, 1)
		}
		if n.Projects["commerce_foundation"] {
			commerce += 3
		}
		if n.Projects["basic_power_grid"] {
			r.PowerCapacity *= 1.10
			powerCapacity = r.PowerCapacity
			if powerUse > powerCapacity && powerUse > 0 {
				r.PowerMultiplier = clamp(powerCapacity/powerUse, 0, 1)
			}
		}
		r.Commerce = math.Min(commerceCap, commerce) * r.PowerMultiplier
		if n.LongTermProjects["commerce_facilitation"] {
			r.Commerce = math.Min(commerceCap*1.08, r.Commerce*1.10)
		}
		r.Pollution = math.Max(0, pollution)
		healthReduction := 0.0
		if n.Projects["public_health_sanitation"] {
			healthReduction = .01
		}
		r.Disease = clamp(.012+r.Pollution*.0007-n.Education*.00008-diseaseReduction-healthReduction, 0, .30)
		r.Crime = clamp(.04-n.Education*.0003+(50-n.Happiness)*.0004-crimeReduction, 0, .30)
		if n.LongTermProjects["public_health_program"] {
			r.Disease *= .75
		}
		if n.LongTermProjects["internal_security_reform"] {
			r.Crime *= .75
		}
		for key, q := range c.Buildings {
			s := buildings[key]
			if s.Category != "extraction" && s.Category != "manufacturing" {
				continue
			}
			spec := 1.0
			if s.Category == "manufacturing" {
				spec = 1 + math.Min(.5, float64(max(0, q-1))*.11)
			}
			extractionBoost := 1.0
			if s.Category == "extraction" && n.Projects["resource_survey"] {
				extractionBoost = 1.12
			}
			eff := (1 + float64(n.Technology)*.012) * (1 + n.Education*.004) * r.PowerMultiplier * spec * doctrineProduction * extractionBoost
			r.Production[s.OutputResource] += s.Output * float64(q) * eff
		}
		happyMult := clamp(1+(n.Happiness-50)*.008, .55, 1.45)
		eduBonus := n.Education / 100 * .22
		r.EffectivePopulation = math.Max(1000, r.BasePopulation*(1+eduBonus)*happyMult*(1-r.Disease)*(1-r.Crime)*r.DensityMultiplier)
		if n.LongTermProjects["population_development"] {
			r.EffectivePopulation *= 1.03
		}
		r.CitizenIncome = balance.BaseCitizenIncome * (1 + (n.Happiness-50)*balance.HappinessIncomePerPoint) * (1 + n.Education/100*balance.EducationIncomeMax) * (1 + float64(n.Technology)*balance.TechnologyIncomePerLevel)
		r.CitizenIncome = math.Max(5, r.CitizenIncome)
		if n.LongTermProjects["national_education_act"] {
			r.CitizenIncome *= 1.04
		}
		r.TaxRevenue = r.EffectivePopulation * r.CitizenIncome * (n.TaxRate / 100) * (1 + r.Commerce/100) * doctrineIncome * (1 + commerceUpgrade*.018)
		if n.LongTermProjects["tax_modernization"] {
			r.TaxRevenue *= 1.08
		}
		upkeepModifier := 1.0
		if n.Projects["civil_engineering_corps"] {
			upkeepModifier = .96
		}
		r.Upkeep = c.Infra * balance.InfraUpkeepBase * (1 + math.Floor(c.Infra/1200)*.12) * upkeepModifier
		r.Contributors = map[string]float64{"happinessMultiplier": happyMult, "educationBonus": eduBonus, "densityMultiplier": r.DensityMultiplier, "diseaseMultiplier": 1 - r.Disease, "crimeMultiplier": 1 - r.Crime, "powerMultiplier": r.PowerMultiplier, "agriculturePopulationBonus": 1 + agriculture*.006, "civilPopulationBonus": 1 + civil*.009, "commerceUpgradeBonus": 1 + commerceUpgrade*.018}
		out.DailyTax += r.TaxRevenue
		out.DailyUpkeep += r.Upkeep
		out.Population += r.EffectivePopulation
		totalBase += r.BasePopulation
		pollutionPenalty := .18
		if n.Projects["public_health_sanitation"] {
			pollutionPenalty = .14
		}
		localH := 55 + localHappiness - r.Pollution*pollutionPenalty - r.Disease*100*.35 - r.Crime*100*.3
		weightedLocal += localH * r.BasePopulation
		for k, v := range r.Production {
			out.Production[k] += v
		}
		out.Cities = append(out.Cities, r)
	}
	taxPenalty := math.Max(0, n.TaxRate-10) * .72
	local := 50.0
	if totalBase > 0 {
		local = weightedLocal / totalBase
	}
	out.HappinessTarget = clamp(local+n.Education*.12-taxPenalty+doctrineHappiness, 0, 100)
	educationModifier := 1.0
	if n.Projects["public_education_initiative"] {
		educationModifier = 1.15
	}
	out.EducationChange = educationBuildings*.035*educationModifier - n.Education*.0015
	if n.LongTermProjects["national_education_act"] {
		out.EducationChange += .15
	}
	if n.Projects["public_education_initiative"] {
		out.HappinessTarget = clamp(out.HappinessTarget+1, 0, 100)
	}
	if n.LongTermProjects["civil_service_reform"] {
		out.HappinessTarget = clamp(out.HappinessTarget+8, 0, 100)
	}
	if n.LongTermProjects["public_health_program"] {
		out.HappinessTarget = clamp(out.HappinessTarget+4, 0, 100)
	}
	out.NetDailyCash = out.DailyTax - out.DailyUpkeep
	out.DailyFoodConsumption = out.Population * balance.FoodPerCitizen
	out.HourlyFoodConsumption = out.DailyFoodConsumption / balance.TurnsPerDay
	out.Contributors = map[string]float64{"localConditions": local, "taxPenalty": -taxPenalty, "educationHappiness": n.Education * .12, "currentHappiness": n.Happiness, "targetHappiness": out.HappinessTarget}
	return out
}
