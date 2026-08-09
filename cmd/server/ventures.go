package main

import (
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"
)

const (
	ventureDailyTransferLimit int64 = 5_000_000
	venturePersonalCapitalCap int64 = 5_000_000
	ventureActiveCapitalCap   int64 = 3_000_000
	ventureActiveLimit              = 3
	ventureBoardSize                = 5
	ventureBoardHours               = 8
	ventureAutoCollectHours         = 7 * 24
)

type ventureTemplate struct {
	Key, Title, Description, Risk string
	Min, Max                      int64
	MinHours, MaxHours            int
	MinReturnBPS, MaxReturnBPS    int
}

var ventureTemplates = []ventureTemplate{
	{"commodity_pool", "Regional Commodity Pool", "A managed position in short-cycle regional commodity contracts.", "low", 100_000, 700_000, 8, 16, -1000, 800},
	{"supply_note", "Municipal Supply Note", "Short-term financing for routine municipal procurement and delivery.", "low", 150_000, 800_000, 10, 18, -1000, 800},
	{"farm_cooperative", "Agricultural Futures Cooperative", "A seasonal position spread across private growers and food distributors.", "low", 125_000, 650_000, 8, 16, -1000, 800},
	{"manufacturing_stake", "Artisan Manufacturing Stake", "Growth capital for a small producer pursuing a larger domestic contract.", "medium", 250_000, 1_100_000, 16, 32, -1200, 4000},
	{"trading_company", "Coastal Trading Company", "A minority stake in a merchant concern opening a new commercial route.", "medium", 300_000, 1_250_000, 18, 36, -1200, 4000},
	{"logistics_coop", "Independent Logistics Cooperative", "Fleet and warehouse financing tied to a private distribution expansion.", "medium", 200_000, 950_000, 16, 30, -1200, 4000},
	{"media_acquisition", "Independent Media Acquisition", "Bridge financing for a small media group acquiring a regional outlet.", "medium", 250_000, 1_000_000, 20, 36, -1200, 4000},
	{"exploration_note", "Frontier Exploration Note", "A speculative survey whose value depends on what the field team discovers.", "high", 350_000, 1_500_000, 24, 72, -2000, 7000},
	{"mineral_prospectus", "Rare Materials Prospectus", "Early capital for a private mineral prospect with uncertain reserves.", "high", 400_000, 1_500_000, 30, 72, -2000, 7000},
	{"salvage_syndicate", "Maritime Salvage Syndicate", "A high-risk claim on the recoverable value of a difficult salvage operation.", "high", 300_000, 1_300_000, 24, 60, -2000, 7000},
}

type ventureOpportunityItem struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Risk          string    `json:"risk"`
	MinInvestment int64     `json:"minInvestment"`
	MaxInvestment int64     `json:"maxInvestment"`
	DurationHours int       `json:"durationHours"`
	MinReturnBPS  int       `json:"minReturnBps"`
	MaxReturnBPS  int       `json:"maxReturnBps"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

type personalVentureItem struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Risk           string     `json:"risk"`
	Status         string     `json:"status"`
	AmountInvested int64      `json:"amountInvested"`
	OutcomeBPS     *int       `json:"outcomeBps"`
	Payout         *int64     `json:"payout"`
	MaturesAt      time.Time  `json:"maturesAt"`
	ResolvedAt     *time.Time `json:"resolvedAt"`
	CollectedAt    *time.Time `json:"collectedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
}

func secureInt(max int) int {
	if max <= 1 {
		return 0
	}
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return int(time.Now().UnixNano() % int64(max))
	}
	return int(n.Int64())
}

func randomBetween(minimum, maximum int) int {
	if maximum <= minimum {
		return minimum
	}
	return minimum + secureInt(maximum-minimum+1)
}

func ventureCancellationRefund(amount int64) int64 { return amount * 75 / 100 }

func ventureOutcome(risk string) int {
	roll := secureInt(10_000)
	rangeBPS := func(minimum, maximum int) int { return randomBetween(minimum, maximum) }
	switch risk {
	case "low":
		if roll < 1000 {
			return rangeBPS(-1000, -200)
		}
		if roll < 3000 {
			return rangeBPS(-100, 100)
		}
		return rangeBPS(200, 800)
	case "medium":
		if roll < 2500 {
			return rangeBPS(-1200, -300)
		}
		if roll < 7500 {
			return rangeBPS(0, 1200)
		}
		if roll < 9700 {
			return rangeBPS(1300, 2500)
		}
		return rangeBPS(2600, 4000)
	default:
		if roll < 4500 {
			return rangeBPS(-2000, -300)
		}
		if roll < 8000 {
			return rangeBPS(-200, 1500)
		}
		if roll < 9700 {
			return rangeBPS(1600, 3500)
		}
		return rangeBPS(3600, 7000)
	}
}

