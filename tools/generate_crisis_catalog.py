"""Build Diplomatia's deterministic, reviewable 250-entry Crisis content catalog."""
import json
import re
from pathlib import Path

developments = [
    ("Supply Disruption", "an abrupt supply disruption has interrupted normal operations"),
    ("Labor Standoff", "workers and management have reached an increasingly public standoff"),
    ("Safety Review", "inspectors have uncovered safety failures that require an immediate decision"),
    ("Regional Dispute", "provincial authorities disagree over who should bear the burden of a regional dispute"),
    ("Budget Overrun", "the current program has exceeded its appropriation and cannot continue unchanged"),
    ("Public Backlash", "a wave of organized public opposition is threatening confidence in the current policy"),
    ("Capacity Bottleneck", "demand has exceeded available capacity at several critical facilities"),
    ("Procurement Scandal", "auditors have found irregular contracts and politically connected intermediaries"),
    ("Leadership Resignation", "senior leadership has resigned after accepting responsibility for persistent failures"),
    ("Emergency Deadline", "a fast-approaching operational deadline leaves little time for ordinary consultation"),
]

themes = [
    ("Agricultural", "the national food and farming network", "growers, distributors, and provincial officials", "foodstuffs", "resource"),
    ("Energy Grid", "the national power grid", "utilities, industrial users, and household advocates", "energy", "resource"),
    ("Mining Sector", "strategic mineral extraction", "mine operators, nearby communities, and safety inspectors", "strategic_minerals", "resource"),
    ("Metals Industry", "basic-metals production", "smelters, manufacturers, and organized labor", "basic_metals", "resource"),
    ("Manufacturing", "the manufacturing base", "factory owners, workers, and downstream buyers", "basic_goods", "resource"),
    ("Construction", "the construction sector", "builders, local governments, and prospective homeowners", "construction_materials", "resource"),
    ("Textile Sector", "textile production and apparel supply", "mills, retailers, and labor associations", "textiles", "resource"),
    ("Food Processing", "processed-food production", "processors, farmers, and consumer groups", "processed_foods", "resource"),
    ("Freight Rail", "the national freight rail network", "rail operators, exporters, and provincial governments", "", "economic"),
    ("Maritime Ports", "the principal commercial ports", "port authorities, shipping firms, and dockworkers", "", "economic"),
    ("Housing", "urban housing construction and rental markets", "tenants, developers, and municipal councils", "", "civic"),
    ("Public Health", "the public-health system", "clinicians, patients, and regional health authorities", "", "civic"),
    ("Education", "the national education system", "teachers, families, and provincial education boards", "", "civic"),
    ("Scientific Research", "publicly supported scientific research", "researchers, universities, and fiscal officials", "", "institutional"),
    ("Cybersecurity", "government and critical-infrastructure networks", "security teams, civil-liberties advocates, and service operators", "", "institutional"),
    ("Banking", "the domestic banking system", "depositors, lenders, and financial regulators", "", "economic"),
    ("Labor Market", "the national labor market", "employers, unions, and unemployed citizens", "", "economic"),
    ("Water System", "drinking-water and irrigation systems", "utilities, farmers, and affected households", "", "civic"),
    ("Environmental", "environmental protection and land management", "conservation groups, producers, and provincial officials", "timber", "environment"),
    ("Emergency Services", "the national emergency-response network", "first responders, municipalities, and relief organizations", "", "civic"),
    ("Public Security", "public-security institutions", "local authorities, community leaders, and security officials", "", "institutional"),
    ("Justice System", "courts and correctional institutions", "judges, legal advocates, and provincial administrators", "", "institutional"),
    ("Foreign Relations", "a sensitive foreign-policy initiative", "diplomats, exporters, and parliamentary critics", "", "foreign"),
    ("Cultural Heritage", "museums, archives, and protected heritage sites", "curators, local communities, and cultural organizations", "", "civic"),
    ("Civil Service", "the national civil service", "agency staff, service users, and treasury officials", "", "institutional"),
]

resource_names = {
    "foodstuffs": "Foodstuffs", "energy": "Energy", "strategic_minerals": "Strategic Minerals",
    "basic_metals": "Basic Metals", "basic_goods": "Basic Goods",
    "construction_materials": "Construction Materials", "textiles": "Textiles",
    "processed_foods": "Processed Foods", "timber": "Timber", "all": "All-resource",
}

def effect(kind, value=0, target=""):
    return {"Type": kind, "Target": target, "Value": value}

