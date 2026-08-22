package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	warRoundHours        = 6
	warMaximumRounds     = 20
	warOffensiveSlots    = 2
	warDefensiveSlots    = 3
	warArmisticeDays     = 7
	warMinimumCapitulate = 4
)

type warObjective struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

var warObjectives = map[string]warObjective{
	"resource_seizure":        {"Resource Seizure", "Win a limited transfer of strategic stockpiles. War remains a poor substitute for trade or raiding."},
	"military_suppression":    {"Military Suppression", "Prioritize destruction of the opposing armed forces and their readiness."},
	"infrastructure_campaign": {"Infrastructure Campaign", "Use strategic operations to pressure productive infrastructure. Permanent province destruction is not part of this first release."},
	"blockade":                {"Blockade", "Reward naval and air control, particularly on maritime approaches."},
	"territorial_pressure":    {"Territorial Pressure", "Apply broad pressure through sustained combined-arms control."},
	"regime_pressure":         {"Regime Pressure", "Target national resolve through sustained political and military pressure."},
}

var warOperations = map[string]string{
	"hold": "Hold Position", "ground_assault": "Ground Assault", "air_campaign": "Air Campaign",
	"naval_blockade": "Naval Blockade", "strategic_strike": "Strategic Strike", "resupply": "Resupply and Reorganize",
}
var warPostures = map[string]string{"entrenched": "Entrenched", "balanced": "Balanced", "aggressive": "Aggressive"}
var warUnitStrength = map[string]float64{"soldiers": 1, "tanks": 34, "ships": 85, "jets": 62, "drones": 18}

type warNation struct {
	ID, Name, Continent string
	Lat, Lng            float64
}

type warState struct {
	ConflictID                                 string
	AttackerID, DefenderID                     string
	Objective, Stage                           string
	AttackerScore, DefenderScore               float64
	AttackerResolve, DefenderResolve           float64
	AttackerReadiness, DefenderReadiness       float64
	AttackerOrganization, DefenderOrganization float64
	Rounds                                     int
	NextRound, EndsAt                          time.Time
	Distance, SupplyFactor                     float64
	Route                                      string
	Mobilization                               int
}

func nextWarRound(t time.Time) time.Time {
	t = t.UTC().Truncate(time.Hour)
	nextHour := ((t.Hour() / warRoundHours) + 1) * warRoundHours
	if nextHour >= 24 {
		return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, time.UTC)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), nextHour, 0, 0, 0, time.UTC)
}

