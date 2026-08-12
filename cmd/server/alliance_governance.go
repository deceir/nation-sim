package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type allianceTaxQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func applicableAllianceTax(ctx context.Context, q allianceTaxQuery, nid string) (aid, name string, cashRate, resourceRate float64) {
	if q.QueryRowContext(ctx, `SELECT m.alliance_id,a.name FROM alliance_members m JOIN alliances a ON a.id=m.alliance_id WHERE m.nation_id=?`, nid).Scan(&aid, &name) != nil {
		return
	}
	if q.QueryRowContext(ctx, `SELECT b.cash_rate,b.resource_rate FROM alliance_tax_assignments x JOIN alliance_tax_brackets b ON b.id=x.bracket_id AND b.alliance_id=x.alliance_id WHERE x.alliance_id=? AND x.nation_id=?`, aid, nid).Scan(&cashRate, &resourceRate) != nil {
		if q.QueryRowContext(ctx, `SELECT cash_rate,resource_rate FROM alliance_tax_brackets WHERE alliance_id=? AND is_default=1 LIMIT 1`, aid).Scan(&cashRate, &resourceRate) != nil {
			q.QueryRowContext(ctx, `SELECT tax_rate FROM alliances WHERE id=?`, aid).Scan(&cashRate)
		}
	}
	return
}

type roleInput struct {
	Name                                                                                                  string `json:"name"`
	Rank                                                                                                  int    `json:"rank"`
	ViewBank, Deposit, Withdraw, Applicants, Remove, Edit, Roles, Tax, Promote, Announcements, Audit, War bool
	WithdrawalLimit                                                                                       int64 `json:"withdrawalLimit"`
}

