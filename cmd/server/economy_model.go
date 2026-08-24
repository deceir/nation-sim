package main

import "math"

// EconomyConfig is the single balance surface for the economic simulation.
// Values are daily; the hourly turn resolver divides flows by TurnsPerDay.
type EconomyConfig struct {
	TurnsPerDay, PopulationPerInfra, InfraPerSlot, BaseCitizenIncome float64
	FoodPerCitizen, HappinessIncomePerPoint, PopulationGrowthRate    float64
	EducationIncomeMax, TechnologyIncomePerLevel, InfraUpkeepBase    float64
}

const yenScale int64 = 100

var balance = EconomyConfig{24, 90, 50, 500, .001, .018, .0020, .55, .0025, 32}

// startingNationPopulation uses the same model as the hourly turn so a newly
// founded nation never presents a temporary placeholder population.
func startingNationPopulation() int64 {
	result := calculateEconomy(ModelNation{
		TaxRate:        25,
		Happiness:      65,
		Education:      40,
		EmploymentRate: 72,
		Technology:     20,
		Doctrine:       "Balanced",
		Cities: []ModelCity{{
			Infra:     100,
			Land:      150,
			Buildings: map[string]int{},
			Upgrades:  map[string]int{},
		}},
	})
	return int64(result.Population)
}

type BuildingSpec struct {
	Name, Category, Description                            string
	Cost, Power, Pollution, Commerce                       float64
	Education, Happiness, CrimeReduction, DiseaseReduction float64
	Employment, TaxCollection, DailyUpkeep                 float64
	Costs                                                  map[string]float64
	MinTech, MaxPerProvince                                int
}

var buildings = map[string]BuildingSpec{
	"renewable_plant":   {Name: "Municipal Utility Grid", Category: "utilities", Description: "Public generation supports power-intensive civic and commercial institutions.", Cost: 42000, Power: -90, Pollution: .5, DailyUpkeep: 1500, Costs: map[string]float64{"construction_materials": 30, "basic_metals": 20, "energy": 10}, MinTech: 8, MaxPerProvince: 2},
	"bank":              {Name: "Commercial Bank", Category: "commerce", Description: "Formal credit and payment services expand taxable commerce and improve revenue collection.", Cost: 22000, Commerce: 5, Employment: 1.2, TaxCollection: 2.5, DailyUpkeep: 900, Costs: map[string]float64{"construction_materials": 14, "basic_goods": 8}, MaxPerProvince: 3},
	"shopping_mall":     {Name: "Commercial Arcade", Category: "commerce", Description: "A large retail district creates jobs and concentrates consumer spending in the formal economy.", Cost: 28000, Power: 8, Commerce: 9, Pollution: 1, Employment: 2.5, TaxCollection: 1, DailyUpkeep: 1200, Costs: map[string]float64{"construction_materials": 20, "consumer_goods": 8}, MaxPerProvince: 4},
	"marketplace":       {Name: "Municipal Marketplace", Category: "commerce", Description: "A regulated market creates accessible work and brings informal trade into the taxable economy.", Cost: 18000, Commerce: 4, Employment: 2.8, TaxCollection: 1, DailyUpkeep: 700, Costs: map[string]float64{"timber": 12, "construction_materials": 10}, MaxPerProvince: 4},
	"trade_exchange":    {Name: "Provincial Trade Exchange", Category: "commerce", Description: "A formal exchange concentrates wholesale trade, finance, and professional employment.", Cost: 34000, Commerce: 8, Employment: 2, TaxCollection: 2, DailyUpkeep: 1400, Costs: map[string]float64{"construction_materials": 24, "consumer_goods": 8}, MinTech: 5, MaxPerProvince: 3},
	"transit_authority": {Name: "Public Transit Authority", Category: "commerce", Description: "Reliable transport connects residents to workplaces and widens the effective labor market.", Cost: 27000, Employment: 3.5, Happiness: .4, DailyUpkeep: 1300, Costs: map[string]float64{"construction_materials": 22, "basic_metals": 14, "energy": 8}, MaxPerProvince: 3},
	"municipal_office":  {Name: "Revenue Administration Office", Category: "government", Description: "Professional assessors and records offices improve compliance and reduce uncollected taxes.", Cost: 20000, Employment: 1, TaxCollection: 3.5, DailyUpkeep: 1100, Costs: map[string]float64{"construction_materials": 12, "basic_goods": 10}, MaxPerProvince: 3},
	"hospital":          {Name: "Provincial Hospital", Category: "services", Description: "Scalable medical coverage offsets disease created by dense extraction and industry.", Cost: 26000, Happiness: 1.5, DiseaseReduction: .008, Employment: .7, DailyUpkeep: 1600, Costs: map[string]float64{"construction_materials": 20, "processed_foods": 10, "basic_goods": 10}, MaxPerProvince: 8},
	"police_station":    {Name: "Civic Police Station", Category: "services", Description: "Scalable public security offsets crime associated with larger commercial and industrial economies.", Cost: 17000, CrimeReduction: .009, TaxCollection: .5, Employment: .5, DailyUpkeep: 950, Costs: map[string]float64{"construction_materials": 10, "basic_goods": 8}, MaxPerProvince: 8},
	"school":            {Name: "Public School System", Category: "education", Description: "Scalable schooling raises national Education and gradually improves workforce quality.", Cost: 16000, Education: .7, Happiness: .4, Employment: .5, DailyUpkeep: 900, Costs: map[string]float64{"construction_materials": 12, "basic_goods": 10}, MaxPerProvince: 8},
	"university":        {Name: "Provincial University", Category: "education", Description: "Higher education accelerates national Education and supports a skilled service economy.", Cost: 38000, Education: 1.6, Employment: 1, Commerce: 2, DailyUpkeep: 1800, Costs: map[string]float64{"construction_materials": 28, "consumer_goods": 10}, MinTech: 4, MaxPerProvince: 3},
	"park":              {Name: "Public Commons", Category: "services", Description: "Maintained civic space improves Satisfaction and slightly reduces local pollution.", Cost: 12000, Happiness: 1.1, Pollution: -1, DailyUpkeep: 450, Costs: map[string]float64{"timber": 8, "basic_goods": 4}, MaxPerProvince: 3},
	"recycling_center":  {Name: "Municipal Recovery Works", Category: "utilities", Description: "Scalable recovery facilities reduce pollution produced by dense provincial development.", Cost: 21000, Pollution: -5, Happiness: .5, Employment: .6, DailyUpkeep: 800, Costs: map[string]float64{"construction_materials": 14, "basic_metals": 8}, MaxPerProvince: 8},
}

