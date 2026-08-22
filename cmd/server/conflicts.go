package main

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func conflictPageValue(raw string, fallback, maximum int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return min(value, maximum)
}

func (a *app) conflictDirectory(w http.ResponseWriter, r *http.Request, _ user) {
	page := conflictPageValue(r.URL.Query().Get("page"), 1, 1000000)
	pageSize := conflictPageValue(r.URL.Query().Get("pageSize"), 10, 50)
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "active" && status != "ended" {
		status = "all"
	}
	stageFilter := ""
	if status == "active" {
		stageFilter = " AND w.stage<>'ended'"
	} else if status == "ended" {
		stageFilter = " AND w.stage='ended'"
	}
	visible := ` NOT EXISTS(SELECT 1 FROM user_bans b WHERE b.user_id=an.owner_id AND (b.expires_at IS NULL OR b.expires_at>NOW()))
		AND NOT EXISTS(SELECT 1 FROM user_bans b WHERE b.user_id=dn.owner_id AND (b.expires_at IS NULL OR b.expires_at>NOW()))`
	var total int
	countQuery := `SELECT COUNT(*) FROM conflicts c JOIN wars w ON w.conflict_id=c.id JOIN nations an ON an.id=c.attacker_id JOIN nations dn ON dn.id=c.defender_id WHERE c.kind='war' AND ` + visible + stageFilter
	if err := a.db.QueryRowContext(r.Context(), countQuery).Scan(&total); err != nil {
		problem(w, http.StatusInternalServerError, "Could not count world conflicts.")
		return
	}
	pages := max(1, int(math.Ceil(float64(total)/float64(pageSize))))
	page = min(page, pages)
	query := `SELECT c.id,c.declared_at,c.attacker_id,an.name,an.leader_name,COALESCE(aa.id,''),COALESCE(aa.name,''),
		c.defender_id,dn.name,dn.leader_name,COALESCE(da.id,''),COALESCE(da.name,''),
		w.objective,w.stage,w.attacker_score,w.defender_score,w.attacker_resolve,w.defender_resolve,w.rounds_resolved,w.distance_km,w.route_type,COALESCE(w.outcome,''),COALESCE(w.winner_nation_id,'')
		FROM conflicts c JOIN wars w ON w.conflict_id=c.id
		JOIN nations an ON an.id=c.attacker_id JOIN nations dn ON dn.id=c.defender_id
		LEFT JOIN alliance_members aam ON aam.nation_id=an.id LEFT JOIN alliances aa ON aa.id=aam.alliance_id
		LEFT JOIN alliance_members dam ON dam.nation_id=dn.id LEFT JOIN alliances da ON da.id=dam.alliance_id
		WHERE c.kind='war' AND ` + visible + stageFilter + ` ORDER BY c.declared_at DESC,c.id DESC LIMIT ? OFFSET ?`
	rows, err := a.db.QueryContext(r.Context(), query, pageSize, (page-1)*pageSize)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Could not load world conflicts.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, attackerID, attackerName, attackerLeader, attackerAllianceID, attackerAllianceName string
		var defenderID, defenderName, defenderLeader, defenderAllianceID, defenderAllianceName string
		var objective, stage, route, outcome, winner string
		var declared time.Time
		var attackerScore, defenderScore, attackerResolve, defenderResolve, distance float64
		var rounds int
		if rows.Scan(&id, &declared, &attackerID, &attackerName, &attackerLeader, &attackerAllianceID, &attackerAllianceName, &defenderID, &defenderName, &defenderLeader, &defenderAllianceID, &defenderAllianceName, &objective, &stage, &attackerScore, &defenderScore, &attackerResolve, &defenderResolve, &rounds, &distance, &route, &outcome, &winner) == nil {
			items = append(items, map[string]any{
				"id": id, "declaredAt": declared, "objective": objective, "objectiveName": warObjectives[objective].Name, "stage": stage,
				"attacker":       map[string]any{"id": attackerID, "name": attackerName, "leaderName": attackerLeader, "allianceID": attackerAllianceID, "allianceName": attackerAllianceName, "score": attackerScore, "resolve": attackerResolve},
				"defender":       map[string]any{"id": defenderID, "name": defenderName, "leaderName": defenderLeader, "allianceID": defenderAllianceID, "allianceName": defenderAllianceName, "score": defenderScore, "resolve": defenderResolve},
				"roundsResolved": rounds, "distanceKm": distance, "routeType": route, "outcome": outcome, "winnerNationID": winner,
			})
		}
	}
	write(w, http.StatusOK, map[string]any{"items": items, "page": page, "pageSize": pageSize, "pages": pages, "total": total, "status": status})
}

