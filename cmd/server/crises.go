package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

//go:embed crises_data.json
var crisisCatalogJSON []byte

type crisisCatalog struct {
	Profiles  map[string][]crisisCatalogOption `json:"profiles"`
	Templates []crisisCatalogTemplate          `json:"templates"`
}
type crisisCatalogTemplate struct {
	ID, Title, Briefing string
	Options             []crisisCatalogOption
}
type crisisEffect struct {
	Type, Target string
	Value        float64
}
type crisisCatalogOption struct {
	Label, Description, EffectType, EffectTarget, EffectText string
	EffectValue                                              float64
	Effects                                                  []crisisEffect
}

type crisisOptionItem struct {
	ID           string  `json:"id"`
	Label        string  `json:"label"`
	Description  string  `json:"description"`
	EffectType   string  `json:"effectType"`
	EffectTarget string  `json:"effectTarget"`
	EffectValue  float64 `json:"effectValue"`
	EffectText   string  `json:"effectText"`
}
type crisisItem struct {
	ID            string             `json:"id"`
	TemplateID    string             `json:"templateId"`
	Title         string             `json:"title"`
	Briefing      string             `json:"briefing"`
	Resolved      bool               `json:"resolved"`
	SelectedLabel string             `json:"selectedLabel,omitempty"`
	EffectSummary string             `json:"effectSummary,omitempty"`
	RespondedAt   *time.Time         `json:"respondedAt,omitempty"`
	Options       []crisisOptionItem `json:"options"`
}

var validCrisisEffects = map[string]bool{"cash_grant": true, "resource_grant": true, "cash_income_pct": true, "production_pct": true, "happiness_pct": true, "population_growth_pct": true, "upkeep_reduction_pct": true, "none": true}

