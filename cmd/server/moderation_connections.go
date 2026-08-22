package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"
)

const connectionObservationRetentionDays = 180

func requestClientIP(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if raw == "" {
		raw = strings.TrimSpace(r.RemoteAddr)
		if host, _, err := net.SplitHostPort(raw); err == nil {
			raw = host
		}
	}
	ip := net.ParseIP(strings.Trim(raw, "[]"))
	if ip == nil {
		return ""
	}
	return ip.String()
}

func connectionToken(secret []byte, ip string) string {
	if len(secret) == 0 || ip == "" {
		return ""
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("diplomatia:connection:v1:" + ip))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *app) recordConnectionObservation(r *http.Request, userID string) {
	token := connectionToken(a.connectionSecret, requestClientIP(r))
	if token == "" {
		return
	}
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO connection_observations(id,user_id,connection_token) VALUES(?,?,?)
		ON DUPLICATE KEY UPDATE last_seen_at=UTC_TIMESTAMP(6),login_count=login_count+1`, uuid(), userID, token)
	_, _ = a.db.ExecContext(r.Context(), `DELETE FROM connection_observations WHERE last_seen_at<DATE_SUB(UTC_TIMESTAMP(),INTERVAL 180 DAY)`)
}

func publicConnectionID(secret []byte, token string) string {
	if len(secret) == 0 || token == "" {
		return ""
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("diplomatia:public-connection:v1:" + token))
	return hex.EncodeToString(mac.Sum(nil))[:20]
}

func (a *app) nationConnections(w http.ResponseWriter, r *http.Request, _ user) {
	targetNationID := strings.TrimSpace(r.PathValue("id"))
	var targetUserID, targetNationName string
	if err := a.db.QueryRowContext(r.Context(), `SELECT owner_id,name FROM nations WHERE id=?`, targetNationID).Scan(&targetUserID, &targetNationName); err != nil {
		problem(w, http.StatusNotFound, "Nation not found.")
		return
	}
	var latestToken string
	_ = a.db.QueryRowContext(r.Context(), `SELECT connection_token FROM connection_observations WHERE user_id=? AND last_seen_at>=DATE_SUB(UTC_TIMESTAMP(),INTERVAL 180 DAY) ORDER BY last_seen_at DESC LIMIT 1`, targetUserID).Scan(&latestToken)
	cutoff := time.Now().UTC().AddDate(0, 0, -connectionObservationRetentionDays)
	rows, err := a.db.QueryContext(r.Context(), `SELECT n.id,n.name,n.leader_name,n.user_type,n.created_at,COALESCE(a.id,''),COALESCE(a.name,''),
		(SELECT MAX(s.last_action_at) FROM sessions s WHERE s.user_id=n.owner_id),MAX(LEAST(source.last_seen_at,related.last_seen_at))
		FROM connection_observations source
		JOIN connection_observations related ON related.connection_token=source.connection_token AND related.user_id<>source.user_id
		JOIN nations n ON n.owner_id=related.user_id
		LEFT JOIN alliance_members am ON am.nation_id=n.id LEFT JOIN alliances a ON a.id=am.alliance_id
		WHERE source.user_id=? AND source.last_seen_at>=? AND related.last_seen_at>=?
		GROUP BY n.id,n.name,n.leader_name,n.user_type,n.created_at,n.owner_id,a.id,a.name
		ORDER BY MAX(LEAST(source.last_seen_at,related.last_seen_at)) DESC,n.name`, targetUserID, cutoff, cutoff)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Potential connections are unavailable.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var nationID, nationName, leaderName, userType, allianceID, allianceName string
		var founded time.Time
		var lastActive sql.NullTime
		var lastMatched time.Time
		if rows.Scan(&nationID, &nationName, &leaderName, &userType, &founded, &allianceID, &allianceName, &lastActive, &lastMatched) == nil {
			var active any
			if lastActive.Valid {
				active = lastActive.Time
			}
			items = append(items, map[string]any{"nationID": nationID, "nationName": nationName, "leaderName": leaderName, "userType": userType, "allianceID": allianceID, "allianceName": allianceName, "foundedAt": founded, "lastActiveAt": active})
		}
	}
	write(w, http.StatusOK, map[string]any{"nationID": targetNationID, "nationName": targetNationName, "connectionID": publicConnectionID(a.connectionSecret, latestToken), "items": items})
}