type ProjectSpec struct {
	Name, Theme, Description string
	Cash                     int64
	Costs                    map[string]float64
}

var beginnerProjects = map[string]ProjectSpec{
	"civil_engineering_corps":     {"Civil Engineering Corps", "Infrastructure growth", "Infrastructure costs −6% and upkeep −4%.", 150000, map[string]float64{"basic_metals": 100, "construction_materials": 120}},
	"public_education_initiative": {"Public Education Initiative", "Education", "Education +5 immediately, stronger passive Education, and Happiness support.", 120000, map[string]float64{"construction_materials": 75, "basic_goods": 75}},
	"commerce_foundation":         {"Commerce Foundation", "Commerce", "Commerce cap rises to 110% and every Province gains 3 Commerce.", 140000, map[string]float64{"construction_materials": 100, "consumer_goods": 50}},
	"resource_survey":             {"Resource Survey & Extraction Boost", "Extraction", "Primary deposit output increases by 12%.", 100000, map[string]float64{"basic_metals": 120, "construction_materials": 50}},
	"basic_power_grid":            {"Basic Power Grid", "Power", "Power capacity rises by 10% and power improvements cost less.", 130000, map[string]float64{"construction_materials": 100, "energy": 150}},
	"public_health_sanitation":    {"Public Health & Sanitation", "Stability", "Lower disease and pollution pressure with better Happiness recovery.", 115000, map[string]float64{"construction_materials": 75, "processed_foods": 100}},
}

func init() {
	for key, spec := range buildings {
		spec.Cost *= float64(yenScale)
		spec.DailyUpkeep *= float64(yenScale)
		buildings[key] = spec
	}
	for key, project := range beginnerProjects {
		project.Cash *= yenScale
		beginnerProjects[key] = project
	}
}

type ModelCity struct {
	ID, Name                                                     string
	IsCapital                                                    bool
	Infra, Land, Population, Commerce, Pollution, Disease, Crime float64
	Buildings                                                    map[string]int
	Upgrades                                                     map[string]int
}

type ModelNation struct {
	TaxRate, Happiness, Education, EmploymentRate float64
	Technology                                    int
	Doctrine                                      string
	Policies                                      map[string]bool
	Projects                                      map[string]bool
	LongTermProjects                              map[string]bool
	Cities                                        []ModelCity
}

