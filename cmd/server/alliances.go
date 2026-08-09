package main

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var allianceResources = map[string]bool{"cash": true, "foodstuffs": true, "timber": true, "fibers": true, "basic_metals": true, "energy": true, "strategic_minerals": true, "textiles": true, "processed_foods": true, "construction_materials": true, "basic_goods": true, "consumer_goods": true, "military_equipment": true, "luxury_goods": true}

type alliancePermission struct {
	AllianceID, NationID, RoleID, Title                                          string
	Rank                                                                         int
	Bank, Tax, Members, War, Announcements                                       bool
	ViewBank, Deposit, Withdraw, Applicants, Remove, Edit, Roles, Promote, Audit bool
	Limit                                                                        int64
}

func (a *app) nationID(ctx context.Context, userID string) (string, error) {
	var id string
	err := a.db.QueryRowContext(ctx, `SELECT id FROM nations WHERE owner_id=?`, userID).Scan(&id)
	return id, err
}
func (a *app) alliancePermission(ctx context.Context, userID, allianceID string) (alliancePermission, error) {
	var p alliancePermission
	err := a.db.QueryRowContext(ctx, `SELECT m.alliance_id,m.nation_id,m.role_id,r.title,r.rank_order,r.can_manage_bank,r.can_set_tax,r.can_manage_members,r.can_declare_war,r.can_post_announcements,r.can_view_bank,r.can_deposit_bank,r.can_withdraw_bank,r.can_accept_applicants,r.can_remove_members,r.can_edit_details,r.can_manage_roles,r.can_promote_members,r.can_view_audit_log,r.daily_withdrawal_limit FROM alliance_members m JOIN alliance_roles r ON r.id=m.role_id JOIN nations n ON n.id=m.nation_id WHERE n.owner_id=? AND m.alliance_id=?`, userID, allianceID).Scan(&p.AllianceID, &p.NationID, &p.RoleID, &p.Title, &p.Rank, &p.Bank, &p.Tax, &p.Members, &p.War, &p.Announcements, &p.ViewBank, &p.Deposit, &p.Withdraw, &p.Applicants, &p.Remove, &p.Edit, &p.Roles, &p.Promote, &p.Audit, &p.Limit)
	return p, err
}

func (a *app) allianceDirectory(w http.ResponseWriter, r *http.Request, u user) {
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	search = strings.ReplaceAll(strings.ReplaceAll(search, "\\", "\\\\"), "%", "\\%")
	search = strings.ReplaceAll(search, "_", "\\_")
	rows, e := a.db.QueryContext(r.Context(), `SELECT a.id,a.name,a.description,a.emblem_url,a.join_policy,a.tax_rate,a.created_at,(SELECT COUNT(*) FROM alliance_members m WHERE m.alliance_id=a.id) members,(SELECT COALESCE(SUM(n.population),0) FROM alliance_members m JOIN nations n ON n.id=m.nation_id WHERE m.alliance_id=a.id) population,(SELECT COUNT(*) FROM alliance_members m JOIN cities c ON c.nation_id=m.nation_id WHERE m.alliance_id=a.id) provinces FROM alliances a WHERE (?='' OR a.name LIKE CONCAT('%',?,'%') ESCAPE '\\') ORDER BY population DESC,a.name ASC LIMIT 100`, search, search)
	if e != nil {
		problem(w, 500, "Alliances unavailable.")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, description, emblem, policy string
		var members, provinces int
		var tax float64
		var pop int64
		var createdAt time.Time
		rows.Scan(&id, &name, &description, &emblem, &policy, &tax, &createdAt, &members, &pop, &provinces)
		out = append(out, map[string]any{"id": id, "name": name, "description": description, "emblemUrl": emblem, "joinPolicy": policy, "taxRate": tax, "createdAt": createdAt, "members": members, "population": pop, "provinces": provinces})
	}
	var membership map[string]any
	nid, _ := a.nationID(r.Context(), u.ID)
	var aid, aname, role string
	if a.db.QueryRowContext(r.Context(), `SELECT a.id,a.name,r.title FROM alliance_members m JOIN alliances a ON a.id=m.alliance_id JOIN alliance_roles r ON r.id=m.role_id WHERE m.nation_id=?`, nid).Scan(&aid, &aname, &role) == nil {
		membership = map[string]any{"allianceID": aid, "allianceName": aname, "role": role}
	}
	write(w, 200, map[string]any{"alliances": out, "membership": membership})
}