func (a *app) ensureVentureAccount(ctx context.Context, nationID string) error {
	_, err := a.db.ExecContext(ctx, `INSERT IGNORE INTO venture_accounts(nation_id,transfer_date) VALUES(?,CURRENT_DATE())`, nationID)
	return err
}

func refreshVentureBoard(ctx context.Context, tx *sql.Tx, nationID string) (time.Time, error) {
	var refresh sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT board_refresh_at FROM venture_accounts WHERE nation_id=? FOR UPDATE`, nationID).Scan(&refresh); err != nil {
		return time.Time{}, err
	}
	now := time.Now().UTC()
	if refresh.Valid && refresh.Time.After(now) {
		return refresh.Time, nil
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM venture_opportunities WHERE nation_id=? AND accepted_at IS NULL`, nationID); err != nil {
		return time.Time{}, err
	}
	indexes := make([]int, len(ventureTemplates))
	for i := range indexes {
		indexes[i] = i
	}
	for i := len(indexes) - 1; i > 0; i-- {
		j := secureInt(i + 1)
		indexes[i], indexes[j] = indexes[j], indexes[i]
	}
	next := now.Add(ventureBoardHours * time.Hour)
	for i := 0; i < ventureBoardSize && i < len(indexes); i++ {
		t := ventureTemplates[indexes[i]]
		duration := randomBetween(t.MinHours, t.MaxHours)
		if _, err := tx.ExecContext(ctx, `INSERT INTO venture_opportunities(id,nation_id,template_key,title,description,min_investment,max_investment,duration_hours,risk,min_return_bps,max_return_bps,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, uuid(), nationID, t.Key, t.Title, t.Description, t.Min, t.Max, duration, t.Risk, t.MinReturnBPS, t.MaxReturnBPS, next); err != nil {
			return time.Time{}, err
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE venture_accounts SET board_refresh_at=? WHERE nation_id=?`, next, nationID)
	return next, err
}

