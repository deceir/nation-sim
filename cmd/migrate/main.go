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
	log.Print("schema ready")
}
