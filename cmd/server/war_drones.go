package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

const (
	warDroneCooldown      = 6 * time.Hour
	warDroneDailyLimit    = 3
	warDroneSortieCap     = 100
	warDroneEnergyCost    = .04
	warDroneEquipmentCost = .03
)

var warDroneMissions = map[string]map[string]string{
	"reconnaissance": {
		"name":        "Reconnaissance Sweep",
		"description": "Survey both fronts and improve conventional combat strength in the next strategic round.",
	},
	"precision_strike": {
		"name":        "Precision Force Strike",
		"description": "Attack one deployed enemy unit type immediately. Dense targets are easier to damage than ships or aircraft.",
	},
	"infrastructure_disruption": {
		"name":        "Infrastructure Disruption",
		"description": "Build a small amount of campaign damage pressure immediately without replacing conventional territorial control.",
	},
}

var warDroneDefenses = map[string]map[string]string{
	"interceptor_screen": {
		"name":        "Interceptor Screen",
		"description": "Fighter patrols inflict more drone losses across every mission.",
	},
	"dispersed_forces": {
		"name":        "Dispersed Forces",
		"description": "Reduces casualties from precision force strikes.",
	},
	"hardened_sites": {
		"name":        "Hardened Sites",
		"description": "Reduces damage pressure caused by infrastructure disruption.",
	},
}

func droneSortieMaximum(owned int64) int64 {
	if owned <= 0 {
		return 0
	}
	maximum := int64(math.Ceil(float64(owned) * .25))
	maximum = max(int64(5), maximum)
	return min(owned, min(int64(warDroneSortieCap), maximum))
}

func droneInterceptionChance(defendingJets int64, posture, mission string) float64 {
	chance := .08 + math.Min(.22, math.Sqrt(float64(max(int64(0), defendingJets)))*.012)
	if posture == "interceptor_screen" {
		chance += .15
	}
	if mission == "reconnaissance" {
		chance *= .75
	}
	return math.Min(.5, math.Max(.03, chance))
}

func deterministicDroneValue(strikeID string, index int, purpose string) float64 {
	return deterministicWarDamageRoll(strikeID, purpose, "drone", index)
}

func droneLossCount(strikeID string, committed int64, chance float64) int64 {
	lost := int64(0)
	for i := int64(0); i < committed; i++ {
		if deterministicDroneValue(strikeID, int(i), "interception") < chance {
			lost++
		}
	}
	return lost
}

func dronePrecisionLossAmount(strikeID, unit string, survivors, available int64, dispersed bool) int64 {
	if survivors <= 0 || available <= 0 {
		return 0
	}
	rates := map[string]float64{"soldiers": 2.0, "tanks": .10, "ships": .025, "jets": .06}
	multiplier := .85 + deterministicDroneValue(strikeID, 0, "precision")*.3
	if dispersed {
		multiplier *= .65
	}
	loss := int64(math.Floor(float64(survivors) * rates[unit] * multiplier))
	if loss == 0 && survivors >= 8 {
		loss = 1
	}
	return min(available, loss)
}

func warDroneIntelBonus(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, conflictID, nationID string, round int) float64 {
	var bonus float64
	_ = q.QueryRowContext(ctx, `SELECT COALESCE(SUM(intel_bonus),0) FROM war_drone_strikes WHERE conflict_id=? AND attacker_nation_id=? AND applies_round=?`, conflictID, nationID, round).Scan(&bonus)
	return math.Min(.08, math.Max(0, bonus))
}

func deployedWarUnit(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, conflictID, nationID, unit string, round int) int64 {
	var amount int64
	_ = q.QueryRowContext(ctx, `SELECT COALESCE(SUM(d.remaining),0) FROM war_deployments d JOIN conflicts c ON c.id=d.conflict_id WHERE d.conflict_id=? AND d.nation_id=? AND d.unit_type=? AND d.arrives_round<=? AND d.remaining>0 AND d.deployment_theater=CASE WHEN d.nation_id=c.attacker_id THEN 'defender_homeland' ELSE 'attacker_homeland' END`, conflictID, nationID, unit, round).Scan(&amount)
	return amount
}