func haversineKM(aLat, aLng, bLat, bLng float64) float64 {
	const radius = 6371.0
	toRad := math.Pi / 180
	dLat, dLng := (bLat-aLat)*toRad, (bLng-aLng)*toRad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(aLat*toRad)*math.Cos(bLat*toRad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return radius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func warRoute(from, to string) string {
	if from == to {
		return "land"
	}
	if from == "Oceania" || to == "Oceania" || from == "Antarctica" || to == "Antarctica" {
		return "maritime"
	}
	american := func(v string) bool { return strings.Contains(v, "America") }
	if american(from) != american(to) {
		return "maritime"
	}
	return "mixed"
}

func orderedNationPair(a, b string) (string, string) {
	if a < b {
		return a, b
	}
	return b, a
}

func loadWarNation(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, clause, value string) (warNation, error) {
	var n warNation
	var lat, lng sql.NullFloat64
	err := q.QueryRowContext(ctx, `SELECT id,name,continent,location_lat,location_lng FROM nations WHERE `+clause+`=?`, value).Scan(&n.ID, &n.Name, &n.Continent, &lat, &lng)
	if err != nil {
		return n, err
	}
	if lat.Valid && lng.Valid {
		n.Lat, n.Lng = lat.Float64, lng.Float64
	} else {
		n.Lat, n.Lng = locationFallback(n.Continent)
	}
	return n, nil
}

func (a *app) warsDashboard(w http.ResponseWriter, r *http.Request, u user) {
	me, err := loadWarNation(r.Context(), a.db, "owner_id", u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT c.id,c.attacker_id,an.name,c.defender_id,dn.name,w.objective,w.stage,w.attacker_score,w.defender_score,w.attacker_resolve,w.defender_resolve,w.rounds_resolved,w.next_round_at,w.ends_at,w.distance_km,w.route_type,w.mobilization_rounds,COALESCE(w.winner_nation_id,''),COALESCE(w.outcome,'') FROM conflicts c JOIN wars w ON w.conflict_id=c.id JOIN nations an ON an.id=c.attacker_id JOIN nations dn ON dn.id=c.defender_id WHERE c.attacker_id=? OR c.defender_id=? ORDER BY (w.stage='ended'),c.declared_at DESC`, me.ID, me.ID)
	if err != nil {
		problem(w, 500, "Could not load wars.")
		return
	}
	defer rows.Close()
	wars := []map[string]any{}
	for rows.Next() {
		var id, aid, an, did, dn, objective, stage, route, winner, outcome string
		var as, ds, ar, dr, distance float64
		var rounds, mobilization int
		var next, ends time.Time
		if rows.Scan(&id, &aid, &an, &did, &dn, &objective, &stage, &as, &ds, &ar, &dr, &rounds, &next, &ends, &distance, &route, &mobilization, &winner, &outcome) == nil {
			wars = append(wars, map[string]any{"id": id, "attackerID": aid, "attackerName": an, "defenderID": did, "defenderName": dn, "objective": objective, "objectiveName": warObjectives[objective].Name, "stage": stage, "attackerScore": as, "defenderScore": ds, "attackerResolve": ar, "defenderResolve": dr, "roundsResolved": rounds, "nextRoundAt": next, "endsAt": ends, "distanceKm": distance, "routeType": route, "mobilizationRounds": mobilization, "winnerNationID": winner, "outcome": outcome, "isAttacker": aid == me.ID})
		}
	}
	status := map[string]any{"warExhaustion": 0.0, "reconstructionUntil": nil}
	var exhaustion float64
	var reconstruction sql.NullTime
	if a.db.QueryRowContext(r.Context(), `SELECT war_exhaustion,reconstruction_until FROM nation_war_status WHERE nation_id=?`, me.ID).Scan(&exhaustion, &reconstruction) == nil {
		status["warExhaustion"] = exhaustion
		if reconstruction.Valid {
			status["reconstructionUntil"] = reconstruction.Time
		}
	}
	write(w, 200, map[string]any{"nationID": me.ID, "wars": wars, "objectives": warObjectives, "operations": warOperations, "postures": warPostures, "status": status, "rules": map[string]any{"roundHours": warRoundHours, "maximumRounds": warMaximumRounds, "offensiveSlots": warOffensiveSlots, "defensiveSlots": warDefensiveSlots, "minimumCapitulationRound": warMinimumCapitulate}})
}

func (a *app) declareWar(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		DefenderID, DefenderName, Objective string
		Forces                              map[string]int64
	}
	if !decode(w, r, &in) {
		return
	}
	if _, ok := warObjectives[in.Objective]; !ok {
		problem(w, 400, "Choose a valid war objective.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not begin the declaration.")
		return
	}
	defer tx.Rollback()
	attacker, err := loadWarNation(r.Context(), tx, "owner_id", u.ID)
	if err != nil {
		problem(w, 409, "Create a nation first.")
		return
	}
	var defender warNation
	if in.DefenderID != "" {
		defender, err = loadWarNation(r.Context(), tx, "id", in.DefenderID)
	} else {
		defender, err = loadWarNation(r.Context(), tx, "name", strings.TrimSpace(in.DefenderName))
	}
	if err != nil {
		problem(w, 404, "Defending nation not found.")
		return
	}
	if attacker.ID == defender.ID {
		problem(w, 400, "You cannot declare war on yourself.")
		return
	}
	lockA, lockB := orderedNationPair(attacker.ID, defender.ID)
	var locked string
	if tx.QueryRowContext(r.Context(), `SELECT id FROM nations WHERE id=? FOR UPDATE`, lockA).Scan(&locked) != nil || tx.QueryRowContext(r.Context(), `SELECT id FROM nations WHERE id=? FOR UPDATE`, lockB).Scan(&locked) != nil {
		problem(w, 409, "A nation involved in this declaration is unavailable.")
		return
	}
	for _, unit := range militaryUnitKeys() {
		var quantity int64
		_ = tx.QueryRowContext(r.Context(), `SELECT quantity FROM military_inventory WHERE nation_id=? AND unit_type=? FOR UPDATE`, attacker.ID, unit).Scan(&quantity)
	}
	var guarded int
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM guardian_grants WHERE nation_id=? AND revoked_at IS NULL AND starts_at<=UTC_TIMESTAMP() AND expires_at>UTC_TIMESTAMP())`, defender.ID).Scan(&guarded); err != nil {
		problem(w, 500, "Could not verify the defending nation's Guardian status.")
		return
	}
	if guarded == 1 {
		problem(w, 409, "That nation has Guardian status.")
		return
	}
	var reconstruction sql.NullTime
	_ = tx.QueryRowContext(r.Context(), `SELECT reconstruction_until FROM nation_war_status WHERE nation_id=?`, attacker.ID).Scan(&reconstruction)
	if reconstruction.Valid && reconstruction.Time.After(time.Now().UTC()) {
		problem(w, 409, "Your nation is still in post-war reconstruction and cannot open an offensive war.")
		return
	}
	var defenderReconstruction sql.NullTime
	_ = tx.QueryRowContext(r.Context(), `SELECT reconstruction_until FROM nation_war_status WHERE nation_id=?`, defender.ID).Scan(&defenderReconstruction)
	var offense, defense, duplicate int
	_ = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM conflicts c JOIN wars w ON w.conflict_id=c.id WHERE c.attacker_id=? AND w.stage<>'ended'`, attacker.ID).Scan(&offense)
	_ = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM conflicts c JOIN wars w ON w.conflict_id=c.id WHERE c.defender_id=? AND w.stage<>'ended'`, defender.ID).Scan(&defense)
	_ = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM conflicts c JOIN wars w ON w.conflict_id=c.id WHERE ((c.attacker_id=? AND c.defender_id=?) OR (c.attacker_id=? AND c.defender_id=?)) AND w.stage<>'ended'`, attacker.ID, defender.ID, defender.ID, attacker.ID).Scan(&duplicate)
	if offense >= warOffensiveSlots {
		problem(w, 409, "All of your offensive war slots are currently occupied.")
		return
	}
	if defense >= warDefensiveSlots {
		problem(w, 409, "That nation already has the maximum number of defensive wars.")
		return
	}
	if defenderReconstruction.Valid && defenderReconstruction.Time.After(time.Now().UTC()) && defense >= 1 {
		problem(w, 409, "That recovering nation already has an active defensive front. Reconstruction prevents another front from being stacked onto it.")
		return
	}
	if duplicate > 0 {
		problem(w, 409, "These nations are already at war.")
		return
	}
	pa, pb := orderedNationPair(attacker.ID, defender.ID)
	var armistice int
	_ = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM war_armistices WHERE nation_a_id=? AND nation_b_id=? AND expires_at>UTC_TIMESTAMP())`, pa, pb).Scan(&armistice)
	if armistice == 1 {
		problem(w, 409, "An active armistice prevents another declaration between these nations.")
		return
	}
	total := int64(0)
	for _, unit := range militaryUnitKeys() {
		amount := in.Forces[unit]
		if amount < 0 {
			problem(w, 400, "Deployment quantities cannot be negative.")
			return
		}
		if amount > committedAvailable(r.Context(), tx, attacker.ID, unit) {
			problem(w, 409, "You do not have enough available "+militaryUnits[unit].Name+" for that deployment.")
			return
		}
		total += amount
	}
	if total <= 0 {
		problem(w, 400, "Commit at least one military unit to the declaration.")
		return
	}
	distance := haversineKM(attacker.Lat, attacker.Lng, defender.Lat, defender.Lng)
	mobilization := 1 + int(math.Floor(distance/3500))
	if mobilization > 5 {
		mobilization = 5
	}
	supplyFactor := 1 + math.Min(1.25, distance/12000)
	next := nextWarRound(time.Now().UTC())
	ends := next.Add(time.Duration(warMaximumRounds*warRoundHours) * time.Hour)
	id := uuid()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO conflicts(id,kind,attacker_id,defender_id,status) VALUES(?,'war',?,?,'active')`, id, attacker.ID, defender.ID); err != nil {
		problem(w, 500, "Could not create the war.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO wars(conflict_id,objective,next_round_at,ends_at,distance_km,route_type,mobilization_rounds,supply_factor) VALUES(?,?,?,?,?,?,?,?)`, id, in.Objective, next, ends, distance, warRoute(attacker.Continent, defender.Continent), mobilization, supplyFactor); err != nil {
		problem(w, 500, "Could not establish the war state.")
		return
	}
	for _, unit := range militaryUnitKeys() {
		if amount := in.Forces[unit]; amount > 0 {
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO war_deployments(id,conflict_id,nation_id,unit_type,quantity,remaining,arrives_round) VALUES(?,?,?,?,?,?,?)`, uuid(), id, attacker.ID, unit, amount, amount, mobilization); err != nil {
				problem(w, 500, "Could not deploy the attacking force.")
				return
			}
		}
	}
	// A home defender automatically commits up to 60% of its free force. This
	// prevents an offline player being represented by an empty battlefield.
	for _, unit := range militaryUnitKeys() {
		amount := int64(math.Floor(float64(committedAvailable(r.Context(), tx, defender.ID, unit)) * .60))
		if amount > 0 {
			_, _ = tx.ExecContext(r.Context(), `INSERT INTO war_deployments(id,conflict_id,nation_id,unit_type,quantity,remaining,arrives_round) VALUES(?,?,?,?,?,?,0)`, uuid(), id, defender.ID, unit, amount, amount)
		}
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE guardian_grants SET revoked_at=UTC_TIMESTAMP(),revoked_reason='initiated_war' WHERE nation_id=? AND revoked_at IS NULL AND starts_at<=UTC_TIMESTAMP() AND expires_at>UTC_TIMESTAMP()`, attacker.ID); err != nil {
		problem(w, 500, "Could not update the attacking nation's Guardian status.")
		return
	}
	_, _ = tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'war','War declared',?),(?,?,'war','Your nation is at war',?)`, uuid(), attacker.ID, fmt.Sprintf("You declared war on %s. Initial forces arrive in %d strategic rounds.", defender.Name, mobilization), uuid(), defender.ID, fmt.Sprintf("%s declared war on your nation. Home forces have begun an automatic defensive deployment.", attacker.Name))
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Could not finalize the declaration.")
		return
	}
	write(w, 201, map[string]any{"ok": true, "warID": id, "distanceKm": distance, "mobilizationRounds": mobilization, "nextRoundAt": next})
}

func committedAvailable(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, nationID, unit string) int64 {
	var owned int64
	_ = q.QueryRowContext(ctx, `SELECT COALESCE((SELECT quantity FROM military_inventory WHERE nation_id=? AND unit_type=?),0)`, nationID, unit).Scan(&owned)
	return max(int64(0), owned-committedMilitary(ctx, q, nationID, unit))
}

func (a *app) warDetails(w http.ResponseWriter, r *http.Request, u user) {
	me, err := loadWarNation(r.Context(), a.db, "owner_id", u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	id := r.PathValue("id")
	var aid, an, did, dn, objective, stage, route, winner, outcome, endReason string
	var as, ds, ar, dr, ard, drd, aorg, dorg, distance, supply float64
	var rounds, mobilization int
	var next, ends time.Time
	var ended sql.NullTime
	err = a.db.QueryRowContext(r.Context(), `SELECT c.attacker_id,an.name,c.defender_id,dn.name,w.objective,w.stage,w.attacker_score,w.defender_score,w.attacker_resolve,w.defender_resolve,w.attacker_readiness,w.defender_readiness,w.attacker_organization,w.defender_organization,w.rounds_resolved,w.next_round_at,w.ends_at,w.distance_km,w.route_type,w.mobilization_rounds,w.supply_factor,COALESCE(w.winner_nation_id,''),COALESCE(w.outcome,''),COALESCE(w.end_reason,''),w.ended_at FROM conflicts c JOIN wars w ON w.conflict_id=c.id JOIN nations an ON an.id=c.attacker_id JOIN nations dn ON dn.id=c.defender_id WHERE c.id=? AND (c.attacker_id=? OR c.defender_id=?)`, id, me.ID, me.ID).Scan(&aid, &an, &did, &dn, &objective, &stage, &as, &ds, &ar, &dr, &ard, &drd, &aorg, &dorg, &rounds, &next, &ends, &distance, &route, &mobilization, &supply, &winner, &outcome, &endReason, &ended)
	if err != nil {
		problem(w, 404, "War not found.")
		return
	}
	forces := map[string]map[string]any{"attacker": {}, "defender": {}}
	rows, _ := a.db.QueryContext(r.Context(), `SELECT nation_id,unit_type,SUM(quantity),SUM(remaining),MIN(arrives_round) FROM war_deployments WHERE conflict_id=? GROUP BY nation_id,unit_type`, id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var nid, unit string
			var qty, remaining int64
			var arrives int
			if rows.Scan(&nid, &unit, &qty, &remaining, &arrives) == nil {
				side := "defender"
				if nid == aid {
					side = "attacker"
				}
				forces[side][unit] = map[string]any{"deployed": qty, "remaining": remaining, "arrivesRound": arrives}
			}
		}
	}
	reports := []map[string]any{}
	rr, _ := a.db.QueryContext(r.Context(), `SELECT round_number,resolved_at,attacker_operation,defender_operation,attacker_strength,defender_strength,attacker_losses,defender_losses,attacker_supply,defender_supply,attacker_score_change,defender_score_change,attacker_resolve_change,defender_resolve_change,summary FROM war_reports WHERE conflict_id=? ORDER BY round_number DESC`, id)
	if rr != nil {
		defer rr.Close()
		for rr.Next() {
			var round int
			var at time.Time
			var ao, do_, al, dl, summary string
			var ast, dst, asu, dsu, asc, dsc, arc, drc float64
			if rr.Scan(&round, &at, &ao, &do_, &ast, &dst, &al, &dl, &asu, &dsu, &asc, &dsc, &arc, &drc, &summary) == nil {
				var alm, dlm map[string]int64
				_ = json.Unmarshal([]byte(al), &alm)
				_ = json.Unmarshal([]byte(dl), &dlm)
				reports = append(reports, map[string]any{"round": round, "resolvedAt": at, "attackerOperation": ao, "defenderOperation": do_, "attackerStrength": ast, "defenderStrength": dst, "attackerLosses": alm, "defenderLosses": dlm, "attackerSupply": asu, "defenderSupply": dsu, "attackerScoreChange": asc, "defenderScoreChange": dsc, "attackerResolveChange": arc, "defenderResolveChange": drc, "summary": summary})
			}
		}
	}
	write(w, 200, map[string]any{"id": id, "attackerID": aid, "attackerName": an, "defenderID": did, "defenderName": dn, "objective": objective, "objectiveName": warObjectives[objective].Name, "objectiveDescription": warObjectives[objective].Description, "stage": stage, "attackerScore": as, "defenderScore": ds, "attackerResolve": ar, "defenderResolve": dr, "attackerReadiness": ard, "defenderReadiness": drd, "attackerOrganization": aorg, "defenderOrganization": dorg, "roundsResolved": rounds, "nextRoundAt": next, "endsAt": ends, "distanceKm": distance, "routeType": route, "mobilizationRounds": mobilization, "supplyFactor": supply, "winnerNationID": winner, "outcome": outcome, "endReason": endReason, "forces": forces, "reports": reports, "myNationID": me.ID, "isAttacker": me.ID == aid, "operations": warOperations, "postures": warPostures})
}

func (a *app) deployWarForces(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Forces map[string]int64 }
	if !decode(w, r, &in) {
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not begin deployment.")
		return
	}
	defer tx.Rollback()
	me, err := loadWarNation(r.Context(), tx, "owner_id", u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	id := r.PathValue("id")
	var aid, did, stage string
	var rounds, mobilization int
	if tx.QueryRowContext(r.Context(), `SELECT c.attacker_id,c.defender_id,w.stage,w.rounds_resolved,w.mobilization_rounds FROM conflicts c JOIN wars w ON w.conflict_id=c.id WHERE c.id=? FOR UPDATE`, id).Scan(&aid, &did, &stage, &rounds, &mobilization) != nil || (me.ID != aid && me.ID != did) {
		problem(w, 404, "War not found.")
		return
	}
	var locked string
	if tx.QueryRowContext(r.Context(), `SELECT id FROM nations WHERE id=? FOR UPDATE`, me.ID).Scan(&locked) != nil {
		problem(w, 409, "Your nation is unavailable for deployment.")
		return
	}
	for _, unit := range militaryUnitKeys() {
		var quantity int64
		_ = tx.QueryRowContext(r.Context(), `SELECT quantity FROM military_inventory WHERE nation_id=? AND unit_type=? FOR UPDATE`, me.ID, unit).Scan(&quantity)
	}
	if stage == "ended" {
		problem(w, 409, "This war has ended.")
		return
	}
	arrives := rounds + 1
	if me.ID == aid {
		arrives = rounds + mobilization
	}
	total := int64(0)
	for _, unit := range militaryUnitKeys() {
		amount := in.Forces[unit]
		if amount < 0 || amount > committedAvailable(r.Context(), tx, me.ID, unit) {
			problem(w, 409, "Invalid or unavailable "+militaryUnits[unit].Name+" deployment.")
			return
		}
		if amount > 0 {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO war_deployments(id,conflict_id,nation_id,unit_type,quantity,remaining,arrives_round) VALUES(?,?,?,?,?,?,?)`, uuid(), id, me.ID, unit, amount, amount, arrives)
			if err != nil {
				problem(w, 500, "Could not deploy forces.")
				return
			}
			total += amount
		}
	}
	if total == 0 {
		problem(w, 400, "Choose at least one unit to deploy.")
		return
	}
	if tx.Commit() != nil {
		problem(w, 500, "Could not complete deployment.")
		return
	}
	write(w, 201, map[string]any{"ok": true, "arrivesRound": arrives})
}

