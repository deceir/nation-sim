from pathlib import Path

from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER, TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import mm
from reportlab.platypus import (
    BaseDocTemplate,
    Frame,
    KeepTogether,
    PageBreak,
    PageTemplate,
    Paragraph,
    Spacer,
    Table,
    TableStyle,
)


ROOT = Path(__file__).resolve().parent
OUTPUT = ROOT / "output" / "pdf" / "diplomatia-war-system-reference.pdf"

PAGE_W, PAGE_H = A4
NAVY = colors.HexColor("#0C1B2A")
NAVY_2 = colors.HexColor("#14283A")
GOLD = colors.HexColor("#C99B48")
INK = colors.HexColor("#19232D")
MUTED = colors.HexColor("#596875")
LINE = colors.HexColor("#CCD5DC")
PALE = colors.HexColor("#F3F6F8")
PALE_GOLD = colors.HexColor("#FBF5E8")
WHITE = colors.white


styles = getSampleStyleSheet()
styles.add(ParagraphStyle(
    name="DocTitle", parent=styles["Title"], fontName="Times-Bold",
    fontSize=25, leading=28, textColor=WHITE, alignment=TA_LEFT,
    spaceAfter=4,
))
styles.add(ParagraphStyle(
    name="DocDeck", parent=styles["Normal"], fontName="Helvetica",
    fontSize=9.5, leading=14, textColor=colors.HexColor("#D8E0E6"),
))
styles.add(ParagraphStyle(
    name="Section", parent=styles["Heading2"], fontName="Times-Bold",
    fontSize=16, leading=19, textColor=NAVY, spaceBefore=10, spaceAfter=7,
))
styles.add(ParagraphStyle(
    name="Subhead", parent=styles["Heading3"], fontName="Helvetica-Bold",
    fontSize=10.5, leading=13, textColor=NAVY, spaceBefore=7, spaceAfter=4,
))
styles.add(ParagraphStyle(
    name="BodyText2", parent=styles["BodyText"], fontName="Helvetica",
    fontSize=8.9, leading=13.1, textColor=INK, spaceAfter=5,
))
styles.add(ParagraphStyle(
    name="Small", parent=styles["BodyText"], fontName="Helvetica",
    fontSize=7.6, leading=10.4, textColor=MUTED,
))
styles.add(ParagraphStyle(
    name="TableHead", parent=styles["Normal"], fontName="Helvetica-Bold",
    fontSize=7.3, leading=9, textColor=WHITE, alignment=TA_LEFT,
))
styles.add(ParagraphStyle(
    name="TableCell", parent=styles["Normal"], fontName="Helvetica",
    fontSize=7.2, leading=9.3, textColor=INK,
))
styles.add(ParagraphStyle(
    name="Formula", parent=styles["Normal"], fontName="Courier-Bold",
    fontSize=7.7, leading=11.2, textColor=NAVY,
))
styles.add(ParagraphStyle(
    name="CalloutTitle", parent=styles["Normal"], fontName="Helvetica-Bold",
    fontSize=8.2, leading=10.5, textColor=NAVY,
))
styles.add(ParagraphStyle(
    name="CalloutBody", parent=styles["Normal"], fontName="Helvetica",
    fontSize=7.8, leading=11, textColor=INK,
))


def P(text, style="BodyText2"):
    return Paragraph(text, styles[style])


def callout(title, body, gold=False):
    bg = PALE_GOLD if gold else PALE
    edge = GOLD if gold else LINE
    table = Table([[P(title, "CalloutTitle")], [P(body, "CalloutBody")]], colWidths=[170 * mm])
    table.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, -1), bg),
        ("BOX", (0, 0), (-1, -1), 0.7, edge),
        ("LINEBEFORE", (0, 0), (0, -1), 3, GOLD if gold else NAVY_2),
        ("LEFTPADDING", (0, 0), (-1, -1), 8),
        ("RIGHTPADDING", (0, 0), (-1, -1), 8),
        ("TOPPADDING", (0, 0), (-1, 0), 6),
        ("BOTTOMPADDING", (0, 0), (-1, 0), 2),
        ("TOPPADDING", (0, 1), (-1, 1), 2),
        ("BOTTOMPADDING", (0, 1), (-1, 1), 7),
    ]))
    return table


def formula(text):
    t = Table([[P(text.replace("\n", "<br/>"), "Formula")]], colWidths=[170 * mm])
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, -1), PALE),
        ("BOX", (0, 0), (-1, -1), 0.6, LINE),
        ("LEFTPADDING", (0, 0), (-1, -1), 9),
        ("RIGHTPADDING", (0, 0), (-1, -1), 9),
        ("TOPPADDING", (0, 0), (-1, -1), 7),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 7),
    ]))
    return t


