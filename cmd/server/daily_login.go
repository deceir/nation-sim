package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

const dailyLoginReward int64 = 25000

func loginRewardForStreak(streak int) int64 {
	return min(int64(50000), dailyLoginReward+int64(max(0, streak-1))*5000)
}

func nextLoginStreak(last time.Time, previous int, today time.Time) int {
	if last.UTC().Format("2006-01-02") == today.UTC().AddDate(0, 0, -1).Format("2006-01-02") {
		return previous + 1
	}
	return 1
}

func (a *app) awardDailyLogin(ctx context.Context, userID string) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	var nationID string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM nations WHERE owner_id=? FOR UPDATE`, userID).Scan(&nationID); err != nil {
		return // Accounts without a nation are not eligible yet.
	}
	var lastDate time.Time
	var previousStreak int
	err = tx.QueryRowContext(ctx, `SELECT reward_date,streak FROM daily_login_rewards WHERE nation_id=? ORDER BY reward_date DESC LIMIT 1`, nationID).Scan(&lastDate, &previousStreak)
	if err != nil && err != sql.ErrNoRows {
		return
	}
	if err == nil && lastDate.UTC().Format("2006-01-02") == today.Format("2006-01-02") {
		return
	}
	streak := 1
	if err == nil {
		streak = nextLoginStreak(lastDate, previousStreak, today)
	}
	reward := loginRewardForStreak(streak)
	if _, err = tx.ExecContext(ctx, `INSERT INTO daily_login_rewards(nation_id,reward_date,streak,amount) VALUES(?,?,?,?)`, nationID, today.Format("2006-01-02"), streak, reward); err != nil {
		return
	}
	if _, err = tx.ExecContext(ctx, `UPDATE nations SET treasury=treasury+? WHERE id=?`, reward, nationID); err != nil {
		return
	}
	message := fmt.Sprintf("You received ¥%s for being active today. Your login streak is now %d %s.", formatWholeNumber(reward), streak, map[bool]string{true: "day", false: "days"}[streak == 1])
	if _, err = tx.ExecContext(ctx, `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'daily_login',?,'Daily activity bonus')`, uuid(), nationID, reward); err != nil {
		return
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'game','Daily login bonus',?)`, uuid(), nationID, message); err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		log.Printf("daily login reward commit failed for nation %s: %v", nationID, err)
	}
}

func formatWholeNumber(value int64) string {
	raw := fmt.Sprintf("%d", value)
	for i := len(raw) - 3; i > 0; i -= 3 {
		raw = raw[:i] + "," + raw[i:]
	}
	return raw
}