func applyDronePrecisionLosses(ctx context.Context, tx *sql.Tx, conflictID, nationID, unit string, round int, loss int64) error {
	if loss <= 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT d.id,d.remaining FROM war_deployments d JOIN conflicts c ON c.id=d.conflict_id WHERE d.conflict_id=? AND d.nation_id=? AND d.unit_type=? AND d.arrives_round<=? AND d.remaining>0 AND d.deployment_theater=CASE WHEN d.nation_id=c.attacker_id THEN 'defender_homeland' ELSE 'attacker_homeland' END ORDER BY d.created_at,d.id FOR UPDATE`, conflictID, nationID, unit, round)
	if err != nil {
		return err
	}
	type deployment struct {
		id        string
		remaining int64
	}
	deployments := []deployment{}
	for rows.Next() {
		var item deployment
		if err = rows.Scan(&item.id, &item.remaining); err != nil {
			rows.Close()
			return err
		}
		deployments = append(deployments, item)
	}
	rows.Close()
	remainingLoss := loss
	for _, item := range deployments {
		take := min(item.remaining, remainingLoss)
		if take > 0 {
			if _, err = tx.ExecContext(ctx, `UPDATE war_deployments SET remaining=remaining-? WHERE id=?`, take, item.id); err != nil {
				return err
			}
			remainingLoss -= take
		}
		if remainingLoss == 0 {
			break
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE military_inventory SET quantity=GREATEST(0,quantity-?) WHERE nation_id=? AND unit_type=?`, loss, nationID, unit)
	return err
}

func (a *app) setWarDroneDefense(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Posture string }
	if !decode(w, r, &in) {
		return
	}
	if _, ok := warDroneDefenses[in.Posture]; !ok {
		problem(w, 400, "Choose a valid drone defense posture.")
		return
	}
	me, err := loadWarNation(r.Context(), a.db, "owner_id", u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	id := r.PathValue("id")
	var stage string
	var party int
	_ = a.db.QueryRowContext(r.Context(), `SELECT w.stage,(c.attacker_id=? OR c.defender_id=?) FROM wars w JOIN conflicts c ON c.id=w.conflict_id WHERE w.conflict_id=?`, me.ID, me.ID, id).Scan(&stage, &party)
	if party != 1 {
		problem(w, 404, "War not found.")
		return
	}
	if stage == "ended" {
		problem(w, 409, "This war has ended.")
		return
	}
	if _, err = a.db.ExecContext(r.Context(), `INSERT INTO war_drone_defenses(conflict_id,nation_id,posture) VALUES(?,?,?) ON DUPLICATE KEY UPDATE posture=VALUES(posture)`, id, me.ID, in.Posture); err != nil {
		problem(w, 500, "The drone defense posture could not be saved.")
		return
	}
	write(w, 200, map[string]any{"ok": true, "posture": in.Posture})
}

