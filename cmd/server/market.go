package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	tradeRegionalTurns     = 3.0
	tradeDistanceTurnScale = 30.0
	tradeDistanceTurnPower = 3.0
	tradeRegionalFeeRate   = .0075
	tradeDistanceFeeScale  = .03
	tradeDistanceFeePower  = 1.30
)

var tradeCommodities = map[string]bool{
	"foodstuffs": true, "timber": true, "fibers": true, "basic_metals": true, "energy": true,
	"strategic_minerals": true, "textiles": true, "processed_foods": true, "construction_materials": true,
	"basic_goods": true, "consumer_goods": true, "military_equipment": true, "luxury_goods": true,
	"tanks": true, "ships": true, "jets": true, "drones": true,
}

var marketCommodities = append(append([]string{}, strategicCommodities...), "tanks", "ships", "jets", "drones")

// The six specified regions use the design table verbatim. Oceania is an extension because it is a playable Diplomatia continent.
var tradeDistances = map[string]map[string]float64{
	"Africa":        {"Africa": 1, "Asia": 1.35, "Europe": 1.25, "North America": 1.70, "South America": 1.55, "Antarctica": 1.80, "Oceania": 1.65},
	"Asia":          {"Africa": 1.35, "Asia": 1, "Europe": 1.30, "North America": 1.65, "South America": 1.90, "Antarctica": 1.85, "Oceania": 1.35},
	"Europe":        {"Africa": 1.25, "Asia": 1.30, "Europe": 1, "North America": 1.55, "South America": 1.75, "Antarctica": 1.90, "Oceania": 1.75},
	"North America": {"Africa": 1.70, "Asia": 1.65, "Europe": 1.55, "North America": 1, "South America": 1.40, "Antarctica": 1.95, "Oceania": 1.60},
	"South America": {"Africa": 1.55, "Asia": 1.90, "Europe": 1.75, "North America": 1.40, "South America": 1, "Antarctica": 1.70, "Oceania": 1.75},
	"Antarctica":    {"Africa": 1.80, "Asia": 1.85, "Europe": 1.90, "North America": 1.95, "South America": 1.70, "Antarctica": 1, "Oceania": 1.55},
	"Oceania":       {"Africa": 1.65, "Asia": 1.35, "Europe": 1.75, "North America": 1.60, "South America": 1.75, "Antarctica": 1.55, "Oceania": 1},
}

type tradeNation struct {
	ID, Name, Continent string
	Lat, Lng            float64
}

func locationFallback(continent string) (float64, float64) {
	centers := map[string][2]float64{"Africa": {5, 20}, "Asia": {34, 100}, "Europe": {50, 15}, "North America": {40, -100}, "South America": {-15, -60}, "Oceania": {-25, 135}, "Antarctica": {-75, 0}}
	p := centers[continent]
	return p[0], p[1]
}

func scanTradeNation(row *sql.Row) (tradeNation, error) {
	var n tradeNation
	var lat, lng sql.NullFloat64
	err := row.Scan(&n.ID, &n.Name, &n.Continent, &lat, &lng)
	if lat.Valid && lng.Valid {
		n.Lat, n.Lng = lat.Float64, lng.Float64
	} else {
		n.Lat, n.Lng = locationFallback(n.Continent)
	}
	return n, err
}

func distanceModifier(from, to string) float64 {
	if row, ok := tradeDistances[from]; ok && row[to] > 0 {
		return row[to]
	}
	return 1.75
}

func shipmentTerms(from, to string, quantity float64, value int64) (distance float64, turns int, fee int64, risk float64) {
	distance = distanceModifier(from, to)
	size := 1.0
	if quantity > 10000 {
		size = 1.3
	} else if quantity > 1000 {
		size = 1.15
	}
	distanceDelta := math.Max(0, distance-1)
	baseTurns := tradeRegionalTurns + tradeDistanceTurnScale*math.Pow(distanceDelta, tradeDistanceTurnPower)
	turns = int(math.Ceil(baseTurns * size))
	feeRate := tradeRegionalFeeRate + tradeDistanceFeeScale*math.Pow(distanceDelta, tradeDistanceFeePower)
	fee = int64(math.Ceil(float64(value) * feeRate))
	risk = .5 + (distance-1)*4
	return
}

