package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
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

type app struct {
	db               *database
	connectionSecret []byte
}
type user struct {
	ID, Email, ThemePreference string
	TurnRevenueNotifications   bool
}

func main() {
	dsn := env("DATABASE_URL", "diplomatia:diplomatia@tcp(localhost:3306)/diplomatia?parseTime=true&multiStatements=true")
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		log.Fatal(err)
	}
	config.ParseTime = true
	config.MultiStatements = true
	config.Loc = time.UTC
	if config.Params == nil {
		config.Params = map[string]string{}
	}
	// Daily gameplay rules use CURRENT_DATE() extensively. Setting the session
	// timezone in the DSN applies UTC to every pooled MySQL connection instead
	// of relying on the database host's local timezone.
	config.Params["time_zone"] = "'+00:00'"
	raw, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}
	db := &database{raw}
	defer db.Close()
	if err = db.PingContext(context.Background()); err != nil {
		log.Fatal(err)
	}
	connectionSecret := os.Getenv("CONNECTION_LINK_SECRET")
	if connectionSecret == "" {
		connectionSecret = dsn
		log.Print("CONNECTION_LINK_SECRET is not set; connection linking will use the database credential as a stable fallback. Set a dedicated secret in Railway.")
	}
	a := &app{db: db, connectionSecret: []byte(connectionSecret)}
	if err = a.syncCrisisCatalog(context.Background()); err != nil {
		log.Fatal(err)
	}
	if err = a.ensureDailyCrises(context.Background()); err != nil {
		log.Fatal(err)
	}
	if err = a.captureWorldResourceSnapshot(context.Background(), time.Now().UTC().Truncate(time.Hour)); err != nil {
		log.Printf("initial world resource snapshot failed: %v", err)
	}
	if err = a.regenerateBotMilitary(context.Background()); err != nil {
		log.Printf("initial BOT military regeneration failed: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/me", a.auth(a.me))
	mux.HandleFunc("PATCH /api/user/settings", a.auth(a.userSettings))
	mux.HandleFunc("POST /api/nations", a.auth(a.createNation))
	mux.HandleFunc("GET /api/nations", a.auth(a.nationDirectory))
	mux.HandleFunc("GET /api/nations/{id}/flag", a.auth(a.nationFlag))
	mux.HandleFunc("GET /api/leaderboards", a.auth(a.leaderboards))
	mux.HandleFunc("GET /api/changelog", a.auth(a.changelog))
	mux.HandleFunc("POST /api/changelog", a.auth(a.createChangelogPost))
	mux.HandleFunc("PATCH /api/changelog/{id}", a.auth(a.updateChangelogPost))
	mux.HandleFunc("GET /api/nations/{id}", a.auth(a.nationProfile))
	mux.HandleFunc("GET /api/nations/{id}/wars", a.auth(a.nationWarHistory))
	mux.HandleFunc("GET /api/nations/{id}/trades", a.auth(a.nationTradeHistory))
	mux.HandleFunc("PATCH /api/nation/settings", a.auth(a.settings))
	mux.HandleFunc("POST /api/nation/flag", a.auth(a.uploadNationFlag))
	mux.HandleFunc("POST /api/nation/location", a.auth(a.resetNationLocation))
	mux.HandleFunc("GET /api/fobs", a.auth(a.myForwardOperatingBases))
	mux.HandleFunc("POST /api/fobs", a.auth(a.buildForwardOperatingBase))
	mux.HandleFunc("DELETE /api/fobs/{id}", a.auth(a.demolishForwardOperatingBase))
	mux.HandleFunc("GET /api/nations/{id}/fobs", a.auth(a.publicForwardOperatingBases))
	mux.HandleFunc("GET /api/cities", a.auth(a.cities))
	mux.HandleFunc("POST /api/cities", a.auth(a.createCity))
	mux.HandleFunc("PATCH /api/cities/{id}", a.auth(a.renameCity))
	mux.HandleFunc("POST /api/cities/invest", a.auth(a.investCity))
	mux.HandleFunc("POST /api/cities/expand", a.auth(a.expandCity))
	mux.HandleFunc("GET /api/income", a.auth(a.income))
	mux.HandleFunc("GET /api/economy", a.auth(a.economyDashboard))
	mux.HandleFunc("POST /api/economy/development", a.auth(a.buyDevelopment))
	mux.HandleFunc("POST /api/economy/improvements", a.auth(a.buildImprovement))
	mux.HandleFunc("POST /api/economy/improvements/deconstruct", a.auth(a.deconstructImprovement))
	mux.HandleFunc("PATCH /api/economy/policy", a.auth(a.economicPolicy))
	mux.HandleFunc("POST /api/economy/projects", a.auth(a.completeProject))
	mux.HandleFunc("PATCH /api/economy/luxury-consumption", a.auth(a.setLuxuryConsumptionRate))
	mux.HandleFunc("GET /api/national-projects", a.auth(a.longTermProjectsDashboard))
	mux.HandleFunc("POST /api/national-projects", a.auth(a.startLongTermProject))
	mux.HandleFunc("DELETE /api/national-projects/{id}", a.auth(a.demolishLongTermProject))
	mux.HandleFunc("GET /api/ventures", a.auth(a.venturesDashboard))
	mux.HandleFunc("POST /api/ventures/transfer", a.auth(a.transferVentureCapital))
	mux.HandleFunc("POST /api/ventures/invest", a.auth(a.investVenture))
	mux.HandleFunc("POST /api/ventures/{id}/collect", a.auth(a.collectVenture))
	mux.HandleFunc("POST /api/ventures/{id}/cancel", a.auth(a.cancelVenture))
	mux.HandleFunc("GET /api/crises", a.auth(a.crises))
	mux.HandleFunc("POST /api/crises/{id}/respond", a.auth(a.respondToCrisis))
	mux.HandleFunc("GET /api/strategy", a.auth(a.strategyDashboard))
	mux.HandleFunc("PATCH /api/strategy/gear", a.auth(a.setGear))
	mux.HandleFunc("PATCH /api/strategy/policies", a.auth(a.setPolicies))
	mux.HandleFunc("PATCH /api/strategy/province", a.auth(a.setProvinceStrategy))
	mux.HandleFunc("POST /api/strategy/province/upgrade", a.auth(a.buyProvinceUpgrade))
	mux.HandleFunc("PATCH /api/strategy/quotas", a.auth(a.setQuotas))
	mux.HandleFunc("GET /api/notifications", a.auth(a.notifications))
	mux.HandleFunc("PATCH /api/notifications/read", a.auth(a.readNotifications))
	mux.HandleFunc("POST /api/dev/notifications", a.auth(a.broadcastGameNotification))
	mux.HandleFunc("POST /api/nations/{id}/report", a.auth(a.reportNation))
	mux.HandleFunc("GET /api/dev/bans", a.auth(a.devBans))
	mux.HandleFunc("GET /api/nations/{id}/connections", a.auth(a.nationConnections))
	mux.HandleFunc("POST /api/dev/bans", a.auth(a.banUser))
	mux.HandleFunc("DELETE /api/dev/bans/{userID}", a.auth(a.unbanUser))
	mux.HandleFunc("DELETE /api/dev/nations/{id}/guardian", a.auth(a.removeGuardianStatus))
	mux.HandleFunc("DELETE /api/nation/guardian", a.auth(a.voluntarilyRemoveGuardianStatus))
	mux.HandleFunc("GET /api/world/status", a.auth(a.worldStatus))
	mux.HandleFunc("GET /api/world/stats", a.worldStats)
	mux.HandleFunc("GET /api/world/resources", a.auth(a.worldResourceHistory))
	mux.HandleFunc("GET /api/market", a.auth(a.market))
	mux.HandleFunc("POST /api/market/orders", a.auth(a.placeOrder))
	mux.HandleFunc("POST /api/market/orders/{id}/accept", a.auth(a.acceptMarketOrder))
	mux.HandleFunc("POST /api/market/orders/{id}/cancel", a.auth(a.cancelMarketOrder))
	mux.HandleFunc("POST /api/market/orders/{id}/reject", a.auth(a.rejectMarketOrder))
	mux.HandleFunc("GET /api/market/shipments/{id}", a.auth(a.shipmentDetail))
	mux.HandleFunc("GET /api/military", a.auth(a.militaryDashboard))
	mux.HandleFunc("PUT /api/military/defense-settings", a.auth(a.saveMilitaryDefenseSettings))
	mux.HandleFunc("POST /api/military/produce", a.auth(a.produceMilitary))
	mux.HandleFunc("POST /api/military/decommission", a.auth(a.decommissionMilitary))
	mux.HandleFunc("GET /api/wars", a.auth(a.warsDashboard))
	mux.HandleFunc("POST /api/wars", a.auth(a.declareWar))
	mux.HandleFunc("GET /api/wars/{id}", a.auth(a.warDetails))
	mux.HandleFunc("POST /api/wars/{id}/deploy", a.auth(a.deployWarForces))
	mux.HandleFunc("PUT /api/wars/{id}/orders", a.auth(a.submitWarOrders))
	mux.HandleFunc("POST /api/wars/{id}/capitulate", a.auth(a.capitulateWar))
	mux.HandleFunc("GET /api/conflicts", a.auth(a.conflictDirectory))
	mux.HandleFunc("GET /api/conflicts/{id}", a.auth(a.publicConflictDetails))
	mux.HandleFunc("POST /api/conflicts", a.auth(a.declareConflict))
	mux.HandleFunc("GET /api/alliances", a.auth(a.allianceDirectory))
	mux.HandleFunc("POST /api/alliances", a.auth(a.createAlliance))
	mux.HandleFunc("GET /api/alliances/{id}/flag", a.auth(a.allianceFlag))
	mux.HandleFunc("POST /api/alliances/{id}/flag", a.auth(a.uploadAllianceFlag))
	mux.HandleFunc("GET /api/alliances/{id}", a.auth(a.allianceDetail))
	mux.HandleFunc("GET /api/alliances/{id}/wars", a.auth(a.allianceWarHistory))
	mux.HandleFunc("PATCH /api/alliances/{id}", a.auth(a.updateAlliance))
	mux.HandleFunc("POST /api/alliances/{id}/apply", a.auth(a.applyAlliance))
	mux.HandleFunc("GET /api/alliances/{id}/applications", a.auth(a.allianceApplications))
	mux.HandleFunc("POST /api/alliances/{id}/applications/{applicationID}/accept", a.auth(a.acceptAllianceApplication))
	mux.HandleFunc("POST /api/alliances/{id}/applications/{applicationID}/reject", a.auth(a.rejectAllianceApplication))
	mux.HandleFunc("POST /api/alliances/{id}/announcements", a.auth(a.postAllianceAnnouncement))
	mux.HandleFunc("POST /api/alliances/{id}/bank", a.auth(a.allianceBankTransfer))
	mux.HandleFunc("POST /api/alliances/{id}/member-balances/adjust", a.auth(a.adjustAllianceMemberBalance))
	mux.HandleFunc("POST /api/alliances/{id}/roles", a.auth(a.createAllianceRole))
	mux.HandleFunc("PATCH /api/alliances/{id}/roles/{roleID}", a.auth(a.updateAllianceRole))
	mux.HandleFunc("DELETE /api/alliances/{id}/roles/{roleID}", a.auth(a.deleteAllianceRole))
	mux.HandleFunc("PATCH /api/alliances/{id}/members/{nationID}/role", a.auth(a.assignAllianceRole))
	mux.HandleFunc("DELETE /api/alliances/{id}/members/{nationID}", a.auth(a.removeAllianceMember))
	mux.HandleFunc("PATCH /api/alliances/{id}/members/{nationID}/tax-bracket", a.auth(a.assignAllianceTaxBracket))
	mux.HandleFunc("POST /api/alliances/{id}/leave", a.auth(a.leaveAlliance))
	mux.HandleFunc("DELETE /api/alliances/{id}", a.auth(a.deleteAlliance))
	mux.HandleFunc("POST /api/alliances/{id}/tax-brackets", a.auth(a.createAllianceTaxBracket))
	mux.HandleFunc("PATCH /api/alliances/{id}/tax-brackets/{bracketID}", a.auth(a.updateAllianceTaxBracket))
	mux.HandleFunc("DELETE /api/alliances/{id}/tax-brackets/{bracketID}", a.auth(a.deleteAllianceTaxBracket))
	mux.HandleFunc("POST /api/alliances/{id}/treaties", a.auth(a.proposeAllianceTreaty))
	mux.HandleFunc("POST /api/alliances/{id}/treaties/{treatyID}/{action}", a.auth(a.resolveAllianceTreaty))
	mux.HandleFunc("DELETE /api/alliances/{id}/treaties/{treatyID}", a.auth(a.cancelAllianceTreaty))
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
	var in struct {
		Theme                    string
		TurnRevenueNotifications *bool
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Theme != "" && in.Theme != "dark" && in.Theme != "light" {
		problem(w, 400, "Theme must be dark or light.")
		return
	}
	if _, e := a.db.ExecContext(r.Context(), `UPDATE users SET theme_preference=CASE WHEN ?='' THEN theme_preference ELSE ? END,turn_revenue_notifications=COALESCE(?,turn_revenue_notifications) WHERE id=?`, in.Theme, in.Theme, in.TurnRevenueNotifications, u.ID); e != nil {
		problem(w, 500, "Settings could not be saved.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *app) newSession(w http.ResponseWriter, r *http.Request, userID string) {
	token := random(32)
	a.db.Exec(r.Context(), `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(?,?,DATE_ADD(NOW(), INTERVAL 30 DAY))`, digest(token), userID)
	a.recordConnectionObservation(r, userID)
	http.SetCookie(w, &http.Cookie{Name: "session", Value: token, Path: "/", HttpOnly: true, Secure: env("COOKIE_SECURE", "") == "true", SameSite: http.SameSiteLaxMode, MaxAge: 2592000})
}

func (a *app) me(w http.ResponseWriter, r *http.Request, u user) {
	if reason, until, banned := a.activeBan(r.Context(), u.ID); banned {
		write(w, 200, map[string]any{"user": u, "nation": nil, "banned": true, "banReason": reason, "banExpiresAt": until})
		return
	}
	var n struct {
		ID, Name, Motto, Currency, LeaderName, Government, Continent, UserType, AllianceID, AllianceName, AllianceRole, EconomicGear string
		Treasury, Population                                                                                                         int64
		Happiness, Education, Technology, QOL                                                                                        int
		ProvinceCount                                                                                                                int
		EmploymentRate, TaxRate                                                                                                      float64
		GuardianUntil                                                                                                                *time.Time
		CreatedAt                                                                                                                    time.Time
		LocationLat, LocationLng                                                                                                     *float64
		Military                                                                                                                     []militaryOverviewItem
		NationalDetails                                                                                                              nationalDetails
	}
	err := a.db.QueryRow(r.Context(), `SELECT n.id,n.name,n.motto,n.currency_name,n.leader_name,n.government_type,n.continent,n.user_type,n.treasury,n.population,n.happiness,n.education,n.technology,n.quality_of_life,(SELECT max(expires_at) FROM guardian_grants g WHERE g.nation_id=n.id AND g.revoked_at IS NULL AND g.starts_at<=now() AND g.expires_at>now()),COALESCE(a.id,''),COALESCE(a.name,''),COALESCE(ar.title,''),n.created_at,n.employment_rate,n.tax_rate,(SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id),COALESCE((SELECT gear FROM nation_economic_strategy s WHERE s.nation_id=n.id),'balanced'),n.location_lat,n.location_lng FROM nations n LEFT JOIN alliance_members am ON am.nation_id=n.id LEFT JOIN alliances a ON a.id=am.alliance_id LEFT JOIN alliance_roles ar ON ar.id=am.role_id WHERE owner_id=?`, u.ID).Scan(&n.ID, &n.Name, &n.Motto, &n.Currency, &n.LeaderName, &n.Government, &n.Continent, &n.UserType, &n.Treasury, &n.Population, &n.Happiness, &n.Education, &n.Technology, &n.QOL, &n.GuardianUntil, &n.AllianceID, &n.AllianceName, &n.AllianceRole, &n.CreatedAt, &n.EmploymentRate, &n.TaxRate, &n.ProvinceCount, &n.EconomicGear, &n.LocationLat, &n.LocationLng)
	if err != nil {
		write(w, 200, map[string]any{"user": u, "nation": nil})
		return
	}
	n.Military = loadMilitaryOverview(r.Context(), a.db, n.ID)
	if economicNation, _, _, economicErr := a.loadEconomicNationContext(r.Context(), u.ID); economicErr == nil {
		result := calculateEconomy(economicNation)
		n.EmploymentRate = result.EffectiveEmploymentRate
		n.NationalDetails = buildNationalDetails(economicNation, result)
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
	initialPopulation := startingNationPopulation()
	if _, err = tx.Exec(r.Context(), `INSERT INTO nations(id,owner_id,name,leader_name,government_type,continent,location_lat,location_lng,population,currency_name,user_type) VALUES(?,?,?,?,?,?,?,?,?,'Yen',?)`, nid, u.ID, p.NationName, p.LeaderName, p.Government, p.Continent, in.Latitude, in.Longitude, initialPopulation, userType); err != nil {
		problem(w, 409, "That nation or leader name is already taken, or this account already has a nation.")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO cities(id,nation_id,name,local_population) VALUES(?,?,?,?);`, cid, nid, p.Capital, initialPopulation)
	if err != nil {
		problem(w, 400, "Could not create capital.")
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE nations SET capital_city_id=? WHERE id=?`, cid, nid); err != nil {
		problem(w, 500, "Could not designate the capital Province.")
		return
	}
	tx.Exec(r.Context(), `INSERT INTO nation_economic_strategy(nation_id) VALUES(?)`, nid)
	tx.Exec(r.Context(), `INSERT INTO province_economies(city_id,latitude,longitude) VALUES(?,?,?)`, cid, in.Latitude, in.Longitude)
	for _, resource := range []string{"foodstuffs", "timber", "fibers", "basic_metals", "energy", "strategic_minerals"} {
		tx.Exec(r.Context(), `INSERT INTO province_deposits(city_id,resource,richness) VALUES(?,?,?)`, cid, resource, startingDepositRichness(p.Continent, resource))
	}
	for _, commodity := range strategicCommodities {
		initial := float64(0)
		if map[string]bool{"foodstuffs": true, "timber": true, "fibers": true, "basic_metals": true, "energy": true}[commodity] {
			initial = 500
		}
		if starter, ok := map[string]float64{"construction_materials": 75, "processed_foods": 100, "basic_goods": 75}[commodity]; ok {
			initial = starter
		}
		tx.Exec(r.Context(), `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?)`, nid, commodity, initial)
	}
	for commodity, priority := range defaultProductionQuotas() {
		tx.Exec(r.Context(), `INSERT INTO production_quotas(nation_id,commodity,priority) VALUES(?,?,?)`, nid, commodity, priority)
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
	const nameChangeCost int64 = 500000
	var in struct{ Motto, Government, NationName, LeaderName string }
	if !decode(w, r, &in) {
		return
	}
	in.Motto = strings.TrimSpace(in.Motto)
	if len(in.Motto) > 120 || !governmentTypes[in.Government] {
		problem(w, 400, "Invalid nation profile.")
		return
	}
	in.NationName = strings.TrimSpace(in.NationName)
	in.LeaderName = strings.TrimSpace(in.LeaderName)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not save settings.")
		return
	}
	defer tx.Rollback()
	var currentName, currentLeader string
	var treasury int64
	if err = tx.QueryRowContext(r.Context(), `SELECT name,leader_name,treasury FROM nations WHERE owner_id=? FOR UPDATE`, u.ID).Scan(&currentName, &currentLeader, &treasury); err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	if in.NationName == "" {
		in.NationName = currentName
	}
	if in.LeaderName == "" {
		in.LeaderName = currentLeader
	}
	if len(in.NationName) < 3 || len(in.NationName) > 100 || len(in.LeaderName) < 2 || len(in.LeaderName) > 100 || !validRomanName(in.NationName) || !validRomanName(in.LeaderName) {
		problem(w, 400, "Nation and leader names must use Roman letters and standard name punctuation only.")
		return
	}
	changes := int64(0)
	if in.NationName != currentName {
		changes++
	}
	if in.LeaderName != currentLeader {
		changes++
	}
	cost := changes * nameChangeCost
	if treasury < cost {
		problem(w, 400, "Insufficient Treasury. Name changes cost ¥500,000 each.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE nations SET motto=?,government_type=?,name=?,leader_name=?,treasury=treasury-?,currency_name='Yen' WHERE owner_id=?`, in.Motto, in.Government, in.NationName, in.LeaderName, cost, u.ID)
	if err != nil {
		if mysqlError, ok := err.(*mysql.MySQLError); ok && mysqlError.Number == 1062 {
			problem(w, 400, "That nation or leader name is already in use.")
			return
		}
		problem(w, 400, "Could not save settings.")
		return
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Could not save settings.")
		return
	}
	write(w, 200, map[string]any{"ok": true, "nameChangeCost": cost})
}
func (a *app) declareConflict(w http.ResponseWriter, r *http.Request, u user) {
	problem(w, http.StatusGone, "The legacy conflict endpoint is retired. Use the War Room for wars. Raiding will be introduced as its own system later.")
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
		e = a.db.QueryRow(r.Context(), `SELECT u.id,u.email,u.theme_preference,u.turn_revenue_notifications FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>now()`, digest(c.Value)).Scan(&u.ID, &u.Email, &u.ThemePreference, &u.TurnRevenueNotifications)
		if e != nil {
			problem(w, 401, "Session expired.")
			return
		}
		if _, until, banned := a.activeBan(r.Context(), u.ID); banned && r.URL.Path != "/api/me" {
			untilText := " indefinitely"
			if until != nil {
				untilText = " until " + until.Format("2006-01-02")
			}
			problem(w, http.StatusForbidden, "This account is banned"+untilText+".")
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
