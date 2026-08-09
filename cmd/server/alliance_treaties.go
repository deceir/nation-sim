package main

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"
)

type treatyDefinition struct{ Acronym, Name, Definition string }

var treatyDefinitions = map[string]treatyDefinition{
	"MADP":   {"MADP/MDAP", "Mutual Aggression and Defense Pact", "Mandatory mutual defense and support in wars of aggression."},
	"MDoAP":  {"MDoAP", "Mutual Defense and Optional Aggression Pact", "Mandatory mutual defense with optional support in wars of aggression."},
	"MnDoAP": {"MnDoAP", "Mutual Non-Chaining Defense and Optional Aggression Pact", "Mutual defense and optional aggression; defense becomes optional when a conflict was triggered through another treaty or aggressive war."},
	"MDP":    {"MDP", "Mutual Defense Pact", "Mandatory mutual defense from foreign conflicts."},
	"MnDP":   {"MnDP", "Mutual Non-Chaining Defense Pact", "Mutual defense that becomes optional when the conflict chains from another treaty or aggressive war."},
	"ODAP":   {"ODAP / ODoAP", "Optional Defense and Optional Aggression Pact", "Optional mutual defense and optional participation in aggressive wars."},
	"ODP":    {"ODP", "Optional Defense Pact", "Optional mutual defense from foreign conflicts."},
	"ToA":    {"ToA", "Treaty of Amity", "Formal friendly relations and close cooperation as a foundation for deeper treaties."},
	"PIAT":   {"PIAT", "Peace, Intelligence, and Aid Treaty", "Peaceful coexistence, intelligence sharing, and stipulated aid."},
	"NAP":    {"NAP", "Non-Aggression Pact", "The parties agree not to wage war or act maliciously against each other."},
	"Prot":   {"Prot.", "Protectorate", "The proposing Protector defends the receiving Protectorate; the Protectorate may optionally defend the Protector."},
	"NPT":    {"NPT", "Non-Proliferation Treaty", "The parties agree that their members will not manufacture nuclear weapons."},
}

func treatyCatalog() []map[string]string {
	order := []string{"MADP", "MDoAP", "MnDoAP", "MDP", "MnDP", "ODAP", "ODP", "ToA", "PIAT", "NAP", "Prot", "NPT"}
	out := make([]map[string]string, 0, len(order))
	for _, key := range order {
		d := treatyDefinitions[key]
		out = append(out, map[string]string{"key": key, "acronym": d.Acronym, "name": d.Name, "definition": d.Definition})
	}
	return out
}

func (a *app) allianceTreaties(ctx context.Context, ctxAlliance string, canManage bool) (active, pending []map[string]any) {
	a.db.ExecContext(ctx, `UPDATE alliance_treaties SET status='expired' WHERE status='active' AND ends_on IS NOT NULL AND ends_on<=CURRENT_DATE()`)
	rows, _ := a.db.QueryContext(ctx, `SELECT t.id,t.treaty_type,t.terms,t.duration_days,t.starts_on,t.ends_on,t.created_at,t.proposed_by_alliance_id,
		a.id,a.name,b.id,b.name,t.status FROM alliance_treaties t JOIN alliances a ON a.id=t.alliance_a_id JOIN alliances b ON b.id=t.alliance_b_id
		WHERE (t.alliance_a_id=? OR t.alliance_b_id=?) AND (t.status='active' OR (? AND t.status='proposed')) ORDER BY t.status,t.created_at DESC`, ctxAlliance, ctxAlliance, canManage)
	if rows == nil {
		return []map[string]any{}, []map[string]any{}
	}
	defer rows.Close()
	for rows.Next() {
		var id, kind, terms, proposer, aid, aname, bid, bname, status string
		var duration sql.NullInt64
		var starts, ends sql.NullTime
		var created time.Time
		if rows.Scan(&id, &kind, &terms, &duration, &starts, &ends, &created, &proposer, &aid, &aname, &bid, &bname, &status) != nil {
			continue
		}
		d := treatyDefinitions[kind]
		item := map[string]any{"id": id, "type": kind, "acronym": d.Acronym, "name": d.Name, "definition": d.Definition, "terms": terms, "indefinite": !duration.Valid, "createdAt": created, "allianceA": map[string]string{"id": aid, "name": aname}, "allianceB": map[string]string{"id": bid, "name": bname}, "proposedByAllianceID": proposer}
		if duration.Valid {
			item["durationDays"] = duration.Int64
		}
		if starts.Valid {
			item["startsOn"] = starts.Time.Format("2006-01-02")
		}
		if ends.Valid {
			item["endsOn"] = ends.Time.Format("2006-01-02")
		}
		if status == "active" {
			active = append(active, item)
		} else {
			pending = append(pending, item)
		}
	}
	if active == nil {
		active = []map[string]any{}
	}
	if pending == nil {
		pending = []map[string]any{}
	}
	return
}

