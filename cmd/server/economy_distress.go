package main

import (
	"context"
	"database/sql"
	"math"
)

const economicDistressPenalty = .50

type economicDistressStatus struct {
	FoodShortage           bool    `json:"foodShortage"`
	UpkeepDefault          bool    `json:"upkeepDefault"`
	ProductivityMultiplier float64 `json:"productivityMultiplier"`
	HourlyFoodRequired     float64 `json:"hourlyFoodRequired"`
	HourlyFoodAvailable    float64 `json:"hourlyFoodAvailable"`
	HourlyCashUpkeep       float64 `json:"hourlyCashUpkeep"`
	CashAvailableForUpkeep float64 `json:"cashAvailableForUpkeep"`
}

func assessEconomicDistress(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, nationID string, hourlyFoodRequired, hourlyFoodProduction, hourlyCashIncome, hourlyCashUpkeep float64) economicDistressStatus {
	var treasury int64
	var food float64
	_ = q.QueryRowContext(ctx, `SELECT treasury,COALESCE((SELECT amount FROM nation_stockpiles WHERE nation_id=n.id AND commodity='foodstuffs'),0) FROM nations n WHERE n.id=?`, nationID).Scan(&treasury, &food)
	status := economicDistressStatus{
		ProductivityMultiplier: 1,
		HourlyFoodRequired:     math.Max(0, hourlyFoodRequired),
		HourlyFoodAvailable:    math.Max(0, food+hourlyFoodProduction),
		HourlyCashUpkeep:       math.Max(0, hourlyCashUpkeep),
		CashAvailableForUpkeep: math.Max(0, float64(treasury)+hourlyCashIncome),
	}
	status.FoodShortage = status.HourlyFoodAvailable+0.000001 < status.HourlyFoodRequired
	status.UpkeepDefault = status.CashAvailableForUpkeep+0.000001 < status.HourlyCashUpkeep
	if status.FoodShortage || status.UpkeepDefault {
		status.ProductivityMultiplier = economicDistressPenalty
	}
	return status
}

func applyEconomicDistress(result *strategicResult, status economicDistressStatus) {
	multiplier := status.ProductivityMultiplier
	if multiplier <= 0 || multiplier >= 1 {
		return
	}
	result.IncomeMultiplier *= multiplier
	result.CommerceMultiplier *= multiplier
	result.ExtractionMultiplier *= multiplier
	result.IndustryMultiplier *= multiplier
	result.MilitaryMultiplier *= multiplier
	for resource := range result.Production {
		result.Production[resource] *= multiplier
	}
	for province := range result.ProvinceProduction {
		for resource := range result.ProvinceProduction[province] {
			result.ProvinceProduction[province][resource] *= multiplier
		}
	}
}
