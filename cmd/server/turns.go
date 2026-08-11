package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"
)

func hourlyPopulationGrowth(city CityResult, happiness, education, nationalMultiplier float64, turn time.Time) int64 {
	headroom := int64(math.Floor(city.PopulationCapacity - city.EffectivePopulation))
	if headroom <= 0 {
		return 0
	}
	seed := sha256.Sum256([]byte(city.ID + turn.UTC().Format(time.RFC3339)))
	variance := .90 + float64(seed[0])/255*.20
	rate := balance.PopulationGrowthRate * (.45 + happiness/100) * (.85 + education/500) / balance.TurnsPerDay * nationalMultiplier
	growth := int64(math.Ceil(city.EffectivePopulation * rate * variance))
	return min(growth, headroom)
}

func nationalPopulationGrowth(nationID string, cities []CityResult, happiness, education, nationalMultiplier float64, turn time.Time) (map[string]int64, int64) {
	growthByCity := make(map[string]int64, len(cities))
	var total, totalHeadroom int64
	for _, city := range cities {
		growth := hourlyPopulationGrowth(city, happiness, education, nationalMultiplier, turn)
		growthByCity[city.ID] = growth
		total += growth
		totalHeadroom += max(int64(0), int64(math.Floor(city.PopulationCapacity-city.EffectivePopulation)))
	}
	if totalHeadroom <= total {
		return growthByCity, total
	}
	seed := sha256.Sum256([]byte(nationID + turn.UTC().Format(time.RFC3339)))
	floor := min(int64(5+seed[0]%16), totalHeadroom)
	remaining := max(int64(0), floor-total)
	for _, city := range cities {
		headroom := max(int64(0), int64(math.Floor(city.PopulationCapacity-city.EffectivePopulation))-growthByCity[city.ID])
		added := min(remaining, headroom)
		growthByCity[city.ID] += added
		total += added
		remaining -= added
		if remaining == 0 {
			break
		}
	}
	return growthByCity, total
}

func (a *app) runHourlyTurns() {
	a.processHourlyTurn(time.Now().UTC().Truncate(time.Hour))
	for {
		next := time.Now().UTC().Truncate(time.Hour).Add(time.Hour)
		time.Sleep(time.Until(next))
		a.processHourlyTurn(next)
	}
}

