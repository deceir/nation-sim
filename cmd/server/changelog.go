package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type changelogPost struct {
	ID, Title, Body, AuthorName string
	AuthorNationID              *string
	CreatedAt, UpdatedAt        time.Time
	CanEdit                     bool
}

func validateChangelogPost(title, body string) (string, string, string) {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	if len(title) < 3 || len(title) > 180 {
		return "", "", "Titles must be between 3 and 180 characters."
	}
	if len(body) < 1 || len(body) > 50000 {
		return "", "", "Posts must be between 1 and 50,000 characters."
	}
	return title, body, ""
}

func (a *app) changelogAuthor(r *http.Request, userID string) (string, string, bool) {
	var nationID, leaderName, userType string
	err := a.db.QueryRowContext(r.Context(), `SELECT id,leader_name,user_type FROM nations WHERE owner_id=?`, userID).Scan(&nationID, &leaderName, &userType)
	return nationID, leaderName, err == nil && userType == "DEV"
}

func (a *app) changelog(w http.ResponseWriter, r *http.Request, u user) {
	if r.URL.Query().Get("summary") == "1" {
		var unread int
		err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM changelog_posts p LEFT JOIN changelog_reads v ON v.user_id=? WHERE p.created_at>COALESCE(v.last_viewed_at,'1970-01-01')`, u.ID).Scan(&unread)
		if err != nil {
			problem(w, 500, "Could not load changelog status.")
			return
		}
		write(w, 200, map[string]int{"unread": unread})
		return
	}
	_, _, canPost := a.changelogAuthor(r, u.ID)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM changelog_posts`).Scan(&total); err != nil {
		problem(w, 500, "Could not load the changelog.")
		return
	}
	const pageSize = 5
	pages := (total + pageSize - 1) / pageSize
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		page = pages
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT p.id,p.author_user_id,p.author_nation_id,COALESCE(n.leader_name,p.author_name),p.title,p.body,p.created_at,p.updated_at FROM changelog_posts p LEFT JOIN nations n ON n.id=p.author_nation_id ORDER BY p.created_at DESC,p.id DESC LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		problem(w, 500, "Could not load the changelog.")
		return
	}
	defer rows.Close()
	posts := make([]changelogPost, 0)
	for rows.Next() {
		var post changelogPost
		var authorUserID, authorNationID sql.NullString
		if err = rows.Scan(&post.ID, &authorUserID, &authorNationID, &post.AuthorName, &post.Title, &post.Body, &post.CreatedAt, &post.UpdatedAt); err != nil {
			problem(w, 500, "Could not load the changelog.")
			return
		}
		if authorNationID.Valid {
			post.AuthorNationID = &authorNationID.String
		}
		post.CanEdit = canPost && authorUserID.Valid && authorUserID.String == u.ID
		posts = append(posts, post)
	}
	if err = rows.Err(); err != nil {
		problem(w, 500, "Could not load the changelog.")
		return
	}
	if _, err = a.db.ExecContext(r.Context(), `INSERT INTO changelog_reads(user_id,last_viewed_at) VALUES(?,NOW(6)) ON DUPLICATE KEY UPDATE last_viewed_at=VALUES(last_viewed_at)`, u.ID); err != nil {
		problem(w, 500, "Could not update changelog status.")
		return
	}
	write(w, 200, map[string]any{"posts": posts, "canPost": canPost, "page": page, "pages": pages, "total": total})
}

func (a *app) createChangelogPost(w http.ResponseWriter, r *http.Request, u user) {
	nationID, leaderName, allowed := a.changelogAuthor(r, u.ID)
	if !allowed {
		problem(w, 403, "Only DEV nations can publish changelog posts.")
		return
	}
	var in struct{ Title, Body string }
	if !decode(w, r, &in) {
		return
	}
	title, body, issue := validateChangelogPost(in.Title, in.Body)
	if issue != "" {
		problem(w, 400, issue)
		return
	}
	id := uuid()
	if _, err := a.db.ExecContext(r.Context(), `INSERT INTO changelog_posts(id,author_user_id,author_nation_id,author_name,title,body) VALUES(?,?,?,?,?,?)`, id, u.ID, nationID, leaderName, title, body); err != nil {
		problem(w, 500, "Could not publish the changelog post.")
		return
	}
	write(w, 201, map[string]string{"id": id})
}

func (a *app) updateChangelogPost(w http.ResponseWriter, r *http.Request, u user) {
	if _, _, allowed := a.changelogAuthor(r, u.ID); !allowed {
		problem(w, 403, "Only DEV nations can edit changelog posts.")
		return
	}
	var in struct{ Title, Body string }
	if !decode(w, r, &in) {
		return
	}
	title, body, issue := validateChangelogPost(in.Title, in.Body)
	if issue != "" {
		problem(w, 400, issue)
		return
	}
	result, err := a.db.ExecContext(r.Context(), `UPDATE changelog_posts SET title=?,body=?,updated_at=NOW(6) WHERE id=? AND author_user_id=?`, title, body, r.PathValue("id"), u.ID)
	if err != nil {
		problem(w, 500, "Could not update the changelog post.")
		return
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		problem(w, 404, "That changelog post was not found or is not yours to edit.")
		return
	}
	write(w, 200, map[string]bool{"updated": true})
}
