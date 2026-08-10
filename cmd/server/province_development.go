package main

import (
	"context"
	"database/sql"
	"math"
	"net/http"
)

type provinceUpgradeSpec struct {
	Name, Description string
	BaseCost          float64
	HardCap           int
}

type provinceDevelopmentBalance struct {
	StartingInfrastructure, BaseUpgradeCapacity, InfrastructurePerCapacity float64
	UpgradeExponent, UpgradeInfrastructureFactor                           float64
	FoundingBaseCash, FoundingExponent                                     float64
	FoundingMaterialBase, FoundingMaterialExponent                         float64
}

var provinceDevelopment = provinceDevelopmentBalance{
	StartingInfrastructure: 100, BaseUpgradeCapacity: 5, InfrastructurePerCapacity: 50,
	UpgradeExponent: 1.72, UpgradeInfrastructureFactor: .08,
	FoundingBaseCash: 250000, FoundingExponent: 2.55,
	FoundingMaterialBase: 40, FoundingMaterialExponent: 2.15,
}

const provinceUpgradeLevelHardCap = 15

var provinceUpgradeSpecs = map[string]provinceUpgradeSpec{
	"agriculture":       {"Agricultural & Food", "Improves food and fiber output and population capacity, but intensive production adds disease and pollution pressure.", 20000, 15},
	"extraction":        {"Extraction Efficiency", "Raises all natural-deposit output while adding substantial pollution and disease pressure.", 26000, 15},
	"light_industry":    {"Light Industry", "Improves textiles, processed foods, and basic goods while adding moderate pollution and disease pressure.", 30000, 15},
	"heavy_industry":    {"Heavy Industry", "Improves construction materials, consumer goods, and luxury goods; it creates the strongest pollution and disease pressure.", 45000, 15},
	"commerce":          {"Commercial & Trade", "Raises citizen tax income and employment potential, with growing crime pressure in a larger commercial economy.", 28000, 15},
	"civil":             {"Civil & Quality of Life", "Supports effective population, Happiness, Education, and long-term social capacity.", 24000, 15},
	"military_industry": {"Military-Industrial", "Improves military-equipment production while adding pollution, disease, and crime pressure.", 50000, 15},
}

func provinceUpgradeCost(spec provinceUpgradeSpec, level int, infra float64) int64 {
	cost := spec.BaseCost * float64(yenScale) * math.Pow(float64(level+1), provinceDevelopment.UpgradeExponent) * (1 + provinceDevelopment.UpgradeInfrastructureFactor*(infra/1000))
	if level >= 12 {
		cost *= math.Pow(float64(level-11), 1.45)
	}
	return int64(math.Ceil(cost))
}

func provinceUpgradeCapacity(infra float64) int {
	raised := math.Max(0, infra-provinceDevelopment.StartingInfrastructure)
	return int(provinceDevelopment.BaseUpgradeCapacity) + int(math.Floor((raised+.000001)/provinceDevelopment.InfrastructurePerCapacity))
}

func nextProvinceUpgradeCapacityAt(infra float64) int {
	capacityGains := provinceUpgradeCapacity(infra) - int(provinceDevelopment.BaseUpgradeCapacity)
	return int(provinceDevelopment.StartingInfrastructure + float64(capacityGains+1)*provinceDevelopment.InfrastructurePerCapacity)
}

func provinceUpgradesUsed(upgrades map[string]int) int {
	used := 0
	for _, level := range upgrades {
		used += max(0, level)
	}
	return used
}

func provinceUpgradeEffect(level int) float64 {
	effect := 0.0
	for i := 1; i <= level; i++ {
		effect += 1 / (1 + math.Max(0, float64(i-8))*.65)
	}
	return effect
}

func expansionGearModifier(gear string) float64 {
	switch gear {
	case "agrarian":
		return .82
	case "industrial":
		return 1.08
	case "commercial":
		return 1.20
	case "militarized":
		return 1.30
	default:
		return 1
	}
}

func expansionPolicyModifier(policies map[string]bool) float64 {
	modifier := 1.0
	if policies["land_grants"] {
		modifier *= .88
	}
	if policies["migration_attraction"] {
		modifier *= .95
	}
	return modifier
}

func provinceFoundingCosts(count int, gear string, policies map[string]bool) (int64, float64, int) {
	n := math.Max(1, float64(count))
	cash := provinceDevelopment.FoundingBaseCash * float64(yenScale) * math.Pow(n, provinceDevelopment.FoundingExponent) * expansionGearModifier(gear) * expansionPolicyModifier(policies)
	materials := provinceDevelopment.FoundingMaterialBase * math.Pow(n, provinceDevelopment.FoundingMaterialExponent)
	strain := min(12, 2+count)
	return int64(math.Ceil(cash)), math.Ceil(materials*100) / 100, strain
}

