package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (a *app) isDev(r *http.Request, userID string) bool {
	var kind string
	return a.db.QueryRowContext(r.Context(), `SELECT user_type FROM nations WHERE owner_id=?`, userID).Scan(&kind) == nil && kind == "DEV"
}

func (a *app) activeBan(ctx context.Context, userID string) (string, *time.Time, bool) {
	var reason string
	var expiry sql.NullTime
	err := a.db.QueryRowContext(ctx, `SELECT reason,expires_at FROM user_bans WHERE user_id=? AND (expires_at IS NULL OR expires_at>NOW())`, userID).Scan(&reason, &expiry)
	if err != nil {
		return "", nil, false
	}
	if expiry.Valid {
		return reason, &expiry.Time, true
	}
	return reason, nil, true
}

func (a *app) reportNation(w http.ResponseWriter, r *http.Request, u user) {
	targetID := r.PathValue("id")
	var targetName string
	if err := a.db.QueryRowContext(r.Context(), `SELECT name FROM nations WHERE id=?`, targetID).Scan(&targetName); err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	var in struct{ Reason string }
	if !decode(w, r, &in) {
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if len(in.Reason) > 500 {
		problem(w, 400, "Report details are too long.")
		return
	}
	var reporter string
	a.db.QueryRowContext(r.Context(), `SELECT name FROM nations WHERE owner_id=?`, u.ID).Scan(&reporter)
	message := fmt.Sprintf("%s reported %s for moderation. Review: /nation/%s", reporter, targetName, targetID)
	if in.Reason != "" {
		message += " Reason: " + in.Reason
	}
	_, err := a.db.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) SELECT UUID(),id,'moderation','Nation report',? FROM nations WHERE user_type='DEV'`, message)
	if err != nil {
		problem(w, 500, "Could not submit report.")
		return
	}
	write(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (a *app) banUser(w http.ResponseWriter, r *http.Request, u user) {
	if !a.isDev(r, u.ID) {
		problem(w, 403, "Developer access required.")
		return
	}
	var in struct{ NationID, Reason, ExpiresAt string }
	if !decode(w, r, &in) {
		return
	}
	var targetUser, targetName string
	if err := a.db.QueryRowContext(r.Context(), `SELECT owner_id,name FROM nations WHERE id=?`, strings.TrimSpace(in.NationID)).Scan(&targetUser, &targetName); err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	if targetUser == u.ID {
		problem(w, 400, "You cannot ban your own account.")
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if len(in.Reason) > 1000 {
		problem(w, 400, "Ban reason is too long.")
		return
	}
	var expires any
	if strings.TrimSpace(in.ExpiresAt) != "" {
		t, err := time.Parse("2006-01-02", in.ExpiresAt)
		if err != nil || !t.After(time.Now()) {
			problem(w, 400, "Choose a future ban expiry date.")
			return
		}
		expires = t
	}
	if _, err := a.db.ExecContext(r.Context(), `INSERT INTO user_bans(user_id,banned_by_user_id,reason,expires_at) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE banned_by_user_id=VALUES(banned_by_user_id),reason=VALUES(reason),expires_at=VALUES(expires_at),created_at=NOW()`, targetUser, u.ID, in.Reason, expires); err != nil {
		problem(w, 500, "Could not apply ban.")
		return
	}
	a.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE user_id=?`, targetUser)
	write(w, http.StatusOK, map[string]any{"ok": true, "nation": targetName})
}

func (a *app) unbanUser(w http.ResponseWriter, r *http.Request, u user) {
	if !a.isDev(r, u.ID) {
		problem(w, 403, "Developer access required.")
		return
	}
	if _, err := a.db.ExecContext(r.Context(), `DELETE FROM user_bans WHERE user_id=?`, r.PathValue("userID")); err != nil {
		problem(w, 500, "Could not remove ban.")
		return
	}
	write(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *app) devBans(w http.ResponseWriter, r *http.Request, u user) {
	if !a.isDev(r, u.ID) {
		problem(w, 403, "Developer access required.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT b.user_id,n.id,n.name,b.reason,b.expires_at,b.created_at FROM user_bans b JOIN nations n ON n.owner_id=b.user_id WHERE b.expires_at IS NULL OR b.expires_at>NOW() ORDER BY b.created_at DESC`)
	if err != nil {
		problem(w, 500, "Bans unavailable.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var uid, nid, name, reason string
		var expires sql.NullTime
		var created time.Time
		if rows.Scan(&uid, &nid, &name, &reason, &expires, &created) == nil {
			var until any
			if expires.Valid {
				until = expires.Time
			}
			items = append(items, map[string]any{"userID": uid, "nationID": nid, "nation": name, "reason": reason, "expiresAt": until, "createdAt": created})
		}
	}
	write(w, http.StatusOK, map[string]any{"items": items})
}

func (a *app) removeGuardianStatus(w http.ResponseWriter, r *http.Request, u user) {
	if !a.isDev(r, u.ID) {
		problem(w, http.StatusForbidden, "Developer access required.")
		return
	}
	targetID := strings.TrimSpace(r.PathValue("id"))
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Could not begin Guardian moderation.")
		return
	}
	defer tx.Rollback()
	var targetName string
	if err = tx.QueryRowContext(r.Context(), `SELECT name FROM nations WHERE id=? FOR UPDATE`, targetID).Scan(&targetName); err != nil {
		problem(w, http.StatusNotFound, "Nation not found.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE guardian_grants SET revoked_at=UTC_TIMESTAMP(),revoked_reason='removed_by_dev' WHERE nation_id=? AND revoked_at IS NULL AND starts_at<=UTC_TIMESTAMP() AND expires_at>UTC_TIMESTAMP()`, targetID)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Could not remove Guardian status.")
		return
	}
	removed := affected(result)
	if removed == 0 {
		problem(w, http.StatusConflict, "That nation does not currently have active Guardian status.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'game','Guardian status removed','Diplomatia has removed Guardian protection from your nation.')`, uuid(), targetID); err != nil {
		problem(w, http.StatusInternalServerError, "Could not record the Guardian moderation action.")
		return
	}
	if err = tx.Commit(); err != nil {
		problem(w, http.StatusInternalServerError, "Could not complete Guardian moderation.")
		return
	}
	write(w, http.StatusOK, map[string]any{"ok": true, "nation": targetName, "grantsRevoked": removed})
}

func (a *app) voluntarilyRemoveGuardianStatus(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Confirmed bool }
	if !decode(w, r, &in) {
		return
	}
	if !in.Confirmed {
		problem(w, http.StatusBadRequest, "You must acknowledge that Guardian status cannot be recovered before removing it.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Could not begin Guardian removal.")
		return
	}
	defer tx.Rollback()
	var nationID string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM nations WHERE owner_id=? FOR UPDATE`, u.ID).Scan(&nationID); err != nil {
		problem(w, http.StatusNotFound, "Nation not found.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE guardian_grants SET revoked_at=UTC_TIMESTAMP(),revoked_reason='voluntarily_removed' WHERE nation_id=? AND revoked_at IS NULL AND starts_at<=UTC_TIMESTAMP() AND expires_at>UTC_TIMESTAMP()`, nationID)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Could not remove Guardian status.")
		return
	}
	if affected(result) == 0 {
		problem(w, http.StatusConflict, "Your nation does not currently have active Guardian status.")
		return
	}
	if err = tx.Commit(); err != nil {
		problem(w, http.StatusInternalServerError, "Could not complete Guardian removal.")
		return
	}
	write(w, http.StatusOK, map[string]bool{"ok": true})
}