func validOptionalURL(raw string) bool {
	if raw == "" {
		return true
	}
	u, e := url.ParseRequestURI(raw)
	return e == nil && (u.Scheme == "https" || u.Scheme == "http")
}
func (a *app) createAlliance(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Name, Description, EmblemURL, CommunityURL, JoinPolicy string }
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if len(in.Name) < 3 || len(in.Name) > 80 || len(in.Description) > 1000 || !validOptionalURL(in.EmblemURL) || !validOptionalURL(in.CommunityURL) {
		problem(w, 400, "Invalid Alliance profile.")
		return
	}
	if !map[string]bool{"open": true, "apply": true, "invite_only": true}[in.JoinPolicy] {
		in.JoinPolicy = "apply"
	}
	nid, e := a.nationID(r.Context(), u.ID)
	if e != nil {
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		return
	}
	defer tx.Rollback()
	var exists int
	tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_members WHERE nation_id=?`, nid).Scan(&exists)
	if exists > 0 {
		problem(w, 409, "Leave your current Alliance before creating another.")
		return
	}
	aid, leader, member, applicant := uuid(), uuid(), uuid(), uuid()
	_, e = tx.ExecContext(r.Context(), `INSERT INTO alliances(id,founder_nation_id,name,description,emblem_url,community_url,join_policy) VALUES(?,?,?,?,?,?,?)`, aid, nid, in.Name, in.Description, in.EmblemURL, in.CommunityURL, in.JoinPolicy)
	if e != nil {
		problem(w, 409, "That Alliance name is unavailable.")
		return
	}
	_, e = tx.ExecContext(r.Context(), `INSERT INTO alliance_roles(id,alliance_id,title,rank_order,default_key,can_manage_bank,can_set_tax,can_manage_members,can_declare_war,can_post_announcements,can_view_bank,can_deposit_bank,can_withdraw_bank,can_accept_applicants,can_remove_members,can_edit_details,can_manage_roles,can_promote_members,can_view_audit_log) VALUES
		(?,?,?,100,'leader',1,1,1,1,1,1,1,1,1,1,1,1,1,1),
		(?,?,?,10,'member',0,0,0,0,0,0,1,0,0,0,0,0,0,0),
		(?,?,?,0,'applicant',0,0,0,0,0,0,0,0,0,0,0,0,0,0)`, leader, aid, "Leader", member, aid, "Member", applicant, aid, "Applicant")
	if e != nil {
		return
	}
	tx.ExecContext(r.Context(), `INSERT INTO alliance_members(alliance_id,nation_id,role_id) VALUES(?,?,?)`, aid, nid, leader)
	tx.ExecContext(r.Context(), `INSERT INTO alliance_bank(alliance_id) VALUES(?)`, aid)
	tx.ExecContext(r.Context(), `INSERT INTO alliance_tax_brackets(id,alliance_id,name,is_default,cash_rate,resource_rate) VALUES(?,?,"Default",1,0,0)`, uuid(), aid)
	if tx.Commit() != nil {
		return
	}
	write(w, 201, map[string]any{"id": aid})
}

func (a *app) allianceDetail(w http.ResponseWriter, r *http.Request, u user) {
	id := r.PathValue("id")
	var out struct {
		ID, Name, Description, EmblemURL, CommunityURL, JoinPolicy string
		TaxRate                                                    float64
		CreatedAt                                                  time.Time
	}
	e := a.db.QueryRowContext(r.Context(), `SELECT id,name,description,emblem_url,community_url,join_policy,tax_rate,created_at FROM alliances WHERE id=?`, id).Scan(&out.ID, &out.Name, &out.Description, &out.EmblemURL, &out.CommunityURL, &out.JoinPolicy, &out.TaxRate, &out.CreatedAt)
	if e != nil {
		problem(w, 404, "Alliance not found.")
		return
	}
	memberRows, _ := a.db.QueryContext(r.Context(), `SELECT n.id,n.name,n.user_type,r.id,r.title,r.rank_order,m.cash_contributed,m.resources_contributed,m.joined_at,n.population,(SELECT COUNT(*) FROM cities c WHERE c.nation_id=n.id),(SELECT MAX(s.last_action_at) FROM sessions s WHERE s.user_id=n.owner_id) FROM alliance_members m JOIN nations n ON n.id=m.nation_id JOIN alliance_roles r ON r.id=m.role_id WHERE m.alliance_id=? ORDER BY r.rank_order DESC,n.name`, id)
	members := []map[string]any{}
	for memberRows.Next() {
		var nid, name, userType, roleID, role string
		var rank, provinces int
		var cash, res int64
		var population int64
		var joined time.Time
		var lastActive *time.Time
		memberRows.Scan(&nid, &name, &userType, &roleID, &role, &rank, &cash, &res, &joined, &population, &provinces, &lastActive)
		members = append(members, map[string]any{"nationID": nid, "name": name, "userType": userType, "roleID": roleID, "role": role, "rank": rank, "cashContributed": cash, "resourcesContributed": res, "joinedAt": joined, "population": population, "provinces": provinces, "seniorityDays": int(time.Since(joined).Hours() / 24), "lastActiveAt": lastActive})
	}
	memberRows.Close()
	p, e := a.alliancePermission(r.Context(), u.ID, id)
	isMember := e == nil
	bank := map[string]float64{}
	if isMember && p.ViewBank {
		var cash float64
		a.db.QueryRowContext(r.Context(), `SELECT cash FROM alliance_bank WHERE alliance_id=?`, id).Scan(&cash)
		bank["cash"] = cash
		rows, _ := a.db.QueryContext(r.Context(), `SELECT commodity,amount FROM alliance_stockpiles WHERE alliance_id=?`, id)
		for rows.Next() {
			var k string
			var v float64
			rows.Scan(&k, &v)
			bank[k] = v
		}
		rows.Close()
	}
	logs := []map[string]any{}
	if isMember && p.Audit {
		rows, _ := a.db.QueryContext(r.Context(), `SELECT t.kind,t.resource,t.amount,t.memo,t.created_at,COALESCE(n.id,''),COALESCE(n.name,'System'),COALESCE(recipient.id,''),COALESCE(recipient.name,''),COALESCE(t.batch_id,'') FROM alliance_bank_transactions t LEFT JOIN nations n ON n.id=t.actor_nation_id LEFT JOIN nations recipient ON recipient.id=t.recipient_nation_id WHERE t.alliance_id=? AND t.kind<>'tax' ORDER BY t.created_at DESC LIMIT 50`, id)
		for rows.Next() {
			var kind, res, memo, actorID, actor, recipientID, recipient, batchID string
			var amount float64
			var at time.Time
			rows.Scan(&kind, &res, &amount, &memo, &at, &actorID, &actor, &recipientID, &recipient, &batchID)
			logs = append(logs, map[string]any{"kind": kind, "resource": res, "amount": amount, "memo": memo, "createdAt": at, "actorID": actorID, "actor": actor, "recipientID": recipientID, "recipient": recipient, "batchID": batchID})
		}
		rows.Close()
	}
	taxHistory := []map[string]any{}
	if isMember {
		rows, _ := a.db.QueryContext(r.Context(), `SELECT t.resource,t.amount,t.memo,t.created_at,COALESCE(n.name,'System') FROM alliance_bank_transactions t LEFT JOIN nations n ON n.id=t.actor_nation_id WHERE t.alliance_id=? AND t.kind='tax' ORDER BY t.created_at DESC LIMIT 100`, id)
		if rows != nil {
			for rows.Next() {
				var resource, memo, nation string
				var amount float64
				var at time.Time
				rows.Scan(&resource, &amount, &memo, &at, &nation)
				taxHistory = append(taxHistory, map[string]any{"resource": resource, "amount": amount, "memo": memo, "createdAt": at, "nation": nation})
			}
			rows.Close()
		}
	}
	applications := []map[string]any{}
	if isMember && p.Applicants {
		rows, _ := a.db.QueryContext(r.Context(), `SELECT ap.id,n.name,ap.message,ap.created_at FROM alliance_applications ap JOIN nations n ON n.id=ap.nation_id WHERE ap.alliance_id=? AND ap.status='pending' ORDER BY ap.created_at`, id)
		for rows.Next() {
			var appID, name, message string
			var at time.Time
			rows.Scan(&appID, &name, &message, &at)
			applications = append(applications, map[string]any{"id": appID, "nation": name, "message": message, "createdAt": at})
		}
		rows.Close()
	}
	announcements := []map[string]any{}
	announcementRows, _ := a.db.QueryContext(r.Context(), `SELECT x.id,x.title,x.body,x.created_at,n.name FROM alliance_announcements x JOIN nations n ON n.id=x.author_nation_id WHERE x.alliance_id=? ORDER BY x.created_at DESC LIMIT 20`, id)
	if announcementRows != nil {
		for announcementRows.Next() {
			var xid, title, body, author string
			var at time.Time
			announcementRows.Scan(&xid, &title, &body, &at, &author)
			announcements = append(announcements, map[string]any{"id": xid, "title": title, "body": body, "createdAt": at, "author": author})
		}
		announcementRows.Close()
	}
	roles := []map[string]any{}
	roleRows, _ := a.db.QueryContext(r.Context(), `SELECT id,title,rank_order,COALESCE(default_key,''),can_view_bank,can_deposit_bank,can_withdraw_bank,can_accept_applicants,can_remove_members,can_edit_details,can_manage_roles,can_set_tax,can_promote_members,can_post_announcements,can_view_audit_log,can_declare_war,daily_withdrawal_limit FROM alliance_roles WHERE alliance_id=? ORDER BY rank_order DESC`, id)
	if roleRows != nil {
		for roleRows.Next() {
			var rid, title, key string
			var rank int
			var view, deposit, withdraw, applicants, remove, edit, manageRoles, tax, promote, announce, audit, war bool
			var limit int64
			roleRows.Scan(&rid, &title, &rank, &key, &view, &deposit, &withdraw, &applicants, &remove, &edit, &manageRoles, &tax, &promote, &announce, &audit, &war, &limit)
			item := map[string]any{"id": rid, "title": title, "rank": rank, "defaultKey": key}
			if isMember && p.Roles {
				item["permissions"] = map[string]any{"viewBank": view, "deposit": deposit, "withdraw": withdraw, "applicants": applicants, "remove": remove, "edit": edit, "roles": manageRoles, "tax": tax, "promote": promote, "announcements": announce, "audit": audit, "war": war, "withdrawalLimit": limit}
			}
			roles = append(roles, item)
		}
		roleRows.Close()
	}
	brackets := []map[string]any{}
	bracketRows, _ := a.db.QueryContext(r.Context(), `SELECT b.id,b.name,b.is_default,b.cash_rate,b.resource_rate,(SELECT COUNT(*) FROM alliance_tax_assignments x WHERE x.bracket_id=b.id) FROM alliance_tax_brackets b WHERE b.alliance_id=? ORDER BY b.is_default DESC,b.name`, id)
	if bracketRows != nil {
		for bracketRows.Next() {
			var bid, name string
			var def bool
			var assignedCount int
			var cashRate, resourceRate float64
			bracketRows.Scan(&bid, &name, &def, &cashRate, &resourceRate, &assignedCount)
			brackets = append(brackets, map[string]any{"id": bid, "name": name, "isDefault": def, "cashRate": cashRate, "resourceRate": resourceRate, "assignedCount": assignedCount})
		}
		bracketRows.Close()
	}
	defaultBracketID, defaultBracketName := "", "Default"
	for _, bracket := range brackets {
		if bracket["isDefault"].(bool) {
			defaultBracketID, defaultBracketName = bracket["id"].(string), bracket["name"].(string)
			break
		}
	}
	assignments := map[string][2]string{}
	assignmentRows, _ := a.db.QueryContext(r.Context(), `SELECT x.nation_id,b.id,b.name FROM alliance_tax_assignments x JOIN alliance_tax_brackets b ON b.id=x.bracket_id AND b.alliance_id=x.alliance_id WHERE x.alliance_id=?`, id)
	if assignmentRows != nil {
		for assignmentRows.Next() {
			var nationID, bracketID, bracketName string
			assignmentRows.Scan(&nationID, &bracketID, &bracketName)
			assignments[nationID] = [2]string{bracketID, bracketName}
		}
		assignmentRows.Close()
	}
	for _, bracket := range brackets {
		if bracket["isDefault"].(bool) {
			bracket["assignedCount"] = len(members) - len(assignments)
		}
	}
	for _, member := range members {
		assignment, ok := assignments[member["nationID"].(string)]
		if !ok {
			assignment = [2]string{defaultBracketID, defaultBracketName}
		}
		member["taxBracketID"], member["taxBracketName"] = assignment[0], assignment[1]
	}
	activeTreaties, pendingTreaties := a.allianceTreaties(r.Context(), id, isMember && p.War)
	military := map[string]int64{"soldiers": 0, "tanks": 0, "ships": 0, "jets": 0, "drones": 0}
	militaryRows, _ := a.db.QueryContext(r.Context(), `SELECT unit_type,CAST(SUM(quantity) AS SIGNED) FROM (
		SELECT inventory.unit_type,inventory.quantity FROM military_inventory inventory JOIN alliance_members member ON member.nation_id=inventory.nation_id WHERE member.alliance_id=?
		UNION ALL
		SELECT orders.resource,orders.escrow_goods FROM market_orders orders JOIN alliance_members member ON member.nation_id=orders.nation_id WHERE member.alliance_id=? AND orders.side='sell' AND orders.status IN('open','pending') AND orders.resource IN('tanks','ships','jets','drones')
	) alliance_units GROUP BY unit_type`, id, id)
	if militaryRows != nil {
		for militaryRows.Next() {
			var unitType string
			var quantity int64
			militaryRows.Scan(&unitType, &quantity)
			military[unitType] = quantity
		}
		militaryRows.Close()
	}
	memberBalances := map[string]map[string]float64{}
	if isMember {
		query, args := `SELECT nation_id,resource,amount FROM alliance_member_balances WHERE alliance_id=? AND nation_id=?`, []any{id, p.NationID}
		if p.Withdraw {
			query, args = `SELECT nation_id,resource,amount FROM alliance_member_balances WHERE alliance_id=?`, []any{id}
		}
		rows, _ := a.db.QueryContext(r.Context(), query, args...)
		if rows != nil {
			for rows.Next() {
				var nationID, resource string
				var amount float64
				rows.Scan(&nationID, &resource, &amount)
				if memberBalances[nationID] == nil {
					memberBalances[nationID] = map[string]float64{}
				}
				memberBalances[nationID][resource] = amount
			}
			rows.Close()
		}
	}
	write(w, 200, map[string]any{"alliance": out, "members": members, "roles": roles, "taxBrackets": brackets, "taxHistory": taxHistory, "announcements": announcements, "treatyTypes": treatyCatalog(), "treaties": activeTreaties, "treatyProposals": pendingTreaties, "military": military, "isMember": isMember, "permissions": map[string]any{"nationID": p.NationID, "role": p.Title, "rank": p.Rank, "viewBank": p.ViewBank, "deposit": p.Deposit, "withdraw": p.Withdraw, "tax": p.Tax, "applicants": p.Applicants, "remove": p.Remove, "edit": p.Edit, "roles": p.Roles, "promote": p.Promote, "announcements": p.Announcements, "audit": p.Audit, "war": p.War}, "bank": bank, "memberBalances": memberBalances, "transactions": logs, "applications": applications})
}

func (a *app) updateAlliance(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Edit {
		problem(w, 403, "Alliance leadership required.")
		return
	}
	var in struct{ Description, CommunityURL, JoinPolicy string }
	if !decode(w, r, &in) {
		return
	}
	if len(in.Description) > 5000 || !validOptionalURL(in.CommunityURL) || !map[string]bool{"open": true, "apply": true, "invite_only": true}[in.JoinPolicy] {
		problem(w, 400, "Invalid Alliance settings.")
		return
	}
	_, e = a.db.ExecContext(r.Context(), `UPDATE alliances SET description=?,community_url=?,join_policy=? WHERE id=?`, strings.TrimSpace(in.Description), in.CommunityURL, in.JoinPolicy, aid)
	if e != nil {
		problem(w, 500, "Alliance settings could not be saved.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) applyAlliance(w http.ResponseWriter, r *http.Request, u user) {
	var in struct{ Message string }
	if !decode(w, r, &in) {
		return
	}
	nid, e := a.nationID(r.Context(), u.ID)
	if e != nil {
		return
	}
	aid := r.PathValue("id")
	var policy string
	e = a.db.QueryRowContext(r.Context(), `SELECT join_policy FROM alliances WHERE id=?`, aid).Scan(&policy)
	if e != nil {
		problem(w, 404, "Alliance not found.")
		return
	}
	if policy == "invite_only" {
		problem(w, 403, "This Alliance only accepts invited nations.")
		return
	}
	if policy == "open" {
		var role string
		a.db.QueryRowContext(r.Context(), `SELECT id FROM alliance_roles WHERE alliance_id=? AND default_key='member' LIMIT 1`, aid).Scan(&role)
		_, e = a.db.ExecContext(r.Context(), `INSERT INTO alliance_members(alliance_id,nation_id,role_id) VALUES(?,?,?)`, aid, nid, role)
		if e != nil {
			problem(w, 409, "Your nation is already in an Alliance.")
			return
		}
		write(w, 201, map[string]bool{"joined": true})
		return
	}
	var pending int
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_applications WHERE alliance_id=? AND nation_id=? AND status='pending'`, aid, nid).Scan(&pending)
	if pending > 0 {
		problem(w, 409, "Your application is already pending.")
		return
	}
	_, e = a.db.ExecContext(r.Context(), `INSERT INTO alliance_applications(id,alliance_id,nation_id,message) VALUES(?,?,?,?)`, uuid(), aid, nid, strings.TrimSpace(in.Message))
	if e != nil {
		problem(w, 409, "An application is already pending or your nation is unavailable.")
		return
	}
	write(w, 201, map[string]bool{"applied": true})
}