func tradeValue(quantity float64, unitPrice int64) int64 {
	return int64(math.Ceil(quantity * float64(unitPrice)))
}

// Notification copy is intentionally less granular than settlement math.
// Always round upward so the displayed tenth never understates the shipment.
func marketNotificationQuantity(quantity float64) string {
	return fmt.Sprintf("%.1f", math.Ceil(quantity*10-0.000000001)/10)
}

func (a *app) market(w http.ResponseWriter, r *http.Request, u user) {
	me, err := scanTradeNation(a.db.QueryRowContext(r.Context(), `SELECT id,name,continent,location_lat,location_lng FROM nations WHERE owner_id=?`, u.ID))
	if err != nil {
		problem(w, 409, "Create a nation first.")
		return
	}
	offers := []map[string]any{}
	rows, err := a.db.QueryContext(r.Context(), `SELECT o.id,o.nation_id,n.name,n.continent,o.side,o.resource,o.remaining,o.unit_price,o.channel,COALESCE(t.name,''),COALESCE(t.continent,''),o.status,COALESCE(a.id,''),COALESCE(a.name,'') FROM market_orders o JOIN nations n ON n.id=o.nation_id LEFT JOIN nations t ON t.id=o.target_nation_id LEFT JOIN alliance_members am ON am.nation_id=n.id LEFT JOIN alliances a ON a.id=am.alliance_id WHERE o.status='open' OR (o.status='pending' AND (o.nation_id=? OR o.target_nation_id=?)) ORDER BY o.created_at DESC`, me.ID, me.ID)
	if err != nil {
		problem(w, 500, "Market unavailable.")
		return
	}
	for rows.Next() {
		var id, makerID, nation, continent, side, resource, channel, target, targetContinent, status, allianceID, allianceName string
		var quantity float64
		var price int64
		if err = rows.Scan(&id, &makerID, &nation, &continent, &side, &resource, &quantity, &price, &channel, &target, &targetContinent, &status, &allianceID, &allianceName); err != nil {
			rows.Close()
			problem(w, 500, "A market offer could not be read. Run the latest database migration and try again.")
			return
		}
		channel = strings.ToLower(strings.TrimSpace(channel))
		status = strings.ToLower(strings.TrimSpace(status))
		// Orders created before channels were introduced were all public orders.
		if channel == "" && status == "open" {
			channel = "public"
		}
		value := tradeValue(quantity, price)
		counterpartContinent := me.Continent
		if channel == "private" && makerID == me.ID {
			counterpartContinent = targetContinent
		}
		distance, turns, fee, risk := shipmentTerms(continent, counterpartContinent, quantity, value)
		offers = append(offers, map[string]any{"id": id, "makerID": makerID, "nation": nation, "continent": continent, "allianceID": allianceID, "allianceName": allianceName, "side": side, "resource": resource, "quantity": quantity, "unitPrice": price, "channel": channel, "targetNation": target, "status": status, "distanceModifier": distance, "estimatedTurns": turns, "estimatedFee": fee, "riskPercent": risk, "isMine": strings.TrimSpace(makerID) == strings.TrimSpace(me.ID)})
	}
	rows.Close()
	shipments, err := a.shipmentsForNation(r.Context(), me.ID)
	if err != nil {
		// A shipment migration or timestamp issue must not hide the independent
		// public order book. Surface the warning while keeping offers usable.
		shipments = []map[string]any{}
		write(w, 200, map[string]any{"marketVersion": 2, "nation": me, "offers": offers, "shipments": shipments, "commodities": marketCommodities, "warning": "Shipment tracking is temporarily unavailable."})
		return
	}
	write(w, 200, map[string]any{"marketVersion": 2, "nation": me, "offers": offers, "shipments": shipments, "commodities": marketCommodities})
}