func (a *app) buyProvinceUpgrade(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ ProvinceID, Upgrade, Action string }
	if !decode(w, r, &in) {
		return
	}
	spec, ok := provinceUpgradeSpecs[in.Upgrade]
	if !ok {
		problem(w, http.StatusBadRequest, "Unknown Province upgrade.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, http.StatusInternalServerError, "Could not begin Province investment.")
		return
	}
	defer tx.Rollback()
	var nid string
	var cash int64
	var infra float64
	var upgradesUsed int
	if err = tx.QueryRowContext(r.Context(), `SELECT n.id,n.treasury,c.infrastructure,COALESCE((SELECT SUM(level) FROM province_upgrades WHERE city_id=c.id),0) FROM nations n JOIN cities c ON c.nation_id=n.id WHERE n.owner_id=? AND c.id=? FOR UPDATE`, u.ID, in.ProvinceID).Scan(&nid, &cash, &infra, &upgradesUsed); err != nil {
		problem(w, http.StatusNotFound, "Province not found.")
		return
	}
	level := 0
	err = tx.QueryRowContext(r.Context(), `SELECT level FROM province_upgrades WHERE city_id=? AND upgrade_key=? FOR UPDATE`, in.ProvinceID, in.Upgrade).Scan(&level)
	if err != nil && err != sql.ErrNoRows {
		problem(w, http.StatusInternalServerError, "Province upgrades unavailable.")
		return
	}
	if in.Action == "downgrade" {
		if level <= 0 {
			problem(w, http.StatusConflict, "This Province upgrade is already at level 0.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE province_upgrades SET level=level-1 WHERE city_id=? AND upgrade_key=? AND level>0`, in.ProvinceID, in.Upgrade); err != nil {
			problem(w, http.StatusInternalServerError, "Could not downgrade Province upgrade.")
			return
		}
		// total_invested intentionally remains unchanged: downgrades never refund prior spending.
		tx.ExecContext(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'province_downgrade',0,?)`, uuid(), nid, "Downgraded "+spec.Name+" with no refund")
		if err = tx.Commit(); err != nil {
			problem(w, http.StatusInternalServerError, "Could not complete Province downgrade.")
			return
		}
		write(w, http.StatusOK, map[string]any{"ok": true, "cost": 0, "refund": 0, "level": level - 1})
		return
	}
	if in.Action != "" && in.Action != "upgrade" {
		problem(w, http.StatusBadRequest, "Unknown Province upgrade action.")
		return
	}
	if level >= spec.HardCap {
		problem(w, http.StatusConflict, "This Province upgrade has reached its hard cap.")
		return
	}
	capacity := provinceUpgradeCapacity(infra)
	if upgradesUsed >= capacity {
		problem(w, http.StatusConflict, "Purchase more Infrastructure to unlock another Province upgrade.")
		return
	}
	cost := provinceUpgradeCost(spec, level, infra)
	var hasInfrastructureBank int
	tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM national_long_term_projects WHERE nation_id=? AND project_type='infrastructure_bank'`, nid).Scan(&hasInfrastructureBank)
	if hasInfrastructureBank > 0 {
		cost = int64(math.Ceil(float64(cost) * .90))
	}
	if cash < cost {
		problem(w, http.StatusConflict, "Insufficient treasury for this Province upgrade.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO province_upgrades(city_id,upgrade_key,level,total_invested) VALUES(?,?,1,?) ON DUPLICATE KEY UPDATE level=level+1,total_invested=total_invested+VALUES(total_invested)`, in.ProvinceID, in.Upgrade, cost); err != nil {
		problem(w, http.StatusInternalServerError, "Could not upgrade Province.")
		return
	}
	tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, cost, nid)
	tx.ExecContext(r.Context(), `UPDATE cities SET total_invested=total_invested+? WHERE id=?`, cost, in.ProvinceID)
	tx.ExecContext(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'province_upgrade',?,?)`, uuid(), nid, -cost, "Raised "+spec.Name)
	if err = tx.Commit(); err != nil {
		problem(w, http.StatusInternalServerError, "Could not complete Province upgrade.")
		return
	}
	write(w, http.StatusOK, map[string]any{"ok": true, "cost": cost, "level": level + 1})
}

func loadProvinceUpgrades(ctx context.Context, db *database, cityID string) map[string]int {
	out := map[string]int{}
	rows, err := db.QueryContext(ctx, `SELECT upgrade_key,level FROM province_upgrades WHERE city_id=?`, cityID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var level int
		if rows.Scan(&key, &level) == nil {
			out[key] = level
		}
	}
	return out
}