func (a *app) proposeAllianceTreaty(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.War {
		problem(w, 403, "Treaty-management permission required.")
		return
	}
	var in struct {
		AllianceName, TreatyType, Terms string
		Indefinite                      bool
		DurationDays                    int
	}
	if !decode(w, r, &in) {
		return
	}
	in.AllianceName = strings.TrimSpace(in.AllianceName)
	in.TreatyType = strings.TrimSpace(in.TreatyType)
	in.Terms = strings.TrimSpace(in.Terms)
	if _, ok := treatyDefinitions[in.TreatyType]; !ok {
		problem(w, 400, "Choose a valid treaty type.")
		return
	}
	if !in.Indefinite && (in.DurationDays < 1 || in.DurationDays > 3650) {
		problem(w, 400, "Treaty duration must be between 1 and 3,650 server days.")
		return
	}
	if len(in.Terms) > 5000 {
		problem(w, 400, "Treaty terms are too long.")
		return
	}
	var targetID, targetName string
	if a.db.QueryRowContext(r.Context(), `SELECT id,name FROM alliances WHERE LOWER(name)=LOWER(?)`, in.AllianceName).Scan(&targetID, &targetName) != nil {
		problem(w, 404, "No Alliance has that exact name.")
		return
	}
	if targetID == aid {
		problem(w, 400, "An Alliance cannot make a treaty with itself.")
		return
	}
	var exists int
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_treaties WHERE treaty_type=? AND status IN('proposed','active') AND ((alliance_a_id=? AND alliance_b_id=?) OR (alliance_a_id=? AND alliance_b_id=?))`, in.TreatyType, aid, targetID, targetID, aid).Scan(&exists)
	if exists > 0 {
		problem(w, 409, "This treaty type is already active or proposed between these Alliances.")
		return
	}
	var duration any = nil
	if !in.Indefinite {
		duration = in.DurationDays
	}
	id := uuid()
	_, e = a.db.ExecContext(r.Context(), `INSERT INTO alliance_treaties(id,alliance_a_id,alliance_b_id,proposed_by_alliance_id,proposed_by_nation_id,treaty_type,terms,duration_days) VALUES(?,?,?,?,?,?,?,?)`, id, aid, targetID, aid, p.NationID, in.TreatyType, in.Terms, duration)
	if e != nil {
		problem(w, 500, "Treaty proposal could not be sent.")
		return
	}
	a.db.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) SELECT UUID(),m.nation_id,'game','Alliance treaty proposal',CONCAT((SELECT name FROM alliances WHERE id=?),' proposed a ',?,' treaty with your Alliance.') FROM alliance_members m JOIN alliance_roles ar ON ar.id=m.role_id WHERE m.alliance_id=? AND ar.can_declare_war=1`, aid, treatyDefinitions[in.TreatyType].Acronym, targetID)
	write(w, 201, map[string]any{"ok": true, "id": id, "targetAlliance": targetName})
}

func (a *app) resolveAllianceTreaty(w http.ResponseWriter, r *http.Request, u user) {
	aid, tid := r.PathValue("id"), r.PathValue("treatyID")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.War {
		problem(w, 403, "Treaty-management permission required.")
		return
	}
	action := r.PathValue("action")
	if action != "accept" && action != "reject" {
		problem(w, 400, "Invalid treaty action.")
		return
	}
	var proposer string
	var duration sql.NullInt64
	if a.db.QueryRowContext(r.Context(), `SELECT proposed_by_alliance_id,duration_days FROM alliance_treaties WHERE id=? AND status='proposed' AND (alliance_a_id=? OR alliance_b_id=?)`, tid, aid, aid).Scan(&proposer, &duration) != nil {
		problem(w, 404, "Pending treaty proposal not found.")
		return
	}
	if proposer == aid {
		problem(w, 403, "The proposing Alliance cannot resolve its own proposal.")
		return
	}
	if action == "reject" {
		a.db.ExecContext(r.Context(), `UPDATE alliance_treaties SET status='rejected',resolved_by_nation_id=?,resolved_at=NOW() WHERE id=? AND status='proposed'`, p.NationID, tid)
	} else if duration.Valid {
		a.db.ExecContext(r.Context(), `UPDATE alliance_treaties SET status='active',starts_on=CURRENT_DATE(),ends_on=DATE_ADD(CURRENT_DATE(),INTERVAL ? DAY),resolved_by_nation_id=?,resolved_at=NOW() WHERE id=? AND status='proposed'`, duration.Int64, p.NationID, tid)
	} else {
		a.db.ExecContext(r.Context(), `UPDATE alliance_treaties SET status='active',starts_on=CURRENT_DATE(),resolved_by_nation_id=?,resolved_at=NOW() WHERE id=? AND status='proposed'`, p.NationID, tid)
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) cancelAllianceTreaty(w http.ResponseWriter, r *http.Request, u user) {
	aid, tid := r.PathValue("id"), r.PathValue("treatyID")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.War {
		problem(w, 403, "Treaty-management permission required.")
		return
	}
	result, e := a.db.ExecContext(r.Context(), `UPDATE alliance_treaties SET status='cancelled',resolved_by_nation_id=?,resolved_at=NOW() WHERE id=? AND status IN('active','proposed') AND (alliance_a_id=? OR alliance_b_id=?)`, p.NationID, tid, aid, aid)
	if e != nil || affected(result) != 1 {
		problem(w, 404, "Treaty not found.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