func (a *app) allianceApplications(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Applicants {
		problem(w, 403, "Member-management permission required.")
		return
	}
	rows, _ := a.db.QueryContext(r.Context(), `SELECT ap.id,n.id,n.name,ap.message,ap.created_at FROM alliance_applications ap JOIN nations n ON n.id=ap.nation_id WHERE ap.alliance_id=? AND ap.status='pending' ORDER BY ap.created_at`, aid)
	out := []map[string]any{}
	for rows.Next() {
		var id, nid, name, msg string
		var at time.Time
		rows.Scan(&id, &nid, &name, &msg, &at)
		out = append(out, map[string]any{"id": id, "nationID": nid, "nation": name, "message": msg, "createdAt": at})
	}
	rows.Close()
	write(w, 200, out)
}
func (a *app) acceptAllianceApplication(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Applicants {
		problem(w, 403, "Member-management permission required.")
		return
	}
	tx, _ := a.db.BeginTx(r.Context(), nil)
	defer tx.Rollback()
	var nid string
	if tx.QueryRowContext(r.Context(), `SELECT nation_id FROM alliance_applications WHERE id=? AND alliance_id=? AND status='pending' FOR UPDATE`, r.PathValue("applicationID"), aid).Scan(&nid) != nil {
		problem(w, 404, "Application not found.")
		return
	}
	var role string
	tx.QueryRowContext(r.Context(), `SELECT id FROM alliance_roles WHERE alliance_id=? AND default_key='member' LIMIT 1`, aid).Scan(&role)
	if _, e = tx.ExecContext(r.Context(), `INSERT INTO alliance_members(alliance_id,nation_id,role_id) VALUES(?,?,?)`, aid, nid, role); e != nil {
		problem(w, 409, "Nation has already joined an Alliance.")
		return
	}
	tx.ExecContext(r.Context(), `UPDATE alliance_applications SET status='accepted',resolved_at=NOW(),resolved_by=? WHERE id=?`, p.NationID, r.PathValue("applicationID"))
	tx.Commit()
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) allianceBankTransfer(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		Kind, Resource, RecipientNationID, Memo string
		Amount                                  float64
		Deposits                                map[string]float64
		Payouts                                 map[string]float64
	}
	if !decode(w, r, &in) {
		problem(w, 400, "Invalid bank transaction.")
		return
	}
	aid := r.PathValue("id")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil {
		problem(w, 403, "Alliance membership required.")
		return
	}
	if in.Kind == "" {
		in.Kind = "withdrawal"
	}
	if in.Kind != "deposit" && in.Kind != "withdrawal" && in.Kind != "grant" {
		problem(w, 400, "Invalid bank transaction type.")
		return
	}
	if (in.Kind == "deposit" && !p.Deposit) || (in.Kind != "deposit" && !p.Withdraw) {
		problem(w, 403, "Your Alliance role does not permit this bank transaction.")
		return
	}
	if in.Kind == "deposit" && len(in.Deposits) > 0 {
		keys := make([]string, 0, len(in.Deposits))
		for resource, amount := range in.Deposits {
			if !allianceResources[resource] || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) || math.Abs(amount*1000-math.Round(amount*1000)) > .000001 || (resource == "cash" && amount != math.Trunc(amount)) {
				problem(w, 400, "Deposit amounts must be positive and use no more than three decimal places. Treasury deposits must be whole Yen.")
				return
			}
			keys = append(keys, resource)
		}
		sort.Strings(keys)
		tx, err := a.db.BeginTx(r.Context(), nil)
		if err != nil {
			problem(w, 500, "Alliance bank is temporarily unavailable.")
			return
		}
		defer tx.Rollback()
		for _, resource := range keys {
			amount := in.Deposits[resource]
			var balance float64
			if resource == "cash" {
				err = tx.QueryRowContext(r.Context(), `SELECT treasury FROM nations WHERE id=? FOR UPDATE`, p.NationID).Scan(&balance)
			} else {
				err = tx.QueryRowContext(r.Context(), `SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity=? FOR UPDATE`, p.NationID, resource).Scan(&balance)
			}
			if err != nil || balance+0.0000001 < amount {
				problem(w, 409, "One or more deposit amounts exceed your national holdings.")
				return
			}
		}
		memo, batchID := strings.TrimSpace(in.Memo), uuid()
		for _, resource := range keys {
			amount := in.Deposits[resource]
			if resource == "cash" {
				_, err = tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, amount, p.NationID)
				if err == nil {
					_, err = tx.ExecContext(r.Context(), `UPDATE alliance_bank SET cash=cash+? WHERE alliance_id=?`, amount, aid)
				}
				if err == nil {
					_, err = tx.ExecContext(r.Context(), `UPDATE alliance_members SET cash_contributed=cash_contributed+? WHERE alliance_id=? AND nation_id=?`, amount, aid, p.NationID)
				}
			} else {
				_, err = tx.ExecContext(r.Context(), `UPDATE nation_stockpiles SET amount=amount-? WHERE nation_id=? AND commodity=?`, amount, p.NationID, resource)
				if err == nil {
					_, err = tx.ExecContext(r.Context(), `INSERT INTO alliance_stockpiles(alliance_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, aid, resource, amount)
				}
				if err == nil {
					_, err = tx.ExecContext(r.Context(), `UPDATE alliance_members SET resources_contributed=resources_contributed+? WHERE alliance_id=? AND nation_id=?`, amount, aid, p.NationID)
				}
			}
			if err == nil {
				_, err = tx.ExecContext(r.Context(), `INSERT INTO alliance_member_balances(alliance_id,nation_id,resource,amount) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, aid, p.NationID, resource, amount)
			}
			if err == nil {
				_, err = tx.ExecContext(r.Context(), `INSERT INTO alliance_bank_transactions(id,alliance_id,actor_nation_id,kind,resource,amount,memo,batch_id) VALUES(?,?,?,'deposit',?,?,?,?)`, uuid(), aid, p.NationID, resource, amount, memo, batchID)
			}
			if err != nil {
				problem(w, 500, "The deposit could not be completed.")
				return
			}
		}
		if err = tx.Commit(); err != nil {
			problem(w, 500, "The deposit could not be completed.")
			return
		}
		write(w, 200, map[string]any{"ok": true, "resourcesDeposited": len(keys)})
		return
	}
	if (in.Kind == "withdrawal" || in.Kind == "grant") && len(in.Payouts) > 0 {
		keys := make([]string, 0, len(in.Payouts))
		var requestedTotal float64
		for resource, amount := range in.Payouts {
			if !allianceResources[resource] || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) || math.Abs(amount*1000-math.Round(amount*1000)) > .000001 || (resource == "cash" && amount != math.Trunc(amount)) {
				problem(w, 400, "Payout amounts must be positive and use no more than three decimal places. Treasury payouts must be whole Yen.")
				return
			}
			keys = append(keys, resource)
			requestedTotal += amount
		}
		sort.Strings(keys)
		recipient := strings.TrimSpace(in.RecipientNationID)
		if recipient == "" {
			recipient = p.NationID
		}
		tx, err := a.db.BeginTx(r.Context(), nil)
		if err != nil {
			problem(w, 500, "Alliance bank is temporarily unavailable.")
			return
		}
		defer tx.Rollback()
		var member int
		tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_members WHERE alliance_id=? AND nation_id=?`, aid, recipient).Scan(&member)
		if member == 0 {
			problem(w, 404, "Payout recipient is not an Alliance member.")
			return
		}
		if p.Limit > 0 {
			var used float64
			tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(ABS(amount)),0) FROM alliance_bank_transactions WHERE alliance_id=? AND actor_nation_id=? AND kind IN('withdrawal','grant') AND created_at>=CURRENT_DATE()`, aid, p.NationID).Scan(&used)
			if used+requestedTotal > float64(p.Limit) {
				problem(w, 429, "Your role's daily withdrawal limit would be exceeded.")
				return
			}
		}
		for _, resource := range keys {
			amount := in.Payouts[resource]
			var bankBalance float64
			if resource == "cash" {
				err = tx.QueryRowContext(r.Context(), `SELECT cash FROM alliance_bank WHERE alliance_id=? FOR UPDATE`, aid).Scan(&bankBalance)
			} else {
				err = tx.QueryRowContext(r.Context(), `SELECT amount FROM alliance_stockpiles WHERE alliance_id=? AND commodity=? FOR UPDATE`, aid, resource).Scan(&bankBalance)
			}
			if err != nil || bankBalance+0.0000001 < amount {
				problem(w, 409, "One or more payout amounts exceed the Alliance bank holdings.")
				return
			}
			if in.Kind == "withdrawal" {
				tx.ExecContext(r.Context(), `INSERT IGNORE INTO alliance_member_balances(alliance_id,nation_id,resource,amount) VALUES(?,?,?,0)`, aid, recipient, resource)
				var claim float64
				if err = tx.QueryRowContext(r.Context(), `SELECT amount FROM alliance_member_balances WHERE alliance_id=? AND nation_id=? AND resource=? FOR UPDATE`, aid, recipient, resource).Scan(&claim); err != nil || claim+0.0000001 < amount {
					problem(w, 409, "One or more payout amounts exceed the member's tracked balance. Use an Alliance grant to preserve their balance.")
					return
				}
			}
		}
		memo, batchID := strings.TrimSpace(in.Memo), uuid()
		for _, resource := range keys {
			amount := in.Payouts[resource]
			if resource == "cash" {
				_, err = tx.ExecContext(r.Context(), `UPDATE alliance_bank SET cash=cash-? WHERE alliance_id=?`, amount, aid)
				if err == nil {
					_, err = tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury+? WHERE id=?`, amount, recipient)
				}
			} else {
				_, err = tx.ExecContext(r.Context(), `UPDATE alliance_stockpiles SET amount=amount-? WHERE alliance_id=? AND commodity=?`, amount, aid, resource)
				if err == nil {
					_, err = tx.ExecContext(r.Context(), `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, recipient, resource, amount)
				}
			}
			if err == nil && in.Kind == "withdrawal" {
				_, err = tx.ExecContext(r.Context(), `UPDATE alliance_member_balances SET amount=amount-? WHERE alliance_id=? AND nation_id=? AND resource=?`, amount, aid, recipient, resource)
			}
			if err == nil {
				_, err = tx.ExecContext(r.Context(), `INSERT INTO alliance_bank_transactions(id,alliance_id,actor_nation_id,recipient_nation_id,kind,resource,amount,memo,batch_id) VALUES(?,?,?,?,?,?,?,?,?)`, uuid(), aid, p.NationID, recipient, in.Kind, resource, amount, memo, batchID)
			}
			if err != nil {
				problem(w, 500, "The multi-asset payout could not be completed.")
				return
			}
		}
		var allianceName string
		tx.QueryRowContext(r.Context(), `SELECT name FROM alliances WHERE id=?`, aid).Scan(&allianceName)
		balanceEffect := "This Alliance grant did not reduce your tracked member balance."
		if in.Kind == "withdrawal" {
			balanceEffect = "The same amounts were deducted from your tracked member balance."
		}
		message := fmt.Sprintf("%s sent your nation %s. %s", allianceName, formatAllianceAssets(in.Payouts, keys), balanceEffect)
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'economic','Alliance bank payout',?)`, uuid(), recipient, message); err != nil {
			problem(w, 500, "The payout notification could not be recorded.")
			return
		}
		if err = tx.Commit(); err != nil {
			problem(w, 500, "The multi-asset payout could not be completed.")
			return
		}
		write(w, 200, map[string]any{"ok": true, "assetsPaid": len(keys)})
		return
	}
	if in.Amount <= 0 || !allianceResources[in.Resource] || math.IsNaN(in.Amount) || math.IsInf(in.Amount, 0) || math.Abs(in.Amount*1000-math.Round(in.Amount*1000)) > .000001 || (in.Resource == "cash" && in.Amount != math.Trunc(in.Amount)) {
		problem(w, 400, "Invalid bank transaction.")
		return
	}
	tx, _ := a.db.BeginTx(r.Context(), nil)
	defer tx.Rollback()
	isCash := in.Resource == "cash"
	if in.Kind == "deposit" {
		var balance float64
		if isCash {
			tx.QueryRowContext(r.Context(), `SELECT treasury FROM nations WHERE id=? FOR UPDATE`, p.NationID).Scan(&balance)
		} else {
			tx.QueryRowContext(r.Context(), `SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity=? FOR UPDATE`, p.NationID, in.Resource).Scan(&balance)
		}
		if balance < in.Amount {
			problem(w, 409, "Insufficient national stockpile.")
			return
		}
		if isCash {
			tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury-? WHERE id=?`, in.Amount, p.NationID)
			tx.ExecContext(r.Context(), `UPDATE alliance_bank SET cash=cash+? WHERE alliance_id=?`, in.Amount, aid)
		} else {
			tx.ExecContext(r.Context(), `UPDATE nation_stockpiles SET amount=amount-? WHERE nation_id=? AND commodity=?`, in.Amount, p.NationID, in.Resource)
			tx.ExecContext(r.Context(), `INSERT INTO alliance_stockpiles(alliance_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, aid, in.Resource, in.Amount)
		}
		if isCash {
			tx.ExecContext(r.Context(), `UPDATE alliance_members SET cash_contributed=cash_contributed+? WHERE alliance_id=? AND nation_id=?`, in.Amount, aid, p.NationID)
		} else {
			tx.ExecContext(r.Context(), `UPDATE alliance_members SET resources_contributed=resources_contributed+? WHERE alliance_id=? AND nation_id=?`, in.Amount, aid, p.NationID)
		}
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO alliance_member_balances(alliance_id,nation_id,resource,amount) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, aid, p.NationID, in.Resource, in.Amount); e != nil {
			problem(w, 500, "Member deposit balance could not be updated.")
			return
		}
	} else {
		var bank float64
		if isCash {
			tx.QueryRowContext(r.Context(), `SELECT cash FROM alliance_bank WHERE alliance_id=? FOR UPDATE`, aid).Scan(&bank)
		} else {
			tx.QueryRowContext(r.Context(), `SELECT amount FROM alliance_stockpiles WHERE alliance_id=? AND commodity=? FOR UPDATE`, aid, in.Resource).Scan(&bank)
		}
		if bank < in.Amount {
			problem(w, 409, "Alliance bank has insufficient funds.")
			return
		}
		if p.Limit > 0 {
			var used float64
			tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(amount),0) FROM alliance_bank_transactions WHERE alliance_id=? AND actor_nation_id=? AND kind IN('withdrawal','grant') AND created_at>=CURRENT_DATE()`, aid, p.NationID).Scan(&used)
			if used+in.Amount > float64(p.Limit) {
				problem(w, 429, "Your role's daily withdrawal limit would be exceeded.")
				return
			}
		}
		recipient := strings.TrimSpace(in.RecipientNationID)
		if recipient == "" {
			recipient = p.NationID
		}
		var member int
		tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_members WHERE alliance_id=? AND nation_id=?`, aid, recipient).Scan(&member)
		if member == 0 {
			problem(w, 404, "Payout recipient is not an Alliance member.")
			return
		}
		if in.Kind == "withdrawal" {
			tx.ExecContext(r.Context(), `INSERT IGNORE INTO alliance_member_balances(alliance_id,nation_id,resource,amount) VALUES(?,?,?,0)`, aid, recipient, in.Resource)
			var memberBalance float64
			if e = tx.QueryRowContext(r.Context(), `SELECT amount FROM alliance_member_balances WHERE alliance_id=? AND nation_id=? AND resource=? FOR UPDATE`, aid, recipient, in.Resource).Scan(&memberBalance); e != nil || memberBalance+0.0000001 < in.Amount {
				problem(w, 409, "The member does not have enough tracked balance for this payout. Use an Alliance grant to preserve their balance.")
				return
			}
			if _, e = tx.ExecContext(r.Context(), `UPDATE alliance_member_balances SET amount=amount-? WHERE alliance_id=? AND nation_id=? AND resource=?`, in.Amount, aid, recipient, in.Resource); e != nil {
				problem(w, 500, "Member balance could not be updated.")
				return
			}
		}
		if isCash {
			tx.ExecContext(r.Context(), `UPDATE alliance_bank SET cash=cash-? WHERE alliance_id=?`, in.Amount, aid)
			tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury+? WHERE id=?`, in.Amount, recipient)
		} else {
			tx.ExecContext(r.Context(), `UPDATE alliance_stockpiles SET amount=amount-? WHERE alliance_id=? AND commodity=?`, in.Amount, aid, in.Resource)
			tx.ExecContext(r.Context(), `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, recipient, in.Resource, in.Amount)
		}
		in.RecipientNationID = recipient
		var allianceName string
		tx.QueryRowContext(r.Context(), `SELECT name FROM alliances WHERE id=?`, aid).Scan(&allianceName)
		balanceEffect := "This Alliance grant did not reduce your tracked member balance."
		if in.Kind == "withdrawal" {
			balanceEffect = "The same amount was deducted from your tracked member balance."
		}
		message := fmt.Sprintf("%s sent your nation %s. %s", allianceName, formatAllianceAssets(map[string]float64{in.Resource: in.Amount}, []string{in.Resource}), balanceEffect)
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'economic','Alliance bank payout',?)`, uuid(), recipient, message); e != nil {
			problem(w, 500, "The payout notification could not be recorded.")
			return
		}
	}
	kind := in.Kind
	if kind == "" {
		kind = "withdrawal"
	}
	_, e = tx.ExecContext(r.Context(), `INSERT INTO alliance_bank_transactions(id,alliance_id,actor_nation_id,recipient_nation_id,kind,resource,amount,memo) VALUES(?,?,?,?,?,?,?,?)`, uuid(), aid, p.NationID, nullString(in.RecipientNationID), kind, in.Resource, in.Amount, strings.TrimSpace(in.Memo))
	if e != nil {
		return
	}
	if e = tx.Commit(); e != nil {
		problem(w, 500, "The bank transaction could not be completed.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func (a *app) adjustAllianceMemberBalance(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		NationID, Resource, Memo string
		Amount                   float64
		Adjustments              map[string]float64
	}
	if !decode(w, r, &in) {
		problem(w, 400, "Invalid member balance adjustment.")
		return
	}
	if len(in.Adjustments) == 0 && in.Resource != "" {
		in.Adjustments = map[string]float64{in.Resource: in.Amount}
	}
	keys := make([]string, 0, len(in.Adjustments))
	for resource, amount := range in.Adjustments {
		if !allianceResources[resource] || amount == 0 || math.IsNaN(amount) || math.IsInf(amount, 0) || math.Abs(amount*1000-math.Round(amount*1000)) > .000001 || (resource == "cash" && amount != math.Trunc(amount)) {
			problem(w, 400, "Adjustments must use valid assets and no more than three decimal places. Treasury adjustments must be whole Yen.")
			return
		}
		keys = append(keys, resource)
	}
	if len(keys) == 0 {
		problem(w, 400, "Enter at least one member balance adjustment.")
		return
	}
	sort.Strings(keys)
	aid := r.PathValue("id")
	p, err := a.alliancePermission(r.Context(), u.ID, aid)
	if err != nil || !p.Withdraw {
		problem(w, 403, "Your Alliance role does not permit balance adjustments.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Member balances are temporarily unavailable.")
		return
	}
	defer tx.Rollback()
	var member int
	tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_members WHERE alliance_id=? AND nation_id=?`, aid, in.NationID).Scan(&member)
	if member == 0 {
		problem(w, 404, "Balance owner is not an Alliance member.")
		return
	}
	for _, resource := range keys {
		tx.ExecContext(r.Context(), `INSERT IGNORE INTO alliance_member_balances(alliance_id,nation_id,resource,amount) VALUES(?,?,?,0)`, aid, in.NationID, resource)
		var current float64
		if err = tx.QueryRowContext(r.Context(), `SELECT amount FROM alliance_member_balances WHERE alliance_id=? AND nation_id=? AND resource=? FOR UPDATE`, aid, in.NationID, resource).Scan(&current); err != nil {
			problem(w, 500, "Member balances could not be loaded.")
			return
		}
		if current+in.Adjustments[resource] < -0.0000001 {
			problem(w, 409, "One or more adjustments would reduce the member's balance below zero.")
			return
		}
	}
	batchID, memo := uuid(), strings.TrimSpace(in.Memo)
	for _, resource := range keys {
		amount := in.Adjustments[resource]
		if _, err = tx.ExecContext(r.Context(), `UPDATE alliance_member_balances SET amount=amount+? WHERE alliance_id=? AND nation_id=? AND resource=?`, amount, aid, in.NationID, resource); err == nil {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO alliance_bank_transactions(id,alliance_id,actor_nation_id,recipient_nation_id,kind,resource,amount,memo,batch_id) VALUES(?,?,?,?,'balance_adjustment',?,?,?,?)`, uuid(), aid, p.NationID, in.NationID, resource, amount, memo, batchID)
		}
		if err != nil {
			problem(w, 500, "Member balance adjustments could not be saved.")
			return
		}
	}
	var allianceName string
	tx.QueryRowContext(r.Context(), `SELECT name FROM alliances WHERE id=?`, aid).Scan(&allianceName)
	message := fmt.Sprintf("%s modified your Alliance member balances: %s. No assets were transferred.", allianceName, formatAllianceAssets(in.Adjustments, keys))
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notifications(id,nation_id,category,title,message) VALUES(?,?,'economic','Alliance balance adjusted',?)`, uuid(), in.NationID, message); err != nil {
		problem(w, 500, "The balance adjustment notification could not be recorded.")
		return
	}
	if tx.Commit() != nil {
		problem(w, 500, "Member balance adjustment could not be saved.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

func formatAllianceAssets(amounts map[string]float64, keys []string) string {
	parts := make([]string, 0, len(keys))
	for _, resource := range keys {
		amount := amounts[resource]
		if resource == "cash" {
			parts = append(parts, fmt.Sprintf("%+.0f Yen", amount))
			continue
		}
		value := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%+.3f", amount), "0"), ".")
		parts = append(parts, value+" t "+commodityName(resource))
	}
	return strings.Join(parts, ", ")
}
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
