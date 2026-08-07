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
		{"cities", "total_invested", "BIGINT NOT NULL DEFAULT 0"},
		{"cities", "improvement_slots", "INT NOT NULL DEFAULT 2"},
		{"cities", "population_capacity", "BIGINT NOT NULL DEFAULT 100000"},
		{"cities", "created_at", "TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)"},
	}
	for _, u := range upgrades {
		if err = ensureColumn(db, u.table, u.column, u.definition); err != nil {
			log.Fatal(err)
		}
	}
	if err = ensureUniqueIndex(db, "nations", "uq_nations_leader_name", "leader_name"); err != nil { log.Fatal(err) }
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
	if err := db.QueryRow(`SELECT count(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND INDEX_NAME=?`, table, index).Scan(&count); err != nil { return err }
	if count > 0 { return nil }
	_, err := db.Exec("ALTER TABLE `" + table + "` ADD UNIQUE INDEX `" + index + "` (`" + column + "`)")
	return err
}