func (a *app) publicConflictDetails(w http.ResponseWriter, r *http.Request, _ user) {
	id := r.PathValue("id")
	var attackerID, attackerName, attackerLeader, attackerAllianceID, attackerAllianceName string
	var defenderID, defenderName, defenderLeader, defenderAllianceID, defenderAllianceName string
	var objective, stage, route, winner, outcome, endReason string
	var declared, next, ends time.Time
	var ended sql.NullTime
	var attackerScore, defenderScore, attackerResolve, defenderResolve, attackerReadiness, defenderReadiness, attackerOrganization, defenderOrganization, distance, supplyFactor float64
	var attackerLat, attackerLng, defenderLat, defenderLng float64
	var rounds, mobilization int
	err := a.db.QueryRowContext(r.Context(), `SELECT c.declared_at,c.attacker_id,an.name,an.leader_name,COALESCE(aa.id,''),COALESCE(aa.name,''),
		c.defender_id,dn.name,dn.leader_name,COALESCE(da.id,''),COALESCE(da.name,''),w.objective,w.stage,
		w.attacker_score,w.defender_score,w.attacker_resolve,w.defender_resolve,w.attacker_readiness,w.defender_readiness,w.attacker_organization,w.defender_organization,
		w.rounds_resolved,w.next_round_at,w.ends_at,w.distance_km,w.route_type,COALESCE(w.attacker_lat,an.location_lat,0),COALESCE(w.attacker_lng,an.location_lng,0),COALESCE(w.defender_lat,dn.location_lat,0),COALESCE(w.defender_lng,dn.location_lng,0),w.mobilization_rounds,w.supply_factor,COALESCE(w.winner_nation_id,''),COALESCE(w.outcome,''),COALESCE(w.end_reason,''),w.ended_at
		FROM conflicts c JOIN wars w ON w.conflict_id=c.id JOIN nations an ON an.id=c.attacker_id JOIN nations dn ON dn.id=c.defender_id
		LEFT JOIN alliance_members aam ON aam.nation_id=an.id LEFT JOIN alliances aa ON aa.id=aam.alliance_id
		LEFT JOIN alliance_members dam ON dam.nation_id=dn.id LEFT JOIN alliances da ON da.id=dam.alliance_id
		WHERE c.id=? AND c.kind='war'
		AND NOT EXISTS(SELECT 1 FROM user_bans b WHERE b.user_id=an.owner_id AND (b.expires_at IS NULL OR b.expires_at>NOW()))
		AND NOT EXISTS(SELECT 1 FROM user_bans b WHERE b.user_id=dn.owner_id AND (b.expires_at IS NULL OR b.expires_at>NOW()))`, id).Scan(
		&declared, &attackerID, &attackerName, &attackerLeader, &attackerAllianceID, &attackerAllianceName,
		&defenderID, &defenderName, &defenderLeader, &defenderAllianceID, &defenderAllianceName, &objective, &stage,
		&attackerScore, &defenderScore, &attackerResolve, &defenderResolve, &attackerReadiness, &defenderReadiness, &attackerOrganization, &defenderOrganization,
		&rounds, &next, &ends, &distance, &route, &attackerLat, &attackerLng, &defenderLat, &defenderLng, &mobilization, &supplyFactor, &winner, &outcome, &endReason, &ended)
	if err != nil {
		problem(w, http.StatusNotFound, "Conflict not found.")
		return
	}
	forces := map[string]map[string]any{"attacker": {}, "defender": {}}
	forceRows, _ := a.db.QueryContext(r.Context(), `SELECT nation_id,unit_type,SUM(quantity),SUM(remaining),MIN(arrives_round) FROM war_deployments WHERE conflict_id=? GROUP BY nation_id,unit_type`, id)
	if forceRows != nil {
		defer forceRows.Close()
		for forceRows.Next() {
			var nationID, unit string
			var deployed, remaining int64
			var arrives int
			if forceRows.Scan(&nationID, &unit, &deployed, &remaining, &arrives) == nil {
				side := "defender"
				if nationID == attackerID {
					side = "attacker"
				}
				forces[side][unit] = map[string]any{"deployed": deployed, "remaining": remaining, "lost": max(int64(0), deployed-remaining), "arrivesRound": arrives}
			}
		}
	}
	reports := []map[string]any{}
	reportRows, _ := a.db.QueryContext(r.Context(), `SELECT round_number,resolved_at,attacker_operation,defender_operation,attacker_strength,defender_strength,attacker_losses,defender_losses,attacker_supply,defender_supply,attacker_score_change,defender_score_change,attacker_resolve_change,defender_resolve_change,summary FROM war_reports WHERE conflict_id=? ORDER BY round_number DESC`, id)
	if reportRows != nil {
		defer reportRows.Close()
		for reportRows.Next() {
			var round int
			var resolved time.Time
			var attackerOperation, defenderOperation, attackerLossesJSON, defenderLossesJSON, summary string
			var attackerStrength, defenderStrength, attackerSupply, defenderSupply, attackerScoreChange, defenderScoreChange, attackerResolveChange, defenderResolveChange float64
			if reportRows.Scan(&round, &resolved, &attackerOperation, &defenderOperation, &attackerStrength, &defenderStrength, &attackerLossesJSON, &defenderLossesJSON, &attackerSupply, &defenderSupply, &attackerScoreChange, &defenderScoreChange, &attackerResolveChange, &defenderResolveChange, &summary) == nil {
				attackerLosses, defenderLosses := map[string]int64{}, map[string]int64{}
				_ = json.Unmarshal([]byte(attackerLossesJSON), &attackerLosses)
				_ = json.Unmarshal([]byte(defenderLossesJSON), &defenderLosses)
				reports = append(reports, map[string]any{"round": round, "resolvedAt": resolved, "attackerOperation": attackerOperation, "defenderOperation": defenderOperation, "attackerStrength": attackerStrength, "defenderStrength": defenderStrength, "attackerLosses": attackerLosses, "defenderLosses": defenderLosses, "attackerSupply": attackerSupply, "defenderSupply": defenderSupply, "attackerScoreChange": attackerScoreChange, "defenderScoreChange": defenderScoreChange, "attackerResolveChange": attackerResolveChange, "defenderResolveChange": defenderResolveChange, "summary": summary})
			}
		}
	}
	var endedAt any
	if ended.Valid {
		endedAt = ended.Time
	}
	deployments, _ := warDeploymentBatches(r.Context(), a.db, id, attackerID, stage, rounds, next)
	write(w, http.StatusOK, map[string]any{
		"id": id, "declaredAt": declared, "objective": objective, "objectiveName": warObjectives[objective].Name, "objectiveDescription": warObjectives[objective].Description, "stage": stage,
		"attacker":       map[string]any{"id": attackerID, "name": attackerName, "leaderName": attackerLeader, "allianceID": attackerAllianceID, "allianceName": attackerAllianceName, "score": attackerScore, "resolve": attackerResolve, "readiness": attackerReadiness, "organization": attackerOrganization, "lat": attackerLat, "lng": attackerLng},
		"defender":       map[string]any{"id": defenderID, "name": defenderName, "leaderName": defenderLeader, "allianceID": defenderAllianceID, "allianceName": defenderAllianceName, "score": defenderScore, "resolve": defenderResolve, "readiness": defenderReadiness, "organization": defenderOrganization, "lat": defenderLat, "lng": defenderLng},
		"roundsResolved": rounds, "maximumRounds": warMaximumRounds, "nextRoundAt": next, "endsAt": ends, "endedAt": endedAt, "distanceKm": distance, "routeType": route, "mobilizationRounds": mobilization, "supplyFactor": supplyFactor,
		"winnerNationID": winner, "outcome": outcome, "endReason": endReason, "forces": forces, "deployments": deployments, "reports": reports,
	})
}