func (a *app) placeOrder(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		Side, Resource, Channel, TargetNationID, TargetNationName string
		Quantity                                                  float64
		UnitPrice                                                 int64
	}
	if !decode(w, r, &in) {
		return
	}
	in.TargetNationName = strings.TrimSpace(in.TargetNationName)
	if (in.Side != "buy" && in.Side != "sell") || !tradeCommodities[in.Resource] || in.Quantity <= 0 || in.Quantity > 1e9 || in.UnitPrice < 1 || in.UnitPrice > 1e12 {
		problem(w, 400, "Invalid trade offer.")
		return
	}
	if isMilitaryEquipment(in.Resource) {
		if _, ok := militaryTradeQuantity(in.Quantity); !ok {
			problem(w, 400, "Tanks, Ships, Fighter Jets, and Drones must be traded in whole units.")
			return
		}
	}
	if in.Quantity > float64(math.MaxInt64)/float64(in.UnitPrice) {
		problem(w, 400, "The total trade value is too large.")
		return
	}
	if in.Channel == "" {
		in.Channel = "public"
	}
	if in.Channel != "public" && in.Channel != "private" {
		problem(w, 400, "Invalid trading channel.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not create offer.")
		return
	}
	defer tx.Rollback()
	var nid, nationName string
	if tx.QueryRowContext(r.Context(), `SELECT id,name FROM nations WHERE owner_id=? FOR UPDATE`, u.ID).Scan(&nid, &nationName) != nil {
		problem(w, 409, "Create a nation first.")
		return
	}
	if in.Channel == "private" {
		if in.TargetNationName != "" {
			if err = tx.QueryRowContext(r.Context(), `SELECT id FROM nations WHERE LOWER(name)=LOWER(?) AND id<>?`, in.TargetNationName, nid).Scan(&in.TargetNationID); err != nil {
				problem(w, 404, "No other nation exists with that exact name.")
				return
			}
		} else {
			var exists bool
			tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM nations WHERE id=? AND id<>?)`, in.TargetNationID, nid).Scan(&exists)
			if !exists {
				problem(w, 400, "Enter another nation's exact name for this direct trade.")
				return
			}
		}
		if in.TargetNationID == "" {
			problem(w, 400, "Enter another nation's exact name for this direct trade.")
			return
		}
	} else {
		in.TargetNationID = ""
	}
	value := tradeValue(in.Quantity, in.UnitPrice)
	escrowCash, escrowGoods := int64(0), float64(0)
	if in.Side == "sell" {
		if isMilitaryEquipment(in.Resource) {
			if e := removeMilitaryInventory(r.Context(), tx, nid, in.Resource, in.Quantity); e != nil {
				problem(w, 409, "Not enough of that military equipment to escrow this offer.")
				return
			}
		} else {
			result, e := tx.ExecContext(r.Context(), `UPDATE nation_stockpiles SET amount=amount-? WHERE nation_id=? AND commodity=? AND amount>=?`, in.Quantity, nid, in.Resource, in.Quantity)
			if e != nil || affected(result) != 1 {
				problem(w, 409, "Not enough of that commodity to escrow this offer.")
				return
			}
		}
		escrowGoods = in.Quantity
	} else {
		result, e := tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=? AND treasury>=?`, value, nid, value)
		if e != nil || affected(result) != 1 {
			problem(w, 409, "Not enough treasury to escrow this buy offer.")
			return
		}
		escrowCash = value
	}
	status := "open"
	var target any
	if in.Channel == "private" {
		status, target = "pending", in.TargetNationID
	}
	orderID := uuid()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO market_orders(id,nation_id,side,resource,quantity,remaining,unit_price,channel,target_nation_id,escrow_cash,escrow_goods,status) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, orderID, nid, in.Side, in.Resource, in.Quantity, in.Quantity, in.UnitPrice, in.Channel, target, escrowCash, escrowGoods, status); err != nil {
		problem(w, 500, "Could not publish offer.")
		return
	}
	var savedStatus string
	var savedCash int64
	var savedGoods float64
	if err = tx.QueryRowContext(r.Context(), `SELECT status,escrow_cash,escrow_goods FROM market_orders WHERE id=? FOR UPDATE`, orderID).Scan(&savedStatus, &savedCash, &savedGoods); err != nil || savedStatus != status || savedCash != escrowCash || math.Abs(savedGoods-escrowGoods) > 0.000001 {
		problem(w, 500, "The offer failed escrow verification and was not published.")
		return
	}
	if in.Channel == "private" {
		tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'market','Direct trade offered',?)`, uuid(), in.TargetNationID, fmt.Sprintf("%s sent you a direct %s offer for %s %s.", nationName, in.Side, marketNotificationQuantity(in.Quantity), commodityName(in.Resource)))
	} else {
		tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'market','Market offer published',?)`, uuid(), nid, fmt.Sprintf("Your public %s offer for %s %s is now open.", in.Side, marketNotificationQuantity(in.Quantity), commodityName(in.Resource)))
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Could not publish offer.")
		return
	}
	write(w, 201, map[string]any{"ok": true, "marketVersion": 2, "orderID": orderID, "status": status, "escrowCash": escrowCash, "escrowGoods": escrowGoods})
}

