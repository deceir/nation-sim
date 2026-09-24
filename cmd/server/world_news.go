package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

const worldNewsInterval = 3 * time.Hour

type worldNewsMetrics struct {
	Population int64
	Education  float64
	Employment float64
	Disease    float64
	Crime      float64
	Prices     map[string]float64
}

type worldNewsCandidate struct {
	Category, Headline, Summary, Subject, Fingerprint, Link string
	Importance                                              int
}

func worldNewsSlot(at time.Time) time.Time {
	return at.UTC().Truncate(worldNewsInterval)
}

func newsFingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func newsVariant(key string, count int) int {
	if count <= 1 {
		return 0
	}
	sum := sha256.Sum256([]byte(key))
	return int(sum[0]) % count
}

func (a *app) currentWorldNewsMetrics(ctx context.Context, slot time.Time) (worldNewsMetrics, error) {
	metrics := worldNewsMetrics{Prices: map[string]float64{}}
	if err := a.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(population),0),COALESCE(AVG(education),0),COALESCE(AVG(employment_rate),0) FROM nations`).Scan(&metrics.Population, &metrics.Education, &metrics.Employment); err != nil {
		return metrics, err
	}
	if err := a.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(disease_rate*local_population)/NULLIF(SUM(local_population),0),0),COALESCE(SUM(crime_rate*local_population)/NULLIF(SUM(local_population),0),0) FROM cities`).Scan(&metrics.Disease, &metrics.Crime); err != nil {
		return metrics, err
	}
	rows, err := a.db.QueryContext(ctx, `SELECT resource,AVG(unit_price) FROM trade_shipments WHERE departed_at>=DATE_SUB(?,INTERVAL 24 HOUR) AND departed_at<? GROUP BY resource HAVING COUNT(*)>=2`, slot, slot)
	if err != nil {
		return metrics, err
	}
	defer rows.Close()
	for rows.Next() {
		var resource string
		var price float64
		if err = rows.Scan(&resource, &price); err != nil {
			return metrics, err
		}
		metrics.Prices[resource] = price
	}
	return metrics, rows.Err()
}

func (a *app) previousWorldNewsMetrics(ctx context.Context, slot time.Time) (worldNewsMetrics, bool) {
	var metrics worldNewsMetrics
	var raw []byte
	err := a.db.QueryRowContext(ctx, `SELECT population,education,employment,disease,crime,market_prices FROM world_news_snapshots WHERE slot_at<? ORDER BY slot_at DESC LIMIT 1`, slot).Scan(&metrics.Population, &metrics.Education, &metrics.Employment, &metrics.Disease, &metrics.Crime, &raw)
	if err != nil {
		return metrics, false
	}
	metrics.Prices = map[string]float64{}
	if json.Unmarshal(raw, &metrics.Prices) != nil {
		metrics.Prices = map[string]float64{}
	}
	return metrics, true
}

func (a *app) ensureWorldNews(ctx context.Context, at time.Time) error {
	slot := worldNewsSlot(at)
	current, err := a.currentWorldNewsMetrics(ctx, slot)
	if err != nil {
		return err
	}
	prices, _ := json.Marshal(current.Prices)
	result, err := a.db.ExecContext(ctx, `INSERT IGNORE INTO world_news_snapshots(slot_at,population,education,employment,disease,crime,market_prices) VALUES(?,?,?,?,?,?,?)`, slot, current.Population, current.Education, current.Employment, current.Disease, current.Crime, prices)
	if err != nil {
		return err
	}
	claimed, _ := result.RowsAffected()
	if claimed == 0 {
		return nil
	}
	previous, hasPrevious := a.previousWorldNewsMetrics(ctx, slot)
	candidates := make([]worldNewsCandidate, 0, 10)
	candidates = append(candidates, a.warNewsCandidates(ctx, slot)...)
	if hasPrevious {
		candidates = append(candidates, marketNewsCandidates(current, previous, slot)...)
		if metric := metricNewsCandidate(current, previous, slot); metric != nil {
			candidates = append(candidates, *metric)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Importance > candidates[j].Importance })
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	for _, item := range candidates {
		_, err = a.db.ExecContext(ctx, `INSERT IGNORE INTO world_news_items(id,slot_at,category,headline,summary,subject_key,fingerprint,importance,link_path,expires_at) VALUES(?,?,?,?,?,?,?,?,?,DATE_ADD(?,INTERVAL 7 DAY))`, uuid(), slot, item.Category, item.Headline, item.Summary, item.Subject, newsFingerprint(item.Fingerprint), item.Importance, nullString(item.Link), slot)
		if err != nil {
			return err
		}
	}
	_, _ = a.db.ExecContext(ctx, `DELETE FROM world_news_items WHERE expires_at<UTC_TIMESTAMP()`)
	return nil
}