func (a *app) syncCrisisCatalog(ctx context.Context) error {
	var catalog crisisCatalog
	if err := json.Unmarshal(crisisCatalogJSON, &catalog); err != nil {
		return fmt.Errorf("decode crisis catalog: %w", err)
	}
	if len(catalog.Templates) != 250 {
		return fmt.Errorf("crisis catalog contains %d templates; expected 250", len(catalog.Templates))
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE crisis_templates SET enabled=0`); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, template := range catalog.Templates {
		template.ID = strings.TrimSpace(template.ID)
		options := template.Options
		if template.ID == "" || seen[template.ID] || len(options) != 3 {
			return fmt.Errorf("invalid crisis template %q", template.ID)
		}
		seen[template.ID] = true
		if _, err = tx.ExecContext(ctx, `INSERT INTO crisis_templates(id,internal_name,title,briefing,enabled) VALUES(?,?,?,?,1) ON DUPLICATE KEY UPDATE internal_name=VALUES(internal_name),title=VALUES(title),briefing=VALUES(briefing),enabled=1`, template.ID, template.ID, template.Title, template.Briefing); err != nil {
			return err
		}
		for index, option := range options {
			if len(option.Effects) == 0 {
				option.Effects = []crisisEffect{{Type: option.EffectType, Target: option.EffectTarget, Value: option.EffectValue}}
			}
			for _, effect := range option.Effects {
				if !validCrisisEffects[effect.Type] {
					return fmt.Errorf("unknown crisis effect %q", effect.Type)
				}
			}
			payload, _ := json.Marshal(option.Effects)
			primary := option.Effects[0]
			optionID := fmt.Sprintf("%s_%d", template.ID, index+1)
			if _, err = tx.ExecContext(ctx, `INSERT INTO crisis_options(id,template_id,sort_order,label,description,effect_type,effect_target,effect_value,effect_text,effect_payload) VALUES(?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE label=VALUES(label),description=VALUES(description),effect_type=VALUES(effect_type),effect_target=VALUES(effect_target),effect_value=VALUES(effect_value),effect_text=VALUES(effect_text),effect_payload=VALUES(effect_payload)`, optionID, template.ID, index+1, option.Label, option.Description, primary.Type, primary.Target, primary.Value, option.EffectText, payload); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (a *app) ensureDailyCrises(ctx context.Context) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	count := secureInt(4)
	result, err := tx.ExecContext(ctx, `INSERT IGNORE INTO crisis_days(server_date,crisis_count) VALUES(CURRENT_DATE(),?)`, count)
	if err != nil {
		return err
	}
	created, _ := result.RowsAffected()
	if created == 0 {
		return tx.Commit()
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM crisis_templates WHERE enabled=1 ORDER BY id`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if len(ids) < count {
		return fmt.Errorf("not enough enabled crisis templates")
	}
	for i := len(ids) - 1; i > 0; i-- {
		j := secureInt(i + 1)
		ids[i], ids[j] = ids[j], ids[i]
	}
	for slot := 0; slot < count; slot++ {
		if _, err = tx.ExecContext(ctx, `INSERT INTO daily_crises(id,server_date,template_id,slot_number) VALUES(?,CURRENT_DATE(),?,?)`, uuid(), ids[slot], slot+1); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (a *app) crises(w http.ResponseWriter, r *http.Request, u user) {
	nid, err := a.nationID(r.Context(), u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	if err = a.ensureDailyCrises(r.Context()); err != nil {
		problem(w, 500, "Daily Crises are unavailable.")
		return
	}
	var total, unresolved int
	a.db.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(CASE WHEN response.daily_crisis_id IS NULL THEN 1 ELSE 0 END),0) FROM daily_crises crisis LEFT JOIN nation_crisis_responses response ON response.daily_crisis_id=crisis.id AND response.nation_id=? WHERE crisis.server_date=CURRENT_DATE()`, nid).Scan(&total, &unresolved)
	if r.URL.Query().Get("summary") == "1" {
		write(w, 200, map[string]any{"total": total, "unresolved": unresolved})
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT crisis.id,template.id,template.title,template.briefing,response.responded_at,response.effect_summary,COALESCE(choice.label,'') FROM daily_crises crisis JOIN crisis_templates template ON template.id=crisis.template_id LEFT JOIN nation_crisis_responses response ON response.daily_crisis_id=crisis.id AND response.nation_id=? LEFT JOIN crisis_options choice ON choice.id=response.option_id WHERE crisis.server_date=CURRENT_DATE() ORDER BY crisis.slot_number`, nid)
	if err != nil {
		log.Printf("load detailed Daily Crises: %v", err)
		problem(w, 500, "Daily Crises are unavailable.")
		return
	}
	items := []crisisItem{}
	for rows.Next() {
		var item crisisItem
		var responded sql.NullTime
		var summary, selected sql.NullString
		if rows.Scan(&item.ID, &item.TemplateID, &item.Title, &item.Briefing, &responded, &summary, &selected) == nil {
			if responded.Valid {
				item.Resolved = true
				v := responded.Time
				item.RespondedAt = &v
			}
			if summary.Valid {
				item.EffectSummary = summary.String
			}
			if selected.Valid {
				item.SelectedLabel = selected.String
			}
			item.Options = []crisisOptionItem{}
			items = append(items, item)
		}
	}
	rows.Close()
	byTemplate := map[string]*crisisItem{}
	templateIDs := []string{}
	for index := range items {
		byTemplate[items[index].TemplateID] = &items[index]
		templateIDs = append(templateIDs, items[index].TemplateID)
	}
	if len(templateIDs) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(templateIDs)), ",")
		args := make([]any, len(templateIDs))
		for i, v := range templateIDs {
			args[i] = v
		}
		optionRows, e := a.db.QueryContext(r.Context(), `SELECT id,template_id,label,description,effect_type,effect_target,effect_value,effect_text FROM crisis_options WHERE template_id IN (`+placeholders+`) ORDER BY template_id,sort_order`, args...)
		if e == nil {
			for optionRows.Next() {
				var templateID string
				var option crisisOptionItem
				if optionRows.Scan(&option.ID, &templateID, &option.Label, &option.Description, &option.EffectType, &option.EffectTarget, &option.EffectValue, &option.EffectText) == nil {
					if item := byTemplate[templateID]; item != nil {
						item.Options = append(item.Options, option)
					}
				}
			}
			optionRows.Close()
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return !items[i].Resolved && items[j].Resolved })
	now := time.Now().UTC()
	expires := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	write(w, 200, map[string]any{"serverDate": now.Format("2006-01-02"), "expiresAt": expires, "total": total, "unresolved": unresolved, "items": items})
}