def data_table(headers, rows, widths):
    data = [[P(h, "TableHead") for h in headers]]
    for row in rows:
        data.append([P(str(cell), "TableCell") for cell in row])
    t = Table(data, colWidths=widths, repeatRows=1, hAlign="LEFT")
    commands = [
        ("BACKGROUND", (0, 0), (-1, 0), NAVY_2),
        ("GRID", (0, 0), (-1, -1), 0.45, LINE),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("LEFTPADDING", (0, 0), (-1, -1), 5),
        ("RIGHTPADDING", (0, 0), (-1, -1), 5),
        ("TOPPADDING", (0, 0), (-1, -1), 5),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 5),
    ]
    for idx in range(1, len(data)):
        commands.append(("BACKGROUND", (0, idx), (-1, idx), WHITE if idx % 2 else PALE))
    t.setStyle(TableStyle(commands))
    return t


def first_page(canvas, doc):
    canvas.saveState()
    canvas.setFillColor(NAVY)
    canvas.rect(0, PAGE_H - 57 * mm, PAGE_W, 57 * mm, fill=1, stroke=0)
    canvas.setFillColor(GOLD)
    canvas.rect(18 * mm, PAGE_H - 59 * mm, 32 * mm, 2 * mm, fill=1, stroke=0)
    canvas.setFont("Helvetica-Bold", 8)
    canvas.setFillColor(GOLD)
    canvas.drawString(18 * mm, PAGE_H - 15 * mm, "DIPLOMATIA")
    canvas.setFont("Helvetica", 7.5)
    canvas.setFillColor(colors.HexColor("#D8E0E6"))
    canvas.drawRightString(PAGE_W - 18 * mm, PAGE_H - 15 * mm, "IMPLEMENTED RULES REFERENCE")
    canvas.setFont("Times-Bold", 25)
    canvas.setFillColor(WHITE)
    canvas.drawString(18 * mm, PAGE_H - 32 * mm, "War System")
    deck = Paragraph(
        "A concise reference to the rules currently running in Diplomatia: declarations, distance, strategic rounds, combat strength, losses, supply, and post-war consequences.",
        styles["DocDeck"],
    )
    deck.wrapOn(canvas, 170 * mm, 18 * mm)
    deck.drawOn(canvas, 18 * mm, PAGE_H - 49 * mm)
    footer(canvas, doc)
    canvas.restoreState()


def later_page(canvas, doc):
    canvas.saveState()
    canvas.setFillColor(NAVY)
    canvas.rect(0, PAGE_H - 15 * mm, PAGE_W, 15 * mm, fill=1, stroke=0)
    canvas.setFont("Helvetica-Bold", 8)
    canvas.setFillColor(GOLD)
    canvas.drawString(18 * mm, PAGE_H - 10 * mm, "DIPLOMATIA - WAR SYSTEM")
    footer(canvas, doc)
    canvas.restoreState()


def footer(canvas, doc):
    canvas.setStrokeColor(LINE)
    canvas.line(18 * mm, 13 * mm, PAGE_W - 18 * mm, 13 * mm)
    canvas.setFont("Helvetica", 7)
    canvas.setFillColor(MUTED)
    canvas.drawString(18 * mm, 8.5 * mm, "Current pre-alpha implementation - 22 August 2026")
    canvas.drawRightString(PAGE_W - 18 * mm, 8.5 * mm, f"Page {doc.page}")