func (a *app) venturesDashboard(w http.ResponseWriter, r *http.Request, u user) {
	nid, err := a.nationID(r.Context(), u.ID)
	if err != nil {
		problem(w, http.StatusNotFound, "Nation not found.")
		return
	}
	if err = a.ensureVentureAccount(r.Context(), nid); err != nil {
		problem(w, http.StatusInternalServerError, "Private Ventures are unavailable.")
		return
	}
	a.processMatureVentures(r.Context())
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Private Ventures are unavailable.")
		return
	}
	defer tx.Rollback()
	if _, err = refreshVentureBoard(r.Context(), tx, nid); err != nil || tx.Commit() != nil {
		problem(w, http.StatusInternalServerError, "Could not refresh private opportunities.")
		return
	}

	var capital, usedToday, treasury, activeCapital int64
	var transferDate sql.NullTime
	var nextRefresh sql.NullTime
	if err = a.db.QueryRowContext(r.Context(), `SELECT va.personal_capital,va.transfer_used_today,va.transfer_date,va.board_refresh_at,n.treasury FROM venture_accounts va JOIN nations n ON n.id=va.nation_id WHERE va.nation_id=?`, nid).Scan(&capital, &usedToday, &transferDate, &nextRefresh, &treasury); err != nil {
		problem(w, http.StatusInternalServerError, "Private Ventures are unavailable.")
		return
	}
	if !transferDate.Valid || transferDate.Time.Format("2006-01-02") != time.Now().UTC().Format("2006-01-02") {
		usedToday = 0
	}
	var activeCount int
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(amount_invested),0) FROM personal_ventures WHERE nation_id=? AND status='active'`, nid).Scan(&activeCount, &activeCapital)

	opportunities := []ventureOpportunityItem{}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,title,description,min_investment,max_investment,duration_hours,risk,min_return_bps,max_return_bps,expires_at FROM venture_opportunities WHERE nation_id=? AND accepted_at IS NULL AND expires_at>NOW() ORDER BY risk,duration_hours`, nid)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var item ventureOpportunityItem
			if rows.Scan(&item.ID, &item.Title, &item.Description, &item.MinInvestment, &item.MaxInvestment, &item.DurationHours, &item.Risk, &item.MinReturnBPS, &item.MaxReturnBPS, &item.ExpiresAt) == nil {
				opportunities = append(opportunities, item)
			}
		}
	}
	ventures := []personalVentureItem{}
	rows, err = a.db.QueryContext(r.Context(), `SELECT id,title,description,risk,amount_invested,outcome_bps,payout,status,matures_at,resolved_at,collected_at,created_at FROM personal_ventures WHERE nation_id=? ORDER BY FIELD(status,'claimable','active','collected','cancelled'),created_at DESC LIMIT 50`, nid)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var item personalVentureItem
			var outcome sql.NullInt64
			var payout sql.NullInt64
			var resolved, collected sql.NullTime
			if rows.Scan(&item.ID, &item.Title, &item.Description, &item.Risk, &item.AmountInvested, &outcome, &payout, &item.Status, &item.MaturesAt, &resolved, &collected, &item.CreatedAt) == nil {
				if outcome.Valid {
					v := int(outcome.Int64)
					item.OutcomeBPS = &v
				}
				if payout.Valid {
					v := payout.Int64
					item.Payout = &v
				}
				if resolved.Valid {
					v := resolved.Time
					item.ResolvedAt = &v
				}
				if collected.Valid {
					v := collected.Time
					item.CollectedAt = &v
				}
				ventures = append(ventures, item)
			}
		}
	}
	write(w, http.StatusOK, map[string]any{
		"account":       map[string]any{"personalCapital": capital, "treasury": treasury, "personalCapitalCap": venturePersonalCapitalCap, "dailyTransferLimit": ventureDailyTransferLimit, "transferUsedToday": usedToday, "transferRemaining": max64(0, ventureDailyTransferLimit-usedToday), "activeCapital": activeCapital, "activeCapitalCap": ventureActiveCapitalCap, "activeCount": activeCount, "activeLimit": ventureActiveLimit, "nextRefreshAt": nextRefresh.Time},
		"opportunities": opportunities, "ventures": ventures,
	})
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (a *app) transferVentureCapital(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		Direction string `json:"direction"`
		Amount    int64  `json:"amount"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Direction = strings.ToLower(strings.TrimSpace(in.Direction))
	if in.Amount <= 0 || (in.Direction != "to_personal" && in.Direction != "to_treasury") {
		problem(w, http.StatusBadRequest, "Choose a valid transfer direction and whole-Yen amount.")
		return
	}
	nid, err := a.nationID(r.Context(), u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	if err = a.ensureVentureAccount(r.Context(), nid); err != nil {
		problem(w, 500, "Transfer unavailable.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Transfer unavailable.")
		return
	}
	defer tx.Rollback()
	var capital, used, treasury int64
	var transferDate sql.NullTime
	if err = tx.QueryRowContext(r.Context(), `SELECT va.personal_capital,va.transfer_used_today,va.transfer_date,n.treasury FROM venture_accounts va JOIN nations n ON n.id=va.nation_id WHERE va.nation_id=? FOR UPDATE`, nid).Scan(&capital, &used, &transferDate, &treasury); err != nil {
		problem(w, 500, "Transfer unavailable.")
		return
	}
	today := time.Now().UTC().Format("2006-01-02")
	if !transferDate.Valid || transferDate.Time.Format("2006-01-02") != today {
		used = 0
	}
	if used+in.Amount > ventureDailyTransferLimit {
		problem(w, 400, "This transfer exceeds today's combined ¥5,000,000 allowance.")
		return
	}
	if in.Direction == "to_personal" {
		if treasury < in.Amount {
			problem(w, 400, "Your national Treasury does not hold enough Yen.")
			return
		}
		if capital+in.Amount > venturePersonalCapitalCap {
			problem(w, 400, "Personal Capital deposits cannot exceed ¥5,000,000.")
			return
		}
		capital += in.Amount
		treasury -= in.Amount
	} else {
		if capital < in.Amount {
			problem(w, 400, "Your Personal Capital does not hold enough Yen.")
			return
		}
		capital -= in.Amount
		treasury += in.Amount
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE venture_accounts SET personal_capital=?,transfer_used_today=?,transfer_date=CURRENT_DATE() WHERE nation_id=?`, capital, used+in.Amount, nid); err != nil {
		problem(w, 500, "Transfer unavailable.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE nations SET treasury=? WHERE id=?`, treasury, nid); err != nil {
		problem(w, 500, "Transfer unavailable.")
		return
	}
	memo := "Moved Yen from national Treasury to Personal Capital"
	ledgerAmount := -in.Amount
	if in.Direction == "to_treasury" {
		memo = "Returned Personal Capital to national Treasury"
		ledgerAmount = in.Amount
	}
	tx.ExecContext(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'private_venture_transfer',?,?)`, uuid(), nid, ledgerAmount, memo)
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Transfer unavailable.")
		return
	}
	write(w, http.StatusOK, map[string]any{"personalCapital": capital, "treasury": treasury, "transferRemaining": ventureDailyTransferLimit - used - in.Amount})
}

func (a *app) investVenture(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		OpportunityID string `json:"opportunityId"`
		Amount        int64  `json:"amount"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Amount <= 0 || strings.TrimSpace(in.OpportunityID) == "" {
		problem(w, 400, "Choose an opportunity and valid investment amount.")
		return
	}
	nid, err := a.nationID(r.Context(), u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	if err = a.ensureVentureAccount(r.Context(), nid); err != nil {
		problem(w, 500, "Investment unavailable.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Investment unavailable.")
		return
	}
	defer tx.Rollback()
	var title, description, risk string
	var minimum, maximum int64
	var duration int
	if err = tx.QueryRowContext(r.Context(), `SELECT title,description,risk,min_investment,max_investment,duration_hours FROM venture_opportunities WHERE id=? AND nation_id=? AND accepted_at IS NULL AND expires_at>NOW() FOR UPDATE`, in.OpportunityID, nid).Scan(&title, &description, &risk, &minimum, &maximum, &duration); err != nil {
		problem(w, 400, "That private opportunity is no longer available.")
		return
	}
	if in.Amount < minimum || in.Amount > maximum {
		problem(w, 400, fmt.Sprintf("Invest between ¥%d and ¥%d in this opportunity.", minimum, maximum))
		return
	}
	var capital, activeCapital int64
	var activeCount int
	if err = tx.QueryRowContext(r.Context(), `SELECT personal_capital FROM venture_accounts WHERE nation_id=? FOR UPDATE`, nid).Scan(&capital); err != nil {
		problem(w, 500, "Investment unavailable.")
		return
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(amount_invested),0) FROM personal_ventures WHERE nation_id=? AND status='active'`, nid).Scan(&activeCount, &activeCapital); err != nil {
		problem(w, 500, "Investment unavailable.")
		return
	}
	if activeCount >= ventureActiveLimit {
		problem(w, 400, "All three active venture slots are occupied.")
		return
	}
	if activeCapital+in.Amount > ventureActiveCapitalCap {
		problem(w, 400, "This investment exceeds the ¥3,000,000 active-capital limit.")
		return
	}
	if capital < in.Amount {
		problem(w, 400, "Your Personal Capital does not hold enough Yen.")
		return
	}
	ventureID, matures := uuid(), time.Now().UTC().Add(time.Duration(duration)*time.Hour)
	if _, err = tx.ExecContext(r.Context(), `UPDATE venture_accounts SET personal_capital=personal_capital-? WHERE nation_id=?`, in.Amount, nid); err != nil {
		problem(w, 500, "Investment unavailable.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE venture_opportunities SET accepted_at=NOW() WHERE id=?`, in.OpportunityID); err != nil {
		problem(w, 500, "Investment unavailable.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO personal_ventures(id,nation_id,opportunity_id,title,description,risk,amount_invested,matures_at) VALUES(?,?,?,?,?,?,?,?)`, ventureID, nid, in.OpportunityID, title, description, risk, in.Amount, matures); err != nil {
		problem(w, 500, "Investment unavailable.")
		return
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Investment unavailable.")
		return
	}
	write(w, http.StatusCreated, map[string]any{"id": ventureID, "maturesAt": matures})
}

func (a *app) cancelVenture(w http.ResponseWriter, r *http.Request, u user) {
	nid, err := a.nationID(r.Context(), u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Cancellation unavailable.")
		return
	}
	defer tx.Rollback()
	var amount int64
	if err = tx.QueryRowContext(r.Context(), `SELECT amount_invested FROM personal_ventures WHERE id=? AND nation_id=? AND status='active' FOR UPDATE`, r.PathValue("id"), nid).Scan(&amount); err != nil {
		problem(w, 400, "Only an active venture can be cancelled.")
		return
	}
	refund := ventureCancellationRefund(amount)
	if _, err = tx.ExecContext(r.Context(), `UPDATE personal_ventures SET status='cancelled',payout=?,outcome_bps=-2500,resolved_at=NOW(),collected_at=NOW() WHERE id=?`, refund, r.PathValue("id")); err != nil {
		problem(w, 500, "Cancellation unavailable.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE venture_accounts SET personal_capital=personal_capital+? WHERE nation_id=?`, refund, nid); err != nil {
		problem(w, 500, "Cancellation unavailable.")
		return
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Cancellation unavailable.")
		return
	}
	write(w, http.StatusOK, map[string]any{"refunded": refund})
}

func (a *app) collectVenture(w http.ResponseWriter, r *http.Request, u user) {
	nid, err := a.nationID(r.Context(), u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Collection unavailable.")
		return
	}
	defer tx.Rollback()
	var payout int64
	if err = tx.QueryRowContext(r.Context(), `SELECT payout FROM personal_ventures WHERE id=? AND nation_id=? AND status='claimable' FOR UPDATE`, r.PathValue("id"), nid).Scan(&payout); err != nil {
		problem(w, 400, "This venture has no claimable result.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE venture_accounts SET personal_capital=personal_capital+? WHERE nation_id=?`, payout, nid); err != nil {
		problem(w, 500, "Collection unavailable.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE personal_ventures SET status='collected',collected_at=NOW() WHERE id=?`, r.PathValue("id")); err != nil {
		problem(w, 500, "Collection unavailable.")
		return
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Collection unavailable.")
		return
	}
	write(w, http.StatusOK, map[string]any{"collected": payout})
}

func (a *app) processMatureVentures(ctx context.Context) {
	rows, err := a.db.QueryContext(ctx, `SELECT id FROM personal_ventures WHERE status='active' AND matures_at<=NOW() LIMIT 500`)
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
		tx, e := a.db.BeginTx(ctx, nil)
		if e != nil {
			continue
		}
		var nid, title, risk string
		var amount int64
		e = tx.QueryRowContext(ctx, `SELECT nation_id,title,risk,amount_invested FROM personal_ventures WHERE id=? AND status='active' AND matures_at<=NOW() FOR UPDATE`, id).Scan(&nid, &title, &risk, &amount)
		if e != nil {
			tx.Rollback()
			continue
		}
		bps := ventureOutcome(risk)
		payout := int64(math.Round(float64(amount) * (1 + float64(bps)/10_000)))
		if payout < 0 {
			payout = 0
		}
		if _, e = tx.ExecContext(ctx, `UPDATE personal_ventures SET status='claimable',outcome_bps=?,payout=?,resolved_at=NOW() WHERE id=?`, bps, payout, id); e != nil {
			tx.Rollback()
			continue
		}
		result := payout - amount
		message := fmt.Sprintf("%s matured with a result of %s¥%d. ¥%d is ready to collect into Personal Capital.", title, map[bool]string{true: "+", false: ""}[result >= 0], result, payout)
		if _, e = tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'economic','Private venture matured',?)`, uuid(), nid, message); e != nil {
			tx.Rollback()
			continue
		}
		tx.Commit()
	}
	rows, err = a.db.QueryContext(ctx, `SELECT id,nation_id,payout FROM personal_ventures WHERE status='claimable' AND resolved_at<=DATE_SUB(NOW(),INTERVAL ? HOUR) LIMIT 500`, ventureAutoCollectHours)
	if err != nil {
		return
	}
	type claim struct {
		id, nid string
		payout  int64
	}
	claims := []claim{}
	for rows.Next() {
		var c claim
		if rows.Scan(&c.id, &c.nid, &c.payout) == nil {
			claims = append(claims, c)
		}
	}
	rows.Close()
	for _, c := range claims {
		tx, e := a.db.BeginTx(ctx, nil)
		if e != nil {
			continue
		}
		res, e := tx.ExecContext(ctx, `UPDATE personal_ventures SET status='collected',collected_at=NOW() WHERE id=? AND status='claimable'`, c.id)
		if e != nil {
			tx.Rollback()
			continue
		}
		affected, _ := res.RowsAffected()
		if affected != 1 {
			tx.Rollback()
			continue
		}
		if _, e = tx.ExecContext(ctx, `UPDATE venture_accounts SET personal_capital=personal_capital+? WHERE nation_id=?`, c.payout, c.nid); e != nil {
			tx.Rollback()
			continue
		}
		tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'economic','Venture proceeds auto-collected',?)`, uuid(), c.nid, fmt.Sprintf("¥%d in mature venture proceeds was moved into Personal Capital after seven days.", c.payout))
		tx.Commit()
	}
}
