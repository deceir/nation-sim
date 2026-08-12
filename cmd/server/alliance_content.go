package main

import (
	"net/http"
	"strings"
)

func (a *app) rejectAllianceApplication(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	p, err := a.alliancePermission(r.Context(), u.ID, aid)
	if err != nil || !p.Applicants {
		problem(w, http.StatusForbidden, "Applicant-management permission required.")
		return
	}
	result, err := a.db.ExecContext(r.Context(), `UPDATE alliance_applications SET status='rejected',resolved_at=NOW(),resolved_by=? WHERE id=? AND alliance_id=? AND status='pending'`, p.NationID, r.PathValue("applicationID"), aid)
	if err != nil || affected(result) != 1 {
		problem(w, http.StatusNotFound, "Application not found.")
		return
	}
	write(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *app) postAllianceAnnouncement(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	p, err := a.alliancePermission(r.Context(), u.ID, aid)
	if err != nil || !p.Announcements {
		problem(w, http.StatusForbidden, "Announcement permission required.")
		return
	}
	var in struct{ Title, Body string }
	if !decode(w, r, &in) {
		return
	}
	in.Title, in.Body = strings.TrimSpace(in.Title), strings.TrimSpace(in.Body)
	if len(in.Title) < 1 || len(in.Title) > 120 || len(in.Body) < 1 || len(in.Body) > 5000 {
		problem(w, http.StatusBadRequest, "Invalid announcement.")
		return
	}
	if _, err = a.db.ExecContext(r.Context(), `INSERT INTO alliance_announcements(id,alliance_id,author_nation_id,title,body) VALUES(?,?,?,?,?)`, uuid(), aid, p.NationID, in.Title, in.Body); err != nil {
		problem(w, http.StatusInternalServerError, "Announcement could not be posted.")
		return
	}
	write(w, http.StatusCreated, map[string]bool{"ok": true})
}
