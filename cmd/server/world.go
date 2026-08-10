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
	military := map[string]int64{"soldiers": 0, "tanks": 0, "jets": 0, "drones": 0}
	rows, militaryErr := a.db.QueryContext(r.Context(), `SELECT unit_type,CAST(SUM(quantity) AS SIGNED) FROM (
		SELECT unit_type,quantity FROM military_inventory WHERE unit_type IN('soldiers','tanks','jets','drones')
		UNION ALL
		SELECT resource,escrow_goods FROM market_orders WHERE side='sell' AND status IN('open','pending') AND resource IN('tanks','jets','drones')
		UNION ALL
		SELECT resource,quantity FROM trade_shipments WHERE status IN('in_transit','delayed') AND resource IN('tanks','jets','drones')
	) world_military GROUP BY unit_type`)
	if militaryErr == nil {
		defer rows.Close()
		for rows.Next() {
			var unitType string
			var quantity int64
			if rows.Scan(&unitType, &quantity) == nil {
				military[unitType] = quantity
			}
		}
	}
	write(w, 200, map[string]any{"nations": nations, "cities": cities, "population": population, "alliances": alliances, "openMarketOrders": openOrders, "activePlayers": active, "activePlayers24Hours": active24, "totalTrades": totalTrades, "completedTrades": completedTrades, "activeShipments": activeShipments, "delayedTrades": delayedTrades, "military": military})
}