func (a *app) launchWarDroneStrike(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		Mission, TargetUnit string
		Drones              int64
	}
	if !decode(w, r, &in) {
		return
	}
	if _, ok := warDroneMissions[in.Mission]; !ok {
		problem(w, 400, "Choose a valid drone mission.")
		return
	}
	if in.Mission == "precision_strike" {
		valid := false
		for _, unit := range warConventionalUnitKeys() {
			valid = valid || in.TargetUnit == unit
		}
		if !valid {
			problem(w, 400, "Choose a deployed conventional unit type to target.")
			return
		}
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "The drone mission could not be started.")
		return
	}
	defer tx.Rollback()
	me, err := loadWarNation(r.Context(), tx, "owner_id", u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	state, err := loadWarState(r.Context(), tx, r.PathValue("id"), true)
	if err != nil || (me.ID != state.AttackerID && me.ID != state.DefenderID) {
		problem(w, 404, "War not found.")
		return
	}
	if state.Stage == "ended" {
		problem(w, 409, "This war has ended.")
		return
	}
	opponentID := state.AttackerID
	if me.ID == state.AttackerID {
		opponentID = state.DefenderID
	}
	var lockedNation string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM nations WHERE id=? FOR UPDATE`, me.ID).Scan(&lockedNation); err != nil {
		problem(w, 409, "Your nation is unavailable for a drone mission.")
		return
	}
	targetAvailable := int64(0)
	if in.Mission == "precision_strike" {
		targetAvailable = deployedWarUnit(r.Context(), tx, state.ConflictID, opponentID, in.TargetUnit, state.Rounds)
		if targetAvailable <= 0 {
			problem(w, 409, "There are no deployed "+militaryUnits[in.TargetUnit].Name+" available to strike.")
			return
		}
	}
	var owned, defendingJets int64
	_ = tx.QueryRowContext(r.Context(), `SELECT quantity FROM military_inventory WHERE nation_id=? AND unit_type='drones' FOR UPDATE`, me.ID).Scan(&owned)
	_ = tx.QueryRowContext(r.Context(), `SELECT quantity FROM military_inventory WHERE nation_id=? AND unit_type='jets' FOR UPDATE`, opponentID).Scan(&defendingJets)
	maximum := droneSortieMaximum(owned)
	if in.Drones <= 0 || in.Drones > maximum {
		problem(w, 409, fmt.Sprintf("Choose between 1 and %s available drones for this sortie.", formatWholeNumber(maximum)))
		return
	}
	var dailyUsed int
	_ = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM war_drone_strikes WHERE attacker_nation_id=? AND DATE(launched_at)=UTC_DATE()`, me.ID).Scan(&dailyUsed)
	if dailyUsed >= warDroneDailyLimit {
		problem(w, 409, "Your nation has used all three drone sorties available today.")
		return
	}
	var nextAvailable sql.NullTime
	_ = tx.QueryRowContext(r.Context(), `SELECT next_available_at FROM war_drone_strikes WHERE conflict_id=? AND attacker_nation_id=? ORDER BY launched_at DESC LIMIT 1`, state.ConflictID, me.ID).Scan(&nextAvailable)
	if nextAvailable.Valid && nextAvailable.Time.After(time.Now().UTC()) {
		problem(w, 409, "Drone Command is rearming. The next sortie is available at "+nextAvailable.Time.UTC().Format("15:04 UTC")+".")
		return
	}
	energyCost := float64(in.Drones) * warDroneEnergyCost
	equipmentCost := float64(in.Drones) * warDroneEquipmentCost
	for resource, cost := range map[string]float64{"energy": energyCost, "military_equipment": equipmentCost} {
		var amount float64
		_ = tx.QueryRowContext(r.Context(), `SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity=? FOR UPDATE`, me.ID, resource).Scan(&amount)
		if amount+1e-9 < cost {
			label := map[string]string{"energy": "Energy", "military_equipment": "Military Equipment"}[resource]
			problem(w, 409, fmt.Sprintf("This sortie requires %.2f %s; your nation has %.2f.", cost, label, amount))
			return
		}
	}
	for resource, cost := range map[string]float64{"energy": energyCost, "military_equipment": equipmentCost} {
		if _, err = tx.ExecContext(r.Context(), `UPDATE nation_stockpiles SET amount=amount-? WHERE nation_id=? AND commodity=?`, cost, me.ID, resource); err != nil {
			problem(w, 500, "The sortie operating cost could not be paid.")
			return
		}
	}
	defense := "interceptor_screen"
	_ = tx.QueryRowContext(r.Context(), `SELECT posture FROM war_drone_defenses WHERE conflict_id=? AND nation_id=?`, state.ConflictID, opponentID).Scan(&defense)
	strikeID := uuid()
	lost := droneLossCount(strikeID, in.Drones, droneInterceptionChance(defendingJets, defense, in.Mission))
	survivors := in.Drones - lost
	if _, err = tx.ExecContext(r.Context(), `UPDATE military_inventory SET quantity=GREATEST(0,quantity-?) WHERE nation_id=? AND unit_type='drones'`, lost, me.ID); err != nil {
		problem(w, 500, "Drone losses could not be recorded.")
		return
	}
	targetLosses := map[string]int64{}
	intelBonus, pressure := 0.0, 0.0
	summary := fmt.Sprintf("%s launched %d drones; %d were intercepted.", me.Name, in.Drones, lost)
	if survivors == 0 {
		summary += " The mission caused no effect."
	} else {
		switch in.Mission {
		case "reconnaissance":
			intelBonus = math.Min(.08, .02+float64(survivors)*.003)
			summary += fmt.Sprintf(" Reconnaissance secured a %.0f%% combat-intelligence bonus for the next round.", intelBonus*100)
		case "precision_strike":
			loss := dronePrecisionLossAmount(strikeID, in.TargetUnit, survivors, targetAvailable, defense == "dispersed_forces")
			if err = applyDronePrecisionLosses(r.Context(), tx, state.ConflictID, opponentID, in.TargetUnit, state.Rounds, loss); err != nil {
				problem(w, 500, "The strike result could not be applied.")
				return
			}
			targetLosses[in.TargetUnit] = loss
			if loss > 0 {
				summary += fmt.Sprintf(" The strike destroyed %d deployed %s.", loss, militaryUnits[in.TargetUnit].Name)
			} else {
				summary += " The strike found no vulnerable deployed target."
			}
		case "infrastructure_disruption":
			pressure = math.Min(.65, float64(survivors)*.015)
			if defense == "hardened_sites" {
				pressure *= .55
			}
			column := "attacker_damage_pressure"
			if me.ID == state.DefenderID {
				column = "defender_damage_pressure"
			}
			if _, err = tx.ExecContext(r.Context(), `UPDATE wars SET `+column+`=`+column+`+? WHERE conflict_id=?`, pressure, state.ConflictID); err != nil {
				problem(w, 500, "The disruption result could not be applied.")
				return
			}
			summary += fmt.Sprintf(" The raid added %.2f campaign damage pressure.", pressure)
		}
	}
	lossesJSON, _ := json.Marshal(targetLosses)
	next := time.Now().UTC().Add(warDroneCooldown)
	_, err = tx.ExecContext(r.Context(), `INSERT INTO war_drone_strikes(id,conflict_id,attacker_nation_id,defender_nation_id,mission,target_unit,drones_committed,drones_lost,target_losses,damage_pressure,intel_bonus,applies_round,next_available_at,summary) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, strikeID, state.ConflictID, me.ID, opponentID, in.Mission, in.TargetUnit, in.Drones, lost, lossesJSON, pressure, intelBonus, state.Rounds+1, next, summary)
	if err != nil {
		problem(w, 500, "The drone mission could not be recorded.")
		return
	}
	_, _ = tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'war','Drone activity',?)`, uuid(), opponentID, summary)
	if err = tx.Commit(); err != nil {
		problem(w, 500, "The drone mission could not be finalized.")
		return
	}
	write(w, 201, map[string]any{"ok": true, "summary": summary, "dronesLost": lost, "targetLosses": targetLosses, "nextAvailableAt": next})
}

