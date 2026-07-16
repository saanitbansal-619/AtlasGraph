import csv
import random
from datetime import date, timedelta
from pathlib import Path

random.seed(42)

countries = [
    ("USA", "United States"),
    ("CHN", "China"),
    ("TWN", "Taiwan"),
    ("JPN", "Japan"),
    ("KOR", "Korea, Rep."),
    ("DEU", "Germany"),
    ("IND", "India"),
    ("RUS", "Russia"),
    ("UKR", "Ukraine"),
    ("SAU", "Saudi Arabia"),
    ("CHL", "Chile"),
    ("COD", "Congo, Dem. Rep."),
]

event_types = [
    "conflict",
    "protest",
    "sanction",
    "export_control",
    "shipping_disruption",
    "port_disruption",
    "energy_disruption",
    "supply_chain_disruption",
    "political_risk",
]

profiles = {
    "conflict": (-8.5, -4.0, -9.5, -5.0),
    "protest": (-5.0, -1.5, -4.0, -1.0),
    "sanction": (-6.5, -3.0, -5.5, -2.0),
    "export_control": (-5.0, -2.0, -4.5, -1.5),
    "shipping_disruption": (-6.0, -2.5, -5.0, -2.0),
    "port_disruption": (-5.5, -2.0, -4.5, -1.5),
    "energy_disruption": (-6.5, -3.0, -5.5, -2.0),
    "supply_chain_disruption": (-5.0, -1.5, -4.0, -1.0),
    "political_risk": (-4.5, -1.0, -3.5, -0.5),
}

notes_by_type = {
    "conflict": [
        "Armed clash reports near contested corridor",
        "Border security incident elevates regional tension",
        "Military posture shift reported in open sources",
        "Cross-border fire exchange cited in regional media",
    ],
    "protest": [
        "Large demonstration reported in major city",
        "Labor unrest linked to cost-of-living pressures",
        "Civic marches disrupt downtown logistics corridors",
        "Student and civil-society protests expand",
    ],
    "sanction": [
        "New restrictive measures discussed in policy forums",
        "Targeted financial restrictions proposed",
        "Secondary sanctions risk flagged by analysts",
        "Trade restriction package under review",
    ],
    "export_control": [
        "Technology export licensing tightened",
        "Dual-use goods screening intensified",
        "Semiconductor-related control updates reported",
        "Critical minerals export scrutiny increased",
    ],
    "shipping_disruption": [
        "Commercial vessel delays reported on key lane",
        "Insurance premiums rise on chokepoint routes",
        "Rerouting advice issued for merchant fleets",
        "Transit slowdown linked to security advisories",
    ],
    "port_disruption": [
        "Container backlog reported at major terminal",
        "Port labor stoppage slows cargo clearance",
        "Weather and congestion delay berth schedules",
        "Customs backlog extends dwell times",
    ],
    "energy_disruption": [
        "Pipeline flow interruption cited by monitors",
        "Refinery outage pressures regional fuel supply",
        "Power-grid disturbance affects industrial zones",
        "Energy export schedule revisions announced",
    ],
    "supply_chain_disruption": [
        "Component shortage lengthens lead times",
        "Factory logistics bottleneck reported",
        "Critical input scarcity affects assembly lines",
        "Inventory drawdowns flagged for key inputs",
    ],
    "political_risk": [
        "Cabinet reshuffle raises policy uncertainty",
        "Election-related volatility noted in coverage",
        "Regulatory uncertainty for foreign investors",
        "Diplomatic friction increases commercial caution",
    ],
}

weights = {
    "USA": 18,
    "CHN": 20,
    "TWN": 16,
    "JPN": 12,
    "KOR": 12,
    "DEU": 14,
    "IND": 14,
    "RUS": 22,
    "UKR": 24,
    "SAU": 12,
    "CHL": 10,
    "COD": 12,
}

start = date(2024, 1, 5)
end = date(2024, 12, 20)


def rand_date():
    span = (end - start).days
    return start + timedelta(days=random.randint(0, span))


rows = []
for code, name in countries:
    for _ in range(weights[code]):
        et = random.choice(event_types)
        if code in ("UKR", "RUS") and random.random() < 0.45:
            et = random.choice(["conflict", "sanction", "energy_disruption", "political_risk"])
        if code in ("TWN", "CHN", "USA", "KOR", "JPN") and random.random() < 0.35:
            et = random.choice(
                ["export_control", "supply_chain_disruption", "sanction", "political_risk"]
            )
        if code == "SAU" and random.random() < 0.4:
            et = random.choice(["energy_disruption", "shipping_disruption", "political_risk"])
        if code == "DEU" and random.random() < 0.3:
            et = random.choice(["energy_disruption", "protest", "supply_chain_disruption"])
        if code in ("CHL", "COD", "IND") and random.random() < 0.3:
            et = random.choice(
                ["supply_chain_disruption", "protest", "political_risk", "port_disruption"]
            )

        tlo, thi, glo, ghi = profiles[et]
        rows.append(
            {
                "date": rand_date().isoformat(),
                "country_code": code,
                "country_name": name,
                "event_type": et,
                "tone": round(random.uniform(tlo, thi), 1),
                "goldstein_score": round(random.uniform(glo, ghi), 1),
                "mention_count": random.randint(3, 120),
                "source": "GDELT",
                "notes": random.choice(notes_by_type[et]),
            }
        )

rows.sort(key=lambda r: (r["date"], r["country_code"], r["event_type"]))

out = Path(r"c:\Users\Saanit\Desktop\AtlasGraph\data\raw\gdelt_events\gdelt_events_2024_expanded.csv")
out.parent.mkdir(parents=True, exist_ok=True)
fields = [
    "date",
    "country_code",
    "country_name",
    "event_type",
    "tone",
    "goldstein_score",
    "mention_count",
    "source",
    "notes",
]
with out.open("w", newline="", encoding="utf-8") as f:
    w = csv.DictWriter(f, fieldnames=fields)
    w.writeheader()
    w.writerows(rows)

print(f"wrote {len(rows)} rows to {out}")
print("countries:", sorted({r["country_code"] for r in rows}))
print("types:", sorted({r["event_type"] for r in rows}))
print(f"date range: {rows[0]['date']} .. {rows[-1]['date']}")
