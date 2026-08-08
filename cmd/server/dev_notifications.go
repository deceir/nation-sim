package main

import (
	"net/http"
	"strings"
)

func (a *app) broadcastGameNotification(w http.ResponseWriter, r *http.Request, u user) {
	var userType string
	if err := a.db.QueryRowContext(r.Context(), `SELECT user_type FROM nations WHERE owner_id=?`, u.ID).Scan(&userType); err != nil || userType != "DEV" {
		problem(w, http.StatusForbidden, "Developer access required.")
		return
	}
	var in struct{ Title, Message string }
	if !decode(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Message = strings.TrimSpace(in.Message)
	if len(in.Title) < 3 || len(in.Title) > 120 || len(in.Message) < 3 || len(in.Message) > 1000 {
		problem(w, http.StatusBadRequest, "Use a title of 3–120 characters and a message of 3–1,000 characters.")
		return
	}
	result, err := a.db.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) SELECT UUID(),id,'game',?,? FROM nations WHERE user_type IN('PLAYER','DEV')`, in.Title, in.Message)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Could not send the Game notification.")
		return
	}
	count, _ := result.RowsAffected()
	write(w, http.StatusCreated, map[string]any{"ok": true, "recipients": count})
}