func (a *app) warDroneHistory(ctx context.Context, conflictID, viewerNationID string) []map[string]any {
	strikes := []map[string]any{}
	rows, err := a.db.QueryContext(ctx, `SELECT s.id,s.attacker_nation_id,an.name,dn.name,s.mission,s.target_unit,s.drones_committed,s.drones_lost,s.target_losses,s.damage_pressure,s.intel_bonus,s.launched_at,s.summary FROM war_drone_strikes s JOIN nations an ON an.id=s.attacker_nation_id JOIN nations dn ON dn.id=s.defender_nation_id WHERE s.conflict_id=? ORDER BY s.launched_at DESC LIMIT 20`, conflictID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, attackerID, attackerName, defenderName, mission, target, rawLosses, summary string
			var committed, lost int64
			var pressure, intel float64
			var launched time.Time
			if rows.Scan(&id, &attackerID, &attackerName, &defenderName, &mission, &target, &committed, &lost, &rawLosses, &pressure, &intel, &launched, &summary) == nil {
				losses := map[string]int64{}
				_ = json.Unmarshal([]byte(rawLosses), &losses)
				strikes = append(strikes, map[string]any{"id": id, "attackerName": attackerName, "defenderName": defenderName, "mission": mission, "targetUnit": target, "dronesCommitted": committed, "dronesLost": lost, "targetLosses": losses, "damagePressure": pressure, "intelBonus": intel, "launchedAt": launched, "summary": summary, "mine": attackerID == viewerNationID})
			}
		}
	}
	return strikes
}

func (a *app) warDroneDashboard(ctx context.Context, conflictID, nationID, opponentID string, rounds int, stage string) map[string]any {
	available := int64(0)
	_ = a.db.QueryRowContext(ctx, `SELECT COALESCE((SELECT quantity FROM military_inventory WHERE nation_id=? AND unit_type='drones'),0)`, nationID).Scan(&available)
	posture := "interceptor_screen"
	_ = a.db.QueryRowContext(ctx, `SELECT posture FROM war_drone_defenses WHERE conflict_id=? AND nation_id=?`, conflictID, nationID).Scan(&posture)
	dailyUsed := 0
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM war_drone_strikes WHERE attacker_nation_id=? AND DATE(launched_at)=UTC_DATE()`, nationID).Scan(&dailyUsed)
	var nextAvailable sql.NullTime
	_ = a.db.QueryRowContext(ctx, `SELECT next_available_at FROM war_drone_strikes WHERE conflict_id=? AND attacker_nation_id=? ORDER BY launched_at DESC LIMIT 1`, conflictID, nationID).Scan(&nextAvailable)
	strikes := a.warDroneHistory(ctx, conflictID, nationID)
	var next any
	if nextAvailable.Valid && nextAvailable.Time.After(time.Now().UTC()) {
		next = nextAvailable.Time
	}
	targetAvailability := map[string]int64{}
	for _, unit := range warConventionalUnitKeys() {
		targetAvailability[unit] = deployedWarUnit(ctx, a.db, conflictID, opponentID, unit, rounds)
	}
	return map[string]any{
		"availableDrones": available, "maxSortie": droneSortieMaximum(available), "dailyUsed": dailyUsed, "dailyLimit": warDroneDailyLimit,
		"nextAvailableAt": next, "defensePosture": posture, "missions": warDroneMissions, "defenses": warDroneDefenses,
		"missionOrder": []string{"reconnaissance", "precision_strike", "infrastructure_disruption"}, "defenseOrder": []string{"interceptor_screen", "dispersed_forces", "hardened_sites"},
		"energyPerDrone": warDroneEnergyCost, "equipmentPerDrone": warDroneEquipmentCost, "strikes": strikes,
		"targetAvailability": targetAvailability,
		"nextRound":          rounds + 1, "active": stage != "ended", "opponentID": opponentID,
	}
}