type CityResult struct {
	ID, Name                string             `json:"id"`
	IsCapital               bool               `json:"isCapital"`
	Slots, Used             int                `json:"slots"`
	BasePopulation          float64            `json:"basePopulation"`
	PopulationCapacity      float64            `json:"populationCapacity"`
	EffectivePopulation     float64            `json:"effectivePopulation"`
	DensityMultiplier       float64            `json:"densityMultiplier"`
	Commerce                float64            `json:"commerce"`
	PowerCapacity           float64            `json:"powerCapacity"`
	PowerUsage              float64            `json:"powerUsage"`
	PowerMultiplier         float64            `json:"powerMultiplier"`
	Pollution               float64            `json:"pollution"`
	Disease                 float64            `json:"disease"`
	Crime                   float64            `json:"crime"`
	CitizenIncome           float64            `json:"citizenIncome"`
	EmploymentRate          float64            `json:"employmentRate"`
	EmploymentMultiplier    float64            `json:"employmentMultiplier"`
	TaxCollectionMultiplier float64            `json:"taxCollectionMultiplier"`
	TaxRevenue              float64            `json:"taxRevenue"`
	InfrastructureUpkeep    float64            `json:"infrastructureUpkeep"`
	CivicUpkeep             float64            `json:"civicUpkeep"`
	Upkeep                  float64            `json:"upkeep"`
	Production              map[string]float64 `json:"production"`
	Contributors            map[string]float64 `json:"contributors"`
}

type NationResult struct {
	Cities                           []CityResult       `json:"cities"`
	DailyTax                         float64            `json:"dailyTax"`
	DailyUpkeep                      float64            `json:"dailyUpkeep"`
	DailyInfrastructureUpkeep        float64            `json:"dailyInfrastructureUpkeep"`
	DailyCivicUpkeep                 float64            `json:"dailyCivicUpkeep"`
	NetDailyCash                     float64            `json:"netDailyCash"`
	Population                       float64            `json:"population"`
	HappinessTarget                  float64            `json:"happinessTarget"`
	EducationChange                  float64            `json:"educationChange"`
	Production                       map[string]float64 `json:"production"`
	Contributors                     map[string]float64 `json:"contributors"`
	DailyFoodConsumption             float64            `json:"dailyFoodConsumption"`
	HourlyFoodConsumption            float64            `json:"hourlyFoodConsumption"`
	DailyFoodProduction              float64            `json:"dailyFoodProduction"`
	NetDailyFood                     float64            `json:"netDailyFood"`
	EffectiveEmploymentRate          float64            `json:"effectiveEmploymentRate"`
	EffectiveTaxCollectionMultiplier float64            `json:"effectiveTaxCollectionMultiplier"`
	ProjectedHourlyPopulationGrowth  int64              `json:"projectedHourlyPopulationGrowth"`
	ProjectedDailyPopulationGrowth   int64              `json:"projectedDailyPopulationGrowth"`
}

func civicInstitutionCapacity(infrastructure float64) int {
	return 5 + int(math.Floor(math.Max(0, infrastructure-100)/100))
}

func infraUnitCost(current float64, tech int) float64 {
	x := math.Max(0, current-5)
	discount := math.Max(.72, 1-.004*float64(tech))
	return (475 + math.Pow(x, 2.15)/760) * discount * float64(yenScale)
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
	return math.Ceil(total)
}

// Infrastructure keeps expanding the taxable population base without allowing
// mature Provinces to scale linearly forever. The curve is linear through the
// starter level, then its marginal population benefit declines continuously.
func incomeEffectiveInfrastructure(infrastructure float64) float64 {
	const starterLevel = 100.0
	const diminishingScale = 800.0
	if infrastructure <= starterLevel {
		return math.Max(0, infrastructure)
	}
	return starterLevel + diminishingScale*math.Log1p((infrastructure-starterLevel)/diminishingScale)
}