def build():
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    doc = BaseDocTemplate(
        str(OUTPUT), pagesize=A4,
        leftMargin=18 * mm, rightMargin=18 * mm,
        topMargin=22 * mm, bottomMargin=18 * mm,
        title="Diplomatia War System - Implemented Rules Reference",
        author="Diplomatia",
        subject="Current war, combat, supply, mobilization, and outcome rules",
    )
    first_frame = Frame(18 * mm, 18 * mm, PAGE_W - 36 * mm, PAGE_H - 81 * mm, id="first")
    later_frame = Frame(18 * mm, 18 * mm, PAGE_W - 36 * mm, PAGE_H - 38 * mm, id="later")
    doc.addPageTemplates([
        PageTemplate(id="First", frames=[first_frame], onPage=first_page, autoNextPageTemplate="Later"),
        PageTemplate(id="Later", frames=[later_frame], onPage=later_page, autoNextPageTemplate="Later"),
    ])

    story = [
        P("The basic shape of a war", "Section"),
        P("A war is a scheduled contest between two nations. Each side commits real military inventory, chooses an operation and posture for the next round, pays the supply cost of the force in the field, and receives a report after resolution. Rounds occur every six hours at 00:00, 06:00, 12:00, and 18:00 UTC. A war lasts no more than 20 rounds, or five days."),
        data_table(
            ["Rule", "Current value", "What it means"],
            [
                ["War slots", "2 offensive / 3 defensive", "A nation cannot open a third offensive war or be placed into a fourth defensive war."],
                ["Guardian", "Blocks declarations", "A protected nation cannot be targeted. Declaring war immediately revokes the attacker's own Guardian status."],
                ["Initial deployment", "Attacker: chosen force; Defender: saved settings", "The defender's saved percentage is applied separately to each available unit type. Every setting defaults to 60%. Travel rounds resolve without combat."],
                ["Repeat conflict", "7-day armistice", "The same pair of nations cannot immediately redeclare after a war ends."],
                ["Reconstruction", "3 / 5 / 7 days", "The losing nation cannot open an offensive war during reconstruction. A recovering nation can face only one defensive front."],
            ],
            [34 * mm, 38 * mm, 98 * mm],
        ),
        Spacer(1, 3 * mm),
        callout("Automatic defense settings", "Each unit type can be set from 0% to 100% in Military Command. A 0% setting leaves that unit entirely manual. Percentages are applied to forces available when a new declaration is made; changing them does not move forces already committed to an active war."),
        P("Distance and arrival time", "Section"),
        P("Distance is calculated from the two stored nation locations using the Haversine formula. It affects both how long attacking forces take to arrive and how expensive they are to supply. The defender's home force is present immediately, but it cannot attack an absent force during the attacker's initial transit."),
        formula("a = sin^2(dLat/2) + cos(lat1) * cos(lat2) * sin^2(dLon/2)\ndistance = 2 * 6371 km * atan2(sqrt(a), sqrt(1-a))"),
        Spacer(1, 3 * mm),
        formula("Mobilization rounds = min(5, 1 + floor(distance / 3500 km))\nAttacker supply factor = 1 + min(1.25, distance / 12000 km)"),
        Spacer(1, 3 * mm),
        callout("Initial transit rounds", "If the attacking force has not arrived, the round records mobilization only. Neither side attacks, takes casualties, earns score, loses resolve, or pays combat supply for that round. Combat begins in the calculated arrival round; a one-round mobilization therefore begins combat in round one."),
        Spacer(1, 3 * mm),
        callout("Reinforcements", "Defensive reinforcements arrive in the next round. Attacking reinforcements use the same distance-based mobilization delay as the initial force. A deployed unit remains committed to that war until it is lost or the war ends.", gold=True),
        PageBreak(),

        P("How a strategic round is resolved", "Section"),
        P("Every deployed unit contributes a base strength. The chosen operation changes which unit types are most effective, while posture, readiness, organization, supply, war exhaustion, and a small deterministic combat variation scale the final result."),
        formula("Effective strength = sum(units * base strength * operation modifier)\n  * posture * readiness/100 * organization/100 * supply\n  * exhaustion factor * combat variation"),
        Spacer(1, 4 * mm),
        data_table(
            ["Unit", "Base strength", "Supply per unit per round"],
            [
                ["Soldier", "1", "2 cash + 0.001 Foodstuffs"],
                ["Tank", "34", "100 cash + 0.02 Energy + 0.002 Military Equipment"],
                ["Ship", "85", "300 cash + 0.08 Energy + 0.004 Military Equipment"],
                ["Fighter jet", "62", "250 cash + 0.06 Energy + 0.006 Military Equipment"],
                ["Drone", "18", "80 cash + 0.02 Energy + 0.004 Military Equipment"],
            ],
            [45 * mm, 35 * mm, 90 * mm],
        ),
        P("Operations and posture", "Subhead"),
        data_table(
            ["Choice", "Main effect"],
            [
                ["Ground Assault", "Soldiers and tanks x1.25; other units x0.90."],
                ["Air Campaign", "Jets and drones x1.30; other units x0.92."],
                ["Naval Blockade", "Ships and jets x1.32; other units x0.82. Ships gain another x1.15 on maritime routes."],
                ["Strategic Strike", "Jets and drones x1.22; other units x0.88."],
                ["Resupply", "All units fight at x0.72, but readiness rises by 7 and organization by 9."],
                ["Aggressive posture", "Strength x1.13, but that side takes 15% more casualties."],
                ["Entrenched posture", "Strength x1.08 without the aggressive casualty penalty."],
            ],
            [48 * mm, 122 * mm],
        ),
        P("Matching the declared objective matters. A Naval Blockade used for a Blockade objective gains x1.18; Ground Assault or Air Campaign used for Military Suppression gains x1.10; and Strategic Strike used for an Infrastructure Campaign gains x1.18. Orders not submitted in time default to Hold Position and Entrenched."),
        P("Supply, exhaustion, and losses", "Subhead"),
        P("The attacker pays the listed supply costs multiplied by the distance factor. If cash or a required resource is short, supply effectiveness falls to the scarcest available ratio, with a floor of 0.35. Supply is therefore never an immunity switch, but an undersupplied force fights well below full strength."),
        formula("Attacker exhaustion factor = 1 - min(0.35, exhaustion / 250)\nDefender exhaustion factor = 1 + min(0.25, exhaustion / 400)"),
        Spacer(1, 3 * mm),
        formula("Attacker loss rate = 0.004 + 0.03 * defender strength share\nDefender loss rate = 0.004 + 0.03 * attacker strength share"),
        P("Losses are removed from military inventory immediately. Each completed war round adds 2 war exhaustion to both nations; the normal hourly turn reduces exhaustion by 0.1. Combat variation is fixed from the war, round, and side, and ranges from 0.92 to 1.08."),
        PageBreak(),

        P("Winning, capitulation, and consequences", "Section"),
        P("Each round awards score according to effective strength share. At the same time, the opposing share reduces national resolve. A war ends immediately when one side reaches zero resolve, or after the round limit when the score is compared."),
        formula("Score gained = 2 + 7 * own strength share\nResolve lost = 1 + 7 * opposing strength share"),
        Spacer(1, 4 * mm),
        data_table(
            ["End state", "Threshold", "Result"],
            [
                ["Stalemate", "Score difference 3 or less", "No winner is recorded."],
                ["Minor victory", "Difference above 3", "Loser receives 3 days of reconstruction."],
                ["Major victory", "Difference 15 or more", "Loser receives 5 days of reconstruction."],
                ["Decisive victory", "Difference 35 or more, or resolve reaches 0", "Loser receives 7 days of reconstruction."],
                ["Capitulation", "After round 4, or at 35 resolve or lower", "The opponent receives a major victory."],
            ],
            [40 * mm, 55 * mm, 75 * mm],
        ),
        P("Economic damage and objective rewards", "Subhead"),
        P("Every clear defeat now damages the losing nation's Infrastructure. The Infrastructure Campaign objective increases that damage when the attacker wins; other objectives retain their normal strategic effects."),
        data_table(
            ["Result", "Implemented effect"],
            [
                ["Any clear defeat", "Infrastructure loss is 0.75% / 1.5% / 2.5% after a minor / major / decisive defeat. Stalemates inflict no concluding Infrastructure damage."],
                ["Resource Seizure", "Transfers 1% / 2% / 3% of each strategic commodity for a minor / major / decisive victory, capped at 250 units of each commodity."],
                ["Infrastructure Campaign", "With at least one Strategic Strike, total Infrastructure loss becomes 2% / 4% / 6%. Without one, only the campaign bonus is halved, producing 1.375% / 2.75% / 4.25%."],
            ],
            [49 * mm, 121 * mm],
        ),
        Spacer(1, 3 * mm),
        callout("Civic institution damage", "Institutions use a separate per-building damage roll: 0.4% / 0.8% / 1.2% after a minor / major / decisive defeat, plus 0.1 percentage point per winning Strategic Strike round (up to 1.2 points) and 0.6 points for a successful Infrastructure Campaign. Risk is capped at 3% per institution. Infrastructure never falls below 50, and falling below institution capacity does not delete or disable existing institutions; only this independent combat-damage roll can destroy one."),
        callout("No reward for losing", "Reconstruction limits fresh offensive declarations and prevents defensive dogpiling beyond one front, but it does not grant Guardian protection or an economic bonus. Every completed war also creates a seven-day armistice between the same two nations.", gold=True),
        PageBreak(),
        P("Military capacity and daily mobilization", "Section"),
        P("Military ownership is capped by population and province count. Domestic production is also paced so that moving from an empty force to the cap normally takes about ten server days."),
        formula("Unit capacity = floor(A * population) + B * provinces + C\nDaily production limit = min(capacity, max(unit floor, ceil(0.10 * capacity)))"),
        Spacer(1, 3 * mm),
        data_table(
            ["Unit", "A", "B", "C", "Daily floor"],
            [
                ["Soldiers", "0.10", "1,000", "5,000", "500"],
                ["Tanks", "0.005", "50", "50", "10"],
                ["Ships", "0.0015", "20", "10", "2"],
                ["Fighter jets", "0.002", "25", "15", "3"],
                ["Drones", "0.006", "40", "30", "5"],
            ],
            [42 * mm, 27 * mm, 30 * mm, 30 * mm, 41 * mm],
        ),
        Spacer(1, 4 * mm),
        callout("Current testing switches", "National Project production gates are wired but disabled by default. Setting MILITARY_PROJECT_REQUIREMENTS=true restores them without a migration. Bot nations are also restored each turn to testing floors of 1,000 soldiers, 25 tanks, 5 ships, and 25 fighter jets; this does not spend resources or use their daily production allowance."),
    ]

    doc.build(story)
    print(OUTPUT)


if __name__ == "__main__":
    build()
