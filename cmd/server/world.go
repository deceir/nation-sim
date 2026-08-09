package main

import "net/http"

func (a *app) worldStats(w http.ResponseWriter, r *http.Request) {
	var nations, cities, openOrders, active, active24, alliances, totalTrades, completedTrades, activeShipments, delayedTrades int64
	var population int64
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(population),0) FROM nations`).Scan(&nations, &population); err != nil {
		problem(w, 500, "World statistics unavailable.")
		return
	}
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM cities`).Scan(&cities)
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM market_orders WHERE status='open'`).Scan(&openOrders)
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(DISTINCT user_id) FROM sessions WHERE last_action_at>=DATE_SUB(NOW(),INTERVAL 5 MINUTE)`).Scan(&active)
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(DISTINCT user_id) FROM sessions WHERE last_action_at>=DATE_SUB(NOW(),INTERVAL 24 HOUR)`).Scan(&active24)
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliances`).Scan(&alliances)
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(status='delivered'),0),COALESCE(SUM(status IN('in_transit','delayed')),0),COALESCE(SUM(delay_count>0),0) FROM trade_shipments`).Scan(&totalTrades, &completedTrades, &activeShipments, &delayedTrades)
	write(w, 200, map[string]any{"nations": nations, "cities": cities, "population": population, "alliances": alliances, "openMarketOrders": openOrders, "activePlayers": active, "activePlayers24Hours": active24, "totalTrades": totalTrades, "completedTrades": completedTrades, "activeShipments": activeShipments, "delayedTrades": delayedTrades})
}
