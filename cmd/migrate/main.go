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
		{"nations", "leader_name", "VARCHAR(100) NOT NULL DEFAULT 'Unknown Leader'"},
		{"nations", "government_type", "VARCHAR(60) NOT NULL DEFAULT 'Presidential Republic'"},
		{"nations", "continent", "VARCHAR(30) NOT NULL DEFAULT 'Asia'"},
		{"nations", "user_type", "ENUM('PLAYER','DEV','BOT') NOT NULL DEFAULT 'PLAYER'"},
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
	}
	for _, u := range upgrades {
		if err = ensureColumn(db, u.table, u.column, u.definition); err != nil {
			log.Fatal(err)
		}
	}
	if err = ensureUniqueIndex(db, "nations", "uq_nations_leader_name", "leader_name"); err != nil {
		log.Fatal(err)
	}
	if _, err = db.Exec(`ALTER TABLE nations ALTER COLUMN currency_name SET DEFAULT 'Yen'`); err != nil {
		log.Fatal(err)
	}
	log.Print("schema ready")
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