func (a *app) warNewsCandidates(ctx context.Context, slot time.Time) []worldNewsCandidate {
	items := []worldNewsCandidate{}
	windowStart, windowEnd := slot.Add(-worldNewsInterval), slot
	rows, err := a.db.QueryContext(ctx, `SELECT c.id,an.name,an.continent,dn.name,dn.continent,w.objective,w.stage,w.rounds_resolved FROM conflicts c JOIN wars w ON w.conflict_id=c.id JOIN nations an ON an.id=c.attacker_id JOIN nations dn ON dn.id=c.defender_id WHERE c.kind='war' AND c.declared_at>=? AND c.declared_at<? ORDER BY c.declared_at DESC LIMIT 2`, windowStart, windowEnd)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, attacker, attackerContinent, defender, defenderContinent, objective, stage string
			var rounds int
			if rows.Scan(&id, &attacker, &attackerContinent, &defender, &defenderContinent, &objective, &stage, &rounds) != nil {
				continue
			}
			headlines := []string{fmt.Sprintf("War erupts between %s and %s", attacker, defender), fmt.Sprintf("%s opens hostilities against %s", attacker, defender), fmt.Sprintf("Armed conflict breaks out: %s–%s", attacker, defender)}
			summaries := []string{fmt.Sprintf("%s has declared war on %s. Mobilization is under way across %s and %s.", attacker, defender, attackerContinent, defenderContinent), fmt.Sprintf("Forces belonging to %s and %s are entering a new conflict spanning %s and %s.", attacker, defender, attackerContinent, defenderContinent), fmt.Sprintf("A new war now pits %s against %s, with both governments preparing their armed forces.", attacker, defender)}
			choice := newsVariant(id, len(headlines))
			items = append(items, worldNewsCandidate{"war", headlines[choice], summaries[choice], "war:" + id, "war-declared:" + id, "/conflict/" + id, 100})
		}
	}
	ended, err := a.db.QueryContext(ctx, `SELECT c.id,an.name,dn.name,COALESCE(wn.name,''),COALESCE(w.outcome,'concluded') FROM conflicts c JOIN wars w ON w.conflict_id=c.id JOIN nations an ON an.id=c.attacker_id JOIN nations dn ON dn.id=c.defender_id LEFT JOIN nations wn ON wn.id=w.winner_nation_id WHERE w.ended_at>=? AND w.ended_at<? ORDER BY w.ended_at DESC LIMIT 2`, windowStart, windowEnd)
	if err == nil {
		defer ended.Close()
		for ended.Next() {
			var id, attacker, defender, winner, outcome string
			if ended.Scan(&id, &attacker, &defender, &winner, &outcome) != nil {
				continue
			}
			summary := fmt.Sprintf("The war between %s and %s has concluded.", attacker, defender)
			if winner != "" {
				summary = fmt.Sprintf("%s has emerged victorious as the war between %s and %s comes to an end.", winner, attacker, defender)
			}
			items = append(items, worldNewsCandidate{"war", fmt.Sprintf("War concludes between %s and %s", attacker, defender), summary, "war:" + id, "war-ended:" + id + ":" + outcome, "/conflict/" + id, 95})
		}
	}
	if len(items) == 0 {
		var id, attacker, defender string
		var rounds int
		if a.db.QueryRowContext(ctx, `SELECT c.id,an.name,dn.name,w.rounds_resolved FROM conflicts c JOIN wars w ON w.conflict_id=c.id JOIN nations an ON an.id=c.attacker_id JOIN nations dn ON dn.id=c.defender_id WHERE w.stage<>'ended' ORDER BY w.rounds_resolved DESC,c.declared_at ASC LIMIT 1`).Scan(&id, &attacker, &defender, &rounds) == nil && rounds > 0 {
			date := slot.Format("2006-01-02")
			headlines := []string{fmt.Sprintf("Fighting continues between %s and %s", attacker, defender), fmt.Sprintf("%s–%s conflict remains active", attacker, defender)}
			choice := newsVariant(id+date, len(headlines))
			items = append(items, worldNewsCandidate{"war", headlines[choice], fmt.Sprintf("The campaign has reached round %d as both nations continue operations on active fronts.", rounds), "war:" + id, "war-ongoing:" + id + ":" + date, "/conflict/" + id, 65})
		}
	}
	return items
}