def describe_effects(effects):
    immediate, temporary = [], []
    for item in effects:
        kind, target, value = item["Type"], item["Target"], item["Value"]
        sign = "+" if value > 0 else ""
        if kind == "cash_grant":
            immediate.append(f"Receive ¥{value:,.0f}" if value > 0 else f"Pay ¥{-value:,.0f}")
        elif kind == "resource_grant":
            immediate.append(f"{'Gain' if value > 0 else 'Consume'} {abs(value):g} t {resource_names[target]}")
        elif kind == "cash_income_pct":
            temporary.append(f"Cash income {sign}{value:g}%")
        elif kind == "production_pct":
            temporary.append(f"{resource_names[target]} production {sign}{value:g}%")
        elif kind == "happiness_pct":
            temporary.append(f"Happiness target {sign}{value:g}%")
        elif kind == "population_growth_pct":
            temporary.append(f"Population growth {sign}{value:g}%")
        elif kind == "upkeep_reduction_pct":
            temporary.append(f"Civil upkeep {-value:+g}%")
    clauses = []
    if immediate:
        clauses.append("; ".join(immediate) + " immediately")
    if temporary:
        clauses.append("; ".join(temporary) + " until day change")
    return ". ".join(clauses) + "."

def option(label, description, effects):
    return {"Label": label, "Description": description, "Effects": effects, "EffectText": describe_effects(effects)}

def performance(profile, resource, value):
    if profile in ("resource", "environment"):
        return effect("production_pct", value, resource)
    if profile in ("economic", "foreign"):
        return effect("cash_income_pct", value)
    if profile == "civic":
        return effect("population_growth_pct", value)
    return effect("upkeep_reduction_pct", value)

