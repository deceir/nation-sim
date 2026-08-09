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

def effect(kind, value=0, target=""):
    return {"Type": kind, "Target": target, "Value": value}

def choices(theme, resource, profile, index):
    cost = 90000 + (index % 4) * 20000
    if profile == "resource":
        return [
            {"Label": f"Fund a targeted {theme.lower()} intervention", "Description": "Pay for emergency staffing, inspections, and logistics while keeping the public fully informed.", "Effects": [effect("cash_grant", -cost), effect("production_pct", 5, resource)], "EffectText": f"Pay ¥{cost:,}; {resource.replace('_',' ').title()} production +5% until day change."},
            {"Label": "Relax operating rules temporarily", "Description": "Let producers restore output quickly under reduced oversight, accepting public concern over the shortcut.", "Effects": [effect("production_pct", 8, resource), effect("happiness_pct", -3)], "EffectText": f"{resource.replace('_',' ').title()} production +8%; Happiness target -3% until day change."},
            {"Label": "Broker a limited local settlement", "Description": "Keep national resources out of the dispute and direct the parties toward a narrow compromise.", "Effects": [effect("none")], "EffectText": "Crisis resolved without a national modifier."},
        ]
    if profile == "economic":
        return [
            {"Label": "Provide temporary public support", "Description": "Stabilize affected firms and households with a tightly limited national appropriation.", "Effects": [effect("cash_grant", -cost), effect("happiness_pct", 3)], "EffectText": f"Pay ¥{cost:,}; Happiness target +3% until day change."},
            {"Label": "Prioritize commercial continuity", "Description": "Give operators broad latitude to restore activity, despite predictable objections from workers and residents.", "Effects": [effect("cash_income_pct", 5), effect("happiness_pct", -3)], "EffectText": "Cash income +5%; Happiness target -3% until day change."},
            {"Label": "Accept a negotiated pause", "Description": "Suspend the disputed activity long enough for the parties to settle immediate responsibilities.", "Effects": [effect("none")], "EffectText": "Crisis resolved without a national modifier."},
        ]
    if profile == "civic":
        return [
            {"Label": "Fully fund the public response", "Description": "Cover emergency services and direct assistance rather than shifting costs onto local institutions.", "Effects": [effect("cash_grant", -cost), effect("happiness_pct", 4)], "EffectText": f"Pay ¥{cost:,}; Happiness target +4% until day change."},
            {"Label": "Impose emergency rationing", "Description": "Stretch existing capacity through strict priorities that improve efficiency but frustrate the public.", "Effects": [effect("upkeep_reduction_pct", 6), effect("happiness_pct", -3)], "EffectText": "Civil upkeep -6%; Happiness target -3% until day change."},
            {"Label": "Authorize a local compromise", "Description": "Let provincial institutions settle the immediate question within their existing budgets.", "Effects": [effect("none")], "EffectText": "Crisis resolved without a national modifier."},
        ]
    if profile == "environment":
        return [
            {"Label": "Order immediate restoration work", "Description": "Fund cleanup crews and temporary closures to protect public confidence and damaged land.", "Effects": [effect("cash_grant", -cost), effect("happiness_pct", 4)], "EffectText": f"Pay ¥{cost:,}; Happiness target +4% until day change."},
            {"Label": "Keep productive sites operating", "Description": "Limit closures to preserve output while accepting greater public anxiety about environmental harm.", "Effects": [effect("production_pct", 6, resource), effect("happiness_pct", -4)], "EffectText": f"{resource.title()} production +6%; Happiness target -4% until day change."},
            {"Label": "Adopt the inspectors' minimum plan", "Description": "Meet immediate legal obligations without expanding either extraction or restoration programs.", "Effects": [effect("none")], "EffectText": "Crisis resolved without a national modifier."},
        ]
    if profile == "foreign":
        return [
            {"Label": "Offer a funded diplomatic package", "Description": "Use aid, expert missions, and formal talks to preserve the relationship and public confidence.", "Effects": [effect("cash_grant", -cost), effect("happiness_pct", 3)], "EffectText": f"Pay ¥{cost:,}; Happiness target +3% until day change."},
            {"Label": "Defend national commercial interests", "Description": "Take a harder negotiating line that benefits exporters but increases domestic controversy.", "Effects": [effect("cash_income_pct", 5), effect("happiness_pct", -2)], "EffectText": "Cash income +5%; Happiness target -2% until day change."},
            {"Label": "Issue a narrow joint statement", "Description": "Resolve the immediate diplomatic question without promises of money or policy changes.", "Effects": [effect("none")], "EffectText": "Crisis resolved without a national modifier."},
        ]
    return [
        {"Label": "Launch a transparent national review", "Description": "Fund independent specialists and publish their findings to restore confidence in the institution.", "Effects": [effect("cash_grant", -cost), effect("happiness_pct", 4)], "EffectText": f"Pay ¥{cost:,}; Happiness target +4% until day change."},
        {"Label": "Centralize emergency authority", "Description": "Reduce duplication and act quickly, accepting public resistance to the concentration of power.", "Effects": [effect("upkeep_reduction_pct", 7), effect("happiness_pct", -3)], "EffectText": "Civil upkeep -7%; Happiness target -3% until day change."},
        {"Label": "Appoint an interim mediator", "Description": "Settle the immediate administrative dispute without changing national budgets or operating rules.", "Effects": [effect("none")], "EffectText": "Crisis resolved without a national modifier."},
    ]

templates = []
for theme_index, (theme, subject, stakeholders, resource, profile) in enumerate(themes):
    for issue_index, (development, situation) in enumerate(developments):
        title = f"{theme} {development}"
        slug = re.sub(r"[^a-z0-9]+", "_", title.lower()).strip("_")
        briefing = f"Within {subject}, {situation}. {stakeholders.capitalize()} have presented incompatible remedies, and the national government must choose how deeply to intervene before the issue spreads."
        templates.append({"ID": slug, "Title": title, "Briefing": briefing, "Options": choices(theme, resource, profile, theme_index + issue_index)})

assert len(templates) == 250
catalog = {"templates": templates}
target = Path(__file__).resolve().parents[1] / "cmd" / "server" / "crises_data.json"
target.write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