func affected(result sql.Result) int64 { n, _ := result.RowsAffected(); return n }

func (a *app) acceptMarketOrder(w http.ResponseWriter, r *http.Request, u user) {
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not accept trade.")
		return
	}
	defer tx.Rollback()
	var makerID, side, resource, channel, status string
	var targetID sql.NullString
	var quantity, escrowGoods float64
	var unitPrice, escrowCash int64
	err = tx.QueryRowContext(r.Context(), `SELECT nation_id,side,resource,channel,target_nation_id,quantity,unit_price,escrow_cash,escrow_goods,status FROM market_orders WHERE id=? FOR UPDATE`, r.PathValue("id")).Scan(&makerID, &side, &resource, &channel, &targetID, &quantity, &unitPrice, &escrowCash, &escrowGoods, &status)
	if err != nil || (status != "open" && status != "pending") {
		problem(w, 409, "That offer is no longer available.")
		return
	}
	taker, err := scanTradeNation(tx.QueryRowContext(r.Context(), `SELECT id,name,continent,location_lat,location_lng FROM nations WHERE owner_id=? FOR UPDATE`, u.ID))
	if err != nil || taker.ID == makerID || (channel == "private" && (!targetID.Valid || targetID.String != taker.ID)) {
		problem(w, 403, "You cannot accept this offer.")
		return
	}
	maker, err := scanTradeNation(tx.QueryRowContext(r.Context(), `SELECT id,name,continent,location_lat,location_lng FROM nations WHERE id=? FOR UPDATE`, makerID))
	if err != nil {
		problem(w, 404, "Offering nation no longer exists.")
		return
	}
	value := tradeValue(quantity, unitPrice)
	var seller, buyer tradeNation
	if side == "sell" {
		seller, buyer = maker, taker
	} else {
		seller, buyer = taker, maker
	}
	distance, turns, fee, risk := shipmentTerms(seller.Continent, buyer.Continent, quantity, value)
	var optimized int
	tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM national_long_term_projects WHERE nation_id=? AND project_type='logistical_optimization'`, buyer.ID).Scan(&optimized)
	if optimized > 0 {
		fee = int64(math.Ceil(float64(fee) * .85))
	}
	if side == "sell" {
		if err = ensureMilitaryPurchaseCapacity(r.Context(), tx, buyer.ID, resource, quantity); err != nil {
			problem(w, 409, err.Error()+".")
			return
		}
		result, e := tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=? AND treasury>=?`, value+fee, buyer.ID, value+fee)
		if e != nil || affected(result) != 1 {
			problem(w, 409, "Not enough treasury for the goods and shipping fee.")
			return
		}
	} else {
		if err = ensureMilitaryPurchaseCapacity(r.Context(), tx, buyer.ID, resource, quantity); err != nil {
			problem(w, 409, err.Error()+".")
			return
		}
		if isMilitaryEquipment(resource) {
			if err = removeMilitaryInventory(r.Context(), tx, seller.ID, resource, quantity); err != nil {
				problem(w, 409, "Not enough military equipment to fulfill this buy offer.")
				return
			}
		} else {
			result, e := tx.ExecContext(r.Context(), `UPDATE nation_stockpiles SET amount=amount-? WHERE nation_id=? AND commodity=? AND amount>=?`, quantity, seller.ID, resource, quantity)
			if e != nil || affected(result) != 1 {
				problem(w, 409, "Not enough goods to fulfill this buy offer.")
				return
			}
		}
		feeResult, feeErr := tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=? AND treasury>=?`, fee, buyer.ID, fee)
		if feeErr != nil || affected(feeResult) != 1 || escrowCash < value {
			problem(w, 409, "The buyer cannot cover the shipping fee.")
			return
		}
	}
	shipmentID, arrival := uuid(), time.Now().UTC().Add(time.Duration(turns)*time.Hour)
	_, err = tx.ExecContext(r.Context(), `INSERT INTO trade_shipments(id,order_id,seller_nation_id,buyer_nation_id,resource,quantity,unit_price,goods_value,shipping_fee,distance_modifier,risk_percent,turns_total,turns_remaining,origin_lat,origin_lng,destination_lat,destination_lng,estimated_arrival_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, shipmentID, r.PathValue("id"), seller.ID, buyer.ID, resource, quantity, unitPrice, value, fee, distance, risk, turns, turns, seller.Lat, seller.Lng, buyer.Lat, buyer.Lng, arrival)
	if err != nil {
		log.Printf("market shipment insert failed for order %s: %v", r.PathValue("id"), err)
		problem(w, 500, "Could not dispatch shipment.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE market_orders SET remaining=0,status='filled',escrow_goods=0,escrow_cash=0 WHERE id=?`, r.PathValue("id")); err != nil {
		problem(w, 500, "Could not close offer.")
		return
	}
	message := fmt.Sprintf("%s %s is now in transit from %s to %s. Estimated delivery: %d turns.", marketNotificationQuantity(quantity), commodityName(resource), seller.Name, buyer.Name, turns)
	for _, id := range []string{seller.ID, buyer.ID} {
		tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'market','Shipment dispatched',?)`, uuid(), id, message)
	}
	tx.ExecContext(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'trade_purchase',?,?)`, uuid(), buyer.ID, -(value + fee), fmt.Sprintf("Escrowed trade purchase and shipping fee for %.3f %s", quantity, commodityName(resource)))
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Could not accept trade.")
		return
	}
	write(w, 201, map[string]any{"ok": true, "shipmentID": shipmentID, "turns": turns, "shippingFee": fee})
}