func (a *app) respondToCrisis(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		OptionID string `json:"optionId"`
	}
	if !decode(w, r, &in) {
		return
	}
	nid, err := a.nationID(r.Context(), u.ID)
	if err != nil {
		problem(w, 404, "Nation not found.")
		return
	}
	if err = a.ensureDailyCrises(r.Context()); err != nil {
		problem(w, 500, "Daily Crises are unavailable.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		problem(w, 500, "Could not record that response.")
		return
	}
	defer tx.Rollback()
	var effectType, target, effectText string
	var value float64
	var payload []byte
	err = tx.QueryRowContext(r.Context(), `SELECT choice.effect_type,choice.effect_target,choice.effect_value,choice.effect_text,COALESCE(choice.effect_payload,JSON_ARRAY()) FROM daily_crises crisis JOIN crisis_options choice ON choice.template_id=crisis.template_id WHERE crisis.id=? AND crisis.server_date=CURRENT_DATE() AND choice.id=? FOR UPDATE`, r.PathValue("id"), strings.TrimSpace(in.OptionID)).Scan(&effectType, &target, &value, &effectText, &payload)
	if err != nil {
		problem(w, 400, "That Crisis or response is no longer available.")
		return
	}
	var answered int
	tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM nation_crisis_responses WHERE nation_id=? AND daily_crisis_id=?`, nid, r.PathValue("id")).Scan(&answered)
	if answered > 0 {
		problem(w, 409, "Your nation has already responded to this Crisis.")
		return
	}
	effects := []crisisEffect{}
	if json.Unmarshal(payload, &effects) != nil || len(effects) == 0 {
		effects = []crisisEffect{{Type: effectType, Target: target, Value: value}}
	}
	var cashDelta float64
	resourceDeltas := map[string]float64{}
	for _, effect := range effects {
		if effect.Type == "cash_grant" {
			cashDelta += effect.Value
		} else if effect.Type == "resource_grant" {
			resourceDeltas[effect.Target] += effect.Value
		}
	}
	var treasury float64
	if tx.QueryRowContext(r.Context(), `SELECT treasury FROM nations WHERE id=? FOR UPDATE`, nid).Scan(&treasury) != nil || treasury+cashDelta < 0 {
		problem(w, 409, "Your nation cannot afford that response.")
		return
	}
	for resource, delta := range resourceDeltas {
		var available float64
		resourceErr := tx.QueryRowContext(r.Context(), `SELECT amount FROM nation_stockpiles WHERE nation_id=? AND commodity=? FOR UPDATE`, nid, resource).Scan(&available)
		if resourceErr != nil && resourceErr != sql.ErrNoRows {
			problem(w, 500, "Could not verify the resources required for that response.")
			return
		}
		if available+delta < 0 {
			problem(w, 409, "Your nation lacks the resources required for that response.")
			return
		}
	}
	for _, effect := range effects {
		switch effect.Type {
		case "cash_grant":
			_, err = tx.ExecContext(r.Context(), `UPDATE nations SET treasury=treasury+? WHERE id=?`, int64(effect.Value), nid)
		case "resource_grant":
			_, err = tx.ExecContext(r.Context(), `INSERT INTO nation_stockpiles(nation_id,commodity,amount) VALUES(?,?,?) ON DUPLICATE KEY UPDATE amount=amount+VALUES(amount)`, nid, effect.Target, effect.Value)
		case "cash_income_pct", "production_pct", "happiness_pct", "population_growth_pct", "upkeep_reduction_pct":
			_, err = tx.ExecContext(r.Context(), `INSERT INTO crisis_modifiers(nation_id,daily_crisis_id,modifier_type,target,value,expires_on) VALUES(?,?,?,?,?,DATE_ADD(CURRENT_DATE(),INTERVAL 1 DAY))`, nid, r.PathValue("id"), effect.Type, effect.Target, effect.Value)
		case "none":
		default:
			err = fmt.Errorf("unsupported effect")
		}
		if err != nil {
			break
		}
	}
	if err != nil {
		problem(w, 500, "Could not apply that Crisis outcome.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO nation_crisis_responses(nation_id,daily_crisis_id,option_id,effect_summary) VALUES(?,?,?,?)`, nid, r.PathValue("id"), in.OptionID, effectText)
	if err != nil {
		problem(w, 409, "Your nation has already responded to this Crisis.")
		return
	}
	if cashDelta != 0 {
		tx.ExecContext(r.Context(), `INSERT INTO ledger_entries(id,nation_id,category,amount,memo) VALUES(?,?,'daily_crisis',?,'Daily Crisis response')`, uuid(), nid, int64(cashDelta))
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "Could not record that response.")
		return
	}
	write(w, 200, map[string]any{"ok": true, "effect": effectText})
}

type crisisTurnModifiers struct {
	CashIncomePct, HappinessPct, PopulationGrowthPct, UpkeepReductionPct float64
	Production                                                           map[string]float64
}

func applyCrisisTurnModifiers(result *strategicResult, modifiers crisisTurnModifiers) {
	result.IncomeMultiplier *= 1 + modifiers.CashIncomePct/100
	result.HappinessMultiplier *= 1 + modifiers.HappinessPct/100
	result.PopulationMultiplier *= 1 + modifiers.PopulationGrowthPct/100
	for target, percent := range modifiers.Production {
		if target == "all" {
			for resource := range result.Production {
				result.Production[resource] *= 1 + percent/100
			}
		} else {
			result.Production[target] *= 1 + percent/100
		}
	}
}

func (a *app) loadCrisisModifiers(ctx context.Context, nationID string) crisisTurnModifiers {
	result := crisisTurnModifiers{Production: map[string]float64{}}
	rows, err := a.db.QueryContext(ctx, `SELECT modifier_type,target,value FROM crisis_modifiers WHERE nation_id=? AND expires_on>CURRENT_DATE()`, nationID)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var kind, target string
		var value float64
		if rows.Scan(&kind, &target, &value) != nil {
			continue
		}
		switch kind {
		case "cash_income_pct":
			result.CashIncomePct += value
		case "happiness_pct":
			result.HappinessPct += value
		case "population_growth_pct":
			result.PopulationGrowthPct += value
		case "upkeep_reduction_pct":
			result.UpkeepReductionPct += value
		case "production_pct":
			result.Production[target] += value
		}
	}
	return result
}
