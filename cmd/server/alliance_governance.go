package main

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

type allianceTaxQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func applicableAllianceTax(ctx context.Context, q allianceTaxQuery, nid string) (aid, name string, cashRate, resourceRate float64) {
	var roleID string
	var provinces int
	if q.QueryRowContext(ctx, `SELECT m.alliance_id,a.name,m.role_id,(SELECT COUNT(*) FROM cities WHERE nation_id=m.nation_id) FROM alliance_members m JOIN alliances a ON a.id=m.alliance_id WHERE m.nation_id=?`, nid).Scan(&aid, &name, &roleID, &provinces) != nil {
		return
	}
	if q.QueryRowContext(ctx, `SELECT cash_rate,resource_rate FROM alliance_tax_brackets WHERE alliance_id=? AND minimum_provinces<=? AND (role_id IS NULL OR role_id=?) ORDER BY (role_id=?) DESC,is_default ASC,minimum_provinces DESC LIMIT 1`, aid, provinces, roleID, roleID).Scan(&cashRate, &resourceRate) != nil {
		q.QueryRowContext(ctx, `SELECT tax_rate FROM alliances WHERE id=?`, aid).Scan(&cashRate)
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

type taxBracketInput struct {
	Name, RoleID           string
	MinimumProvinces       int
	CashRate, ResourceRate float64
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
	if !decode(w, r, &in) || !validBracket(in) {
		problem(w, 400, "Invalid tax bracket.")
		return
	}
	_, e = a.db.ExecContext(r.Context(), `INSERT INTO alliance_tax_brackets(id,alliance_id,name,role_id,minimum_provinces,cash_rate,resource_rate) VALUES(?,?,?,?,?,?,?)`, uuid(), aid, strings.TrimSpace(in.Name), nullString(in.RoleID), in.MinimumProvinces, in.CashRate, in.ResourceRate)
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
	if !decode(w, r, &in) || !validBracket(in) {
		problem(w, 400, "Invalid tax bracket.")
		return
	}
	_, e = a.db.ExecContext(r.Context(), `UPDATE alliance_tax_brackets SET name=?,role_id=?,minimum_provinces=?,cash_rate=?,resource_rate=? WHERE id=? AND alliance_id=?`, strings.TrimSpace(in.Name), nullString(in.RoleID), in.MinimumProvinces, in.CashRate, in.ResourceRate, bid, aid)
	if e != nil {
		problem(w, 409, "Tax bracket could not be updated.")
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