func (a *app) submitWarOrders(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Operation, Posture string }
	if !decode(w, r, &in) {
		return
	}
	if _, ok := warOperations[in.Operation]; !ok {
		problem(w, 400, "Choose a valid operation.")
		return
	}
	if _, ok := warPostures[in.Posture]; !ok {
		problem(w, 400, "Choose a valid posture.")
		return
	}
	me, err := loadWarNation(r.Context(), a.db, "owner_id", u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	id := r.PathValue("id")
	var rounds int
	var stage string
	var party int
	_ = a.db.QueryRowContext(r.Context(), `SELECT w.rounds_resolved,w.stage,(c.attacker_id=? OR c.defender_id=?) FROM wars w JOIN conflicts c ON c.id=w.conflict_id WHERE w.conflict_id=?`, me.ID, me.ID, id).Scan(&rounds, &stage, &party)
	if party != 1 {
		problem(w, 404, "War not found.")
		return
	}
	if stage == "ended" {
		problem(w, 409, "This war has ended.")
		return
	}
	_, err = a.db.ExecContext(r.Context(), `INSERT INTO war_orders(conflict_id,nation_id,round_number,operation,posture) VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE operation=VALUES(operation),posture=VALUES(posture),submitted_at=CURRENT_TIMESTAMP(6)`, id, me.ID, rounds+1, in.Operation, in.Posture)
	if err != nil {
		problem(w, 500, "Could not save orders.")
		return
	}
	write(w, 200, map[string]any{"ok": true, "round": rounds + 1})
}

