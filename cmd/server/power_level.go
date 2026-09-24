package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
)

const (
	powerProvinceWeight       = 500.0
	powerInfrastructureWeight = 25.0
	powerProjectWeight        = 300.0
	powerSoldierWeight        = 0.02
	powerTankWeight           = 2.0
	powerShipWeight           = 10.0
	powerJetWeight            = 6.0
	powerDroneWeight          = 3.0
)

type powerLevelComponents struct {
	Provinces, Projects                  int
	Infrastructure                       float64
	Soldiers, Tanks, Ships, Jets, Drones int64
}

type powerLevelBreakdown struct {
	Provinces      int64 `json:"provinces"`
	Infrastructure int64 `json:"infrastructure"`
	Projects       int64 `json:"projects"`
	Military       int64 `json:"military"`
	Total          int64 `json:"total"`
}

func calculatePowerLevel(c powerLevelComponents) powerLevelBreakdown {
	provinceScore := powerProvinceWeight * float64(max(0, c.Provinces))
	infrastructureScore := powerInfrastructureWeight * math.Sqrt(math.Max(0, c.Infrastructure))
	projectScore := powerProjectWeight * float64(max(0, c.Projects))
	militaryScore := powerSoldierWeight*float64(max(int64(0), c.Soldiers)) +
		powerTankWeight*float64(max(int64(0), c.Tanks)) +
		powerShipWeight*float64(max(int64(0), c.Ships)) +
		powerJetWeight*float64(max(int64(0), c.Jets)) +
		powerDroneWeight*float64(max(int64(0), c.Drones))
	breakdown := powerLevelBreakdown{
		Provinces:      int64(math.Round(provinceScore)),
		Infrastructure: int64(math.Round(infrastructureScore)),
		Projects:       int64(math.Round(projectScore)),
		Military:       int64(math.Round(militaryScore)),
	}
	breakdown.Total = breakdown.Provinces + breakdown.Infrastructure + breakdown.Projects + breakdown.Military
	return breakdown
}

// powerLevelSQL keeps database rankings on the same coefficients as profile calculations.
func powerLevelSQL(cityAlias, projectAlias, militaryAlias string) string {
	return fmt.Sprintf(`CAST(
		ROUND(%g*COALESCE(%s.province_count,0))+
		ROUND(%g*SQRT(GREATEST(0,COALESCE(%s.total_infrastructure,0))))+
		ROUND(%g*COALESCE(%s.project_count,0))+
		ROUND(%g*COALESCE(%s.soldiers,0)+%g*COALESCE(%s.tanks,0)+%g*COALESCE(%s.ships,0)+%g*COALESCE(%s.jets,0)+%g*COALESCE(%s.drones,0))
	AS SIGNED)`, powerProvinceWeight, cityAlias, powerInfrastructureWeight, cityAlias, powerProjectWeight, projectAlias,
		powerSoldierWeight, militaryAlias, powerTankWeight, militaryAlias, powerShipWeight, militaryAlias, powerJetWeight, militaryAlias, powerDroneWeight, militaryAlias)
}

func loadNationPowerLevel(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, nationID string) (powerLevelBreakdown, error) {
	var c powerLevelComponents
	err := q.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM cities WHERE nation_id=?),
		(SELECT COALESCE(SUM(infrastructure),0) FROM cities WHERE nation_id=?),
		(SELECT (SELECT COUNT(*) FROM national_projects WHERE nation_id=?)+(SELECT COUNT(*) FROM national_long_term_projects WHERE nation_id=?)),
		COALESCE((SELECT SUM(CASE WHEN unit_type='soldiers' THEN quantity ELSE 0 END) FROM military_inventory WHERE nation_id=?),0),
		COALESCE((SELECT SUM(CASE WHEN unit_type='tanks' THEN quantity ELSE 0 END) FROM military_inventory WHERE nation_id=?),0)+COALESCE((SELECT CAST(SUM(escrow_goods) AS SIGNED) FROM market_orders WHERE nation_id=? AND side='sell' AND status IN('open','pending') AND resource='tanks'),0),
		COALESCE((SELECT SUM(CASE WHEN unit_type='ships' THEN quantity ELSE 0 END) FROM military_inventory WHERE nation_id=?),0)+COALESCE((SELECT CAST(SUM(escrow_goods) AS SIGNED) FROM market_orders WHERE nation_id=? AND side='sell' AND status IN('open','pending') AND resource='ships'),0),
		COALESCE((SELECT SUM(CASE WHEN unit_type='jets' THEN quantity ELSE 0 END) FROM military_inventory WHERE nation_id=?),0)+COALESCE((SELECT CAST(SUM(escrow_goods) AS SIGNED) FROM market_orders WHERE nation_id=? AND side='sell' AND status IN('open','pending') AND resource='jets'),0),
		COALESCE((SELECT SUM(CASE WHEN unit_type='drones' THEN quantity ELSE 0 END) FROM military_inventory WHERE nation_id=?),0)+COALESCE((SELECT CAST(SUM(escrow_goods) AS SIGNED) FROM market_orders WHERE nation_id=? AND side='sell' AND status IN('open','pending') AND resource='drones'),0)`,
		nationID, nationID, nationID, nationID, nationID, nationID, nationID, nationID, nationID, nationID, nationID, nationID, nationID).Scan(
		&c.Provinces, &c.Infrastructure, &c.Projects, &c.Soldiers, &c.Tanks, &c.Ships, &c.Jets, &c.Drones)
	return calculatePowerLevel(c), err
}
