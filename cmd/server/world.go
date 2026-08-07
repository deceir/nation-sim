package main

import "net/http"

func (a *app) worldStats(w http.ResponseWriter, r *http.Request, _ user) {
	var nations, cities, openOrders, active int64
	var population int64
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(population),0) FROM nations`).Scan(&nations, &population); err != nil {
		problem(w, 500, "World statistics unavailable.")
		return
	}
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM cities`).Scan(&cities)
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM market_orders WHERE status='open'`).Scan(&openOrders)
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(DISTINCT user_id) FROM sessions WHERE last_action_at>=DATE_SUB(NOW(),INTERVAL 5 MINUTE)`).Scan(&active)
	write(w, 200, map[string]any{"nations": nations, "cities": cities, "population": population, "openMarketOrders": openOrders, "activePlayers": active})
}