func (a *app) capitulateWar(w http.ResponseWriter, r *http.Request, u user) {
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not begin capitulation.")
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
	myResolve := state.DefenderResolve
	if me.ID == state.AttackerID {
		myResolve = state.AttackerResolve
	}
	if state.Rounds < warMinimumCapitulate && myResolve > 35 {
		problem(w, 409, "Capitulation is available after four rounds, or earlier when resolve falls to 35 or below.")
		return
	}
	winner := state.AttackerID
	if me.ID == state.AttackerID {
		winner = state.DefenderID
	}
	if err = endWar(r.Context(), tx, &state, winner, "major", "capitulation"); err != nil {
		problem(w, 500, "Could not conclude the war.")
		return
	}
	if tx.Commit() != nil {
		problem(w, 500, "Could not conclude the war.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func loadWarState(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string, lock bool) (warState, error) {
	var s warState
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, `SELECT c.id,c.attacker_id,c.defender_id,w.objective,w.stage,w.attacker_score,w.defender_score,w.attacker_resolve,w.defender_resolve,w.attacker_readiness,w.defender_readiness,w.attacker_organization,w.defender_organization,w.rounds_resolved,w.next_round_at,w.ends_at,w.distance_km,w.route_type,w.mobilization_rounds,w.supply_factor FROM conflicts c JOIN wars w ON w.conflict_id=c.id WHERE c.id=?`+suffix, id).Scan(&s.ConflictID, &s.AttackerID, &s.DefenderID, &s.Objective, &s.Stage, &s.AttackerScore, &s.DefenderScore, &s.AttackerResolve, &s.DefenderResolve, &s.AttackerReadiness, &s.DefenderReadiness, &s.AttackerOrganization, &s.DefenderOrganization, &s.Rounds, &s.NextRound, &s.EndsAt, &s.Distance, &s.Route, &s.Mobilization, &s.SupplyFactor)
	return s, err
}

func operationMultiplier(operation, unit, route, objective string) float64 {
	m := 1.0
	switch operation {
	case "ground_assault":
		if unit == "soldiers" || unit == "tanks" {
			m = 1.25
		} else {
			m = .9
		}
	case "air_campaign":
		if unit == "jets" || unit == "drones" {
			m = 1.3
		} else {
			m = .92
		}
	case "naval_blockade":
		if unit == "ships" || unit == "jets" {
			m = 1.32
		} else {
			m = .82
		}
	case "strategic_strike":
		if unit == "jets" || unit == "drones" {
			m = 1.22
		} else {
			m = .88
		}
	case "resupply":
		m = .72
	}
	if objective == "blockade" && operation == "naval_blockade" {
		m *= 1.18
	}
	if objective == "military_suppression" && (operation == "ground_assault" || operation == "air_campaign") {
		m *= 1.1
	}
	if objective == "infrastructure_campaign" && operation == "strategic_strike" {
		m *= 1.18
	}
	if route == "maritime" && unit == "ships" {
		m *= 1.15
	}
	return m
}

func postureMultiplier(posture string) float64 {
	if posture == "aggressive" {
		return 1.13
	}
	if posture == "entrenched" {
		return 1.08
	}
	return 1
}
func deterministicWarJitter(id string, round int, side string) float64 {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", id, round, side)))
	return .92 + float64(sum[0])/255*.16
}

func warForces(ctx context.Context, tx *sql.Tx, id, nid string, round int) (map[string]int64, error) {
	result := map[string]int64{}
	rows, err := tx.QueryContext(ctx, `SELECT unit_type,SUM(remaining) FROM war_deployments WHERE conflict_id=? AND nation_id=? AND arrives_round<=? AND remaining>0 GROUP BY unit_type`, id, nid, round)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var unit string
		var n int64
		if rows.Scan(&unit, &n) == nil {
			result[unit] = n
		}
	}
	return result, nil
}

