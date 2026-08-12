package main

import (
	"database/sql"
	"net/http"
	"strings"
	"time"
)

var notificationCategories = map[string]bool{"economic": true, "war": true, "market": true, "game": true, "moderation": true}

type notificationItem struct {
	ID        string     `json:"id"`
	Category  string     `json:"category"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	CreatedAt time.Time  `json:"createdAt"`
	ReadAt    *time.Time `json:"readAt"`
}

func (a *app) notifications(w http.ResponseWriter, r *http.Request, u user) {
	nid, err := a.nationID(r.Context(), u.ID)
	if err != nil {
		problem(w, http.StatusNotFound, "Nation not found.")
		return
	}
	category := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if category != "" && !notificationCategories[category] {
		problem(w, http.StatusBadRequest, "Unknown notification type.")
		return
	}
	var unread int
	if r.URL.Query().Get("summary") == "1" {
		// Routine economic-turn reports remain in the log but do not create
		// attention debt in the global navigation badge.
		a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM notifications WHERE nation_id=? AND read_at IS NULL AND category<>'economic'`, nid).Scan(&unread)
		write(w, http.StatusOK, map[string]any{"unread": unread})
		return
	}
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM notifications WHERE nation_id=? AND read_at IS NULL`, nid).Scan(&unread)
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,category,title,message,created_at,read_at FROM notifications WHERE nation_id=? AND (?='' OR category=?) ORDER BY created_at DESC LIMIT 200`, nid, category, category)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Notifications unavailable.")
		return
	}
	defer rows.Close()
	items := []notificationItem{}
	for rows.Next() {
		var item notificationItem
		var read sql.NullTime
		if rows.Scan(&item.ID, &item.Category, &item.Title, &item.Message, &item.CreatedAt, &read) != nil {
			continue
		}
		if read.Valid {
			item.ReadAt = &read.Time
		}
		items = append(items, item)
	}
	write(w, http.StatusOK, map[string]any{"items": items, "unread": unread})
}

func (a *app) readNotifications(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		ID, Category string
		All          bool
	}
	if !decode(w, r, &in) {
		return
	}
	nid, err := a.nationID(r.Context(), u.ID)
	if err != nil {
		problem(w, http.StatusNotFound, "Nation not found.")
		return
	}
	if in.All {
		category := strings.ToLower(strings.TrimSpace(in.Category))
		if category != "" && !notificationCategories[category] {
			problem(w, http.StatusBadRequest, "Unknown notification type.")
			return
		}
		_, err = a.db.ExecContext(r.Context(), `UPDATE notifications SET read_at=COALESCE(read_at,NOW()) WHERE nation_id=? AND (?='' OR category=?)`, nid, category, category)
	} else if strings.TrimSpace(in.ID) != "" {
		_, err = a.db.ExecContext(r.Context(), `UPDATE notifications SET read_at=COALESCE(read_at,NOW()) WHERE id=? AND nation_id=?`, strings.TrimSpace(in.ID), nid)
	} else {
		problem(w, http.StatusBadRequest, "Choose a notification or mark all as read.")
		return
	}
	if err != nil {
		problem(w, http.StatusInternalServerError, "Could not update notifications.")
		return
	}
	write(w, http.StatusOK, map[string]bool{"ok": true})
}
