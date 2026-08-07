package main

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var allianceResources = map[string]bool{"cash": true, "foodstuffs": true, "timber": true, "fibers": true, "basic_metals": true, "energy": true, "strategic_minerals": true, "textiles": true, "processed_foods": true, "construction_materials": true, "basic_goods": true, "consumer_goods": true, "military_equipment": true, "luxury_goods": true}

type alliancePermission struct {
	AllianceID, NationID, RoleID, Title    string
	Rank                                   int
	Bank, Tax, Members, War, Announcements bool
	Limit                                  int64
}

func (a *app) nationID(ctx context.Context, userID string) (string, error) {
	var id string
	err := a.db.QueryRowContext(ctx, `SELECT id FROM nations WHERE owner_id=?`, userID).Scan(&id)
	return id, err
}
func (a *app) alliancePermission(ctx context.Context, userID, allianceID string) (alliancePermission, error) {
	var p alliancePermission
	err := a.db.QueryRowContext(ctx, `SELECT m.alliance_id,m.nation_id,m.role_id,r.title,r.rank_order,r.can_manage_bank,r.can_set_tax,r.can_manage_members,r.can_declare_war,r.can_post_announcements,r.daily_withdrawal_limit FROM alliance_members m JOIN alliance_roles r ON r.id=m.role_id JOIN nations n ON n.id=m.nation_id WHERE n.owner_id=? AND m.alliance_id=?`, userID, allianceID).Scan(&p.AllianceID, &p.NationID, &p.RoleID, &p.Title, &p.Rank, &p.Bank, &p.Tax, &p.Members, &p.War, &p.Announcements, &p.Limit)
	return p, err
}