func (a *app) cancelMarketOrder(w http.ResponseWriter, r *http.Request, u user) {
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not cancel offer.")
		return
	}
	defer tx.Rollback()
	var id, resource, status string
	var cash int64
	var goods float64
	err = tx.QueryRowContext(r.Context(), `SELECT o.nation_id,o.resource,o.status,o.escrow_cash,o.escrow_goods FROM market_orders o JOIN nations n ON n.id=o.nation_id WHERE o.id=? AND n.owner_id=? FOR UPDATE`, r.PathValue("id"), u.ID).Scan(&id, &resource, &status, &cash, &goods)
	if err != nil || (status != "open" && status != "pending") {
		problem(w, 409, "That offer cannot be cancelled.")
		return
	}
	if cash > 0 {
		tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury+? WHERE id=?`, cash, id)
	}
	if goods > 0 {
		if isMilitaryEquipment(resource) {
			if err = addMilitaryInventory(r.Context(), tx, id, resource, goods); err != nil {
				problem(w, 500, "Could not return military escrow.")
				return
			}
		} else {
			tx.ExecContext(r.Context(), `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, id, resource, goods)
		}
	}
	tx.ExecContext(r.Context(), `UPDATE market_orders SET status='cancelled',escrow_cash=0,escrow_goods=0 WHERE id=?`, r.PathValue("id"))
	if tx.Commit() != nil {
		problem(w, 500, "Could not cancel offer.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) rejectMarketOrder(w http.ResponseWriter, r *http.Request, u user) {
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not decline offer.")
		return
	}
	defer tx.Rollback()
	var makerID, targetID, resource, status string
	var cash int64
	var goods float64
	err = tx.QueryRowContext(r.Context(), `SELECT o.nation_id,o.target_nation_id,o.resource,o.status,o.escrow_cash,o.escrow_goods FROM market_orders o JOIN nations target ON target.id=o.target_nation_id WHERE o.id=? AND o.channel='private' AND target.owner_id=? FOR UPDATE`, r.PathValue("id"), u.ID).Scan(&makerID, &targetID, &resource, &status, &cash, &goods)
	if err != nil || status != "pending" {
		problem(w, 409, "That direct offer cannot be declined.")
		return
	}
	if cash > 0 {
		tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury+? WHERE id=?`, cash, makerID)
	}
	if goods > 0 {
		if isMilitaryEquipment(resource) {
			if err = addMilitaryInventory(r.Context(), tx, makerID, resource, goods); err != nil {
				problem(w, 500, "Could not return military escrow.")
				return
			}
		} else {
			tx.ExecContext(r.Context(), `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, makerID, resource, goods)
		}
	}
	tx.ExecContext(r.Context(), `UPDATE market_orders SET status='rejected',escrow_cash=0,escrow_goods=0 WHERE id=?`, r.PathValue("id"))
	tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'market','Direct trade declined','Your direct trade offer was declined and its escrow was returned.')`, uuid(), makerID)
	if tx.Commit() != nil {
		problem(w, 500, "Could not decline offer.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) shipmentsForNation(ctx context.Context, nationID string) ([]map[string]any, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT s.id,s.resource,s.quantity,s.unit_price,s.goods_value,s.shipping_fee,s.distance_modifier,s.risk_percent,s.turns_total,s.turns_remaining,s.delay_count,s.origin_lat,s.origin_lng,s.destination_lat,s.destination_lng,s.departed_at,s.estimated_arrival_at,s.delivered_at,s.status,seller.name,buyer.name,seller.continent,buyer.continent FROM trade_shipments s JOIN nations seller ON seller.id=s.seller_nation_id JOIN nations buyer ON buyer.id=s.buyer_nation_id WHERE s.seller_nation_id=? OR s.buyer_nation_id=? ORDER BY s.departed_at DESC LIMIT 100`, nationID, nationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, resource, status, seller, buyer, sellerContinent, buyerContinent string
		var quantity, distance, risk, oLat, oLng, dLat, dLng float64
		var price, value, fee int64
		var total, remaining, delays int
		var departed, eta time.Time
		var delivered *time.Time
		if rows.Scan(&id, &resource, &quantity, &price, &value, &fee, &distance, &risk, &total, &remaining, &delays, &oLat, &oLng, &dLat, &dLng, &departed, &eta, &delivered, &status, &seller, &buyer, &sellerContinent, &buyerContinent) != nil {
			continue
		}
		progress := math.Max(0, math.Min(1, float64(total-remaining)/float64(max(1, total))))
		if status == "delivered" {
			progress = 1
		}
		out = append(out, map[string]any{"id": id, "resource": resource, "quantity": quantity, "unitPrice": price, "goodsValue": value, "shippingFee": fee, "distanceModifier": distance, "riskPercent": risk, "turnsTotal": total, "turnsRemaining": remaining, "delayCount": delays, "originLat": oLat, "originLng": oLng, "destinationLat": dLat, "destinationLng": dLng, "departedAt": departed, "estimatedArrivalAt": eta, "deliveredAt": delivered, "status": status, "seller": seller, "buyer": buyer, "sellerContinent": sellerContinent, "buyerContinent": buyerContinent, "progress": progress})
	}
	return out, rows.Err()
}