func warOrder(ctx context.Context, tx *sql.Tx, id, nid string, round int) (string, string) {
	op, posture := "hold", "entrenched"
	_ = tx.QueryRowContext(ctx, `SELECT operation,posture FROM war_orders WHERE conflict_id=? AND nation_id=? AND round_number=?`, id, nid, round).Scan(&op, &posture)
	return op, posture
}

func consumeWarSupply(ctx context.Context, tx *sql.Tx, nid string, forces map[string]int64, distanceFactor float64) (float64, error) {
	var cash int64
	var food, energy, equipment float64
	for unit, n := range forces {
		v := float64(n)
		switch unit {
		case "soldiers":
			cash += int64(v * 2)
			food += v * .001
		case "tanks":
			cash += int64(v * 100)
			energy += v * .02
			equipment += v * .002
		case "ships":
			cash += int64(v * 300)
			energy += v * .08
			equipment += v * .004
		case "jets":
			cash += int64(v * 250)
			energy += v * .06
			equipment += v * .006
		case "drones":
			cash += int64(v * 80)
			energy += v * .02
			equipment += v * .004
		}
	}
	cash = int64(math.Ceil(float64(cash) * distanceFactor))
	food *= distanceFactor
	energy *= distanceFactor
	equipment *= distanceFactor
	var treasury int64
	if err := tx.QueryRowContext(ctx, `SELECT treasury FROM nations WHERE id=? FOR UPDATE`, nid).Scan(&treasury); err != nil {
		return 0, err
	}
	eff := 1.0
	if cash > 0 {
		eff = math.Min(eff, float64(treasury)/float64(cash))
	}
	needs := map[string]float64{"foodstuffs": food, "energy": energy, "military_equipment": equipment}
	for resource, need := range needs {
		if need <= 0 {
			continue
		}
		var have float64
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity=?),0)`, nid, resource).Scan(&have)
		eff = math.Min(eff, have/need)
	}
	eff = math.Max(.35, math.Min(1, eff))
	spendCash := int64(math.Floor(float64(cash) * eff))
	_, err := tx.ExecContext(ctx, `UPDATE nations SET treasury=GREATEST(0,treasury-?) WHERE id=?`, spendCash, nid)
	if err != nil {
		return 0, err
	}
	for resource, need := range needs {
		if need > 0 {
			_, err = tx.ExecContext(ctx, `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,0) ON DUPLICATE KEY UPDATE amount=GREATEST(0,amount-?)`, nid, resource, need*eff)
			if err != nil {
				return 0, err
			}
		}
	}
	return eff, nil
}

func forceStrength(forces map[string]int64, operation, posture, route, objective string, readiness, organization, supply, exhaustion float64, defending bool) float64 {
	total := 0.0
	for unit, n := range forces {
		total += float64(n) * warUnitStrength[unit] * operationMultiplier(operation, unit, route, objective)
	}
	exhaustionFactor := 1 - math.Min(.35, exhaustion/250)
	if defending {
		// Exhaustion cannot turn a repeatedly attacked nation into progressively
		// easier prey. Local resistance offsets part of the strategic damage, but
		// never creates immunity or a Guardian-style reward for losing.
		exhaustionFactor = 1 + math.Min(.25, exhaustion/400)
	}
	return total * postureMultiplier(posture) * (readiness / 100) * (organization / 100) * supply * exhaustionFactor
}

func applyWarLosses(ctx context.Context, tx *sql.Tx, id, nid string, round int, forces map[string]int64, rate float64) (map[string]int64, error) {
	losses := map[string]int64{}
	keys := militaryUnitKeys()
	sort.Strings(keys)
	for _, unit := range keys {
		available := forces[unit]
		if available <= 0 {
			continue
		}
		loss := int64(math.Floor(float64(available) * rate))
		if loss == 0 && rate > .015 && available > 0 {
			loss = 1
		}
		loss = min(loss, available)
		if loss <= 0 {
			continue
		}
		rows, err := tx.QueryContext(ctx, `SELECT id,remaining FROM war_deployments WHERE conflict_id=? AND nation_id=? AND unit_type=? AND arrives_round<=? AND remaining>0 ORDER BY created_at,id FOR UPDATE`, id, nid, unit, round)
		if err != nil {
			return losses, err
		}
		remainingLoss := loss
		type dep struct {
			id string
			n  int64
		}
		deps := []dep{}
		for rows.Next() {
			var d dep
			if rows.Scan(&d.id, &d.n) == nil {
				deps = append(deps, d)
			}
		}
		rows.Close()
		for _, d := range deps {
			take := min(d.n, remainingLoss)
			if take > 0 {
				if _, err = tx.ExecContext(ctx, `UPDATE war_deployments SET remaining=remaining-? WHERE id=?`, take, d.id); err != nil {
					return losses, err
				}
				remainingLoss -= take
			}
			if remainingLoss == 0 {
				break
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE military_inventory SET quantity=GREATEST(0,quantity-?) WHERE nation_id=? AND unit_type=?`, loss, nid, unit); err != nil {
			return losses, err
		}
		losses[unit] = loss
	}
	return losses, nil
}