func (a *app) allianceDirectory(w http.ResponseWriter, r *http.Request, u user) {
	rows, e := a.db.QueryContext(r.Context(), `SELECT a.id,a.name,a.description,a.emblem_url,a.join_policy,a.level,a.tax_rate,COUNT(DISTINCT m.nation_id),COALESCE(SUM(DISTINCT n.population),0),COALESCE(SUM(c.infrastructure),0) FROM alliances a LEFT JOIN alliance_members m ON m.alliance_id=a.id LEFT JOIN nations n ON n.id=m.nation_id LEFT JOIN cities c ON c.nation_id=n.id GROUP BY a.id ORDER BY COUNT(DISTINCT m.nation_id) DESC,a.name LIMIT 100`)
	if e != nil {
		problem(w, 500, "Alliances unavailable.")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, description, emblem, policy string
		var level, members int
		var tax float64
		var pop, infra int64
		rows.Scan(&id, &name, &description, &emblem, &policy, &level, &tax, &members, &pop, &infra)
		out = append(out, map[string]any{"id": id, "name": name, "description": description, "emblemUrl": emblem, "joinPolicy": policy, "level": level, "taxRate": tax, "members": members, "population": pop, "infrastructure": infra})
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
	aid, leader, heir, officer, member := uuid(), uuid(), uuid(), uuid(), uuid()
	_, e = tx.ExecContext(r.Context(), `INSERT INTO alliances(id,founder_nation_id,name,description,emblem_url,community_url,join_policy) VALUES(?,?,?,?,?,?,?)`, aid, nid, in.Name, in.Description, in.EmblemURL, in.CommunityURL, in.JoinPolicy)
	if e != nil {
		problem(w, 409, "That Alliance name is unavailable.")
		return
	}
	roles := []struct {
		id, title                         string
		rank                              int
		bank, tax, members, war, announce bool
		limit                             int64
	}{{leader, "Founder", 100, true, true, true, true, true, 0}, {heir, "Heir", 90, true, true, true, true, true, 0}, {officer, "Officer", 60, true, false, true, false, true, 250000}, {member, "Member", 10, false, false, false, false, false, 0}}
	for _, x := range roles {
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO alliance_roles(id,alliance_id,title,rank_order,can_manage_bank,can_set_tax,can_manage_members,can_declare_war,can_post_announcements,daily_withdrawal_limit) VALUES(?,?,?,?,?,?,?,?,?,?)`, x.id, aid, x.title, x.rank, x.bank, x.tax, x.members, x.war, x.announce, x.limit); e != nil {
			return
		}
	}
	tx.ExecContext(r.Context(), `INSERT INTO alliance_members(alliance_id,nation_id,role_id) VALUES(?,?,?)`, aid, nid, leader)
	tx.ExecContext(r.Context(), `INSERT INTO alliance_bank(alliance_id) VALUES(?)`, aid)
	if tx.Commit() != nil {
		return
	}
	write(w, 201, map[string]any{"id": aid})
}

func (a *app) allianceDetail(w http.ResponseWriter, r *http.Request, u user) {
	id := r.PathValue("id")
	var out struct {
		ID, Name, Description, EmblemURL, CommunityURL, JoinPolicy  string
		Level, MinimumCities, MinimumAgeDays, MinimumInfrastructure int
		TaxRate                                                     float64
	}
	e := a.db.QueryRowContext(r.Context(), `SELECT id,name,description,emblem_url,community_url,join_policy,level,minimum_cities,minimum_age_days,minimum_infrastructure,tax_rate FROM alliances WHERE id=?`, id).Scan(&out.ID, &out.Name, &out.Description, &out.EmblemURL, &out.CommunityURL, &out.JoinPolicy, &out.Level, &out.MinimumCities, &out.MinimumAgeDays, &out.MinimumInfrastructure, &out.TaxRate)
	if e != nil {
		problem(w, 404, "Alliance not found.")
		return
	}
	memberRows, _ := a.db.QueryContext(r.Context(), `SELECT n.id,n.name,n.user_type,r.title,m.cash_contributed,m.resources_contributed,m.joined_at FROM alliance_members m JOIN nations n ON n.id=m.nation_id JOIN alliance_roles r ON r.id=m.role_id WHERE m.alliance_id=? ORDER BY r.rank_order DESC,n.name`, id)
	members := []map[string]any{}
	for memberRows.Next() {
		var nid, name, userType, role string
		var cash, res int64
		var joined time.Time
		memberRows.Scan(&nid, &name, &userType, &role, &cash, &res, &joined)
		members = append(members, map[string]any{"nationID": nid, "name": name, "userType": userType, "role": role, "cashContributed": cash, "resourcesContributed": res, "joinedAt": joined})
	}
	memberRows.Close()
	p, e := a.alliancePermission(r.Context(), u.ID, id)
	isMember := e == nil
	bank := map[string]float64{}
	if isMember {
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
	if isMember {
		rows, _ := a.db.QueryContext(r.Context(), `SELECT t.kind,t.resource,t.amount,t.memo,t.created_at,COALESCE(n.name,'System') FROM alliance_bank_transactions t LEFT JOIN nations n ON n.id=t.actor_nation_id WHERE t.alliance_id=? ORDER BY t.created_at DESC LIMIT 50`, id)
		for rows.Next() {
			var kind, res, memo, actor string
			var amount int64
			var at time.Time
			rows.Scan(&kind, &res, &amount, &memo, &at, &actor)
			logs = append(logs, map[string]any{"kind": kind, "resource": res, "amount": amount, "memo": memo, "createdAt": at, "actor": actor})
		}
		rows.Close()
	}
	applications := []map[string]any{}
	if isMember && p.Members {
		rows, _ := a.db.QueryContext(r.Context(), `SELECT ap.id,n.name,ap.message,ap.created_at FROM alliance_applications ap JOIN nations n ON n.id=ap.nation_id WHERE ap.alliance_id=? AND ap.status='pending' ORDER BY ap.created_at`, id)
		for rows.Next() {
			var appID, name, message string
			var at time.Time
			rows.Scan(&appID, &name, &message, &at)
			applications = append(applications, map[string]any{"id": appID, "nation": name, "message": message, "createdAt": at})
		}
		rows.Close()
	}
	write(w, 200, map[string]any{"alliance": out, "members": members, "isMember": isMember, "permissions": map[string]any{"role": p.Title, "bank": p.Bank, "tax": p.Tax, "members": p.Members, "announcements": p.Announcements}, "bank": bank, "transactions": logs, "applications": applications})
}

func (a *app) updateAlliance(w http.ResponseWriter, r *http.Request, u user) {
	aid := r.PathValue("id")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil || !p.Tax {
		problem(w, 403, "Alliance leadership required.")
		return
	}
	var in struct{ Description, CommunityURL, JoinPolicy, TaxRate, MinimumCities, MinimumAgeDays, MinimumInfrastructure string }
	if !decode(w, r, &in) {
		return
	}
	tax, _ := strconv.ParseFloat(in.TaxRate, 64)
	cities, _ := strconv.Atoi(in.MinimumCities)
	age, _ := strconv.Atoi(in.MinimumAgeDays)
	infra, _ := strconv.Atoi(in.MinimumInfrastructure)
	if len(in.Description) > 1000 || !validOptionalURL(in.CommunityURL) || !map[string]bool{"open": true, "apply": true, "invite_only": true}[in.JoinPolicy] || tax < 0 || tax > 100 || cities < 1 || age < 0 || infra < 0 {
		problem(w, 400, "Invalid Alliance settings.")
		return
	}
	_, e = a.db.ExecContext(r.Context(), `UPDATE alliances SET description=?,community_url=?,join_policy=?,tax_rate=?,minimum_cities=?,minimum_age_days=?,minimum_infrastructure=? WHERE id=?`, strings.TrimSpace(in.Description), in.CommunityURL, in.JoinPolicy, tax, cities, age, infra, aid)
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
	var minCities, minAge, minInfra int
	e = a.db.QueryRowContext(r.Context(), `SELECT join_policy,minimum_cities,minimum_age_days,minimum_infrastructure FROM alliances WHERE id=?`, aid).Scan(&policy, &minCities, &minAge, &minInfra)
	if e != nil {
		problem(w, 404, "Alliance not found.")
		return
	}
	var cities, age, infra int
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*),TIMESTAMPDIFF(DAY,n.created_at,NOW()),COALESCE(SUM(c.infrastructure),0) FROM nations n LEFT JOIN cities c ON c.nation_id=n.id WHERE n.id=? GROUP BY n.id`, nid).Scan(&cities, &age, &infra)
	if cities < minCities || age < minAge || infra < minInfra {
		problem(w, 409, "Your nation does not meet this Alliance's entry requirements.")
		return
	}
	if policy == "invite_only" {
		problem(w, 403, "This Alliance only accepts invited nations.")
		return
	}
	if policy == "open" {
		var role string
		a.db.QueryRowContext(r.Context(), `SELECT id FROM alliance_roles WHERE alliance_id=? ORDER BY rank_order LIMIT 1`, aid).Scan(&role)
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
	if e != nil || !p.Members {
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
	if e != nil || !p.Members {
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
	tx.QueryRowContext(r.Context(), `SELECT id FROM alliance_roles WHERE alliance_id=? ORDER BY rank_order LIMIT 1`, aid).Scan(&role)
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
		Amount                                  int64
	}
	if !decode(w, r, &in) || in.Amount <= 0 || !allianceResources[in.Resource] {
		problem(w, 400, "Invalid bank transaction.")
		return
	}
	aid := r.PathValue("id")
	p, e := a.alliancePermission(r.Context(), u.ID, aid)
	if e != nil {
		problem(w, 403, "Alliance membership required.")
		return
	}
	if in.Kind != "deposit" && !p.Bank {
		problem(w, 403, "Bank-management permission required.")
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
		if balance < float64(in.Amount) {
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
	} else {
		var bank float64
		if isCash {
			tx.QueryRowContext(r.Context(), `SELECT cash FROM alliance_bank WHERE alliance_id=? FOR UPDATE`, aid).Scan(&bank)
		} else {
			tx.QueryRowContext(r.Context(), `SELECT amount FROM alliance_stockpiles WHERE alliance_id=? AND commodity=? FOR UPDATE`, aid, in.Resource).Scan(&bank)
		}
		if bank < float64(in.Amount) {
			problem(w, 409, "Alliance bank has insufficient funds.")
			return
		}
		if p.Limit > 0 {
			var used int64
			tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(amount),0) FROM alliance_bank_transactions WHERE alliance_id=? AND actor_nation_id=? AND kind IN('withdrawal','grant') AND created_at>=CURRENT_DATE()`, aid, p.NationID).Scan(&used)
			if used+in.Amount > p.Limit {
				problem(w, 429, "Your role's daily withdrawal limit would be exceeded.")
				return
			}
		}
		recipient := p.NationID
		if in.Kind == "grant" {
			recipient = in.RecipientNationID
			var member int
			tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM alliance_members WHERE alliance_id=? AND nation_id=?`, aid, recipient).Scan(&member)
			if member == 0 {
				problem(w, 404, "Grant recipient is not an Alliance member.")
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
	}
	kind := in.Kind
	if kind == "" {
		kind = "withdrawal"
	}
	_, e = tx.ExecContext(r.Context(), `INSERT INTO alliance_bank_transactions(id,alliance_id,actor_nation_id,recipient_nation_id,kind,resource,amount,memo) VALUES(?,?,?,?,?,?,?,?)`, uuid(), aid, p.NationID, nullString(in.RecipientNationID), kind, in.Resource, in.Amount, strings.TrimSpace(in.Memo))
	if e != nil {
		return
	}
	tx.Commit()
	write(w, 200, map[string]bool{"ok": true})
}
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