func marketNewsCandidates(current, previous worldNewsMetrics, slot time.Time) []worldNewsCandidate {
	items := []worldNewsCandidate{}
	for resource, price := range current.Prices {
		old := previous.Prices[resource]
		if old <= 0 || price <= 0 {
			continue
		}
		change := (price/old - 1) * 100
		if math.Abs(change) < 8 {
			continue
		}
		direction, verb := "up", "climb"
		if change < 0 {
			direction, verb = "down", "fall"
		}
		name := commodityName(resource)
		headlines := []string{fmt.Sprintf("%s prices %s in global trade", name, verb), fmt.Sprintf("Market watch: %s moves %s", name, direction)}
		choice := newsVariant(resource+slot.Format("2006-01-02"), len(headlines))
		items = append(items, worldNewsCandidate{"market", headlines[choice], fmt.Sprintf("The average completed-trade price for %s has moved %s %.1f%% against the previous reporting window.", name, direction, math.Abs(change)), "market:" + resource, "market:" + resource + ":" + direction + ":" + slot.Format("2006-01-02"), "/market/public", 55 + int(math.Min(25, math.Abs(change)))})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Importance > items[j].Importance })
	if len(items) > 2 {
		items = items[:2]
	}
	return items
}

func metricNewsCandidate(current, previous worldNewsMetrics, slot time.Time) *worldNewsCandidate {
	type movement struct {
		name, direction   string
		change, threshold float64
		positive          bool
	}
	movements := []movement{
		{"global education", direction(current.Education - previous.Education), current.Education - previous.Education, .15, true},
		{"world employment", direction(current.Employment - previous.Employment), current.Employment - previous.Employment, .15, true},
		{"global disease pressure", direction(current.Disease - previous.Disease), (current.Disease - previous.Disease) * 100, .08, false},
		{"world crime rates", direction(current.Crime - previous.Crime), (current.Crime - previous.Crime) * 100, .08, false},
	}
	sort.SliceStable(movements, func(i, j int) bool {
		return math.Abs(movements[i].change)/movements[i].threshold > math.Abs(movements[j].change)/movements[j].threshold
	})
	for _, movement := range movements {
		if math.Abs(movement.change) < movement.threshold {
			continue
		}
		verb := "edges higher"
		if movement.change < 0 {
			verb = "moves lower"
		}
		return &worldNewsCandidate{"world", strings.ToUpper(movement.name[:1]) + movement.name[1:] + " " + verb, fmt.Sprintf("The latest worldwide reading moved %s by %.2f percentage points during the reporting period.", movement.direction, math.Abs(movement.change)), "metric:" + movement.name, "metric:" + movement.name + ":" + movement.direction + ":" + slot.Format("2006-01-02"), "/world-data", 45}
	}
	if previous.Population > 0 {
		change := float64(current.Population-previous.Population) / float64(previous.Population) * 100
		if math.Abs(change) >= .1 {
			return &worldNewsCandidate{"world", "World population continues to shift", fmt.Sprintf("Diplomatia's recorded population has moved %s %.2f%% during the reporting period.", direction(change), math.Abs(change)), "metric:population", "metric:population:" + direction(change) + ":" + slot.Format("2006-01-02"), "/world-data", 40}
		}
	}
	return nil
}

func direction(change float64) string {
	if change < 0 {
		return "down"
	}
	return "up"
}

func (a *app) worldNews(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,category,headline,summary,COALESCE(link_path,''),created_at FROM world_news_items WHERE expires_at>UTC_TIMESTAMP() ORDER BY created_at DESC,importance DESC LIMIT 20`)
	if err != nil {
		problem(w, http.StatusInternalServerError, "World news is temporarily unavailable.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, category, headline, summary, link string
		var created time.Time
		if rows.Scan(&id, &category, &headline, &summary, &link, &created) == nil {
			items = append(items, map[string]any{"id": id, "category": category, "headline": headline, "summary": summary, "link": link, "createdAt": created})
		}
	}
	write(w, http.StatusOK, items)
}
