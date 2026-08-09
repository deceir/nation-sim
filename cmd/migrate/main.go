package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"os"
)

func main() {
	b, err := os.ReadFile("db/schema.sql")
	if err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("mysql", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(string(b)); err != nil {
		log.Fatal(err)
	}
	upgrades := []struct{ table, column, definition string }{
		{"sessions", "last_action_at", "TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)"},
		{"users", "theme_preference", "ENUM('dark','light') NOT NULL DEFAULT 'dark'"},
		{"nations", "leader_name", "VARCHAR(100) NOT NULL DEFAULT 'Unknown Leader'"},
		{"nations", "government_type", "VARCHAR(60) NOT NULL DEFAULT 'Presidential Republic'"},
		{"nations", "continent", "VARCHAR(30) NOT NULL DEFAULT 'Asia'"},
		{"nations", "location_lat", "DECIMAL(9,6) NULL"},
		{"nations", "location_lng", "DECIMAL(9,6) NULL"},
		{"nations", "user_type", "ENUM('PLAYER','DEV','BOT') NOT NULL DEFAULT 'PLAYER'"},
		{"nations", "technology_progress", "DECIMAL(8,4) NOT NULL DEFAULT 0"},
		{"nations", "employment_rate", "DECIMAL(5,2) NOT NULL DEFAULT 72.00"},
		{"nations", "tax_rate", "DECIMAL(5,2) NOT NULL DEFAULT 25.00"},
		{"nations", "doctrine", "VARCHAR(30) NOT NULL DEFAULT 'Balanced'"},
		{"nations", "oil", "BIGINT NOT NULL DEFAULT 250"},
		{"nations", "iron", "BIGINT NOT NULL DEFAULT 500"},
		{"nations", "bauxite", "BIGINT NOT NULL DEFAULT 250"},
		{"nations", "lead_resource", "BIGINT NOT NULL DEFAULT 100"},
		{"nations", "uranium", "BIGINT NOT NULL DEFAULT 0"},
		{"nations", "aluminum", "BIGINT NOT NULL DEFAULT 0"},
		{"nations", "gasoline", "BIGINT NOT NULL DEFAULT 0"},
		{"nations", "munitions", "BIGINT NOT NULL DEFAULT 0"},
		{"cities", "total_invested", "BIGINT NOT NULL DEFAULT 0"},
		{"cities", "improvement_slots", "INT NOT NULL DEFAULT 2"},
		{"cities", "population_capacity", "BIGINT NOT NULL DEFAULT 100000"},
		{"cities", "created_at", "TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)"},
		{"cities", "land", "INT NOT NULL DEFAULT 150"},
		{"cities", "local_population", "BIGINT NOT NULL DEFAULT 9000"},
		{"cities", "commerce_percent", "DECIMAL(7,3) NOT NULL DEFAULT 0"},
		{"cities", "power_capacity", "DECIMAL(12,3) NOT NULL DEFAULT 0"},
		{"cities", "power_usage", "DECIMAL(12,3) NOT NULL DEFAULT 0"},
		{"cities", "pollution", "DECIMAL(7,3) NOT NULL DEFAULT 0"},
		{"cities", "disease_rate", "DECIMAL(7,5) NOT NULL DEFAULT 0.02"},
		{"cities", "crime_rate", "DECIMAL(7,5) NOT NULL DEFAULT 0.03"},
		{"market_orders", "channel", "ENUM('public','private') NOT NULL DEFAULT 'public'"},
		{"market_orders", "target_nation_id", "CHAR(36) NULL"},
		{"market_orders", "escrow_cash", "BIGINT NOT NULL DEFAULT 0"},
		{"market_orders", "escrow_goods", "DECIMAL(20,3) NOT NULL DEFAULT 0"},
		{"alliance_roles", "default_key", "ENUM('leader','member','applicant') NULL"},
		{"alliance_roles", "can_view_bank", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"alliance_roles", "can_deposit_bank", "BOOLEAN NOT NULL DEFAULT TRUE"},
		{"alliance_roles", "can_withdraw_bank", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"alliance_roles", "can_accept_applicants", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"alliance_roles", "can_remove_members", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"alliance_roles", "can_edit_details", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"alliance_roles", "can_manage_roles", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"alliance_roles", "can_promote_members", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"alliance_roles", "can_view_audit_log", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"alliance_tax_brackets", "nation_id", "CHAR(36) NULL"},
		{"alliance_treaties", "proposed_by_alliance_id", "CHAR(36) NULL"},
		{"alliance_treaties", "proposed_by_nation_id", "CHAR(36) NULL"},
		{"alliance_treaties", "duration_days", "INT NULL"},
		{"alliance_treaties", "starts_on", "DATE NULL"},
		{"alliance_treaties", "ends_on", "DATE NULL"},
		{"alliance_treaties", "resolved_by_nation_id", "CHAR(36) NULL"},
		{"alliance_treaties", "resolved_at", "TIMESTAMP(6) NULL"},
		{"trade_shipments", "order_id", "CHAR(36) NULL"},
		{"trade_shipments", "seller_nation_id", "CHAR(36) NOT NULL"},
		{"trade_shipments", "buyer_nation_id", "CHAR(36) NOT NULL"},
		{"trade_shipments", "resource", "VARCHAR(40) NOT NULL"},
		{"trade_shipments", "quantity", "DECIMAL(20,3) NOT NULL"},
		{"trade_shipments", "unit_price", "BIGINT NOT NULL"},
		{"trade_shipments", "goods_value", "BIGINT NOT NULL"},
		{"trade_shipments", "shipping_fee", "BIGINT NOT NULL"},
		{"trade_shipments", "distance_modifier", "DECIMAL(6,3) NOT NULL"},
		{"trade_shipments", "risk_percent", "DECIMAL(6,3) NOT NULL"},
		{"trade_shipments", "turns_total", "INT NOT NULL"},
		{"trade_shipments", "turns_remaining", "INT NOT NULL"},
		{"trade_shipments", "delay_count", "INT NOT NULL DEFAULT 0"},
		{"trade_shipments", "origin_lat", "DECIMAL(9,6) NOT NULL"},
		{"trade_shipments", "origin_lng", "DECIMAL(9,6) NOT NULL"},
		{"trade_shipments", "destination_lat", "DECIMAL(9,6) NOT NULL"},
		{"trade_shipments", "destination_lng", "DECIMAL(9,6) NOT NULL"},
		{"trade_shipments", "departed_at", "TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)"},
		{"trade_shipments", "estimated_arrival_at", "TIMESTAMP(6) NOT NULL"},
		{"trade_shipments", "delivered_at", "TIMESTAMP(6) NULL"},
		{"trade_shipments", "status", "ENUM('in_transit','delivered','delayed','cancelled') NOT NULL DEFAULT 'in_transit'"},
	}
	for _, u := range upgrades {
		if err = ensureColumn(db, u.table, u.column, u.definition); err != nil {
			log.Fatal(err)
		}
	}
	if err = ensureUniqueIndex(db, "nations", "uq_nations_leader_name", "leader_name"); err != nil {
		log.Fatal(err)
	}
	if err = ensureUniqueIndex(db, "alliance_tax_brackets", "uq_alliance_tax_nation", "nation_id"); err != nil {
		log.Fatal(err)
	}
	if _, err = db.Exec(`ALTER TABLE nations ALTER COLUMN currency_name SET DEFAULT 'Yen'`); err != nil {
		log.Fatal(err)
	}
	for _, q := range []string{
		`ALTER TABLE market_orders MODIFY resource VARCHAR(40) NOT NULL`,
		`ALTER TABLE market_orders MODIFY quantity DECIMAL(20,3) NOT NULL`,
		`ALTER TABLE market_orders MODIFY remaining DECIMAL(20,3) NOT NULL`,
		`ALTER TABLE market_orders MODIFY status ENUM('open','pending','filled','cancelled','rejected') NOT NULL DEFAULT 'open'`,
		`ALTER TABLE trade_shipments MODIFY resource VARCHAR(40) NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY quantity DECIMAL(20,3) NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY unit_price BIGINT NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY goods_value BIGINT NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY shipping_fee BIGINT NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY distance_modifier DECIMAL(6,3) NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY risk_percent DECIMAL(6,3) NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY origin_lat DECIMAL(9,6) NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY origin_lng DECIMAL(9,6) NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY destination_lat DECIMAL(9,6) NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY destination_lng DECIMAL(9,6) NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY departed_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)`,
		`ALTER TABLE trade_shipments MODIFY estimated_arrival_at TIMESTAMP(6) NOT NULL`,
		`ALTER TABLE trade_shipments MODIFY delivered_at TIMESTAMP(6) NULL`,
		`ALTER TABLE trade_shipments MODIFY status ENUM('in_transit','delivered','delayed','cancelled') NOT NULL DEFAULT 'in_transit'`,
		`ALTER TABLE alliance_treaties MODIFY treaty_type VARCHAR(16) NOT NULL`,
		`ALTER TABLE alliance_treaties MODIFY status ENUM('proposed','active','rejected','cancelled','expired') NOT NULL DEFAULT 'proposed'`,
	} {
		if _, err = db.Exec(q); err != nil {
			log.Fatal(err)
		}
	}
	bootstrap := []string{
		`UPDATE alliance_roles r JOIN (SELECT alliance_id,MAX(rank_order) mx FROM alliance_roles GROUP BY alliance_id) x ON x.alliance_id=r.alliance_id AND x.mx=r.rank_order SET r.default_key='leader',r.can_view_bank=1,r.can_deposit_bank=1,r.can_withdraw_bank=1,r.can_accept_applicants=1,r.can_remove_members=1,r.can_edit_details=1,r.can_manage_roles=1,r.can_promote_members=1,r.can_view_audit_log=1`,
		`UPDATE alliance_roles r JOIN (SELECT alliance_id,MIN(rank_order) mn FROM alliance_roles WHERE default_key IS NULL GROUP BY alliance_id) x ON x.alliance_id=r.alliance_id AND x.mn=r.rank_order SET r.default_key='member',r.can_deposit_bank=1`,
		`INSERT INTO alliance_roles(id,alliance_id,title,rank_order,default_key,can_deposit_bank) SELECT UUID(),a.id,'Applicant',0,'applicant',0 FROM alliances a WHERE NOT EXISTS(SELECT 1 FROM alliance_roles r WHERE r.alliance_id=a.id AND r.default_key='applicant')`,
		`INSERT INTO alliance_tax_brackets(id,alliance_id,name,is_default,cash_rate,resource_rate) SELECT UUID(),a.id,'Default',1,a.tax_rate,0 FROM alliances a WHERE NOT EXISTS(SELECT 1 FROM alliance_tax_brackets b WHERE b.alliance_id=a.id AND b.is_default=1)`,
		`UPDATE alliance_tax_brackets SET role_id=NULL WHERE role_id IS NOT NULL`,
		`UPDATE alliance_treaties SET proposed_by_alliance_id=alliance_a_id WHERE proposed_by_alliance_id IS NULL`,
		`UPDATE alliance_treaties t JOIN alliances a ON a.id=t.proposed_by_alliance_id SET t.proposed_by_nation_id=a.founder_nation_id WHERE t.proposed_by_nation_id IS NULL`,
		`UPDATE market_orders SET status='cancelled' WHERE status IN('open','pending') AND escrow_cash=0 AND escrow_goods=0`,
		`INSERT IGNORE INTO nation_economic_strategy(nation_id) SELECT id FROM nations`,
		`INSERT IGNORE INTO province_economies(city_id,latitude,longitude) SELECT c.id,CASE n.continent WHEN 'Africa' THEN 5 WHEN 'Asia' THEN 34 WHEN 'Europe' THEN 50 WHEN 'North America' THEN 40 WHEN 'South America' THEN -15 WHEN 'Oceania' THEN -25 ELSE -75 END,CASE n.continent WHEN 'Africa' THEN 20 WHEN 'Asia' THEN 100 WHEN 'Europe' THEN 15 WHEN 'North America' THEN -100 WHEN 'South America' THEN -60 WHEN 'Oceania' THEN 135 ELSE 0 END FROM cities c JOIN nations n ON n.id=c.nation_id`,
		`INSERT IGNORE INTO province_deposits(city_id,resource,richness) SELECT c.id,r.resource,CASE r.resource WHEN 'foodstuffs' THEN 1.15 WHEN 'timber' THEN IF(n.continent IN('South America','North America'),1.35,.85) WHEN 'fibers' THEN IF(n.continent IN('Asia','Africa'),1.3,.8) WHEN 'basic_metals' THEN IF(n.continent IN('Africa','South America'),1.3,.9) WHEN 'energy' THEN IF(n.continent IN('Asia','North America'),1.25,.85) ELSE IF(n.continent IN('Africa','Oceania'),1.25,.75) END FROM cities c JOIN nations n ON n.id=c.nation_id CROSS JOIN (SELECT 'foodstuffs' resource UNION ALL SELECT 'timber' UNION ALL SELECT 'fibers' UNION ALL SELECT 'basic_metals' UNION ALL SELECT 'energy' UNION ALL SELECT 'strategic_minerals') r`,
		`INSERT IGNORE INTO nation_stockpiles(nation_id,commodity,amount) SELECT n.id,r.commodity,CASE WHEN r.commodity IN('foodstuffs','timber','fibers','basic_metals','energy') THEN 500 ELSE 0 END FROM nations n CROSS JOIN (SELECT 'foodstuffs' commodity UNION ALL SELECT 'timber' UNION ALL SELECT 'fibers' UNION ALL SELECT 'basic_metals' UNION ALL SELECT 'energy' UNION ALL SELECT 'strategic_minerals' UNION ALL SELECT 'textiles' UNION ALL SELECT 'processed_foods' UNION ALL SELECT 'construction_materials' UNION ALL SELECT 'basic_goods' UNION ALL SELECT 'consumer_goods' UNION ALL SELECT 'military_equipment' UNION ALL SELECT 'luxury_goods') r`,
		`UPDATE nation_stockpiles SET amount=GREATEST(amount,CASE commodity WHEN 'construction_materials' THEN 75 WHEN 'processed_foods' THEN 100 WHEN 'basic_goods' THEN 75 ELSE amount END)`,
		`INSERT INTO production_quotas(nation_id,commodity,priority) SELECT n.id,q.commodity,q.priority FROM nations n CROSS JOIN (SELECT 'processed_foods' commodity,35 priority UNION ALL SELECT 'construction_materials',45 UNION ALL SELECT 'basic_goods',20) q WHERE NOT EXISTS(SELECT 1 FROM production_quotas x WHERE x.nation_id=n.id)`,
	}
	for _, q := range bootstrap {
		if _, err = db.Exec(q); err != nil {
			log.Fatal(err)
		}
	}
	if err = applyYenRedenomination(db); err != nil {
		log.Fatal(err)
	}
	log.Print("schema ready")
}

func applyYenRedenomination(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT IGNORE INTO balance_migrations(migration_key) VALUES('yen_scale_v1_100x')`)
	if err != nil {
		return err
	}
	applied, err := result.RowsAffected()
	if err != nil || applied == 0 {
		return err
	}
	statements := []string{
		`UPDATE nations SET treasury=treasury*100`,
		`UPDATE cities SET total_invested=total_invested*100`,
		`UPDATE city_investments SET amount=amount*100`,
		`UPDATE city_industries SET total_invested=total_invested*100`,
		`UPDATE market_orders SET unit_price=unit_price*100,escrow_cash=escrow_cash*100`,
		`UPDATE trade_shipments SET unit_price=unit_price*100,goods_value=goods_value*100,shipping_fee=shipping_fee*100`,
		`UPDATE ledger_entries SET amount=amount*100`,
		`UPDATE daily_login_rewards SET amount=amount*100`,
		`UPDATE national_project_construction SET cash_locked=cash_locked*100`,
		`UPDATE economic_snapshots SET cash_income=cash_income*100,upkeep=upkeep*100`,
		`UPDATE alliance_roles SET daily_withdrawal_limit=daily_withdrawal_limit*100`,
		`UPDATE alliance_members SET cash_contributed=cash_contributed*100`,
		`UPDATE alliance_bank SET cash=cash*100`,
		`UPDATE alliance_bank_transactions SET amount=amount*100 WHERE resource='cash'`,
		`UPDATE alliance_loans SET principal=principal*100,outstanding=outstanding*100`,
	}
	for _, statement := range statements {
		if _, err = tx.Exec(statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ensureColumn(db *sql.DB, table, column, definition string) error {
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?`, table, column).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.Exec("ALTER TABLE `" + table + "` ADD COLUMN `" + column + "` " + definition)
	return err
}

func ensureUniqueIndex(db *sql.DB, table, index, column string) error {
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND INDEX_NAME=?`, table, index).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.Exec("ALTER TABLE `" + table + "` ADD UNIQUE INDEX `" + index + "` (`" + column + "`)")
	return err
}
