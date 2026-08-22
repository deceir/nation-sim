package main

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type warHistoryEntry struct {
	ID             string         `json:"id"`
	DeclaredAt     time.Time      `json:"declaredAt"`
	Objective      string         `json:"objective"`
	ObjectiveName  string         `json:"objectiveName"`
	Stage          string         `json:"stage"`
	Outcome        string         `json:"outcome"`
	WinnerNationID string         `json:"winnerNationID"`
	Attacker       map[string]any `json:"attacker"`
	Defender       map[string]any `json:"defender"`
}

func (a *app) warHistory(ctx context.Context, condition string, args []any, page, pageSize int) ([]warHistoryEntry, int, int, error) {
	visible := ` NOT EXISTS(SELECT 1 FROM user_bans b WHERE b.user_id=an.owner_id AND (b.expires_at IS NULL OR b.expires_at>NOW()))
		AND NOT EXISTS(SELECT 1 FROM user_bans b WHERE b.user_id=dn.owner_id AND (b.expires_at IS NULL OR b.expires_at>NOW()))`
	base := ` FROM conflicts c JOIN wars w ON w.conflict_id=c.id
		JOIN nations an ON an.id=c.attacker_id JOIN nations dn ON dn.id=c.defender_id
		LEFT JOIN alliance_members aam ON aam.nation_id=an.id LEFT JOIN alliances aa ON aa.id=aam.alliance_id
		LEFT JOIN alliance_members dam ON dam.nation_id=dn.id LEFT JOIN alliances da ON da.id=dam.alliance_id
		WHERE c.kind='war' AND ` + visible + ` AND (` + condition + `)`
	var total int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT c.id)`+base, args...).Scan(&total); err != nil {
		return nil, 0, 0, err
	}
	pages := max(1, (total+pageSize-1)/pageSize)
	page = min(max(1, page), pages)
	query := `SELECT DISTINCT c.id,c.declared_at,c.attacker_id,an.name,an.leader_name,COALESCE(aa.id,''),COALESCE(aa.name,''),
		c.defender_id,dn.name,dn.leader_name,COALESCE(da.id,''),COALESCE(da.name,''),
		w.objective,w.stage,COALESCE(w.outcome,''),COALESCE(w.winner_nation_id,'')` + base + ` ORDER BY c.declared_at DESC,c.id DESC LIMIT ? OFFSET ?`
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := a.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	items := []warHistoryEntry{}
	for rows.Next() {
		var item warHistoryEntry
		var attackerID, attackerName, attackerLeader, attackerAllianceID, attackerAllianceName string
		var defenderID, defenderName, defenderLeader, defenderAllianceID, defenderAllianceName string
		if err := rows.Scan(&item.ID, &item.DeclaredAt, &attackerID, &attackerName, &attackerLeader, &attackerAllianceID, &attackerAllianceName,
			&defenderID, &defenderName, &defenderLeader, &defenderAllianceID, &defenderAllianceName,
			&item.Objective, &item.Stage, &item.Outcome, &item.WinnerNationID); err != nil {
			return nil, 0, 0, err
		}
		item.ObjectiveName = warObjectives[item.Objective].Name
		if strings.TrimSpace(item.ObjectiveName) == "" {
			item.ObjectiveName = strings.ReplaceAll(strings.Title(strings.ReplaceAll(item.Objective, "_", " ")), "_", " ")
		}
		item.Attacker = map[string]any{"id": attackerID, "name": attackerName, "leaderName": attackerLeader, "allianceID": attackerAllianceID, "allianceName": attackerAllianceName}
		item.Defender = map[string]any{"id": defenderID, "name": defenderName, "leaderName": defenderLeader, "allianceID": defenderAllianceID, "allianceName": defenderAllianceName}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}
	return items, pages, total, nil
}

func (a *app) nationWarHistory(w http.ResponseWriter, r *http.Request, _ user) {
	page := conflictPageValue(r.URL.Query().Get("page"), 1, 1000000)
	nationID := r.PathValue("id")
	items, pages, total, err := a.warHistory(r.Context(), `(c.attacker_id=? OR c.defender_id=?)`, []any{nationID, nationID}, page, 5)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Could not load this nation's war history.")
		return
	}
	page = min(max(1, page), pages)
	write(w, http.StatusOK, map[string]any{"items": items, "page": page, "pageSize": 5, "pages": pages, "total": total, "perspectiveNationID": nationID})
}

func (a *app) allianceWarHistory(w http.ResponseWriter, r *http.Request, _ user) {
	page := conflictPageValue(r.URL.Query().Get("page"), 1, 1000000)
	allianceID := r.PathValue("id")
	condition := `EXISTS(SELECT 1 FROM alliance_members scope_am WHERE scope_am.alliance_id=? AND scope_am.nation_id=c.attacker_id)
		OR EXISTS(SELECT 1 FROM alliance_members scope_dm WHERE scope_dm.alliance_id=? AND scope_dm.nation_id=c.defender_id)`
	items, pages, total, err := a.warHistory(r.Context(), condition, []any{allianceID, allianceID}, page, 10)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Could not load this Alliance's war history.")
		return
	}
	page = min(max(1, page), pages)
	write(w, http.StatusOK, map[string]any{"items": items, "page": page, "pageSize": 10, "pages": pages, "total": total})
}