func (a *app) shipmentDetail(w http.ResponseWriter, r *http.Request, u user) {
	var nid string
	if a.db.QueryRowContext(r.Context(), `SELECT id FROM nations WHERE owner_id=?`, u.ID).Scan(&nid) != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	items, err := a.shipmentsForNation(r.Context(), nid)
	if err != nil {
		problem(w, 500, "Shipment unavailable.")
		return
	}
	for _, item := range items {
		if item["id"] == r.PathValue("id") {
			write(w, 200, item)
			return
		}
	}
	problem(w, 404, "Shipment not found.")
}

func (a *app) nationTradeHistory(w http.ResponseWriter, r *http.Request, _ user) {
	nationID := r.PathValue("id")
	var nationName string
	if a.db.QueryRowContext(r.Context(), `SELECT name FROM nations WHERE id=?`, nationID).Scan(&nationName) != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT s.id,s.resource,s.quantity,s.unit_price,s.goods_value,s.shipping_fee,s.status,s.departed_at,s.estimated_arrival_at,s.delivered_at,CASE WHEN s.seller_nation_id=? THEN 'sale' ELSE 'purchase' END,CASE WHEN s.seller_nation_id=? THEN buyer.name ELSE seller.name END,CASE WHEN s.seller_nation_id=? THEN buyer.id ELSE seller.id END FROM trade_shipments s JOIN nations seller ON seller.id=s.seller_nation_id JOIN nations buyer ON buyer.id=s.buyer_nation_id WHERE s.seller_nation_id=? OR s.buyer_nation_id=? ORDER BY s.departed_at DESC LIMIT 200`, nationID, nationID, nationID, nationID, nationID)
	if err != nil {
		problem(w, 500, "Trade history unavailable.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, resource, status, direction, partner, partnerID string
		var quantity float64
		var unitPrice, goodsValue, shippingFee int64
		var departed, estimated time.Time
		var delivered *time.Time
		if err = rows.Scan(&id, &resource, &quantity, &unitPrice, &goodsValue, &shippingFee, &status, &departed, &estimated, &delivered, &direction, &partner, &partnerID); err != nil {
			problem(w, 500, "Trade history could not be read.")
			return
		}
		items = append(items, map[string]any{"id": id, "resource": resource, "quantity": quantity, "unitPrice": unitPrice, "goodsValue": goodsValue, "shippingFee": shippingFee, "status": status, "departedAt": departed, "estimatedArrivalAt": estimated, "deliveredAt": delivered, "direction": direction, "partner": partner, "partnerID": partnerID})
	}
	bankItems := []map[string]any{}
	bankRows, err := a.db.QueryContext(r.Context(), `SELECT t.id,COALESCE(t.batch_id,''),t.kind,t.resource,t.amount,t.created_at,a.id,a.name,COALESCE(actor.id,''),COALESCE(actor.name,'') FROM alliance_bank_transactions t JOIN alliances a ON a.id=t.alliance_id LEFT JOIN nations actor ON actor.id=t.actor_nation_id WHERE (t.kind='deposit' AND t.actor_nation_id=?) OR (t.kind IN('withdrawal','grant','balance_adjustment') AND t.recipient_nation_id=?) ORDER BY t.created_at DESC LIMIT 200`, nationID, nationID)
	if err != nil {
		problem(w, 500, "Alliance bank history unavailable.")
		return
	}
	defer bankRows.Close()
	for bankRows.Next() {
		var id, batchID, kind, resource, allianceID, allianceName, actorID, actorName string
		var amount float64
		var createdAt time.Time
		if err = bankRows.Scan(&id, &batchID, &kind, &resource, &amount, &createdAt, &allianceID, &allianceName, &actorID, &actorName); err != nil {
			problem(w, 500, "Alliance bank history could not be read.")
			return
		}
		bankItems = append(bankItems, map[string]any{"id": id, "batchID": batchID, "kind": kind, "resource": resource, "amount": amount, "createdAt": createdAt, "allianceID": allianceID, "allianceName": allianceName, "actorID": actorID, "actorName": actorName})
	}
	write(w, 200, map[string]any{"nationID": nationID, "nationName": nationName, "items": items, "bankItems": bankItems})
}

func shipmentDelayRoll(id string, turn time.Time) float64 {
	h := sha256.Sum256([]byte(id + turn.UTC().Format(time.RFC3339)))
	return float64(uint16(h[0])<<8|uint16(h[1])) / 65535 * 100
}

func (a *app) processTradeShipments(ctx context.Context, turn time.Time) {
	rows, err := a.db.QueryContext(ctx, `SELECT id FROM trade_shipments WHERE status IN('in_transit','delayed') ORDER BY departed_at`)
	if err != nil {
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		tx, err := a.db.BeginTx(ctx, nil)
		if err != nil {
			continue
		}
		var seller, buyer, resource, status string
		var quantity, risk float64
		var value int64
		var remaining, delays int
		err = tx.QueryRowContext(ctx, `SELECT seller_nation_id,buyer_nation_id,resource,quantity,goods_value,risk_percent,turns_remaining,delay_count,status FROM trade_shipments WHERE id=? FOR UPDATE`, id).Scan(&seller, &buyer, &resource, &quantity, &value, &risk, &remaining, &delays, &status)
		if err != nil {
			tx.Rollback()
			continue
		}
		if delays == 0 && shipmentDelayRoll(id, turn) < risk {
			tx.ExecContext(ctx, `UPDATE trade_shipments SET status='delayed',delay_count=1,estimated_arrival_at=DATE_ADD(estimated_arrival_at,INTERVAL 1 HOUR) WHERE id=?`, id)
			for _, nid := range []string{seller, buyer} {
				tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'market','Shipment delayed',?)`, uuid(), nid, fmt.Sprintf("A %s %s shipment was delayed by one turn.", marketNotificationQuantity(quantity), commodityName(resource)))
			}
			tx.Commit()
			continue
		}
		remaining--
		if remaining > 0 {
			tx.ExecContext(ctx, `UPDATE trade_shipments SET turns_remaining=?,status='in_transit' WHERE id=?`, remaining, id)
			tx.Commit()
			continue
		}
		if isMilitaryEquipment(resource) {
			err = addMilitaryInventory(ctx, tx, buyer, resource, quantity)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, buyer, resource, quantity)
		}
		if err != nil {
			tx.Rollback()
			continue
		}
		if _, err = tx.ExecContext(ctx, `UPDATE nations SET treasury=treasury+? WHERE id=?`, value, seller); err != nil {
			tx.Rollback()
			continue
		}
		tx.ExecContext(ctx, `UPDATE trade_shipments SET turns_remaining=0,status='delivered',delivered_at=NOW() WHERE id=?`, id)
		tx.ExecContext(ctx, `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'trade_sale',?,?)`, uuid(), seller, value, fmt.Sprintf("Escrow released for %.3f %s shipment", quantity, commodityName(resource)))
		for _, nid := range []string{seller, buyer} {
			tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'market','Shipment delivered',?)`, uuid(), nid, fmt.Sprintf("The %s %s shipment arrived and escrow was released.", marketNotificationQuantity(quantity), commodityName(resource)))
		}
		tx.Commit()
	}
}