func (a *app) createAllianceRole(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Roles {
		problem(w, 403, "Role-management permission required.")
		return
	}
	var in roleInput
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 1 || len(in.Name) > 60 || in.Rank <= 0 || in.Rank >= p.Rank {
		problem(w, 400, "Choose a unique role name and a rank below your own.")
		return
	}
	_, e = a.db.ExecContext(r.Context(), `INSERT INTO alliance_roles(id,alliance_id,title,rank_order,can_view_bank,can_deposit_bank,can_withdraw_bank,can_accept_applicants,can_remove_members,can_edit_details,can_manage_roles,can_set_tax,can_promote_members,can_post_announcements,can_view_audit_log,can_declare_war,daily_withdrawal_limit) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, uuid(), aid, in.Name, in.Rank, in.ViewBank, in.Deposit, in.Withdraw, in.Applicants, in.Remove, in.Edit, in.Roles, in.Tax, in.Promote, in.Announcements, in.Audit, in.War, in.WithdrawalLimit)
	if e != nil {
		problem(w, 409, "That role name or rank is unavailable.")
		return
	}
	write(w, 201, map[string]bool{"ok": true})
}

func (a *app) updateAllianceRole(w http.ResponseWriter, r *http.Request, u user) {
	aid, rid := r.PathValue("id"), r.PathValue("roleID")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Roles {
		problem(w, 403, "Role-management permission required.")
		return
	}
	var currentRank int
	var defaultKey sql.NullString
	if a.db.QueryRowContext(r.Context(), `SELECT rank_order,default_key FROM alliance_roles WHERE id=? AND alliance_id=?`, rid, aid).Scan(&currentRank, &defaultKey) != nil || currentRank > p.Rank {
		problem(w, 403, "You cannot edit that role.")
		return
	}
	var in roleInput
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 1 || len(in.Name) > 60 {
		problem(w, 400, "Invalid role name.")
		return
	}
	rank := in.Rank
	if defaultKey.String == "leader" {
		rank = 100
		if p.Rank < 100 {
			problem(w, 403, "Only the Leader role may edit itself.")
			return
		}
	} else if defaultKey.String == "applicant" {
		rank = 0
	} else if rank <= 0 || rank >= p.Rank {
		problem(w, 400, "Role rank must remain below your own.")
		return
	}
	_, e = a.db.ExecContext(r.Context(), `UPDATE alliance_roles SET title=?,rank_order=?,can_view_bank=?,can_deposit_bank=?,can_withdraw_bank=?,can_accept_applicants=?,can_remove_members=?,can_edit_details=?,can_manage_roles=?,can_set_tax=?,can_promote_members=?,can_post_announcements=?,can_view_audit_log=?,can_declare_war=?,daily_withdrawal_limit=? WHERE id=? AND alliance_id=?`, in.Name, rank, in.ViewBank, in.Deposit, in.Withdraw, in.Applicants, in.Remove, in.Edit, in.Roles, in.Tax, in.Promote, in.Announcements, in.Audit, in.War, in.WithdrawalLimit, rid, aid)
	if e != nil {
		problem(w, 409, "Role could not be updated.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) deleteAllianceRole(w http.ResponseWriter, r *http.Request, u user) {
	aid, rid := r.PathValue("id"), r.PathValue("roleID")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Roles {
		problem(w, 403, "Role-management permission required.")
		return
	}
	var rank, members int
	var key sql.NullString
	if a.db.QueryRowContext(r.Context(), `SELECT r.rank_order,r.default_key,(SELECT COUNT(*) FROM alliance_members WHERE role_id=r.id) FROM alliance_roles r WHERE r.id=? AND r.alliance_id=?`, rid, aid).Scan(&rank, &key, &members) != nil {
		problem(w, 404, "Role not found.")
		return
	}
	if key.Valid {
		problem(w, 409, "Default roles cannot be deleted.")
		return
	}
	if rank >= p.Rank || members > 0 {
		problem(w, 409, "Move all members out of this lower-ranked role before deleting it.")
		return
	}
	a.db.ExecContext(r.Context(), `DELETE FROM alliance_roles WHERE id=? AND alliance_id=?`, rid, aid)
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) assignAllianceRole(w http.ResponseWriter, r *http.Request, u user) {
	aid, target := r.PathValue("id"), r.PathValue("nationID")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Promote {
		problem(w, 403, "Promotion permission required.")
		return
	}
	var in struct {
		RoleID string `json:"roleID"`
	}
	if !decode(w, r, &in) {
		return
	}
	var targetRank, newRank int
	if a.db.QueryRowContext(r.Context(), `SELECT r.rank_order FROM alliance_members m JOIN alliance_roles r ON r.id=m.role_id WHERE m.alliance_id=? AND m.nation_id=?`, aid, target).Scan(&targetRank) != nil || a.db.QueryRowContext(r.Context(), `SELECT rank_order FROM alliance_roles WHERE alliance_id=? AND id=?`, aid, in.RoleID).Scan(&newRank) != nil {
		problem(w, 404, "Member or role not found.")
		return
	}
	if target == p.NationID || targetRank >= p.Rank || newRank >= p.Rank || newRank <= 0 {
		problem(w, 403, "You may only change lower-ranked members within your rank range.")
		return
	}
	a.db.ExecContext(r.Context(), `UPDATE alliance_members SET role_id=? WHERE alliance_id=? AND nation_id=?`, in.RoleID, aid, target)
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) removeAllianceMember(w http.ResponseWriter, r *http.Request, u user) {
	aid, target := r.PathValue("id"), r.PathValue("nationID")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Remove {
		problem(w, 403, "Member-removal permission required.")
		return
	}
	var targetRank int
	if a.db.QueryRowContext(r.Context(), `SELECT r.rank_order FROM alliance_members m JOIN alliance_roles r ON r.id=m.role_id WHERE m.alliance_id=? AND m.nation_id=?`, aid, target).Scan(&targetRank) != nil {
		problem(w, 404, "Member not found.")
		return
	}
	if target == p.NationID || targetRank >= p.Rank {
		problem(w, 403, "You may remove only lower-ranked members.")
		return
	}
	a.db.ExecContext(r.Context(), `DELETE FROM alliance_members WHERE alliance_id=? AND nation_id=?`, aid, target)
	write(w, 200, map[string]bool{"ok": true})
}

type flexibleFloat float64

func (f *flexibleFloat) UnmarshalJSON(data []byte) error {
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*f = flexibleFloat(number)
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return err
	}
	*f = flexibleFloat(number)
	return nil
}

type taxBracketInput struct {
	Name, NationID         string
	MinimumProvinces       int
	CashRate, ResourceRate flexibleFloat
}

func validBracket(in taxBracketInput) bool {
	return len(strings.TrimSpace(in.Name)) > 0 && len(in.Name) <= 80 && in.MinimumProvinces >= 0 && in.CashRate >= 0 && in.CashRate <= 100 && in.ResourceRate >= 0 && in.ResourceRate <= 100
}
func (a *app) createAllianceTaxBracket(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Tax {
		problem(w, 403, "Tax-management permission required.")
		return
	}
	var in taxBracketInput
	if !decode(w, r, &in) {
		return
	}
	if !validBracket(in) {
		problem(w, 400, "Invalid tax bracket.")
		return
	}
	_, e = a.db.ExecContext(r.Context(), `INSERT INTO alliance_tax_brackets(id,alliance_id,name,minimum_provinces,cash_rate,resource_rate) VALUES(?,?,?,0,?,?)`, uuid(), aid, strings.TrimSpace(in.Name), float64(in.CashRate), float64(in.ResourceRate))
	if e != nil {
		problem(w, 409, "Tax bracket could not be created.")
		return
	}
	write(w, 201, map[string]bool{"ok": true})
}
func (a *app) updateAllianceTaxBracket(w http.ResponseWriter, r *http.Request, u user) {
	aid, bid := r.PathValue("id"), r.PathValue("bracketID")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Tax {
		problem(w, 403, "Tax-management permission required.")
		return
	}
	var in taxBracketInput
	if !decode(w, r, &in) {
		return
	}
	if !validBracket(in) {
		problem(w, 400, "Invalid tax bracket.")
		return
	}
	var isDefault bool
	if a.db.QueryRowContext(r.Context(), `SELECT is_default FROM alliance_tax_brackets WHERE id=? AND alliance_id=?`, bid, aid).Scan(&isDefault) != nil {
		problem(w, 404, "Tax bracket not found.")
		return
	}
	_, e = a.db.ExecContext(r.Context(), `UPDATE alliance_tax_brackets SET name=?,role_id=NULL,nation_id=NULL,minimum_provinces=0,cash_rate=?,resource_rate=? WHERE id=? AND alliance_id=?`, strings.TrimSpace(in.Name), float64(in.CashRate), float64(in.ResourceRate), bid, aid)
	if e != nil {
		problem(w, 409, "Tax bracket could not be updated.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) assignAllianceTaxBracket(w http.ResponseWriter, r *http.Request, u user) {
	aid, target := r.PathValue("id"), r.PathValue("nationID")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Tax {
		problem(w, 403, "Tax-management permission required.")
		return
	}
	var in struct {
		BracketID string `json:"bracketID"`
	}
	if !decode(w, r, &in) {
		return
	}
	var memberCount int
	var isDefault bool
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_members WHERE alliance_id=? AND nation_id=?`, aid, target).Scan(&memberCount)
	if memberCount != 1 || a.db.QueryRowContext(r.Context(), `SELECT is_default FROM alliance_tax_brackets WHERE alliance_id=? AND id=?`, aid, in.BracketID).Scan(&isDefault) != nil {
		problem(w, 404, "Member or tax bracket not found.")
		return
	}
	if isDefault {
		_, e = a.db.ExecContext(r.Context(), `DELETE FROM alliance_tax_assignments WHERE alliance_id=? AND nation_id=?`, aid, target)
	} else {
		_, e = a.db.ExecContext(r.Context(), `INSERT INTO alliance_tax_assignments(alliance_id,nation_id,bracket_id) VALUES(?,?,?) ON DUPLICATE KEY UPDATE bracket_id=VALUES(bracket_id),assigned_at=CURRENT_TIMESTAMP(6)`, aid, target, in.BracketID)
	}
	if e != nil {
		problem(w, 409, "Tax bracket assignment could not be saved.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) leaveAlliance(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	nid, e := a.nationID(r.Context(), u.ID)
	if e != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		problem(w, 500, "Alliance membership could not be updated.")
		return
	}
	defer tx.Rollback()
	var count int
	var roleKey sql.NullString
	var lockedAlliance string
	if tx.QueryRowContext(r.Context(), `SELECT id FROM alliances WHERE id=? FOR UPDATE`, aid).Scan(&lockedAlliance) != nil || tx.QueryRowContext(r.Context(), `SELECT r.default_key FROM alliance_members m JOIN alliance_roles r ON r.id=m.role_id WHERE m.alliance_id=? AND m.nation_id=?`, aid, nid).Scan(&roleKey) != nil {
		problem(w, 404, "You are not a member of this Alliance.")
		return
	}
	tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_members WHERE alliance_id=?`, aid).Scan(&count)
	if count <= 1 {
		problem(w, 409, "The final member must delete the Alliance instead.")
		return
	}
	if _, e = tx.ExecContext(r.Context(), `DELETE FROM alliance_members WHERE alliance_id=? AND nation_id=?`, aid, nid); e != nil {
		problem(w, 500, "Alliance membership could not be updated.")
		return
	}
	if roleKey.String == "leader" {
		var remainingLeaders int
		tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_members m JOIN alliance_roles r ON r.id=m.role_id WHERE m.alliance_id=? AND r.default_key='leader'`, aid).Scan(&remainingLeaders)
		if remainingLeaders == 0 {
			_, e = tx.ExecContext(r.Context(), `UPDATE alliance_members SET role_id=(SELECT id FROM alliance_roles WHERE alliance_id=? AND default_key='leader' LIMIT 1) WHERE alliance_id=? ORDER BY joined_at,nation_id LIMIT 1`, aid, aid)
			if e != nil {
				problem(w, 500, "Alliance leadership could not be transferred.")
				return
			}
		}
	}
	if e = tx.Commit(); e != nil {
		problem(w, 500, "Alliance membership could not be updated.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) deleteAlliance(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	nid, e := a.nationID(r.Context(), u.ID)
	if e != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		problem(w, 500, "Alliance could not be deleted.")
		return
	}
	defer tx.Rollback()
	var count int
	var lockedAlliance string
	if tx.QueryRowContext(r.Context(), `SELECT id FROM alliances WHERE id=? FOR UPDATE`, aid).Scan(&lockedAlliance) != nil {
		problem(w, 404, "Alliance not found.")
		return
	}
	if tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_members WHERE alliance_id=?`, aid).Scan(&count) != nil || count != 1 {
		problem(w, 409, "An Alliance can only be deleted by its sole remaining member.")
		return
	}
	var member int
	tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_members WHERE alliance_id=? AND nation_id=?`, aid, nid).Scan(&member)
	if member != 1 {
		problem(w, 403, "Only the sole remaining member can delete this Alliance.")
		return
	}
	var cash float64
	if e = tx.QueryRowContext(r.Context(), `SELECT cash FROM alliance_bank WHERE alliance_id=? FOR UPDATE`, aid).Scan(&cash); e != nil && e != sql.ErrNoRows {
		problem(w, 500, "Alliance bank could not be settled.")
		return
	}
	if cash > 0 {
		if _, e = tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury+? WHERE id=?`, cash, nid); e != nil {
			problem(w, 500, "Alliance bank could not be settled.")
			return
		}
	}
	rows, e := tx.QueryContext(r.Context(), `SELECT commodity,amount FROM alliance_stockpiles WHERE alliance_id=? AND amount<>0 FOR UPDATE`, aid)
	if e != nil {
		problem(w, 500, "Alliance stockpiles could not be settled.")
		return
	}
	type transferredAsset struct {
		commodity string
		amount    float64
	}
	assets := []transferredAsset{}
	for rows.Next() {
		var commodity string
		var amount float64
		if e = rows.Scan(&commodity, &amount); e != nil {
			rows.Close()
			problem(w, 500, "Alliance stockpiles could not be settled.")
			return
		}
		assets = append(assets, transferredAsset{commodity, amount})
	}
	rows.Close()
	for _, asset := range assets {
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, nid, asset.commodity, asset.amount); e != nil {
			problem(w, 500, "Alliance stockpiles could not be settled.")
			return
		}
	}
	// Remove members before roles so the role foreign key cannot obstruct the cascade.
	if _, e = tx.ExecContext(r.Context(), `DELETE FROM alliance_members WHERE alliance_id=?`, aid); e == nil {
		_, e = tx.ExecContext(r.Context(), `DELETE FROM alliances WHERE id=?`, aid)
	}
	if e != nil {
		problem(w, 500, "Alliance could not be deleted.")
		return
	}
	if _, e = tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'economic','Alliance treasury transferred','Your former Alliance bank holdings were transferred to your nation when the Alliance was dissolved.')`, uuid(), nid); e != nil {
		problem(w, 500, "Alliance could not be deleted.")
		return
	}
	if e = tx.Commit(); e != nil {
		problem(w, 500, "Alliance could not be deleted.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *app) deleteAllianceTaxBracket(w http.ResponseWriter, r *http.Request, u user) {
	aid, bid := r.PathValue("id"), r.PathValue("bracketID")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Tax {
		problem(w, 403, "Tax-management permission required.")
		return
	}
	result, e := a.db.ExecContext(r.Context(), `DELETE FROM alliance_tax_brackets WHERE id=? AND alliance_id=? AND is_default=0`, bid, aid)
	if e != nil || affected(result) != 1 {
		problem(w, 409, "The default bracket cannot be deleted.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
