package main

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

type worldResourcePoint struct {
	RecordedAt time.Time `json:"recordedAt"`
	Total      float64   `json:"total"`
}

// captureWorldResourceSnapshot records goods wherever they currently exist.
// Escrow and in-transit goods remain part of the world supply until consumed.
func (a *app) captureWorldResourceSnapshot(ctx context.Context, recordedAt time.Time) error {
	totals := make(map[string]float64, len(strategicCommodities))
	rows, err := a.db.QueryContext(ctx, `
		SELECT commodity,SUM(amount) FROM (
			SELECT commodity,amount FROM nation_stockpiles
			UNION ALL SELECT commodity,amount FROM alliance_stockpiles
			UNION ALL SELECT resource,escrow_goods FROM market_orders WHERE side='sell' AND status IN ('open','pending')
			UNION ALL SELECT resource,quantity FROM trade_shipments WHERE status IN ('in_transit','delayed')
		) world_goods GROUP BY commodity`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var resource string
		var total sql.NullFloat64
		if err = rows.Scan(&resource, &total); err != nil {
			return err
		}
		totals[resource] = total.Float64
	}
	if err = rows.Err(); err != nil {
		return err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, resource := range strategicCommodities {
		if _, err = tx.ExecContext(ctx, `INSERT INTO world_resource_snapshots(recorded_at,resource,total) VALUES(?,?,?) ON DUPLICATE KEY UPDATE total=VALUES(total)`, recordedAt.UTC(), resource, totals[resource]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (a *app) worldResourceHistory(w http.ResponseWriter, r *http.Request, _ user) {
	resource := r.URL.Query().Get("resource")
	if resource == "" {
		resource = strategicCommodities[0]
	}
	valid := false
	for _, candidate := range strategicCommodities {
		valid = valid || resource == candidate
	}
	if !valid {
		problem(w, http.StatusBadRequest, "Select a valid resource.")
		return
	}
	rangeKey := r.URL.Query().Get("range")
	if rangeKey == "" {
		rangeKey = "30d"
	}
	durations := map[string]time.Duration{"7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour, "90d": 90 * 24 * time.Hour, "1y": 365 * 24 * time.Hour}
	query := `SELECT recorded_at,total FROM world_resource_snapshots WHERE resource=?`
	args := []any{resource}
	if duration, ok := durations[rangeKey]; ok {
		query += ` AND recorded_at>=?`
		args = append(args, time.Now().UTC().Add(-duration))
	} else if rangeKey != "all" {
		problem(w, http.StatusBadRequest, "Select a valid time range.")
		return
	}
	query += ` ORDER BY recorded_at ASC`
	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		problem(w, http.StatusInternalServerError, "World resource history is unavailable.")
		return
	}
	defer rows.Close()
	points := make([]worldResourcePoint, 0)
	for rows.Next() {
		var point worldResourcePoint
		if err = rows.Scan(&point.RecordedAt, &point.Total); err != nil {
			problem(w, http.StatusInternalServerError, "World resource history is unavailable.")
			return
		}
		points = append(points, point)
	}
	latest := map[string]float64{}
	latestRows, latestErr := a.db.QueryContext(r.Context(), `SELECT s.resource,s.total FROM world_resource_snapshots s JOIN (SELECT resource,MAX(recorded_at) recorded_at FROM world_resource_snapshots GROUP BY resource) latest ON latest.resource=s.resource AND latest.recorded_at=s.recorded_at`)
	if latestErr == nil {
		defer latestRows.Close()
		for latestRows.Next() {
			var key string
			var total float64
			if latestRows.Scan(&key, &total) == nil {
				latest[key] = total
			}
		}
	}
	write(w, http.StatusOK, map[string]any{"resource": resource, "range": rangeKey, "resources": strategicCommodities, "points": points, "latest": latest})
}
