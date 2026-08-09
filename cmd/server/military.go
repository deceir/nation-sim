package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"
)

type militaryUnitSpec struct {
	Name, Project          string
	Cash                   int64
	Resources              map[string]float64
	DailyCash, DailyEnergy float64
	PopulationCoefficient  float64
	ProvinceCoefficient    int64
	BaseCapacity           int64
	Tradable               bool
}

// Military balance is data-driven here so costs and coefficients can be tuned
// without changing acquisition, upkeep, capacity, or decommission logic.
var militaryUnits = map[string]militaryUnitSpec{
	"soldiers": {Name: "Soldiers", Cash: 1500, Resources: map[string]float64{}, DailyCash: .4, PopulationCoefficient: .10, ProvinceCoefficient: 1000, BaseCapacity: 5000},
	"tanks":    {Name: "Tanks", Project: "armored_vehicle_program", Cash: 120000, Resources: map[string]float64{"basic_metals": 8, "construction_materials": 3, "energy": 2}, DailyCash: 350, DailyEnergy: .1, PopulationCoefficient: .005, ProvinceCoefficient: 50, BaseCapacity: 50, Tradable: true},
	"ships":    {Name: "Ships", Project: "naval_shipyard", Cash: 450000, Resources: map[string]float64{"basic_metals": 20, "construction_materials": 12, "energy": 8, "timber": 5}, DailyCash: 1200, DailyEnergy: .4, PopulationCoefficient: .0015, ProvinceCoefficient: 20, BaseCapacity: 10, Tradable: true},
	"drones":   {Name: "Drones", Project: "advanced_ordnance", Cash: 85000, Resources: map[string]float64{"basic_metals": 4, "strategic_minerals": 3, "energy": 2, "basic_goods": 1}, DailyCash: 2500, DailyEnergy: .15, PopulationCoefficient: .006, ProvinceCoefficient: 40, BaseCapacity: 30, Tradable: true},
}

func militaryCapacity(spec militaryUnitSpec, population int64, provinces int) int64 {
	return int64(math.Floor(spec.PopulationCoefficient*float64(population))) + spec.ProvinceCoefficient*int64(provinces) + spec.BaseCapacity
}

func isMilitaryEquipment(unit string) bool {
	spec, ok := militaryUnits[unit]
	return ok && spec.Tradable
}

func militaryTradeQuantity(quantity float64) (int64, bool) {
	if quantity <= 0 || quantity != math.Trunc(quantity) || quantity > float64(math.MaxInt64) {
		return 0, false
	}
	return int64(quantity), true
}

func removeMilitaryInventory(ctx context.Context, tx *sql.Tx, nationID, unit string, quantity float64) error {
	count, ok := militaryTradeQuantity(quantity)
	if !ok {
		return fmt.Errorf("military equipment must be traded in whole units")
	}
	result, err := tx.ExecContext(ctx, `UPDATE military_inventory SET quantity=quantity-? WHERE nation_id=? AND unit_type=? AND quantity>=?`, count, nationID, unit, count)
	if err != nil || affected(result) != 1 {
		return fmt.Errorf("not enough military equipment")
	}
	return nil
}

func addMilitaryInventory(ctx context.Context, tx *sql.Tx, nationID, unit string, quantity float64) error {
	count, ok := militaryTradeQuantity(quantity)
	if !ok {
		return fmt.Errorf("military equipment must be traded in whole units")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO military_inventory(nation_id,unit_type,quantity) VALUES(?,?,?) ON DUPLICATE KEY UPDATE quantity=quantity+VALUES(quantity)`, nationID, unit, count)
	return err
}

func ensureMilitaryPurchaseCapacity(ctx context.Context, tx *sql.Tx, nationID, unit string, quantity float64) error {
	spec, ok := militaryUnits[unit]
	if !ok || !spec.Tradable {
		return nil
	}
	count, valid := militaryTradeQuantity(quantity)
	if !valid {
		return fmt.Errorf("military equipment must be traded in whole units")
	}
	capacity, err := militaryCapacityForNation(ctx, tx, nationID, spec)
	if err != nil {
		return err
	}
	var owned, inbound, escrowed int64
	tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT quantity FROM military_inventory WHERE nation_id=? AND unit_type=?),0)`, nationID, unit).Scan(&owned)
	tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity),0) FROM trade_shipments WHERE buyer_nation_id=? AND resource=? AND status IN('in_transit','delayed')`, nationID, unit).Scan(&inbound)
	tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(escrow_goods),0) FROM market_orders WHERE nation_id=? AND resource=? AND side='sell' AND status IN('open','pending')`, nationID, unit).Scan(&escrowed)
	if owned+inbound+escrowed+count > capacity {
		return fmt.Errorf("purchase would exceed the %s capacity of %d", spec.Name, capacity)
	}
	return nil
}