func (a *app) processWarRounds(ctx context.Context, turn time.Time) {
	for processed := 0; processed < 100; processed++ {
		var id string
		err := a.db.QueryRowContext(ctx, `SELECT conflict_id FROM wars WHERE stage<>'ended' AND next_round_at<=? ORDER BY next_round_at LIMIT 1`, turn).Scan(&id)
		if err != nil {
			return
		}
		tx, err := a.db.BeginTx(ctx, nil)
		if err != nil {
			return
		}
		state, err := loadWarState(ctx, tx, id, true)
		if err != nil || state.Stage == "ended" || state.NextRound.After(turn) {
			tx.Rollback()
			continue
		}
		if err = resolveWarRound(ctx, tx, &state); err != nil {
			tx.Rollback()
			log.Printf("war round %s failed: %v", id, err)
			return
		}
		if err = tx.Commit(); err != nil {
			log.Printf("war round %s commit failed: %v", id, err)
			return
		}
	}
}

func resolveWarRound(ctx context.Context, tx *sql.Tx, s *warState) error {
	round := s.Rounds + 1
	af, err := warForces(ctx, tx, s.ConflictID, s.AttackerID, round)
	if err != nil {
		return err
	}
	df, err := warForces(ctx, tx, s.ConflictID, s.DefenderID, round)
	if err != nil {
		return err
	}
	ao, ap := warOrder(ctx, tx, s.ConflictID, s.AttackerID, round)
	do, dp := warOrder(ctx, tx, s.ConflictID, s.DefenderID, round)
	asupply, err := consumeWarSupply(ctx, tx, s.AttackerID, af, s.SupplyFactor)
	if err != nil {
		return err
	}
	dsupply, err := consumeWarSupply(ctx, tx, s.DefenderID, df, 1)
	if err != nil {
		return err
	}
	var ae, de float64
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT war_exhaustion FROM nation_war_status WHERE nation_id=?),0),COALESCE((SELECT war_exhaustion FROM nation_war_status WHERE nation_id=?),0)`, s.AttackerID, s.DefenderID).Scan(&ae, &de)
	astr := forceStrength(af, ao, ap, s.Route, s.Objective, s.AttackerReadiness, s.AttackerOrganization, asupply, ae, false) * deterministicWarJitter(s.ConflictID, round, "a")
	dstr := forceStrength(df, do, dp, s.Route, s.Objective, s.DefenderReadiness, s.DefenderOrganization, dsupply, de, true) * deterministicWarJitter(s.ConflictID, round, "d")
	total := astr + dstr
	if total < 1 {
		total = 1
	}
	ashare := astr / total
	dshare := dstr / total
	alossRate := .004 + .03*dshare
	dlossRate := .004 + .03*ashare
	if ap == "aggressive" {
		alossRate *= 1.15
	}
	if dp == "aggressive" {
		dlossRate *= 1.15
	}
	aloss, err := applyWarLosses(ctx, tx, s.ConflictID, s.AttackerID, round, af, alossRate)
	if err != nil {
		return err
	}
	dloss, err := applyWarLosses(ctx, tx, s.ConflictID, s.DefenderID, round, df, dlossRate)
	if err != nil {
		return err
	}
	asc := 2 + ashare*7
	dsc := 2 + dshare*7
	if astr == 0 {
		asc = 0
	}
	if dstr == 0 {
		dsc = 0
	}
	arChange := -(1 + 7*dshare)
	drChange := -(1 + 7*ashare)
	if ao == "resupply" {
		s.AttackerReadiness = math.Min(100, s.AttackerReadiness+7)
		s.AttackerOrganization = math.Min(100, s.AttackerOrganization+9)
	} else {
		s.AttackerReadiness = math.Max(25, s.AttackerReadiness-1.5)
		s.AttackerOrganization = math.Max(20, s.AttackerOrganization-2*dshare)
	}
	if do == "resupply" {
		s.DefenderReadiness = math.Min(100, s.DefenderReadiness+7)
		s.DefenderOrganization = math.Min(100, s.DefenderOrganization+9)
	} else {
		s.DefenderReadiness = math.Max(25, s.DefenderReadiness-1.5)
		s.DefenderOrganization = math.Max(20, s.DefenderOrganization-2*ashare)
	}
	s.AttackerScore += asc
	s.DefenderScore += dsc
	s.AttackerResolve = math.Max(0, s.AttackerResolve+arChange)
	s.DefenderResolve = math.Max(0, s.DefenderResolve+drChange)
	s.Rounds = round
	s.Stage = "active"
	s.NextRound = s.NextRound.Add(warRoundHours * time.Hour)
	alj, _ := json.Marshal(aloss)
	dlj, _ := json.Marshal(dloss)
	summary := fmt.Sprintf("Round %d: %s met %s. Effective strength was %.0f to %.0f.", round, warOperations[ao], warOperations[do], astr, dstr)
	_, err = tx.ExecContext(ctx, `INSERT INTO war_reports(id,conflict_id,round_number,attacker_operation,defender_operation,attacker_strength,defender_strength,attacker_losses,defender_losses,attacker_supply,defender_supply,attacker_score_change,defender_score_change,attacker_resolve_change,defender_resolve_change,summary) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, uuid(), s.ConflictID, round, ao, do, astr, dstr, alj, dlj, asupply, dsupply, asc, dsc, arChange, drChange, summary)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE wars SET stage=?,attacker_score=?,defender_score=?,attacker_resolve=?,defender_resolve=?,attacker_readiness=?,defender_readiness=?,attacker_organization=?,defender_organization=?,rounds_resolved=?,next_round_at=? WHERE conflict_id=?`, s.Stage, s.AttackerScore, s.DefenderScore, s.AttackerResolve, s.DefenderResolve, s.AttackerReadiness, s.DefenderReadiness, s.AttackerOrganization, s.DefenderOrganization, s.Rounds, s.NextRound, s.ConflictID)
	if err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO nation_war_status(nation_id,war_exhaustion) VALUES(?,2),(?,2) ON DUPLICATE KEY UPDATE war_exhaustion=LEAST(100,war_exhaustion+2)`, s.AttackerID, s.DefenderID)
	if s.AttackerResolve <= 0 {
		return endWar(ctx, tx, s, s.DefenderID, "decisive", "resolve_broken")
	}
	if s.DefenderResolve <= 0 {
		return endWar(ctx, tx, s, s.AttackerID, "decisive", "resolve_broken")
	}
	if s.Rounds >= warMaximumRounds || !s.NextRound.Before(s.EndsAt) {
		winner := ""
		diff := s.AttackerScore - s.DefenderScore
		if math.Abs(diff) > 3 {
			if diff > 0 {
				winner = s.AttackerID
			} else {
				winner = s.DefenderID
			}
		}
		outcome := "stalemate"
		if winner != "" {
			outcome = "minor"
			if math.Abs(diff) >= 15 {
				outcome = "major"
			}
			if math.Abs(diff) >= 35 {
				outcome = "decisive"
			}
		}
		return endWar(ctx, tx, s, winner, outcome, "round_limit")
	}
	return nil
}

