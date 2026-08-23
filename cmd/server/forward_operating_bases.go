package main

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

const forwardOperatingBaseCost int64 = 25_000_000
const forwardOperatingBaseProvinceRequirement = 3
const forwardOperatingBaseCooldownDays = 60

var forwardOperatingBaseCodes = []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel", "India", "Juliett", "Kilo", "Lima", "Mike", "November", "Oscar", "Papa", "Quebec", "Romeo", "Sierra", "Tango", "Uniform", "Victor", "Whiskey", "X-ray", "Yankee", "Zulu"}

func fobCode(sequence int) string {
	if sequence >= 0 && sequence < len(forwardOperatingBaseCodes) {
		return forwardOperatingBaseCodes[sequence]
	}
	return "Alpha"
}

func (a *app) fobsForNation(ctx context.Context, nationID string) []map[string]any {
	items := []map[string]any{}
	rows, err := a.db.QueryContext(ctx, `SELECT id,name,latitude,longitude,continent,created_at FROM forward_operating_bases WHERE nation_id=? ORDER BY created_at,id`, nationID)
	if err != nil {
		return items
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, continent string
		var lat, lng float64
		var created time.Time
		if rows.Scan(&id, &name, &lat, &lng, &continent, &created) == nil {
			items = append(items, map[string]any{"id": id, "name": name, "latitude": lat, "longitude": lng, "continent": continent, "createdAt": created})
		}
	}
	return items
}

func (a *app) publicForwardOperatingBases(w http.ResponseWriter, r *http.Request, _ user) {
	var exists int
	if a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM nations WHERE id=?`, r.PathValue("id")).Scan(&exists) != nil || exists == 0 {
		problem(w, 404, "Nation not found.")
		return
	}
	items := a.fobsForNation(r.Context(), r.PathValue("id"))
	write(w, 200, map[string]any{"count": len(items), "bases": items})
}

func (a *app) myForwardOperatingBases(w http.ResponseWriter, r *http.Request, u user) {
	var nationID string
	var provinces int
	var rebuild sql.NullTime
	if a.db.QueryRowContext(r.Context(), `SELECT n.id,(SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id),(SELECT rebuild_after FROM forward_operating_base_status s WHERE s.nation_id=n.id) FROM nations n WHERE n.owner_id=?`, u.ID).Scan(&nationID, &provinces, &rebuild) != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	items := a.fobsForNation(r.Context(), nationID)
	var rebuildAfter any
	if rebuild.Valid {
		rebuildAfter = rebuild.Time
	}
	write(w, 200, map[string]any{"bases": items, "count": len(items), "maximum": 1, "cost": forwardOperatingBaseCost, "provinceRequirement": forwardOperatingBaseProvinceRequirement, "provinces": provinces, "cooldownDays": forwardOperatingBaseCooldownDays, "rebuildAfter": rebuildAfter})
}

func (a *app) buildForwardOperatingBase(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Latitude, Longitude float64 }
	if !decode(w, r, &in) {
		return
	}
	continent, ok := continentAt(in.Latitude, in.Longitude)
	if !ok {
		problem(w, 400, "Choose a valid land position on the world map.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not begin FOB construction.")
		return
	}
	defer tx.Rollback()
	var nationID, nationName string
	var treasury int64
	var provinces int
	if tx.QueryRowContext(r.Context(), `SELECT n.id,n.name,n.treasury,(SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id) FROM nations n WHERE n.owner_id=? FOR UPDATE`, u.ID).Scan(&nationID, &nationName, &treasury, &provinces) != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	if provinces < forwardOperatingBaseProvinceRequirement {
		problem(w, 409, "At least 3 Provinces are required to build a Forward Operating Base.")
		return
	}
	var existing int
	_ = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM forward_operating_bases WHERE nation_id=?`, nationID).Scan(&existing)
	if existing >= 1 {
		problem(w, 409, "Your nation already operates its maximum of one Forward Operating Base.")
		return
	}
	var rebuild sql.NullTime
	var sequence int
	err = tx.QueryRowContext(r.Context(), `SELECT rebuild_after,next_sequence FROM forward_operating_base_status WHERE nation_id=? FOR UPDATE`, nationID).Scan(&rebuild, &sequence)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO forward_operating_base_status(nation_id) VALUES(?)`, nationID)
		sequence = 0
	}
	if err != nil {
		problem(w, 500, "Could not verify FOB construction status.")
		return
	}
	if rebuild.Valid && rebuild.Time.After(time.Now().UTC()) {
		problem(w, 409, "A new Forward Operating Base cannot be built until "+rebuild.Time.Format("January 2, 2006")+".")
		return
	}
	if treasury < forwardOperatingBaseCost {
		problem(w, 409, "Insufficient Treasury. Forward Operating Base construction costs ¥25,000,000.")
		return
	}
	name := nationName + " FOB: " + fobCode(sequence)
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO forward_operating_bases(id,nation_id,name,latitude,longitude,continent) VALUES(?,?,?,?,?,?)`, uuid(), nationID, name, in.Latitude, in.Longitude, continent); err != nil {
		problem(w, 500, "Could not build the Forward Operating Base.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, forwardOperatingBaseCost, nationID); err != nil {
		problem(w, 500, "Could not pay for FOB construction.")
		return
	}
	_, _ = tx.ExecContext(r.Context(), `UPDATE forward_operating_base_status SET rebuild_after=NULL,next_sequence=? WHERE nation_id=?`, sequence+1, nationID)
	if tx.Commit() != nil {
		problem(w, 500, "Could not complete FOB construction.")
		return
	}
	write(w, 201, map[string]any{"ok": true, "name": name, "continent": continent, "cost": forwardOperatingBaseCost})
}

func (a *app) demolishForwardOperatingBase(w http.ResponseWriter, r *http.Request, u user) {
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not begin demolition.")
		return
	}
	defer tx.Rollback()
	var nationID string
	if tx.QueryRowContext(r.Context(), `SELECT id FROM nations WHERE owner_id=? FOR UPDATE`, u.ID).Scan(&nationID) != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `DELETE FROM forward_operating_bases WHERE id=? AND nation_id=?`, r.PathValue("id"), nationID)
	if err != nil || affected(result) != 1 {
		problem(w, 404, "Forward Operating Base not found.")
		return
	}
	rebuild := time.Now().UTC().AddDate(0, 0, forwardOperatingBaseCooldownDays)
	_, err = tx.ExecContext(r.Context(), `INSERT INTO forward_operating_base_status(nation_id,rebuild_after,next_sequence) VALUES(?,?,1) ON DUPLICATE KEY UPDATE rebuild_after=VALUES(rebuild_after)`, nationID, rebuild)
	if err != nil {
		problem(w, 500, "Could not record the rebuilding cooldown.")
		return
	}
	if tx.Commit() != nil {
		problem(w, 500, "Could not complete demolition.")
		return
	}
	write(w, 200, map[string]any{"ok": true, "rebuildAfter": rebuild})
}
