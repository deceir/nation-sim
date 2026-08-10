package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"html"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
)

const maxNationFlagBytes = 2 << 20

func (a *app) uploadNationFlag(w http.ResponseWriter, r *http.Request, u user) {
	r.Body = http.MaxBytesReader(w, r.Body, maxNationFlagBytes+1024)
	if err := r.ParseMultipartForm(maxNationFlagBytes); err != nil {
		problem(w, http.StatusBadRequest, "Choose a PNG or JPEG flag under 2 MB.")
		return
	}
	file, _, err := r.FormFile("flag")
	if err != nil {
		problem(w, http.StatusBadRequest, "Choose a flag image to upload.")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxNationFlagBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxNationFlagBytes {
		problem(w, http.StatusBadRequest, "Flag images must be no larger than 2 MB.")
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		problem(w, http.StatusBadRequest, "Flags must be valid PNG or JPEG images.")
		return
	}
	if config.Width < 120 || config.Height < 80 || config.Width > 2400 || config.Height > 2400 {
		problem(w, http.StatusBadRequest, "Flags must be between 120 and 2,400 pixels on each side.")
		return
	}
	ratio := float64(config.Width) / float64(config.Height)
	if ratio < 1.2 || ratio > 2.5 {
		problem(w, http.StatusBadRequest, "Use a flag-shaped image between 1.2:1 and 2.5:1.")
		return
	}
	mime := "image/png"
	if format == "jpeg" {
		mime = "image/jpeg"
	}
	if _, err = a.db.ExecContext(r.Context(), `UPDATE nations SET flag_image=?,flag_mime=?,flag_updated_at=NOW() WHERE owner_id=?`, data, mime, u.ID); err != nil {
		problem(w, http.StatusInternalServerError, "Could not save this flag.")
		return
	}
	write(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *app) nationFlag(w http.ResponseWriter, r *http.Request, _ user) {
	var name, mime string
	var data []byte
	if err := a.db.QueryRowContext(r.Context(), `SELECT n.name,COALESCE(n.flag_image,''),COALESCE(n.flag_mime,'') FROM nations n WHERE n.id=? AND NOT EXISTS(SELECT 1 FROM user_bans b WHERE b.user_id=n.owner_id AND (b.expires_at IS NULL OR b.expires_at>NOW()))`, r.PathValue("id")).Scan(&name, &data, &mime); err != nil {
		problem(w, http.StatusNotFound, "Nation not found.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if len(data) > 0 && (mime == "image/png" || mime == "image/jpeg") {
		w.Header().Set("Content-Type", mime)
		_, _ = w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	_, _ = w.Write([]byte(defaultNationFlagSVG(name)))
}

func defaultNationFlagSVG(name string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(name))))
	colors := [][2]string{{"#173552", "#d2aa68"}, {"#42285a", "#d1d6df"}, {"#344a32", "#dec479"}, {"#553339", "#e7ded0"}, {"#213f47", "#b4c5c9"}}
	pair := colors[int(digest[0])%len(colors)]
	initials := "DN"
	words := strings.Fields(name)
	if len(words) > 0 {
		initials = strings.ToUpper(string([]rune(words[0])[0]))
	}
	if len(words) > 1 {
		initials += strings.ToUpper(string([]rune(words[len(words)-1])[0]))
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 180" role="img" aria-label="Diplomatia national flag"><rect width="300" height="180" fill="%s"/><path d="M0 0h300v36H0zM0 144h300v36H0z" fill="#07131f" opacity=".65"/><path d="M150 22l48 28v56l-48 52-48-52V50z" fill="%s"/><path d="M150 41l29 17v39l-29 31-29-31V58z" fill="%s" opacity=".32"/><text x="150" y="107" fill="#f5f1e9" font-family="Georgia,serif" font-size="44" font-weight="700" text-anchor="middle">%s</text></svg>`, pair[0], pair[1], pair[0], html.EscapeString(initials))
}

func (a *app) uploadAllianceFlag(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	p, err := a.alliancePermission(r.Context(), u.ID, aid)
	if err != nil || !p.Edit {
		problem(w, http.StatusForbidden, "Alliance leadership required.")
		return
	}
	data, mime, err := readFlagUpload(w, r)
	if err != nil {
		return
	}
	if _, err = a.db.ExecContext(r.Context(), `UPDATE alliances SET flag_image=?,flag_mime=?,flag_updated_at=NOW() WHERE id=?`, data, mime, aid); err != nil {
		problem(w, http.StatusInternalServerError, "Could not save this Alliance flag.")
		return
	}
	write(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *app) allianceFlag(w http.ResponseWriter, r *http.Request, _ user) {
	var name, mime string
	var data []byte
	if err := a.db.QueryRowContext(r.Context(), `SELECT name,COALESCE(flag_image,''),COALESCE(flag_mime,'') FROM alliances WHERE id=?`, r.PathValue("id")).Scan(&name, &data, &mime); err != nil {
		problem(w, http.StatusNotFound, "Alliance not found.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if len(data) > 0 && (mime == "image/png" || mime == "image/jpeg") {
		w.Header().Set("Content-Type", mime)
		_, _ = w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	_, _ = w.Write([]byte(defaultAllianceFlagSVG(name)))
}

func readFlagUpload(w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxNationFlagBytes+1024)
	if err := r.ParseMultipartForm(maxNationFlagBytes); err != nil {
		problem(w, http.StatusBadRequest, "Choose a PNG or JPEG flag under 2 MB.")
		return nil, "", err
	}
	file, _, err := r.FormFile("flag")
	if err != nil {
		problem(w, http.StatusBadRequest, "Choose a flag image to upload.")
		return nil, "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxNationFlagBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxNationFlagBytes {
		problem(w, http.StatusBadRequest, "Flag images must be no larger than 2 MB.")
		return nil, "", fmt.Errorf("invalid image size")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		problem(w, http.StatusBadRequest, "Flags must be valid PNG or JPEG images.")
		return nil, "", fmt.Errorf("invalid image format")
	}
	if config.Width < 120 || config.Height < 80 || config.Width > 2400 || config.Height > 2400 {
		problem(w, http.StatusBadRequest, "Flags must be between 120 and 2,400 pixels on each side.")
		return nil, "", fmt.Errorf("invalid image dimensions")
	}
	ratio := float64(config.Width) / float64(config.Height)
	if ratio < 1.2 || ratio > 2.5 {
		problem(w, http.StatusBadRequest, "Use a flag-shaped image between 1.2:1 and 2.5:1.")
		return nil, "", fmt.Errorf("invalid image ratio")
	}
	if format == "jpeg" {
		return data, "image/jpeg", nil
	}
	return data, "image/png", nil
}

func defaultAllianceFlagSVG(name string) string {
	digest := sha256.Sum256([]byte("alliance:" + strings.ToLower(strings.TrimSpace(name))))
	colors := [][2]string{{"#162e47", "#d2aa68"}, {"#303855", "#d1d6df"}, {"#263f3b", "#dec479"}, {"#4a303b", "#e7ded0"}, {"#243a4a", "#b4c5c9"}}
	pair := colors[int(digest[0])%len(colors)]
	initials := "DA"
	words := strings.Fields(name)
	if len(words) > 0 {
		initials = strings.ToUpper(string([]rune(words[0])[0]))
	}
	if len(words) > 1 {
		initials += strings.ToUpper(string([]rune(words[len(words)-1])[0]))
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 300 180" role="img" aria-label="Diplomatia Alliance flag"><rect width="300" height="180" fill="%s"/><path d="M0 0h300v28H0zM0 152h300v28H0z" fill="#07131f" opacity=".65"/><circle cx="150" cy="90" r="51" fill="%s"/><circle cx="150" cy="90" r="38" fill="%s" opacity=".32"/><path d="M150 34l9 25 27 1-21 17 7 27-22-15-22 15 7-27-21-17 27-1z" fill="none" stroke="#f5f1e9" stroke-width="3" opacity=".85"/><text x="150" y="105" fill="#f5f1e9" font-family="Georgia,serif" font-size="32" font-weight="700" text-anchor="middle">%s</text></svg>`, pair[0], pair[1], pair[0], html.EscapeString(initials))
}