func endWar(ctx context.Context, tx *sql.Tx, s *warState, winner, outcome, reason string) error {
	now := time.Now().UTC()
	_, err := tx.ExecContext(ctx, `UPDATE wars SET stage='ended',winner_nation_id=NULLIF(?,''),outcome=?,end_reason=?,ended_at=? WHERE conflict_id=?`, winner, outcome, reason, now, s.ConflictID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE conflicts SET status='ended' WHERE id=?`, s.ConflictID)
	if err != nil {
		return err
	}
	a, b := orderedNationPair(s.AttackerID, s.DefenderID)
	_, err = tx.ExecContext(ctx, `INSERT INTO war_armistices(nation_a_id,nation_b_id,expires_at,source_conflict_id) VALUES(?,?,DATE_ADD(UTC_TIMESTAMP(),INTERVAL ? DAY),?) ON DUPLICATE KEY UPDATE expires_at=VALUES(expires_at),source_conflict_id=VALUES(source_conflict_id)`, a, b, warArmisticeDays, s.ConflictID)
	if err != nil {
		return err
	}
	loser := ""
	if winner == s.AttackerID {
		loser = s.DefenderID
	} else if winner == s.DefenderID {
		loser = s.AttackerID
	}
	days := 0
	if loser != "" {
		days = 3
		if outcome == "major" {
			days = 5
		}
		if outcome == "decisive" {
			days = 7
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO nation_war_status(nation_id,reconstruction_until) VALUES(?,DATE_ADD(UTC_TIMESTAMP(),INTERVAL ? DAY)) ON DUPLICATE KEY UPDATE reconstruction_until=VALUES(reconstruction_until)`, loser, days)
		if err != nil {
			return err
		}
	}
	if winner == s.AttackerID && s.Objective == "resource_seizure" {
		pct := .01
		if outcome == "major" {
			pct = .02
		}
		if outcome == "decisive" {
			pct = .03
		}
		for _, resource := range strategicCommodities {
			var have float64
			_ = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity=?),0)`, loser, resource).Scan(&have)
			amount := math.Min(250, have*pct)
			if amount > 0 {
				_, _ = tx.ExecContext(ctx, `UPDATE nation_stockpiles SET amount=amount-? WHERE nation_id=? AND commodity=?`, amount, loser, resource)
				_, _ = tx.ExecContext(ctx, `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, winner, resource, amount)
			}
		}
	}
	if winner == s.AttackerID && s.Objective == "infrastructure_campaign" {
		damageRate := .02
		if outcome == "major" {
			damageRate = .05
		}
		if outcome == "decisive" {
			damageRate = .08
		}
		var strategicRounds int
		_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM war_orders WHERE conflict_id=? AND nation_id=? AND operation='strategic_strike'`, s.ConflictID, s.AttackerID).Scan(&strategicRounds)
		if strategicRounds == 0 {
			damageRate *= .5
		}
		_, err = tx.ExecContext(ctx, `UPDATE cities SET infrastructure=GREATEST(50,FLOOR(infrastructure*(1-?))) WHERE nation_id=?`, damageRate, s.DefenderID)
		if err != nil {
			return err
		}
		_, _ = tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'war','Infrastructure damaged',?)`, uuid(), s.DefenderID, fmt.Sprintf("The concluded infrastructure campaign reduced provincial Infrastructure by approximately %.0f%%. Infrastructure cannot be damaged below 50.", damageRate*100))
	}
	message := "The war ended in a stalemate."
	if winner != "" {
		message = fmt.Sprintf("The war ended in a %s victory. The defeated nation enters %d days of reconstruction.", outcome, days)
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'war','War concluded',?),(?,?,'war','War concluded',?)`, uuid(), s.AttackerID, message, uuid(), s.DefenderID, message)
	return nil
}
