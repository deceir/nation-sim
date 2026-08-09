package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"time"
)

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
			strategyResult = calculateStrategy(strategy)
		}
		cash := int64(math.Floor(result.NetDailyCash / balance.TurnsPerDay * strategyResult.IncomeMultiplier))
		allianceID, allianceName, allianceRate := "", "", 0.0
		a.db.QueryRowContext(ctx, `SELECT a.id,a.name,a.tax_rate FROM alliance_members m JOIN alliances a ON a.id=m.alliance_id WHERE m.nation_id=?`, nid).Scan(&allianceID, &allianceName, &allianceRate)
		allianceTax := int64(0)
		if cash > 0 && allianceID != "" {
			allianceTax = int64(math.Floor(float64(cash) * allianceRate / 100))
		}
		netCash := cash - allianceTax
		// Happiness uses inertia; education changes only gradually and can decay.
		newHappy := clamp(n.Happiness+(result.HappinessTarget-n.Happiness)*.08/balance.TurnsPerDay, 0, 100)
		newEducation := clamp(n.Education+result.EducationChange/balance.TurnsPerDay, 0, 100)
		tx, e := a.db.BeginTx(ctx, nil)
		if e != nil {
			continue
		}
		ok := true
		for _, c := range result.Cities {
			growthRate := balance.PopulationGrowthRate * (.45 + newHappy/100) * (.85 + newEducation/500) / balance.TurnsPerDay * strategyResult.PopulationMultiplier
			growth := int64(math.Max(0, c.EffectivePopulation*growthRate))
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
		prod := func(k string) int64 { return int64(math.Floor(result.Production[k] / balance.TurnsPerDay)) }
		_, e = tx.ExecContext(ctx, `UPDATE nations SET treasury=GREATEST(0,treasury+?),happiness=?,education=?,population=?,coal=coal+?,iron=iron+?,oil=oil+?,bauxite=bauxite+?,steel=steel+?,aluminum=aluminum+?,gasoline=gasoline+? WHERE id=?`, netCash, newHappy, newEducation, int64(result.Population), prod("coal"), prod("iron"), prod("oil"), prod("bauxite"), prod("steel"), prod("aluminum"), prod("gasoline"), nid)
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
		breakdown, _ := json.Marshal(result)
		_, e = tx.ExecContext(ctx, `INSERT INTO economic_snapshots(id,nation_id,turn_at,cash_income,upkeep,population_change,happiness,education,breakdown) VALUES(?,?,?,?,?,?,?,?,?)`, uuid(), nid, turn, int64(result.DailyTax/balance.TurnsPerDay), int64(result.DailyUpkeep/balance.TurnsPerDay), 0, newHappy, newEducation, breakdown)
		if e != nil {
			tx.Rollback()
			continue
		}
		tx.ExecContext(ctx, `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'hourly_income',?,'Economic turn after upkeep and Alliance tax')`, uuid(), nid, netCash)
		if tx.Commit() == nil {
			processed++
		}
	}
	a.db.ExecContext(ctx, `UPDATE economy_turns SET nations_processed=? WHERE turn_at=?`, processed, turn)
	a.processTradeShipments(ctx, turn)
	log.Printf("hourly economic turn processed %d nations", processed)
}