func landPurchaseCost(current, amount float64, tech int) float64 {
	if amount <= 0 {
		return 0
	}
	total := 0.0
	for i := 0.0; i < math.Ceil(amount); i++ {
		total += (90 + math.Pow(math.Max(0, current+i), 1.72)/520) * math.Max(.78, 1-.003*float64(tech)) * float64(yenScale)
	}
	if amount >= 100 {
		total *= .98
	}
	return math.Ceil(total)
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func calculateEconomy(n ModelNation) NationResult {
	out := NationResult{Production: map[string]float64{}, Contributors: map[string]float64{}}
	policyDisease, policyCrime, policyEmployment, policyFood, policyInfrastructureUpkeep := 1.0, 1.0, 1.0, 1.0, 1.0
	for key := range n.Policies {
		if policy, ok := socialPolicies[key]; ok {
			policyDisease *= policy.Disease
			policyCrime *= policy.Crime
			policyEmployment *= policy.Employment
			policyFood *= policy.FoodConsumption
			policyInfrastructureUpkeep *= policy.InfrastructureUpkeep
		}
	}
	if n.EmploymentRate <= 0 {
		n.EmploymentRate = 72
	}
	weightedLocal, weightedEmployment, weightedTaxCollection, totalBase, educationBuildings := 0.0, 0.0, 0.0, 0.0, 0.0
	doctrineIncome, doctrineHappiness, commerceCap := 1.0, 0.0, 100.0
	switch n.Doctrine {
	case "Capitalist":
		doctrineIncome, commerceCap, doctrineHappiness = 1.06, 115, -2
	case "Planned":
		doctrineIncome = .97
	case "Green":
		doctrineHappiness = 3
	}
	if n.Projects["commerce_foundation"] {
		commerceCap = math.Max(commerceCap, 110)
	}
	for _, c := range n.Cities {
		r := CityResult{ID: c.ID, Name: c.Name, IsCapital: c.IsCapital, Production: map[string]float64{}, Contributors: map[string]float64{}}
		r.Slots = civicInstitutionCapacity(c.Infra)
		for _, q := range c.Buildings {
			r.Used += q
		}
		agriculture := provinceUpgradeEffect(c.Upgrades["agriculture"])
		extraction := provinceUpgradeEffect(c.Upgrades["extraction"])
		lightIndustry := provinceUpgradeEffect(c.Upgrades["light_industry"])
		heavyIndustry := provinceUpgradeEffect(c.Upgrades["heavy_industry"])
		militaryIndustry := provinceUpgradeEffect(c.Upgrades["military_industry"])
		commerceUpgrade := provinceUpgradeEffect(c.Upgrades["commerce"])
		civil := provinceUpgradeEffect(c.Upgrades["civil"])
		incomeInfrastructure := incomeEffectiveInfrastructure(c.Infra)
		r.BasePopulation = incomeInfrastructure * balance.PopulationPerInfra * (1 + agriculture*.008 + civil*.0125)
		density := r.BasePopulation / math.Max(1, c.Land*75)
		r.DensityMultiplier = 1
		if density > 1 {
			r.DensityMultiplier = clamp(1-(density-1)*.08, .70, 1)
		}
		productionPollution := agriculture*.10 + extraction*.55 + lightIndustry*.35 + heavyIndustry*.80 + militaryIndustry*.65
		productionDisease := agriculture*.0008 + extraction*.0007 + lightIndustry*.0005 + heavyIndustry*.0008 + militaryIndustry*.0006
		productionCrime := commerceUpgrade*.0008 + lightIndustry*.0003 + heavyIndustry*.0004 + militaryIndustry*.0003
		powerCapacity, powerUse, commerce, pollution := 0.0, 0.0, 0.0, productionPollution
		diseaseReduction, crimeReduction, localHappiness, employmentBonus, taxCollection := 0.0, 0.0, civil*.70, 0.0, 0.0
		educationBuildings += civil * .15
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
			employmentBonus += s.Employment * v
			taxCollection += s.TaxCollection * v
			r.CivicUpkeep += s.DailyUpkeep * v
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
		r.Disease = clamp(.012+r.Pollution*.0007+productionDisease-n.Education*.00008-diseaseReduction-healthReduction, 0, .30)
		r.Crime = clamp(.04+productionCrime-n.Education*.0003+(50-n.Happiness)*.0004-crimeReduction-employmentBonus*.0008, 0, .30)
		if n.LongTermProjects["public_health_program"] {
			r.Disease *= .75
		}
		if n.LongTermProjects["internal_security_reform"] {
			r.Crime *= .75
		}
		r.Disease *= policyDisease
		r.Crime *= policyCrime
		happyMult := clamp(1+(n.Happiness-50)*.008, .55, 1.45)
		eduBonus := n.Education / 100 * .22
		// Infrastructure establishes the supported population and therefore the
		// taxable economic base. Organic growth may rise modestly above that
		// support level before further Infrastructure is required.
		supportedPopulation := math.Max(1000, r.BasePopulation*(1+eduBonus)*happyMult*(1-r.Disease)*(1-r.Crime)*r.DensityMultiplier)
		r.PopulationCapacity = supportedPopulation * 1.20
		currentPopulation := c.Population
		if currentPopulation <= 0 {
			currentPopulation = supportedPopulation
		}
		r.EffectivePopulation = clamp(math.Max(currentPopulation, supportedPopulation), 1000, r.PopulationCapacity)
		if n.LongTermProjects["population_development"] {
			r.EffectivePopulation *= 1.03
		}
		r.CitizenIncome = balance.BaseCitizenIncome * (1 + (n.Happiness-50)*balance.HappinessIncomePerPoint) * (1 + n.Education/100*balance.EducationIncomeMax) * (1 + float64(n.Technology)*balance.TechnologyIncomePerLevel)
		r.CitizenIncome = math.Max(5, r.CitizenIncome)
		if n.LongTermProjects["national_education_act"] {
			r.CitizenIncome *= 1.04
		}
		educationEmployment := clamp((n.Education-40)*.08, -2, 4)
		r.EmploymentRate = clamp(n.EmploymentRate+educationEmployment+employmentBonus+(policyEmployment-1)*100, 25, 98)
		r.EmploymentMultiplier = r.EmploymentRate / 72
		r.TaxCollectionMultiplier = 1 + taxCollection/100
		r.TaxRevenue = r.EffectivePopulation * r.CitizenIncome * (n.TaxRate / 100) * (1 + r.Commerce/100) * doctrineIncome * (1 + commerceUpgrade*.025) * r.EmploymentMultiplier * r.TaxCollectionMultiplier
		if n.LongTermProjects["tax_modernization"] {
			r.TaxRevenue *= 1.08
		}
		upkeepModifier := 1.0
		if n.Projects["civil_engineering_corps"] {
			upkeepModifier = .96
		}
		r.InfrastructureUpkeep = c.Infra * balance.InfraUpkeepBase * (1 + math.Floor(c.Infra/1200)*.12) * upkeepModifier * policyInfrastructureUpkeep
		r.Upkeep = r.InfrastructureUpkeep + r.CivicUpkeep
		r.Contributors = map[string]float64{"happinessMultiplier": happyMult, "educationBonus": eduBonus, "educationEmploymentBonus": educationEmployment, "civicEmploymentBonus": employmentBonus, "employmentMultiplier": r.EmploymentMultiplier, "taxCollectionMultiplier": r.TaxCollectionMultiplier, "densityMultiplier": r.DensityMultiplier, "diseaseMultiplier": 1 - r.Disease, "crimeMultiplier": 1 - r.Crime, "powerMultiplier": r.PowerMultiplier, "incomeEffectiveInfrastructure": incomeInfrastructure, "infrastructureSupportedPopulation": supportedPopulation, "populationCapacity": r.PopulationCapacity, "productionPollution": productionPollution, "productionDiseasePressure": productionDisease, "productionCrimePressure": productionCrime, "agriculturePopulationBonus": 1 + agriculture*.008, "civilPopulationBonus": 1 + civil*.0125, "commerceUpgradeBonus": 1 + commerceUpgrade*.025}
		out.DailyTax += r.TaxRevenue
		out.DailyUpkeep += r.Upkeep
		out.DailyInfrastructureUpkeep += r.InfrastructureUpkeep
		out.DailyCivicUpkeep += r.CivicUpkeep
		out.Population += r.EffectivePopulation
		totalBase += r.BasePopulation
		weightedEmployment += r.EmploymentRate * r.EffectivePopulation
		weightedTaxCollection += r.TaxCollectionMultiplier * r.EffectivePopulation
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
	if out.Population > 0 {
		out.EffectiveEmploymentRate = weightedEmployment / out.Population
		out.EffectiveTaxCollectionMultiplier = weightedTaxCollection / out.Population
	}
	out.DailyFoodConsumption = out.Population * balance.FoodPerCitizen * policyFood
	out.HourlyFoodConsumption = out.DailyFoodConsumption / balance.TurnsPerDay
	out.Contributors = map[string]float64{"localConditions": local, "taxPenalty": -taxPenalty, "educationHappiness": n.Education * .12, "effectiveEmploymentRate": out.EffectiveEmploymentRate, "taxCollectionMultiplier": out.EffectiveTaxCollectionMultiplier, "currentHappiness": n.Happiness, "targetHappiness": out.HappinessTarget, "socialPolicyDisease": policyDisease, "socialPolicyCrime": policyCrime, "socialPolicyEmployment": policyEmployment, "socialPolicyFoodConsumption": policyFood, "socialPolicyInfrastructureUpkeep": policyInfrastructureUpkeep}
	return out
}
