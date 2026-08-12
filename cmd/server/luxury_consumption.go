package main

import (
	"context"
	"database/sql"
	"math"
	"net/http"
	"time"
)

type luxuryConsumptionConfig struct {
	BaseValue           float64
	PopulationRef       float64
	ProvinceRef         float64
	MinEfficiency       float64
	MaxEfficiency       float64
	BaseCap             float64
	PopulationCapFactor float64
	ProvinceCapFactor   float64
}

// This is the complete balance surface for Luxury Consumption. BaseValue is
// treasury earned per tonne before the nation-size efficiency multiplier.
var luxuryBalance = luxuryConsumptionConfig{
	BaseValue: 35, PopulationRef: 2_000_000, ProvinceRef: 10,
	MinEfficiency: .25, MaxEfficiency: 2.25,
	BaseCap: 20, PopulationCapFactor: .00004, ProvinceCapFactor: 12,
}

type luxuryConsumptionView struct {
	Unlocked             bool    `json:"unlocked"`
	RequiredProject      string  `json:"requiredProject"`
	DailyRate            float64 `json:"dailyRate"`
	MaxRate              float64 `json:"maxRate"`
	Stockpile            float64 `json:"stockpile"`
	SizeEfficiency       float64 `json:"sizeEfficiency"`
	ValuePerGood         float64 `json:"valuePerGood"`
	ProjectedConsumption float64 `json:"projectedConsumption"`
	ProjectedIncome      int64   `json:"projectedIncome"`
	SettledToday         bool    `json:"settledToday"`
	YesterdayConsumed    float64 `json:"yesterdayConsumed"`
	YesterdayIncome      int64   `json:"yesterdayIncome"`
}

func luxurySizeEfficiency(effectivePopulation float64, provinces int) float64 {
	return clamp(effectivePopulation/luxuryBalance.PopulationRef+float64(provinces)/luxuryBalance.ProvinceRef, luxuryBalance.MinEfficiency, luxuryBalance.MaxEfficiency)
}

func luxuryMaxConsumptionRate(effectivePopulation float64, provinces int) float64 {
	return math.Max(0, luxuryBalance.BaseCap+effectivePopulation*luxuryBalance.PopulationCapFactor+float64(provinces)*luxuryBalance.ProvinceCapFactor)
}

func projectedLuxuryConsumption(rate, stockpile, effectivePopulation float64, provinces int) (float64, float64, int64) {
	efficiency := luxurySizeEfficiency(effectivePopulation, provinces)
	consumed := math.Min(math.Max(0, rate), math.Max(0, stockpile))
	income := int64(math.Floor(consumed * luxuryBalance.BaseValue * efficiency))
	return consumed, efficiency, income
}