func militaryUnitKeys() []string {
	keys := make([]string, 0, len(militaryUnits))
	for key := range militaryUnits {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func militaryCapacityForNation(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, nationID string, spec militaryUnitSpec) (int64, error) {
	var population int64
	var provinces int
	err := q.QueryRowContext(ctx, `SELECT n.population,(SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id) FROM nations n WHERE n.id=?`, nationID).Scan(&population, &provinces)
	return militaryCapacity(spec, population, provinces), err
}

func militaryUpkeepProjection(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, nationID string) (float64, float64) {
	rows, err := q.QueryContext(ctx, `SELECT unit_type,SUM(quantity) FROM (SELECT unit_type,quantity FROM military_inventory WHERE nation_id=? UNION ALL SELECT resource,escrow_goods FROM market_orders WHERE nation_id=? AND side='sell' AND status IN('open','pending') AND resource IN('tanks','ships','drones')) military_holdings GROUP BY unit_type`, nationID, nationID)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()
	var cash, energy float64
	for rows.Next() {
		var unit string
		var quantity int64
		if rows.Scan(&unit, &quantity) == nil {
			spec := militaryUnits[unit]
			cash += float64(quantity) * spec.DailyCash
			energy += float64(quantity) * spec.DailyEnergy
		}
	}
	return cash, energy
}

func (a *app) militaryDashboard(w http.ResponseWriter, r *http.Request, u user) {
	var nid string
	var population int64
	var provinces int
	if err := a.db.QueryRowContext(r.Context(), `SELECT n.id,n.population,(SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id) FROM nations n WHERE n.owner_id=?`, u.ID).Scan(&nid, &population, &provinces); err != nil {
		problem(w, http.StatusNotFound, "Nation not found.")
		return
	}
	projects := loadLongTermProjectSet(r.Context(), a.db, nid)
	items := []map[string]any{}
	for _, key := range militaryUnitKeys() {
		spec := militaryUnits[key]
		var quantity, escrowed, producedToday int64
		a.db.QueryRowContext(r.Context(), `SELECT COALESCE((SELECT quantity FROM military_inventory WHERE nation_id=? AND unit_type=?),0),COALESCE((SELECT quantity FROM military_production_daily WHERE nation_id=? AND unit_type=? AND production_date=CURRENT_DATE()),0)`, nid, key, nid, key).Scan(&quantity, &producedToday)
		a.db.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(escrow_goods),0) FROM market_orders WHERE nation_id=? AND resource=? AND side='sell' AND status IN('open','pending')`, nid, key).Scan(&escrowed)
		totalOwned := quantity + escrowed
		items = append(items, map[string]any{"key": key, "name": spec.Name, "quantity": totalOwned, "availableQuantity": quantity, "escrowedQuantity": escrowed, "capacity": militaryCapacity(spec, population, provinces), "cashCost": spec.Cash, "resourceCosts": spec.Resources, "dailyCashUpkeep": float64(totalOwned) * spec.DailyCash, "dailyEnergyUpkeep": float64(totalOwned) * spec.DailyEnergy, "cashUpkeepEach": spec.DailyCash, "energyUpkeepEach": spec.DailyEnergy, "requiredProject": spec.Project, "canProduce": spec.Project == "" || projects[spec.Project], "tradable": spec.Tradable, "decommissionLocked": producedToday > 0, "producedToday": producedToday})
	}
	write(w, http.StatusOK, map[string]any{"units": items, "population": population, "provinces": provinces, "serverDate": time.Now().UTC().Format("2006-01-02")})
}

func (a *app) produceMilitary(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		UnitType string
		Quantity int64
	}
	if !decode(w, r, &in) || in.Quantity <= 0 || in.Quantity > 1000000 {
		problem(w, http.StatusBadRequest, "Choose a production quantity between 1 and 1,000,000.")
		return
	}
	spec, ok := militaryUnits[in.UnitType]
	if !ok {
		problem(w, http.StatusBadRequest, "Unknown military unit type.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not begin military production.")
		return
	}
	defer tx.Rollback()
	var nid string
	var treasury, population int64
	var provinces int
	if err = tx.QueryRowContext(r.Context(), `SELECT n.id,n.treasury,n.population,(SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id) FROM nations n WHERE n.owner_id=? FOR UPDATE`, u.ID).Scan(&nid, &treasury, &population, &provinces); err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	if spec.Project != "" {
		var completed int
		tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM national_long_term_projects WHERE nation_id=? AND project_type=?`, nid, spec.Project).Scan(&completed)
		if completed == 0 {
			problem(w, 409, spec.Name+" require the corresponding National Project for domestic production.")
			return
		}
	}
	var owned, inbound, escrowed int64
	tx.QueryRowContext(r.Context(), `SELECT COALESCE((SELECT quantity FROM military_inventory WHERE nation_id=? AND unit_type=?),0)`, nid, in.UnitType).Scan(&owned)
	tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(quantity),0) FROM trade_shipments WHERE buyer_nation_id=? AND resource=? AND status IN('in_transit','delayed')`, nid, in.UnitType).Scan(&inbound)
	tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(escrow_goods),0) FROM market_orders WHERE nation_id=? AND resource=? AND side='sell' AND status IN('open','pending')`, nid, in.UnitType).Scan(&escrowed)
	cap := militaryCapacity(spec, population, provinces)
	if owned+inbound+escrowed+in.Quantity > cap {
		problem(w, 409, fmt.Sprintf("Production would exceed the %s capacity of %d.", spec.Name, cap))
		return
	}
	cashCost := spec.Cash * in.Quantity
	if treasury < cashCost {
		problem(w, 409, "Insufficient treasury for this production order.")
		return
	}
	for resource, each := range spec.Resources {
		cost := each * float64(in.Quantity)
		result, e := tx.ExecContext(r.Context(), `UPDATE nation_stockpiles SET amount=amount-? WHERE nation_id=? AND commodity=? AND amount>=?`, cost, nid, resource, cost)
		if e != nil || affected(result) != 1 {
			problem(w, 409, "Insufficient "+commodityName(resource)+" for this production order.")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, cashCost, nid); err != nil {
		problem(w, 500, "Could not pay production costs.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO military_inventory(nation_id,unit_type,quantity) VALUES(?,?,?) ON DUPLICATE KEY UPDATE quantity=quantity+VALUES(quantity)`, nid, in.UnitType, in.Quantity); err != nil {
		problem(w, 500, "Could not add produced units.")
		return
	}
	tx.ExecContext(r.Context(), `INSERT INTO military_production_daily(nation_id,unit_type,production_date,quantity) VALUES(?,?,CURRENT_DATE(),?) ON DUPLICATE KEY UPDATE quantity=quantity+VALUES(quantity)`, nid, in.UnitType, in.Quantity)
	tx.ExecContext(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'military_production',?,?)`, uuid(), nid, -cashCost, fmt.Sprintf("Produced %d %s", in.Quantity, spec.Name))
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Could not complete production.")
		return
	}
	write(w, http.StatusCreated, map[string]any{"ok": true, "quantity": in.Quantity, "cashCost": cashCost})
}

func (a *app) decommissionMilitary(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		UnitType string
		Quantity int64
	}
	if !decode(w, r, &in) || in.Quantity <= 0 {
		problem(w, 400, "Choose units to decommission.")
		return
	}
	if _, ok := militaryUnits[in.UnitType]; !ok {
		problem(w, 400, "Unknown military unit type.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not begin decommissioning.")
		return
	}
	defer tx.Rollback()
	var nid string
	if tx.QueryRowContext(r.Context(), `SELECT id FROM nations WHERE owner_id=? FOR UPDATE`, u.ID).Scan(&nid) != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	var produced int64
	tx.QueryRowContext(r.Context(), `SELECT COALESCE((SELECT quantity FROM military_production_daily WHERE nation_id=? AND unit_type=? AND production_date=CURRENT_DATE()),0)`, nid, in.UnitType).Scan(&produced)
	if produced > 0 {
		problem(w, 409, "Units of this type were produced today and cannot be decommissioned until the next server day.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE military_inventory SET quantity=quantity-? WHERE nation_id=? AND unit_type=? AND quantity>=?`, in.Quantity, nid, in.UnitType, in.Quantity)
	if err != nil || affected(result) != 1 {
		problem(w, 409, "You do not own that many units.")
		return
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Could not decommission units.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func militaryHourlyUpkeep(ctx context.Context, tx *sql.Tx, nationID string) (int64, float64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT unit_type,SUM(quantity) FROM (SELECT unit_type,quantity FROM military_inventory WHERE nation_id=? UNION ALL SELECT resource,escrow_goods FROM market_orders WHERE nation_id=? AND side='sell' AND status IN('open','pending') AND resource IN('tanks','ships','drones')) military_holdings GROUP BY unit_type`, nationID, nationID)
	if err != nil {
		return 0, 0, err
	}
	var cashFloat, energy float64
	for rows.Next() {
		var unit string
		var quantity int64
		if rows.Scan(&unit, &quantity) == nil {
			spec := militaryUnits[unit]
			cashFloat += float64(quantity) * spec.DailyCash
			energy += float64(quantity) * spec.DailyEnergy
		}
	}
	rows.Close()
	if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO military_upkeep_state(nation_id,cash_fraction) VALUES(?,0)`, nationID); err != nil {
		return 0, 0, err
	}
	var carried float64
	if err = tx.QueryRowContext(ctx, `SELECT cash_fraction FROM military_upkeep_state WHERE nation_id=? FOR UPDATE`, nationID).Scan(&carried); err != nil {
		return 0, 0, err
	}
	accrued := carried + cashFloat/balance.TurnsPerDay
	cash := int64(math.Floor(accrued + 0.0000001))
	if _, err = tx.ExecContext(ctx, `UPDATE military_upkeep_state SET cash_fraction=? WHERE nation_id=?`, accrued-float64(cash), nationID); err != nil {
		return 0, 0, err
	}
	energy /= balance.TurnsPerDay
	if energy > 0 {
		if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,'energy',0)`, nationID); err != nil {
			return 0, 0, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE nation_stockpiles SET amount=GREATEST(0,amount-?) WHERE nation_id=? AND commodity='energy'`, energy, nationID); err != nil {
			return 0, 0, err
		}
	}
	return cash, energy, nil
}
