package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type database struct{ *sql.DB }
type transaction struct{ *sql.Tx }

func (d *database) Exec(c context.Context, q string, a ...any) (sql.Result, error) {
	return d.ExecContext(c, q, a...)
}
func (d *database) Query(c context.Context, q string, a ...any) (*sql.Rows, error) {
	return d.QueryContext(c, q, a...)
}
func (d *database) QueryRow(c context.Context, q string, a ...any) *sql.Row {
	return d.QueryRowContext(c, q, a...)
}
func (d *database) Begin(c context.Context) (*transaction, error) {
	t, e := d.BeginTx(c, nil)
	return &transaction{t}, e
}
func (t *transaction) Exec(c context.Context, q string, a ...any) (sql.Result, error) {
	return t.ExecContext(c, q, a...)
}
func (t *transaction) QueryRow(c context.Context, q string, a ...any) *sql.Row {
	return t.QueryRowContext(c, q, a...)
}
func (t *transaction) Rollback(context.Context) error { return t.Tx.Rollback() }
func (t *transaction) Commit(context.Context) error   { return t.Tx.Commit() }

type app struct{ db *database }
type user struct{ ID, Email, ThemePreference string }

func main() {
	dsn := env("DATABASE_URL", "diplomatia:diplomatia@tcp(localhost:3306)/diplomatia?parseTime=true&multiStatements=true")
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		log.Fatal(err)
	}
	config.ParseTime = true
	config.MultiStatements = true
	raw, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}
	db := &database{raw}
	defer db.Close()
	if err = db.PingContext(context.Background()); err != nil {
		log.Fatal(err)
	}
	a := &app{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/me", a.auth(a.me))
	mux.HandleFunc("PATCH /api/user/settings", a.auth(a.userSettings))
	mux.HandleFunc("POST /api/nations", a.auth(a.createNation))
	mux.HandleFunc("GET /api/nations", a.auth(a.nationDirectory))
	mux.HandleFunc("GET /api/nations/{id}", a.auth(a.nationProfile))
	mux.HandleFunc("GET /api/nations/{id}/trades", a.auth(a.nationTradeHistory))
	mux.HandleFunc("PATCH /api/nation/settings", a.auth(a.settings))
	mux.HandleFunc("POST /api/nation/location", a.auth(a.resetNationLocation))
	mux.HandleFunc("GET /api/cities", a.auth(a.cities))
	mux.HandleFunc("POST /api/cities", a.auth(a.createCity))
	mux.HandleFunc("POST /api/cities/invest", a.auth(a.investCity))
	mux.HandleFunc("POST /api/cities/expand", a.auth(a.expandCity))
	mux.HandleFunc("POST /api/cities/industry", a.auth(a.investIndustry))
	mux.HandleFunc("GET /api/income", a.auth(a.income))
	mux.HandleFunc("GET /api/economy", a.auth(a.economyDashboard))
	mux.HandleFunc("POST /api/economy/development", a.auth(a.buyDevelopment))
	mux.HandleFunc("POST /api/economy/improvements", a.auth(a.buildImprovement))
	mux.HandleFunc("PATCH /api/economy/policy", a.auth(a.economicPolicy))
	mux.HandleFunc("POST /api/economy/projects", a.auth(a.completeProject))
	mux.HandleFunc("GET /api/national-projects", a.auth(a.longTermProjectsDashboard))
	mux.HandleFunc("POST /api/national-projects", a.auth(a.startLongTermProject))
	mux.HandleFunc("DELETE /api/national-projects/{id}", a.auth(a.demolishLongTermProject))
	mux.HandleFunc("GET /api/strategy", a.auth(a.strategyDashboard))
	mux.HandleFunc("PATCH /api/strategy/gear", a.auth(a.setGear))
	mux.HandleFunc("PATCH /api/strategy/policies", a.auth(a.setPolicies))
	mux.HandleFunc("PATCH /api/strategy/province", a.auth(a.setProvinceStrategy))
	mux.HandleFunc("POST /api/strategy/province/upgrade", a.auth(a.buyProvinceUpgrade))
	mux.HandleFunc("PATCH /api/strategy/quotas", a.auth(a.setQuotas))
	mux.HandleFunc("GET /api/notifications", a.auth(a.notifications))
	mux.HandleFunc("PATCH /api/notifications/read", a.auth(a.readNotifications))
	mux.HandleFunc("POST /api/dev/notifications", a.auth(a.broadcastGameNotification))
	mux.HandleFunc("GET /api/world/status", a.auth(a.worldStatus))
	mux.HandleFunc("GET /api/world/stats", a.worldStats)
	mux.HandleFunc("GET /api/market", a.auth(a.market))
	mux.HandleFunc("POST /api/market/orders", a.auth(a.placeOrder))
	mux.HandleFunc("POST /api/market/orders/{id}/accept", a.auth(a.acceptMarketOrder))
	mux.HandleFunc("POST /api/market/orders/{id}/cancel", a.auth(a.cancelMarketOrder))
	mux.HandleFunc("POST /api/market/orders/{id}/reject", a.auth(a.rejectMarketOrder))
	mux.HandleFunc("GET /api/market/shipments/{id}", a.auth(a.shipmentDetail))
	mux.HandleFunc("POST /api/conflicts", a.auth(a.declareConflict))
	mux.HandleFunc("GET /api/alliances", a.auth(a.allianceDirectory))
	mux.HandleFunc("POST /api/alliances", a.auth(a.createAlliance))
	mux.HandleFunc("GET /api/alliances/{id}", a.auth(a.allianceDetail))
	mux.HandleFunc("PATCH /api/alliances/{id}", a.auth(a.updateAlliance))
	mux.HandleFunc("POST /api/alliances/{id}/apply", a.auth(a.applyAlliance))
	mux.HandleFunc("GET /api/alliances/{id}/applications", a.auth(a.allianceApplications))
	mux.HandleFunc("POST /api/alliances/{id}/applications/{applicationID}/accept", a.auth(a.acceptAllianceApplication))
	mux.HandleFunc("POST /api/alliances/{id}/bank", a.auth(a.allianceBankTransfer))
	addr := ":" + env("PORT", "8080")
	go a.runHourlyTurns()
	log.Printf("api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, logging(cors(mux))))
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.db.PingContext(ctx); err != nil {
		problem(w, http.StatusServiceUnavailable, "Database unavailable.")
		return
	}
	write(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *app) register(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	if !decode(w, r, &in) {
		return
	}
	email, err := normalizeEmail(in.Email)
	if err != nil {
		problem(w, 400, "Enter a valid email address, such as name@example.com.")
		return
	}
	if err := validatePassword(in.Password); err != nil {
		problem(w, 400, err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		problem(w, 500, "Unable to secure the password. Please try again.")
		return
	}
	id := uuid()
	if _, err = a.db.Exec(r.Context(), `INSERT INTO users(id,email,password_hash) VALUES(?,?,?)`, id, email, string(hash)); err != nil {
		problem(w, 409, "That email is already registered.")
		return
	}
	a.newSession(w, r, id)
	write(w, 201, map[string]any{"needsNation": true})
}
func (a *app) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	if !decode(w, r, &in) {
		return
	}
	email, emailErr := normalizeEmail(in.Email)
	var id, hash string
	if emailErr != nil || a.db.QueryRow(r.Context(), `SELECT id,password_hash FROM users WHERE email=?`, email).Scan(&id, &hash) != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		problem(w, 401, "Email or password is incorrect.")
		return
	}
	a.newSession(w, r, id)
	write(w, 200, map[string]bool{"ok": true})
}
func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("session"); e == nil {
		a.db.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash=?`, digest(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(204)
}
func (a *app) userSettings(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Theme string }
	if !decode(w, r, &in) {
		return
	}
	if in.Theme != "dark" && in.Theme != "light" {
		problem(w, 400, "Theme must be dark or light.")
		return
	}
	if _, e := a.db.ExecContext(r.Context(), `UPDATE users SET theme_preference=? WHERE id=?`, in.Theme, u.ID); e != nil {
		problem(w, 500, "Settings could not be saved.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *app) newSession(w http.ResponseWriter, r *http.Request, userID string) {
	token := random(32)
	a.db.Exec(r.Context(), `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(?,?,DATE_ADD(NOW(), INTERVAL 30 DAY))`, digest(token), userID)
	http.SetCookie(w, &http.Cookie{Name: "session", Value: token, Path: "/", HttpOnly: true, Secure: env("COOKIE_SECURE", "") == "true", SameSite: http.SameSiteLaxMode, MaxAge: 2592000})
}

func (a *app) me(w http.ResponseWriter, r *http.Request, u user) {
	var n struct {
		ID, Name, Motto, Currency, LeaderName, Government, Continent, UserType, AllianceID, AllianceName, EconomicGear string
		Treasury, Coal, Steel, Food, Iron, Oil, Bauxite, Aluminum, Gasoline, Munitions, Uranium, Population            int64
		Happiness, Education, Technology, QOL                                                                          int
		ProvinceCount                                                                                                  int
		EmploymentRate, TaxRate                                                                                        float64
		GuardianUntil                                                                                                  *time.Time
		CreatedAt                                                                                                      time.Time
		LocationLat, LocationLng                                                                                       *float64
	}
	err := a.db.QueryRow(r.Context(), `SELECT n.id,n.name,n.motto,n.currency_name,n.leader_name,n.government_type,n.continent,n.user_type,n.treasury,n.coal,n.steel,n.food,n.iron,n.oil,n.bauxite,n.aluminum,n.gasoline,n.munitions,n.uranium,n.population,n.happiness,n.education,n.technology,n.quality_of_life,(SELECT max(expires_at) FROM guardian_grants g WHERE g.nation_id=n.id AND g.revoked_at IS NULL AND g.starts_at<=now() AND g.expires_at>now()),COALESCE(a.id,''),COALESCE(a.name,''),n.created_at,n.employment_rate,n.tax_rate,(SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id),COALESCE((SELECT gear FROM nation_economic_strategy s WHERE s.nation_id=n.id),'balanced'),n.location_lat,n.location_lng FROM nations n LEFT JOIN alliance_members am ON am.nation_id=n.id LEFT JOIN alliances a ON a.id=am.alliance_id WHERE owner_id=?`, u.ID).Scan(&n.ID, &n.Name, &n.Motto, &n.Currency, &n.LeaderName, &n.Government, &n.Continent, &n.UserType, &n.Treasury, &n.Coal, &n.Steel, &n.Food, &n.Iron, &n.Oil, &n.Bauxite, &n.Aluminum, &n.Gasoline, &n.Munitions, &n.Uranium, &n.Population, &n.Happiness, &n.Education, &n.Technology, &n.QOL, &n.GuardianUntil, &n.AllianceID, &n.AllianceName, &n.CreatedAt, &n.EmploymentRate, &n.TaxRate, &n.ProvinceCount, &n.EconomicGear, &n.LocationLat, &n.LocationLng)
	if err != nil {
		write(w, 200, map[string]any{"user": u, "nation": nil})
		return
	}
	write(w, 200, map[string]any{"user": u, "nation": n})
}
func (a *app) createNation(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		Name, Capital, LeaderName, Government string
		Latitude, Longitude                   float64
	}
	if !decode(w, r, &in) {
		return
	}
	strings.TrimSpace(in.Name)
	strings.TrimSpace(in.Capital)
	continent, located := continentAt(in.Latitude, in.Longitude)
	if !located {
		problem(w, 400, "Choose a valid land position on the world map.")
		return
	}
	p, ok := validateFoundingProfile(foundingProfile{in.LeaderName, in.Name, in.Capital, in.Government, continent})
	if !ok {
		problem(w, 400, "Nation and capital names are required.")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		problem(w, 500, "Unable to found nation.")
		return
	}
	defer tx.Rollback(r.Context())
	nid, cid, gid := uuid(), uuid(), uuid()
	userType := nationUserType(p.NationName)
	if _, err = tx.Exec(r.Context(), `INSERT INTO nations(id,owner_id,name,leader_name,government_type,continent,location_lat,location_lng,currency_name,user_type) VALUES(?,?,?,?,?,?,?,?, 'Yen',?)`, nid, u.ID, p.NationName, p.LeaderName, p.Government, p.Continent, in.Latitude, in.Longitude, userType); err != nil {
		problem(w, 409, "That nation or leader name is already taken, or this account already has a nation.")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO cities(id,nation_id,name) VALUES(?,?,?);`, cid, nid, p.Capital)
	if err != nil {
		problem(w, 400, "Could not create capital.")
		return
	}
	tx.Exec(r.Context(), `INSERT INTO nation_economic_strategy(nation_id) VALUES(?)`, nid)
	tx.Exec(r.Context(), `INSERT INTO province_economies(city_id,latitude,longitude) VALUES(?,?,?)`, cid, in.Latitude, in.Longitude)
	for _, resource := range []string{"foodstuffs", "timber", "fibers", "basic_metals", "energy", "strategic_minerals"} {
		tx.Exec(r.Context(), `INSERT INTO province_deposits(city_id,resource,richness) VALUES(?,?,1)`, cid, resource)
	}
	for _, commodity := range strategicCommodities {
		initial := float64(0)
		if map[string]bool{"foodstuffs": true, "timber": true, "fibers": true, "basic_metals": true, "energy": true}[commodity] {
			initial = 500
		}
		tx.Exec(r.Context(), `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?)`, nid, commodity, initial)
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO guardian_grants(id,nation_id,starts_at,expires_at,reason,granted_by) VALUES(?,?,NOW(),DATE_ADD(NOW(), INTERVAL 30 DAY),'new_nation','system')`, gid, nid)
	if err != nil {
		problem(w, 500, "Could not grant Guardian status.")
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'game','Welcome to Diplomatia','Your nation has been founded. Your first 30 days of Guardian status are now active.')`, uuid(), nid); err != nil {
		problem(w, 500, "Could not create welcome notification.")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		problem(w, 500, "Could not found nation.")
		return
	}
	write(w, 201, map[string]any{"id": nid, "guardianDays": 30})
}
func (a *app) settings(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Motto, Government string }
	if !decode(w, r, &in) {
		return
	}
	in.Motto = strings.TrimSpace(in.Motto)
	if len(in.Motto) > 120 || !governmentTypes[in.Government] {
		problem(w, 400, "Invalid nation profile.")
		return
	}
	_, e := a.db.Exec(r.Context(), `UPDATE nations SET motto=?,government_type=?,currency_name='Yen' WHERE owner_id=?`, in.Motto, in.Government, u.ID)
	if e != nil {
		problem(w, 400, "Could not save settings.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *app) declareConflict(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Kind, DefenderID string }
	if !decode(w, r, &in) {
		return
	}
	if in.Kind != "raid" && in.Kind != "war" {
		problem(w, 400, "Conflict must be raid or war.")
		return
	}
	tx, _ := a.db.Begin(r.Context())
	defer tx.Rollback(r.Context())
	var attacker, attackerName string
	if tx.QueryRow(r.Context(), `SELECT id,name FROM nations WHERE owner_id=? FOR UPDATE`, u.ID).Scan(&attacker, &attackerName) != nil {
		problem(w, 409, "Create a nation first.")
		return
	}
	if attacker == in.DefenderID {
		problem(w, 400, "You cannot attack yourself.")
		return
	}
	var protected bool
	tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM guardian_grants WHERE nation_id=? AND revoked_at IS NULL AND starts_at<=now() AND expires_at>now())`, in.DefenderID).Scan(&protected)
	if protected {
		problem(w, 409, "That nation has Guardian status.")
		return
	}
	var defenderName string
	if tx.QueryRow(r.Context(), `SELECT name FROM nations WHERE id=?`, in.DefenderID).Scan(&defenderName) != nil {
		problem(w, 404, "Defending nation not found.")
		return
	}
	res, _ := tx.Exec(r.Context(), `UPDATE guardian_grants SET revoked_at=now(),revoked_reason=CONCAT('initiated_', ?) WHERE nation_id=? AND revoked_at IS NULL AND expires_at>now()`, in.Kind, attacker)
	_ = res
	_, e := tx.Exec(r.Context(), `INSERT INTO conflicts(id,kind,attacker_id,defender_id) VALUES(?,?,?,?)`, uuid(), in.Kind, attacker, in.DefenderID)
	if e != nil {
		problem(w, 400, "Unable to declare conflict.")
		return
	}
	tx.Exec(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'war','Conflict declared',?)`, uuid(), attacker, fmt.Sprintf("You declared a %s against %s.", in.Kind, defenderName))
	tx.Exec(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'war','Your nation is under attack',?)`, uuid(), in.DefenderID, fmt.Sprintf("%s declared a %s against your nation.", attackerName, in.Kind))
	tx.Commit(r.Context())
	write(w, 201, map[string]any{"ok": true, "guardianRevoked": true})
}

type handler func(http.ResponseWriter, *http.Request, user)

func (a *app) auth(next handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie("session")
		if e != nil {
			problem(w, 401, "Sign in required.")
			return
		}
		var u user
		e = a.db.QueryRow(r.Context(), `SELECT u.id,u.email,u.theme_preference FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>now()`, digest(c.Value)).Scan(&u.ID, &u.Email, &u.ThemePreference)
		if e != nil {
			problem(w, 401, "Session expired.")
			return
		}
		a.db.Exec(r.Context(), `UPDATE sessions SET last_action_at=NOW() WHERE token_hash=?`, digest(c.Value))
		a.awardDailyLogin(r.Context(), u.ID)
		next(w, r, u)
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		problem(w, 400, "Invalid request.")
		return false
	}
	return true
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, msg string) {
	write(w, status, map[string]string{"error": msg})
}
func random(n int) string { b := make([]byte, n); rand.Read(b); return hex.EncodeToString(b) }
func uuid() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b)
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o := r.Header.Get("Origin")
		if o != "" {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

var _ = errors.New