func (a *app) processHourlyTurn(turn time.Time) {
	ctx := context.Background()
	if err := a.ensureDailyCrises(ctx); err != nil {
		log.Printf("daily Crisis generation failed: %v", err)
	}
	// Claiming the timestamp first makes turns idempotent across restarts.
	if _, err := a.db.ExecContext(ctx, `INSERT INTO economy_turns(turn_at) VALUES(?)`, turn); err != nil {
		return
	}
	rows, err := a.db.QueryContext(ctx, `SELECT owner_id FROM nations ORDER BY id`)
	if err != nil {
		return
	}
	owners := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			owners = append(owners, id)
		}
	}
	rows.Close()
	processed := 0
	for _, owner := range owners {
		n, nid, _, e := a.loadEconomicNationContext(ctx, owner)
		if e != nil {
			continue
		}
		result := calculateEconomy(n)
		strategy, strategyErr := a.loadStrategy(ctx, nid)
		strategyResult := strategicResult{IncomeMultiplier: 1, PopulationMultiplier: 1, HappinessMultiplier: 1, Production: map[string]float64{}}
		if strategyErr == nil {
			applyProvincialOperatingConditions(&strategy, result)
			strategyResult = calculateStrategy(strategy)
		}
		crisisModifiers := a.loadCrisisModifiers(ctx, nid)
		applyCrisisTurnModifiers(&strategyResult, crisisModifiers)
		cash := int64(math.Floor((result.DailyTax*strategyResult.IncomeMultiplier - result.DailyUpkeep) / balance.TurnsPerDay))
		cash += int64(math.Floor(result.DailyUpkeep / balance.TurnsPerDay * crisisModifiers.UpkeepReductionPct / 100))
		allianceID, allianceName, allianceRate, _ := applicableAllianceTax(ctx, a.db, nid)
		allianceTax := int64(0)
		if cash > 0 && allianceID != "" {
			allianceTax = int64(math.Floor(float64(cash) * allianceRate / 100))
		}
		netCash := cash - allianceTax
		// Happiness uses inertia; Gear and social-policy modifiers shift the target
		// while the nation still moves toward it gradually each hourly turn.
		newHappy := calculateHourlyHappiness(n.Happiness, result.HappinessTarget, strategyResult.HappinessMultiplier)
		newEducation := clamp(n.Education+result.EducationChange/balance.TurnsPerDay, 0, 100)
		gdp := annualizedGDP(result.DailyTax * strategyResult.IncomeMultiplier)
		tx, e := a.db.BeginTx(ctx, nil)
		if e != nil {
			continue
		}
		militaryCashUpkeep, _, e := militaryHourlyUpkeep(ctx, tx, nid)
		if e != nil {
			tx.Rollback()
			continue
		}
		netCash -= militaryCashUpkeep
		luxuryIncome, luxuryConsumed, e := settleLuxuryConsumption(ctx, tx, nid, result.Population, len(result.Cities), turn)
		if e != nil {
			tx.Rollback()
			continue
		}
		netCash += luxuryIncome
		ok := true
		growthByCity, totalPopulationGrowth := nationalPopulationGrowth(nid, result.Cities, newHappy, newEducation, strategyResult.PopulationMultiplier, turn)
		for _, c := range result.Cities {
			growth := growthByCity[c.ID]
			_, e = tx.ExecContext(ctx, `UPDATE cities SET local_population=?,commerce_percent=?,power_capacity=?,power_usage=?,pollution=?,disease_rate=?,crime_rate=? WHERE id=?`, int64(c.EffectivePopulation)+growth, c.Commerce, c.PowerCapacity, c.PowerUsage, c.Pollution, c.Disease, c.Crime, c.ID)
			if e != nil {
				ok = false
				break
			}
		}
		if !ok {
			tx.Rollback()
			continue
		}
		_, e = tx.ExecContext(ctx, `UPDATE nations SET treasury=GREATEST(0,treasury+?),happiness=?,education=?,population=?,gdp=? WHERE id=?`, netCash, newHappy, newEducation, int64(result.Population)+totalPopulationGrowth, gdp, nid)
		if e != nil {
			tx.Rollback()
			continue
		}
		if allianceTax > 0 {
			if _, e = tx.ExecContext(ctx, `UPDATE alliance_bank SET cash=cash+? WHERE alliance_id=?`, allianceTax, allianceID); e != nil {
				tx.Rollback()
				continue
			}
			tx.ExecContext(ctx, `INSERT INTO alliance_bank_transactions(id,alliance_id,actor_nation_id,kind,resource,amount,memo) VALUES(?,?,?,'tax','cash',?,?)`, uuid(), allianceID, nid, allianceTax, "Hourly Alliance tax from "+allianceName)
		}
		if strategyErr == nil {
			if e = applyStrategicTurn(ctx, tx, nid, strategy, strategyResult, result.HourlyFoodConsumption); e != nil {
				tx.Rollback()
				continue
			}
		}
		if n.LongTermProjects["technological_research_council"] {
			gain := .12 / balance.TurnsPerDay
			tx.ExecContext(ctx, `UPDATE nations SET technology=LEAST(100,technology+FLOOR(technology_progress+?)),technology_progress=MOD(technology_progress+?,1) WHERE id=?`, gain, gain, nid)
		}
		breakdown, _ := json.Marshal(result)
		_, e = tx.ExecContext(ctx, `INSERT INTO economic_snapshots(id,nation_id,turn_at,cash_income,upkeep,population_change,happiness,education,breakdown) VALUES(?,?,?,?,?,?,?,?,?)`, uuid(), nid, turn, int64(result.DailyTax*strategyResult.IncomeMultiplier/balance.TurnsPerDay)+luxuryIncome, int64(result.DailyUpkeep/balance.TurnsPerDay)+militaryCashUpkeep, totalPopulationGrowth, newHappy, newEducation, breakdown)
		if e != nil {
			tx.Rollback()
			continue
		}
		tx.ExecContext(ctx, `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'hourly_income',?,'Economic turn after upkeep and Alliance tax')`, uuid(), nid, netCash)
		if luxuryIncome > 0 {
			tx.ExecContext(ctx, `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'luxury_consumption',?,?)`, uuid(), nid, luxuryIncome, fmt.Sprintf("Consumed %.3f Luxury Goods", luxuryConsumed))
		}
		if tx.Commit() == nil {
			processed++
		}
	}
	a.db.ExecContext(ctx, `UPDATE economy_turns SET nations_processed=? WHERE turn_at=?`, processed, turn)
	a.processTradeShipments(ctx, turn)
	a.processLongTermProjects(ctx)
	a.processMatureVentures(ctx)
	if err := a.captureWorldResourceSnapshot(ctx, turn); err != nil {
		log.Printf("world resource snapshot failed: %v", err)
	}
	log.Printf("hourly economic turn processed %d nations", processed)
}

func calculateHourlyHappiness(current, target, strategyMultiplier float64) float64 {
	if strategyMultiplier <= 0 {
		strategyMultiplier = 1
	}
	adjustedTarget := clamp(target*strategyMultiplier, 0, 100)
	return clamp(current+(adjustedTarget-current)*.08/balance.TurnsPerDay, 0, 100)
}