def choices(theme, resource, profile, development, index):
    """Give each incident its own dilemma instead of reskinning one profile-wide choice."""
    cost = 80000 + (index % 5) * 20000
    target = resource if resource else "all"
    neutral = {
        "Supply Disruption": ("Permit local substitution", "Allow affected organizations to source substitutes within their existing means.", "Local substitutions authorized; national figures remain unchanged."),
        "Labor Standoff": ("Continue mediated talks", "Keep both parties at the table without offering national funds or imposing a settlement.", "Mediated talks continue; national figures remain unchanged."),
        "Safety Review": ("Apply the existing safety code", "Require the ordinary corrective plan without either waivers or extraordinary assistance.", "Existing safety rules applied; national figures remain unchanged."),
        "Regional Dispute": ("Accept a provincial compromise", "Let the affected provinces divide responsibility through a limited settlement.", "Provincial compromise recorded; national figures remain unchanged."),
        "Budget Overrun": ("Freeze the unfinished work", "Pause the program at its present scope until a later budget can reconsider it.", "Program frozen; national figures remain unchanged."),
        "Public Backlash": ("Open a public consultation", "Hear objections and defer broader policy changes for the remainder of the day.", "Consultation opened; national figures remain unchanged."),
        "Capacity Bottleneck": ("Queue nonessential demand", "Preserve ordinary operating standards while accepting a temporary backlog.", "Demand queued; national figures remain unchanged."),
        "Procurement Scandal": ("Refer the contracts to court", "Let the ordinary legal process address the disputed contracts without an emergency policy.", "Contracts referred for review; national figures remain unchanged."),
        "Leadership Resignation": ("Name a caretaker", "Appoint a neutral caretaker whose only mandate is maintaining existing operations.", "Caretaker appointed; national figures remain unchanged."),
        "Emergency Deadline": ("Request a short extension", "Seek enough time to finish under normal rules without committing additional national support.", "Extension requested; national figures remain unchanged."),
    }[development]

    if development == "Supply Disruption":
        if resource:
            first = option("Purchase emergency reserves", f"Buy and distribute a limited reserve shipment for the {theme.lower()} network.", [effect("cash_grant", -cost), effect("resource_grant", 20 + index % 4 * 5, resource)])
        else:
            first = option("Fund emergency logistics", f"Pay for temporary routes and crews to keep {theme.lower()} services moving.", [effect("cash_grant", -cost), performance(profile, target, 5)])
        diversion_cost = effect("happiness_pct", -3) if profile in ("economic", "foreign") else effect("cash_income_pct", -3)
        second = option("Redirect national capacity", "Give the affected network priority over other users, restoring throughput at the expense of wider commerce or public access.", [performance(profile, target, 8), diversion_cost])
    elif development == "Labor Standoff":
        first = option("Finance a negotiated settlement", "Cover a one-day settlement package that answers the workers' immediate demands.", [effect("cash_grant", -cost), effect("happiness_pct", 5)])
        second = option("Order operations to resume", "Use emergency authority to restore output quickly despite resentment from the workforce.", [effect("production_pct", 7, target), effect("happiness_pct", -4)])
    elif development == "Safety Review":
        first = option("Suspend and inspect operations", "Accept a temporary slowdown so independent inspectors can correct the most serious failures.", [performance(profile, target, -4), effect("happiness_pct", 4)])
        second = option("Finance an emergency retrofit", "Pay for rapid repairs that preserve most current operations without relaxing the safety findings.", [effect("cash_grant", -cost), performance(profile, target, 4)])
    elif development == "Regional Dispute":
        first = option("Compensate the affected provinces", "Fund a settlement weighted toward the communities carrying the greatest burden.", [effect("cash_grant", -cost), effect("population_growth_pct", 3), effect("happiness_pct", 2)])
        second = option("Impose a centralized settlement", "End the dispute efficiently through a binding national allocation of responsibility.", [effect("upkeep_reduction_pct", 7), effect("happiness_pct", -3)])
    elif development == "Budget Overrun":
        first = option("Approve bridge financing", "Pay the overrun and preserve the program's intended output for the rest of the day.", [effect("cash_grant", -cost), performance(profile, target, 5)])
        if resource:
            second = option("Auction sector reserves", "Sell part of the sector's stockpile to cover the overrun, preserving the treasury but surrendering useful material.", [effect("resource_grant", -12, resource), effect("cash_grant", cost // 2)])
        else:
            scope_cost = effect("cash_income_pct", -4) if profile == "institutional" else performance(profile, target, -4)
            second = option("Cut the program's scope", "Remove costly components immediately, lowering overhead but also reducing performance.", [effect("upkeep_reduction_pct", 10), scope_cost])
    elif development == "Public Backlash":
        first = option("Revise the disputed policy", "Accept a short-term economic slowdown in exchange for visible concessions to the public.", [effect("cash_income_pct", -3), effect("happiness_pct", 6)])
        second = option("Defend the current policy", "Maintain the policy's immediate gains and absorb the political reaction.", [performance(profile, target, 6), effect("happiness_pct", -5)])
    elif development == "Capacity Bottleneck":
        first = option("Lease temporary capacity", "Purchase short-term facilities and staffing to expand service without permanent construction.", [effect("cash_grant", -cost), performance(profile, target, 8)])
        second = option("Ration access by priority", "Concentrate limited capacity on essential users, reducing waste while frustrating those deferred.", [effect("upkeep_reduction_pct", 7), effect("happiness_pct", -3)])
    elif development == "Procurement Scandal":
        first = option("Terminate the contracts and seize bonds", "Recover contractor guarantees for the treasury, accepting an immediate disruption while replacements are found.", [effect("cash_grant", cost // 2), performance(profile, target, -5), effect("happiness_pct", 2)])
        second = option("Honor the contracts for today", "Avoid disruption and retain the contractors' output while accepting the public credibility cost.", [performance(profile, target, 5), effect("happiness_pct", -4)])
    elif development == "Leadership Resignation":
        first = option("Recruit an expert interim team", "Bring in experienced outside leadership at premium short-term rates.", [effect("cash_grant", -cost), performance(profile, target, 5)])
        transition_cost = effect("cash_income_pct", -3) if profile == "institutional" else performance(profile, target, -3)
        second = option("Distribute authority to deputies", "Lower central administrative costs, accepting uneven execution during the transition.", [effect("upkeep_reduction_pct", 6), transition_cost])
    else:
        first = option("Authorize an emergency mobilization", "Spend heavily on personnel and expedited logistics to meet the deadline in full.", [effect("cash_grant", -cost), performance(profile, target, 9)])
        second = option("Deliver only the essential work", "Protect standards and public confidence by allowing nonessential output to miss the deadline.", [performance(profile, target, -4), effect("happiness_pct", 3)])

    third = {"Label": neutral[0], "Description": neutral[1], "Effects": [effect("none")], "EffectText": neutral[2]}
    return [first, second, third]

templates = []
for theme_index, (theme, subject, stakeholders, resource, profile) in enumerate(themes):
    for issue_index, (development, situation) in enumerate(developments):
        title = f"{theme} {development}"
        slug = re.sub(r"[^a-z0-9]+", "_", title.lower()).strip("_")
        briefing = f"Within {subject}, {situation}. {stakeholders.capitalize()} have presented incompatible remedies, and the national government must choose how deeply to intervene before the issue spreads."
        templates.append({"ID": slug, "Title": title, "Briefing": briefing, "Options": choices(theme, resource, profile, development, theme_index + issue_index)})

assert len(templates) == 250
catalog = {"templates": templates}
target = Path(__file__).resolve().parents[1] / "cmd" / "server" / "crises_data.json"
target.write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
