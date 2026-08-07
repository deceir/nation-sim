package main

import (
	"context"
	"log"
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
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO economy_turns(turn_at) VALUES(?)`, turn); err != nil {
		return
	}
	rows, err := tx.QueryContext(ctx, `SELECT n.id,n.population,n.employment_rate,n.education,n.happiness,n.technology,(SELECT COALESCE(sum(population_capacity),0) FROM cities WHERE nation_id=n.id),COALESCE(sum(CASE i.resource WHEN 'food' THEN i.level*5 ELSE 0 END),0),COALESCE(sum(CASE i.resource WHEN 'coal' THEN i.level*2 ELSE 0 END),0),COALESCE(sum(CASE i.resource WHEN 'steel' THEN i.level ELSE 0 END),0) FROM nations n LEFT JOIN cities c ON c.nation_id=n.id LEFT JOIN city_industries i ON i.city_id=c.id GROUP BY n.id,n.population,n.employment_rate,n.education,n.happiness,n.technology`)
	if err != nil {
		return
	}
	count := 0
	type payout struct {
		id                      string
		cash, food, coal, steel int64
		growth, capacity        int64
	}
	payouts := []payout{}
	for rows.Next() {
		var id string
		var pop, populationCapacity, food, coal, steel int64
		var employment, education, satisfaction, technology float64
		if rows.Scan(&id, &pop, &employment, &education, &satisfaction, &technology,&populationCapacity, &food, &coal, &steel) != nil {
			continue
		}
		daily := float64(pop) * 0.02 * (employment / 100) * (0.5 + education/200) * (0.5 + satisfaction/200) * (1 + technology/500)
		cash := int64(daily) / 24
		growth:=int64(float64(pop)*0.002*(satisfaction/100))/24;if remaining:=populationCapacity-pop;remaining<growth{growth=max(int64(0),remaining)}
		payouts = append(payouts, payout{id, cash, food, coal, steel,growth,populationCapacity})
	}
	rows.Close()
	for _, p := range payouts {
		if _, err = tx.Exec(ctx, `UPDATE nations SET treasury=treasury+?,food=food+?,coal=coal+?,steel=steel+?,population=LEAST(?,population+?) WHERE id=?`, p.cash, p.food, p.coal, p.steel,p.capacity,p.growth, p.id); err != nil {
			return
		}
		if _, err = tx.Exec(ctx, `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'hourly_income',?,'Hourly economic turn')`, uuid(), p.id, p.cash); err != nil {
			return
		}
		count++
	}
	tx.Exec(ctx, `UPDATE economy_turns SET nations_processed=? WHERE turn_at=?`, count, turn)
	if err = tx.Commit(ctx); err != nil {
		log.Printf("hourly turn failed: %v", err)
		return
	}
	log.Printf("hourly economic turn processed %d nations", count)
}
