package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type botNation struct {
	Slug, Name, Leader, Capital, Government, Continent, Motto string
}

var bots = []botNation{
	{"asterian-federation", "THE ASTERIAN FEDERATION", "Elena Marcek", "NOVA ASTER", "Federal Republic", "Europe", "Many regions, one republic."},
	{"kingdom-valoria", "KINGDOM OF VALORIA", "King Adrian IV", "CROWNPORT", "Constitutional Monarchy", "Europe", "Continuity, service, and law."},
	{"republic-sundara", "REPUBLIC OF SUNDARA", "Maya Raman", "SURYA CITY", "Parliamentary Democracy", "Asia", "Prosperity through open institutions."},
	{"union-karsovia", "UNION OF KARSOVIA", "Viktor Orlov", "KARSGRAD", "One-Party State", "Europe", "Industry sustains sovereignty."},
	{"commonwealth-oriona", "COMMONWEALTH OF ORIONA", "Amara Okafor", "NEW HORIZON", "Presidential Republic", "Africa", "A future shared by every province."},
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "diplomatia:diplomatia@tcp(localhost:3306)/diplomatia?parseTime=true"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err = db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	created, skipped := 0, 0
	for _, bot := range bots {
		if strings.Contains(strings.ToUpper(bot.Name), "JAPAN") {
			log.Fatalf("reserved nation detected: %s", bot.Name)
		}
		if bot.Name != strings.ToUpper(bot.Name) {
			log.Fatalf("bot nation name must be uppercase: %s", bot.Name)
		}
		wasCreated, err := feedBot(ctx, db, bot)
		if err != nil {
			log.Fatalf("feed %s: %v", bot.Name, err)
		}
		if wasCreated {
			created++
			log.Printf("created %s", bot.Name)
		} else {
			skipped++
			log.Printf("already exists: %s", bot.Name)
		}
	}
	log.Printf("bot feeder complete: %d created, %d already present", created, skipped)
}

func feedBot(ctx context.Context, db *sql.DB, bot botNation) (bool, error) {
	var existing int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM nations WHERE name=?`, bot.Name).Scan(&existing); err != nil {
		return false, err
	}
	if existing > 0 {
		if _, err := db.ExecContext(ctx, `UPDATE nations SET user_type='BOT' WHERE name=?`, bot.Name); err != nil { return false, err }
		return false, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	uid, nid, cid, gid := uuid(), uuid(), uuid(), uuid()
	password := make([]byte, 32)
	if _, err = rand.Read(password); err != nil {
		return false, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(password)), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}
	email := fmt.Sprintf("bot-%s@diplomatia.invalid", bot.Slug)
	if _, err = tx.ExecContext(ctx, `INSERT INTO users(id,email,password_hash) VALUES(?,?,?)`, uid, email, string(hash)); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO nations(id,owner_id,name,leader_name,government_type,continent,motto,currency_name,user_type) VALUES(?,?,?,?,?,?,?,'Yen','BOT')`, nid, uid, bot.Name, bot.Leader, bot.Government, bot.Continent, bot.Motto); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO cities(id,nation_id,name) VALUES(?,?,?)`, cid, nid, bot.Capital); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO guardian_grants(id,nation_id,starts_at,expires_at,reason,granted_by) VALUES(?,?,NOW(),DATE_ADD(NOW(),INTERVAL 30 DAY),'seeded_bot','system')`, gid, nid); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func uuid() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b)
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
