package main

import (
	"database/sql"
	"math"
	"net/http"
)

type geoPoint struct{ Lng, Lat float64 }
type continentShape struct {
	Name   string
	Points []geoPoint
}

var continentShapes = []continentShape{
	{"North America", []geoPoint{{-168, 72}, {-52, 72}, {-52, 48}, {-78, 7}, {-100, 8}, {-118, 18}, {-130, 48}}},
	{"South America", []geoPoint{{-82, 13}, {-34, 8}, {-45, -25}, {-53, -56}, {-74, -50}, {-80, -5}}},
	{"Europe", []geoPoint{{-12, 72}, {45, 72}, {50, 36}, {25, 34}, {-10, 36}}},
	{"Africa", []geoPoint{{-18, 36}, {52, 36}, {50, 10}, {35, -35}, {10, -35}, {-18, 5}}},
	{"Asia", []geoPoint{{40, 78}, {180, 75}, {180, 7}, {135, -10}, {105, 0}, {72, 8}, {40, 36}}},
	{"Oceania", []geoPoint{{105, -8}, {180, -8}, {180, -50}, {108, -50}}},
	{"Antarctica", []geoPoint{{-180, -60}, {180, -60}, {180, -90}, {-180, -90}}},
}

func continentAt(lat, lng float64) (string, bool) {
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 || math.IsNaN(lat) || math.IsNaN(lng) {
		return "", false
	}
	p := geoPoint{lng, lat}
	for _, shape := range continentShapes {
		if pointInPolygon(p, shape.Points) {
			return shape.Name, true
		}
	}
	return "", false
}

func pointInPolygon(p geoPoint, polygon []geoPoint) bool {
	inside := false
	j := len(polygon) - 1
	for i := 0; i < len(polygon); i++ {
		a, b := polygon[i], polygon[j]
		if (a.Lat > p.Lat) != (b.Lat > p.Lat) && p.Lng < (b.Lng-a.Lng)*(p.Lat-a.Lat)/(b.Lat-a.Lat)+a.Lng {
			inside = !inside
		}
		j = i
	}
	return inside
}

func (a *app) resetNationLocation(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Latitude, Longitude float64 }
	if !decode(w, r, &in) {
		return
	}
	continent, ok := continentAt(in.Latitude, in.Longitude)
	if !ok {
		problem(w, http.StatusBadRequest, "Choose a valid land position on the world map.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not begin location change.")
		return
	}
	defer tx.Rollback()
	var nid string
	var treasury int64
	var currentLat, currentLng sql.NullFloat64
	if err = tx.QueryRowContext(r.Context(), `SELECT id,treasury,location_lat,location_lng FROM nations WHERE owner_id=? FOR UPDATE`, u.ID).Scan(&nid, &treasury, &currentLat, &currentLng); err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	cost := int64(0)
	if currentLat.Valid && currentLng.Valid {
		cost = 50000 * yenScale
	}
	if treasury < cost {
		problem(w, http.StatusConflict, "Resetting national location requires ¥5,000,000.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-?,continent=?,location_lat=?,location_lng=? WHERE id=?`, cost, continent, in.Latitude, in.Longitude, nid); err != nil {
		problem(w, 500, "Could not update national location.")
		return
	}
	// The capital anchors the nation's geographic position for future mechanics.
	tx.ExecContext(r.Context(), `UPDATE province_economies p JOIN cities c ON c.id=p.city_id SET p.latitude=?,p.longitude=? WHERE c.nation_id=? AND c.created_at=(SELECT first_created FROM (SELECT MIN(created_at) first_created FROM cities WHERE nation_id=?) x)`, in.Latitude, in.Longitude, nid, nid)
	if cost > 0 {
		tx.ExecContext(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'location_reset',?,?)`, uuid(), nid, -cost, "Reset national location to "+continent)
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Could not complete location change.")
		return
	}
	write(w, 200, map[string]any{"ok": true, "continent": continent, "latitude": in.Latitude, "longitude": in.Longitude, "cost": cost})
}