func (a *app) luxuryConsumptionDashboard(ctx context.Context, nationID string, effectivePopulation float64, provinces int) luxuryConsumptionView {
	view := luxuryConsumptionView{RequiredProject: "Luxury Market Authority", MaxRate: luxuryMaxConsumptionRate(effectivePopulation, provinces)}
	_ = a.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM national_long_term_projects WHERE nation_id=? AND project_type='luxury_market_authority')`, nationID).Scan(&view.Unlocked)
	_ = a.db.QueryRowContext(ctx, `SELECT daily_rate FROM luxury_consumption_settings WHERE nation_id=?`, nationID).Scan(&view.DailyRate)
	_ = a.db.QueryRowContext(ctx, `SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity='luxury_goods'`, nationID).Scan(&view.Stockpile)
	if !view.Unlocked {
		view.DailyRate = 0
	}
	view.ProjectedConsumption, view.SizeEfficiency, view.ProjectedIncome = projectedLuxuryConsumption(view.DailyRate, view.Stockpile, effectivePopulation, provinces)
	view.ValuePerGood = luxuryBalance.BaseValue * view.SizeEfficiency
	today := time.Now().UTC().Format("2006-01-02")
	if err := a.db.QueryRowContext(ctx, `SELECT actual_consumed,income_earned FROM luxury_consumption_history WHERE nation_id=? AND server_date=?`, nationID, today).Scan(&view.ProjectedConsumption, &view.ProjectedIncome); err == nil {
		view.SettledToday = true
	}
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	_ = a.db.QueryRowContext(ctx, `SELECT actual_consumed,income_earned FROM luxury_consumption_history WHERE nation_id=? AND server_date=?`, nationID, yesterday).Scan(&view.YesterdayConsumed, &view.YesterdayIncome)
	return view
}

func (a *app) setLuxuryConsumptionRate(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		DailyRate float64 `json:"dailyRate"`
	}
	if !decode(w, r, &in) || math.IsNaN(in.DailyRate) || math.IsInf(in.DailyRate, 0) || in.DailyRate < 0 {
		problem(w, http.StatusBadRequest, "Enter a valid non-negative daily consumption rate.")
		return
	}
	n, nationID, _, err := a.loadEconomicNationContext(r.Context(), u.ID)
	if err != nil {
		problem(w, http.StatusNotFound, "Nation not found.")
		return
	}
	result := calculateEconomy(n)
	if !n.LongTermProjects["luxury_market_authority"] {
		problem(w, http.StatusConflict, "Complete the Luxury Market Authority National Project first.")
		return
	}
	cap := luxuryMaxConsumptionRate(result.Population, len(result.Cities))
	if in.DailyRate > cap+.0001 {
		problem(w, http.StatusBadRequest, "Daily Luxury Consumption cannot exceed the current national cap.")
		return
	}
	if _, err = a.db.ExecContext(r.Context(), `INSERT INTO luxury_consumption_settings(nation_id,daily_rate) VALUES(?,?) ON DUPLICATE KEY UPDATE daily_rate=VALUES(daily_rate)`, nationID, in.DailyRate); err != nil {
		problem(w, http.StatusInternalServerError, "Unable to save Luxury Consumption.")
		return
	}
	write(w, http.StatusOK, map[string]any{"dailyRate": in.DailyRate, "maxRate": cap})
}

func settleLuxuryConsumption(ctx context.Context, tx *sql.Tx, nationID string, effectivePopulation float64, provinces int, turn time.Time) (int64, float64, error) {
	var unlocked bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM national_long_term_projects WHERE nation_id=? AND project_type='luxury_market_authority')`, nationID).Scan(&unlocked); err != nil || !unlocked {
		return 0, 0, err
	}
	serverDate := turn.UTC().Format("2006-01-02")
	var alreadySettled int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM luxury_consumption_history WHERE nation_id=? AND server_date=?`, nationID, serverDate).Scan(&alreadySettled); err != nil || alreadySettled > 0 {
		return 0, 0, err
	}
	var rate float64
	err := tx.QueryRowContext(ctx, `SELECT daily_rate FROM luxury_consumption_settings WHERE nation_id=? FOR UPDATE`, nationID).Scan(&rate)
	if err == sql.ErrNoRows {
		rate = 0
	} else if err != nil {
		return 0, 0, err
	}
	cap := luxuryMaxConsumptionRate(effectivePopulation, provinces)
	rate = math.Min(rate, cap)
	if _, err = tx.ExecContext(ctx, `INSERT IGNORE INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,'luxury_goods',0)`, nationID); err != nil {
		return 0, 0, err
	}
	var stockpile float64
	if err = tx.QueryRowContext(ctx, `SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity='luxury_goods' FOR UPDATE`, nationID).Scan(&stockpile); err != nil {
		return 0, 0, err
	}
	consumed, efficiency, income := projectedLuxuryConsumption(rate, stockpile, effectivePopulation, provinces)
	if consumed > 0 {
		if _, err = tx.ExecContext(ctx, `UPDATE nation_stockpiles SET amount=amount-? WHERE nation_id=? AND commodity='luxury_goods'`, consumed, nationID); err != nil {
			return 0, 0, err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO luxury_consumption_history(nation_id,server_date,requested_rate,actual_consumed,size_efficiency,income_earned) VALUES(?,?,?,?,?,?)`, nationID, serverDate, rate, consumed, efficiency, income)
	return income, consumed, err
}
