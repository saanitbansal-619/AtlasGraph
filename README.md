# AtlasGraph — Economic Shock Propagation Engine

AtlasGraph is a backend/data-engineering system that models the world economy as
a graph of **countries**, **commodities**, **sectors** and **trade routes**, then
simulates how a disruption in one place cascades through everything that depends
on it.

Ask it a question like:

```bash
atlas shock --source Taiwan --commodity semiconductors --drop 30 --depth 3
```

…and it traces the blast radius: who is directly exposed, who is exposed
second-hand, which dependency paths carry the damage, and how every affected
entity's **fragility score** changes as a result.

> This is **not** a dashboard project. It is a serious, testable, data-driven
> engine. There is no frontend — the deliverable is a clean Go core plus a CLI,
> designed to grow into a full data platform.

---

## Why it exists

Modern economies are deeply interdependent in ways that stay invisible until
something breaks. A single chokepoint — Taiwan's semiconductor fabs, the DRC's
cobalt, the Strait of Hormuz — can ripple outward into AI hardware, cloud
infrastructure, automotive electronics and EV batteries across the globe.

AtlasGraph exists to make that propagation **explicit, queryable and
quantifiable**:

- Represent real dependency structure as a typed, weighted graph loaded from data.
- Compute **fragility** from structural reliance and supply concentration.
- Simulate shocks and measure the **change** in fragility they cause.
- Surface the highest-risk countries, commodities and sectors.

The current milestone runs on a seeded JSON dataset so the engine can be built,
tested and demoed in isolation. Real ingestion, a graph database, an analytics
store and ML forecasting are planned on top of the same core (see Roadmap).

---

## Current MVP features

- **Data-driven typed graph** — entities, dependencies and scenarios load from
  JSON (`data/sample/*.json`), embedded in the binary and overridable via
  `--data`. Every edge carries a **relationship type** and a **commodity scope**.
- **Typed shock profiles** — `export_collapse`, `supply_cut`, `price_spike` and
  `route_disruption`, each defining which relationships a shock travels along,
  how fast it attenuates and whether it may cross commodities.
- **Rule-based propagation** — a shock only spreads along edges its profile and
  the propagation rules permit, so a semiconductor shock can **no longer leak**
  into crude oil, lithium or cobalt through shared country nodes.
- **Fragility scoring** — `dependency × concentration × exposure`, on a clean
  0–100 scale, with baseline vs shocked deltas.
- **Scenario presets** — saved, named scenarios you can list and run.
- **Transparency / `--explain`** — affected paths are labelled with their
  relationship + commodity, and `--explain` prints the propagation logic and the
  branches the rules blocked.
- **Output formats** — clean executive-style text or structured JSON (now with
  `shock_profile`, `propagation_rules_applied` and `blocked_edges`), with
  optional save-to-file.
- **Graph tooling** — `graph summary`, `graph paths` and a baseline
  `risk leaderboard`.
- **External signals** — World Bank macro indicators, Comtrade-style trade
  flows, and a live **GDELT event-risk** layer (`ingest gdelt` / `events risk`)
  for geopolitical and supply-chain disruption signals from global news.
- **Strong validation** — helpful errors for malformed data, unknown entity
  references, out-of-range weights, **invalid relationship types** and
  **unknown shock types**.

---

## Architecture

```
AtlasGraph/
├── cmd/atlas/                 # CLI entry point (thin shell over internal/cli)
│   └── main.go
├── data/
│   ├── embed.go               # //go:embed of the bundled sample dataset
│   ├── sample/                # The graph dataset — single source of truth
│   │   ├── entities.json
│   │   ├── dependencies.json
│   │   └── scenarios.json
│   └── raw/                   # Ingested real data lands here (git-ignored)
├── internal/
│   ├── config/                # Engine tunables + build metadata
│   ├── models/                # Domain types + relationship/shock vocabulary
│   ├── graph/                 # In-memory directed graph + traversal/pathfinding
│   ├── data/                  # JSON loader, validation, Dataset + scenarios
│   ├── scoring/               # Fragility model
│   ├── simulation/            # Shock profiles, propagation rules, simulation
│   ├── ingest/
│   │   └── worldbank/         # World Bank API client, normalisation, summary
│   └── cli/                   # Command dispatch, text rendering, JSON output
├── Makefile
├── go.mod
└── README.md
```

**Separation of concerns** is the guiding principle:

| Layer        | Responsibility                                                      |
| ------------ | ------------------------------------------------------------------- |
| `models`     | Pure data types, no logic. The vocabulary of the domain.            |
| `graph`      | Storage + traversal. No opinion about economics or scoring.         |
| `data`       | Where the graph comes from: JSON in, validated `Dataset` out.       |
| `scoring`    | Turns graph structure into fragility numbers.                       |
| `scoring/macro` | Turns ingested macro indicators into a macro exposure score.      |
| `simulation` | Orchestrates a scenario: inject → propagate → score → summarise.    |
| `ingest`     | Pulls real external data (World Bank, trade CSVs) into normalised records. |
| `tradegraph` | Converts normalised trade flows into a loadable graph dataset.      |
| `cli`        | Human interface, text rendering and JSON shaping. No business logic.|

The `data` package is the only one that knows the graph comes from JSON. Because
`graph` exposes a small read interface, the same engine can later be backed by
Neo4j without touching `scoring` or `simulation`.

### Data model

The dataset has three files:

- **`entities.json`** — `countries`, `commodities`, `sectors`, `routes` and
  `companies` (reserved). Each entity has a `name` and optional `description`.
- **`dependencies.json`** — directed, typed, weighted edges:

```json
{
  "source": "Taiwan",
  "target": "semiconductors",
  "relationship_type": "exports",
  "weight": 0.95,
  "concentration": 0.92,
  "commodity": "semiconductors",
  "sector": "",
  "propagation_enabled": true,
  "allowed_shock_types": [],
  "cross_commodity": false,
  "description": "Taiwan dominates advanced semiconductor fabrication."
}
```

  An edge `A → B` means *B depends on A*. `weight` (0,1] is how strongly the
  target relies on this flow; `concentration` [0,1] is how concentrated the
  supply behind it is (1 = single-source, defaults to `weight`). The remaining
  fields drive **typed propagation**:

  | Field                 | Meaning                                                                       |
  | --------------------- | ----------------------------------------------------------------------------- |
  | `relationship_type`   | What kind of dependency this is (see vocabulary below). **Validated.**        |
  | `commodity`           | Scopes the edge to a commodity; shocks won't cross to a different one.         |
  | `sector`              | Optional sector context for the edge.                                         |
  | `propagation_enabled` | Set `false` to switch the edge off for propagation (default `true`).          |
  | `allowed_shock_types` | If non-empty, only these shock types may travel the edge.                     |
  | `cross_commodity`     | Mark the edge as an explicit cross-commodity bridge.                          |

  **Relationship vocabulary:** `exports`, `imports`, `supplies`, `depends_on`,
  `used_by`, `route_exposure`, `price_exposure`, `industry_dependency`,
  `company_dependency`, `macro_exposure`, `shipping_dependency`.

- **`scenarios.json`** — saved shock presets with `id`, `name`, `source`,
  `commodity`, `shock_type`, `shock_percent`, `depth` and `description`.

Example dependency chains encoded in the sample data (each hop is typed):

```
Taiwan --exports--> semiconductors --imports--> United States --industry_dependency--> AI hardware --used_by--> cloud infrastructure
Taiwan --exports--> semiconductors --imports--> Japan         --industry_dependency--> automotive electronics
China  --exports--> lithium        --used_by--> EV batteries
DRC    --exports--> cobalt         --used_by--> EV batteries
Suez Canal --route_exposure--> crude oil --shipping_dependency--> shipping logistics
crude oil  --imports--------> Europe
```

### Fragility & propagation

**Fragility** for any node:

```
fragility = dependency × concentration × exposure        (each in [0,1])
          → scaled to 0..100 and capped
```

- **dependency** — combined reliance on inbound edges, via noisy-or
  `1 − Π(1 − weight)`.
- **concentration** — the worst (max) supplier concentration among inbound edges.
- **exposure** — baseline ("peacetime") exposure from dependency, *plus* any
  disruption currently propagating into the node.

**Shock propagation**: a `--drop X%` on a `(source, commodity)` pair injects an
initial impact at the commodity equal to `drop × supplier_share`. That impact
spreads downstream breadth-first, attenuating by edge weight **and the shock
profile's attenuation factor** at each hop, capped at 1, and bounded by
`--depth`. Because only the *exposure* term moves under a shock, the **delta**
between baseline and shocked fragility is a clean measure of the damage a
scenario does.

Distance bands from the source: `1` = the shocked commodity, `2` = **direct
exposure**, `3` = **second-order exposure**.

### Typed propagation rules (why a chip shock stays a chip shock)

Earlier the engine spread a shock along *every* outgoing edge. That let a Taiwan
semiconductor shock leak into crude oil, lithium and cobalt simply because those
commodities share country nodes (the US produces oil; China refines lithium).
That is not how the world works.

Now every shock has a **shock profile** and propagation is gated by rules. Before
a shock crosses an edge, [`simulation.Evaluate`](internal/simulation/rules.go)
checks, in order:

1. **`propagation_enabled`** — the edge must be enabled.
2. **Relationship type** — the shock profile must list the edge's
   `relationship_type`.
3. **`allowed_shock_types`** — if the edge restricts shock types, this shock must
   be allowed.
4. **Commodity match** — unless the profile *or* the edge permits crossing
   commodities, the edge's `commodity` must match the shock's commodity.

The built-in profiles ([`internal/simulation/profiles.go`](internal/simulation/profiles.go)):

| Shock type         | Travels along                                                                       | Cross-commodity |
| ------------------ | ----------------------------------------------------------------------------------- | --------------- |
| `export_collapse`  | exports, imports, supplies, depends_on, used_by, industry_dependency, company_dependency | no         |
| `supply_cut`       | exports, imports, supplies, depends_on, used_by                                     | no              |
| `price_spike`      | price_exposure, depends_on, used_by, industry_dependency                            | no              |
| `route_disruption` | route_exposure, imports, exports, shipping_dependency                               | no              |

So a Taiwan `export_collapse` on semiconductors reaches the US, Japan, China,
Germany and the chip-driven sectors — but the branches into crude oil, lithium
and cobalt are **blocked** and reported. Run with `--explain` to see exactly why:

```
PROPAGATION LOGIC
----------------------------------------------------------------
  Shock type                 : export_collapse
  Allowed relationships      : company_dependency, depends_on, exports, imports, industry_dependency, supplies, used_by
  Per-hop attenuation        : 0.85
  Cross-commodity propagation: disabled
  Blocked unrelated branches : cobalt, crude oil, lithium
  Blocked edges:
    United States --exports--> crude oil   [cross-commodity branch blocked: edge commodity "crude oil" != shock commodity "semiconductors"]
    China --exports--> lithium   [cross-commodity branch blocked: ...]
    China --exports--> cobalt   [cross-commodity branch blocked: ...]
```

---

## How to run

### Prerequisites

- Go **1.21+** (developed and tested on Go 1.26).

### Build & test

```bash
make build        # compile ./bin/atlas
make test         # run all unit tests
make run          # run the canonical Taiwan scenario
make scenarios    # list scenario presets
make summary      # graph summary statistics
make leaderboard  # baseline fragility leaderboard
make check        # fmt + vet + test
```

Or use the Go toolchain directly:

```bash
go test ./...
go run ./cmd/atlas shock --source Taiwan --commodity semiconductors --drop 30 --depth 3
```

### CLI commands

```
atlas shock        --source <entity> --commodity <name> [--type kind] [--drop N] [--depth N]
                   [--data dir] [--output text|json] [--save file] [--explain]
atlas scenario list                      [--data dir]
atlas scenario run <id>                  [--data dir] [--output text|json] [--save file] [--explain]
atlas graph summary                      [--data dir] [--top N]
atlas graph paths  --from <e> --to <e>   [--data dir] [--depth N]
atlas graph dump                         [--data dir]
atlas risk leaderboard                   [--data dir] [--top N]
atlas ingest worldbank --countries <ISO3,…> [--start Y] [--end Y] [--out dir]
atlas ingest gdelt     --countries <ISO3,…> [--days N] [--out dir]
atlas indicators country <ISO3>          [--data dir]
atlas score macro                        [--data dir] [--year Y] [--output text|json] [--save file] [--verbose]
atlas events risk                        [--data dir] [--output text|json] [--save file]
atlas version
```

`--type` selects the shock profile (`export_collapse`, `supply_cut`,
`price_spike`, `route_disruption`; default `export_collapse`). `--explain` prints
the propagation logic and the branches the rules blocked.

The dataset is embedded in the binary, so every command works with no flags.
Pass `--data data/sample` to load from disk instead.

### Examples

```bash
# Custom shock, text output
atlas shock --source Taiwan --commodity semiconductors --drop 30 --depth 3 --data data/sample

# JSON output for programmatic use
atlas shock --source Taiwan --commodity semiconductors --drop 30 --depth 3 --output json

# Save structured results to a file
atlas shock --source Taiwan --commodity semiconductors --drop 30 --depth 3 --save results/taiwan_shock.json

# Scenario presets
atlas scenario list --data data/sample
atlas scenario run taiwan_semiconductor_shock --data data/sample

# Graph tooling
atlas graph summary --data data/sample
atlas graph paths --from Taiwan --to "cloud infrastructure" --data data/sample
atlas risk leaderboard --data data/sample
```

---

## Example output

```
atlas shock --source Taiwan --commodity semiconductors --drop 30 --depth 3
```

```
SCENARIO
----------------------------------------------------------------
  Source           : Taiwan (country)
  Commodity        : semiconductors
  Shock type       : export_collapse (Export Collapse)
  Flow drop        : 30%
  Propagation depth: 3 hops
  Initial impact   : 28%  (flow drop x supplier share)

DIRECT EXPOSURE
----------------------------------------------------------------
  ENTITY         TYPE     IMPACT  FRAGILITY (BASE -> SHOCK)
  United States  country  21%     36.6 -> 52.9  (+16.3)
  Japan          country  17%     15.6 -> 22.8  (+7.3)
  China          country  15%     11.6 -> 17.1  (+5.5)
  Germany        country  14%     8.4 -> 12.4  (+3.9)

SECOND-ORDER EXPOSURE
----------------------------------------------------------------
  ENTITY                  TYPE    IMPACT  FRAGILITY (BASE -> SHOCK)
  automotive electronics  sector  19%     28.9 -> 40.3  (+11.4)
  AI hardware             sector  15%     25.3 -> 34.2  (+8.9)
  cloud infrastructure    sector  14%     30.2 -> 39.0  (+8.8)
  consumer devices        sector  10%     25.0 -> 30.3  (+5.3)

AFFECTED DEPENDENCY PATHS
----------------------------------------------------------------
  Taiwan --exports/semiconductors--> semiconductors --imports/semiconductors--> United States --industry_dependency/semiconductors--> AI hardware   [impact 15%, path weight 0.71]
  ... (more) ...

GRAPH IMPACT SUMMARY
----------------------------------------------------------------
  Nodes in graph        : 23
  Affected nodes        : 9  (countries 4, commodities 1, sectors 4)
  Affected paths        : 10
  Avg fragility delta   : +11.1
  Largest single impact : semiconductors (+32.3 fragility)
```

Note what is **absent**: crude oil, lithium and cobalt. Under the old engine they
were collateral damage; under typed rules they are correctly left untouched (and
listed as blocked branches under `--explain`).

JSON output (`--output json`) returns a structured object with these top-level
keys: `scenario`, `shock_profile`, `propagation_rules_applied`, `direct_exposure`,
`second_order_exposure`, `affected_paths` (each hop labelled with its
relationship), `changed_fragility_scores`, `highest_risk_entities`,
`graph_impact_summary`, and `blocked_edges` (when `--explain` is set).

> "Highest-risk entities" are ranked by **shock-driven fragility increase** (the
> delta this scenario caused), not by absolute fragility — the question being
> answered is "what did *this* shock hurt most?".

---

## Real data ingestion: World Bank macro indicators

This is AtlasGraph's **first real external data source**. The
[`internal/ingest/worldbank`](internal/ingest/worldbank) package fetches
macroeconomic indicators from the [World Bank Indicators API v2](https://api.worldbank.org/v2)
using only the Go standard library, normalises them into a flat record set, and
saves them locally. Later milestones will use this data to ground baseline
fragility scoring in real-world numbers instead of seeded weights.

### Indicators fetched

| Code             | Indicator                                   |
| ---------------- | ------------------------------------------- |
| `NY.GDP.MKTP.CD` | GDP (current US$)                           |
| `NE.IMP.GNFS.ZS` | Imports of goods and services (% of GDP)    |
| `NE.EXP.GNFS.ZS` | Exports of goods and services (% of GDP)    |
| `NV.IND.MANF.ZS` | Manufacturing value added (% of GDP)        |
| `FP.CPI.TOTL.ZG` | Inflation, consumer prices (annual %)       |
| `TX.VAL.TECH.CD` | High-technology exports (current US$)       |

### Fetch and inspect

```bash
# Fetch a panel of countries (ISO3 codes) over a year range
go run ./cmd/atlas ingest worldbank --countries USA,CHN,JPN,DEU,KOR --start 2018 --end 2023 --out data/raw/worldbank

# Inspect the latest available indicators for one country
go run ./cmd/atlas indicators country USA --data data/raw/worldbank
```

```
COUNTRY INDICATORS
----------------------------------------------------------------
  Country               : United States (USA)
  Latest year with data : 2023

  INDICATOR                                 YEAR  VALUE
  GDP (current US$)                         2023  US$ 27,292,170,793,214
  Imports of goods and services (% of GDP)  2023  14.11%
  Exports of goods and services (% of GDP)  2023  11.18%
  Manufacturing value added (% of GDP)      2021  10.71%
  Inflation, consumer prices (annual %)     2023  4.12%
  High-technology exports (current US$)     2023  US$ 208,514,376,770
```

### Notes

- **Reliable country set.** Use ISO3 codes the API supports cleanly:
  `USA, CHN, JPN, DEU, KOR, IND, SAU, COD`. Taiwan (TWN) is intentionally **not**
  ingested here because the World Bank API does not serve it cleanly — it remains
  a first-class node in the seeded graph, just not in the macro panel.
- **Batched requests.** All requested countries are fetched in a single call per
  indicator (semicolon-separated), so a five-country panel is six HTTP requests,
  not thirty.
- **Robust client.** Context timeouts, non-200 handling, malformed-JSON
  detection, API error messages, pagination and genuinely missing values (kept as
  JSON `null`, never coerced to 0) are all handled with clear errors.
- **Output.** Normalised records are written to
  `data/raw/worldbank/worldbank_indicators.json` (the directory is created if
  needed). `data/raw/` is git-ignored — only a `.gitkeep` is tracked — so large
  downloads never land in version control.

The output record shape:

```json
{
  "country_code": "USA",
  "country_name": "United States",
  "indicator_code": "NY.GDP.MKTP.CD",
  "indicator_name": "GDP (current US$)",
  "year": 2023,
  "value": 27292170793214,
  "source": "World Bank Indicators API v2",
  "fetched_at": "2024-07-01T00:00:00Z"
}
```

---

## Macro Exposure Scoring

Once macro indicators are ingested, AtlasGraph turns them into an **explainable
Macro Exposure Score** per country — the bridge between Phase 1's seeded graph
weights and real-world fundamentals. This is implemented in
[`internal/scoring/macro`](internal/scoring/macro).

The score is built from a **selected set of World Bank macroeconomic exposure
indicators** — trade exposure, manufacturing dependency, inflation stress,
high-tech export concentration and economic-buffer risk.

> **This is exposure/risk scoring, not forecasting.** It does not predict
> markets, prices or crises. It measures how *structurally exposed* an economy
> is — to trade, supply-chain, price and technology shocks — given its latest
> macro fundamentals, and shows exactly which factors drive that exposure.
>
> **This is not the final, complete AtlasGraph fragility score.** It covers
> macro exposure only. Full fragility scoring will later combine this with graph
> dependency / centrality, supplier concentration, event risk and commodity
> volatility.

### Components

Each country's score blends five normalised components (each 0–100):

| Component                    | Built from                                   | Higher means …                          |
| ---------------------------- | -------------------------------------------- | --------------------------------------- |
| `trade_exposure`             | imports % GDP + exports % GDP                | more exposed to trade disruption        |
| `manufacturing_dependency`   | manufacturing value added % GDP              | more exposed to supply-chain shocks     |
| `inflation_stress`           | inflation, annual %                          | more macro price stress                 |
| `high_tech_concentration`    | high-tech exports ÷ GDP                      | more exposed to tech-trade disruption   |
| `economic_buffer_risk`       | GDP size (log scale), **inverse**            | smaller economy = less shock-absorbing  |

Components use **calibrated absolute reference bands**, not min-max over the
loaded panel, so a country's score is stable no matter which other countries are
present (and a single-country file still scores sensibly).

### Final score and risk bands

```
macro_exposure_score = 0.30·trade_exposure
                     + 0.25·manufacturing_dependency
                     + 0.20·inflation_stress
                     + 0.15·high_tech_concentration
                     + 0.10·economic_buffer_risk        → 0..100
```

Weights sum to 1.0. When an indicator is missing, its component is dropped and
the remaining weights are **renormalised**, so gaps in the data never silently
deflate a score. Each component records the year it actually used (the latest
available at or before the requested `--year`).

| Score   | Risk level |
| ------- | ---------- |
| 0–30    | Low        |
| 30–60   | Medium     |
| 60–80   | High       |
| 80–100  | Critical   |

### Commands

```bash
go run ./cmd/atlas score macro --data data/raw/worldbank
go run ./cmd/atlas score macro --data data/raw/worldbank --year 2023
go run ./cmd/atlas score macro --data data/raw/worldbank --verbose
go run ./cmd/atlas score macro --data data/raw/worldbank --explain-formula
go run ./cmd/atlas score macro --data data/raw/worldbank --output json
go run ./cmd/atlas score macro --data data/raw/worldbank --save results/macro_scores.json
```

```
MACRO EXPOSURE SCORES
----------------------------------------------------------------
  Year lens: 2023 (latest available <= 2023 per indicator)

  COUNTRY        YEAR  SCORE  RISK    TOP DRIVERS
  Korea, Rep.    2023  48.6   Medium  manufacturing dependency, trade exposure
  Germany        2023  40.9   Medium  manufacturing dependency, trade exposure
  China          2023  29.1   Low     manufacturing dependency, high-tech concentration
  Japan          2023  26.5   Low     manufacturing dependency, trade exposure
  United States  2023  9.4    Low     inflation stress, manufacturing dependency

  Risk bands: Low 0-30 | Medium 30-60 | High 60-80 | Critical 80-100
```

`--verbose` adds a per-country breakdown of every component (score, weight,
contribution and the year used); `--output json` emits the same data with
`weights`, `risk_bands` and per-component detail (each country's score under the
`macro_exposure_score` field) for programmatic use. `--explain-formula` prints
the score name, weighted formula, component definitions, risk bands and an
explicit limitation note, then exits without needing ingested data.

---

## Trade Dependency Ingestion

This milestone introduces **trade-flow data ingestion** from local CSV files,
the foundation for measuring country-to-country commodity dependency and
supplier concentration. It deliberately reads **local CSV** (no external APIs)
so real datasets — e.g. UN Comtrade exports — can later be dropped in unchanged.
It is implemented in [`internal/ingest/trade`](internal/ingest/trade).

From the ingested flows it computes **supplier dependency** (how an importer's
purchases of a commodity split across exporters) and **concentration risk** (the
Herfindahl-Hirschman Index over those supplier shares). These signals are meant
to later feed the graph shock-propagation engine's baseline edge weights.

### Input CSV format

```
year,exporter_code,exporter_name,importer_code,importer_name,commodity_code,commodity_name,trade_value_usd,quantity,unit
2023,TWN,Taiwan,USA,United States,8542,semiconductors,85000000000,0,USD
```

The loader validates that all required columns are present (order-independent,
case-insensitive), parses numbers safely (tolerating thousands separators), and
**skips malformed rows with a clear, line-numbered reason** rather than aborting
the whole file. Normalised records are written to
`data/processed/trade/trade_flows.json`.

### Commands

```bash
go run ./cmd/atlas ingest trade --file data/examples/trade_flows_sample.csv --out data/processed/trade
go run ./cmd/atlas trade summary --data data/processed/trade
go run ./cmd/atlas trade dependency --importer USA --commodity semiconductors --data data/processed/trade
go run ./cmd/atlas trade concentration --importer USA --commodity semiconductors --data data/processed/trade
```

Ingestion reports total / valid / skipped rows and the countries, commodities
and total trade value detected:

```
TRADE INGESTION
----------------------------------------------------------------
  Source file       : data/examples/trade_flows_sample.csv
  Output            : data/processed/trade/trade_flows.json
  Total rows        : 19
  Valid rows        : 19
  Skipped rows      : 0
  Countries detected: 9
  Commodities       : 5
  Total trade value : US$ 275.0B
```

`trade dependency` ranks supplier countries by value, with each supplier's share
and a per-supplier dependency band (Low <10% | Medium 10–40% | High ≥40%):

```
SUPPLIER DEPENDENCY
----------------------------------------------------------------
  Importer     : United States
  Commodity    : semiconductors
  Total imports: US$ 137.0B

  SUPPLIER     VALUE       SHARE  DEPENDENCY
  Taiwan       US$ 85.0B   62.0%  High
  Korea Rep.   US$ 21.0B   15.3%  Medium
  Japan        US$ 12.0B   8.8%   Low
  China        US$ 10.0B   7.3%   Low
  Germany      US$ 9.0B    6.6%   Low
```

`trade concentration` reduces the supplier shares to a single HHI and risk band
(HHI < 0.15 Low | 0.15–0.25 Medium | > 0.25 High):

```
SUPPLIER CONCENTRATION
----------------------------------------------------------------
  Importer          : United States
  Commodity         : semiconductors
  HHI               : 0.43
  Concentration risk: High
  Top supplier      : Taiwan, 62.0%
```

Both `trade dependency` and `trade concentration` accept `--output json` for
programmatic use. The importer can be given as an ISO code or country name, and
the commodity as a name or HS code.

---

## UN Comtrade-Style CSV Import

Real trade datasets are most easily obtained as **downloaded UN Comtrade CSV
exports**. This importer ingests those files directly — no API credentials are
required yet — and normalises them into the *same* `trade_flows.json` the rest
of the trade pipeline already consumes, so every downstream command works
unchanged. It is implemented alongside the custom importer in
[`internal/ingest/trade`](internal/ingest/trade); the original custom-schema
ingest (`ingest trade`) is untouched.

### Input columns

Comtrade exports describe a flow from a **reporter** to a **partner** with a
`flowDesc` direction; the importer resolves these into AtlasGraph's
exporter/importer model:

```
refYear,flowDesc,reporterISO,reporterDesc,partnerISO,partnerDesc,cmdCode,cmdDesc,primaryValue,qty,qtyUnitAbbr
2023,Import,USA,United States,TWN,Taiwan,8542,Electronic integrated circuits,85000000000,0,N/A
2023,Export,SAU,Saudi Arabia,DEU,Germany,2709,"Petroleum oils, crude",14000000000,0,N/A
```

- **Import** rows: `importer = reporter`, `exporter = partner`.
- **Export** rows: `exporter = reporter`, `importer = partner`.
- Only `Import` / `Export` flows are kept; other flows (e.g. re-exports) and rows
  missing `reporterISO`, `partnerISO`, `cmdCode` or `primaryValue` are skipped
  with a clear, line-numbered reason.

Commodity descriptions/HS codes are normalised to AtlasGraph's canonical
commodity names so they line up with the curated graph and scenarios:
`electronic integrated circuits` / code `8542` → **semiconductors**,
`lithium`/`batteries` → **lithium batteries**, `cobalt` → **cobalt ores**,
`petroleum oils`/`crude` → **crude oil**, `rare earth` → **rare earths**.
Anything else keeps a cleaned, lower-cased description.

### Commands

```bash
go run ./cmd/atlas ingest trade-comtrade --file data/examples/comtrade_sample.csv --out data/processed/trade
go run ./cmd/atlas trade summary --data data/processed/trade
go run ./cmd/atlas graph build-trade --trade-data data/processed/trade --out data/generated/trade_graph
```

Ingestion reports total / valid / skipped rows, the import vs export flow split,
and the countries, commodities and total trade value detected:

```
COMTRADE TRADE INGESTION
----------------------------------------------------------------
  Source file       : data/examples/comtrade_sample.csv
  Output            : data/processed/trade/trade_flows.json
  Total rows        : 19
  Valid rows        : 19
  Skipped rows      : 0
  Flows imported    : 9
  Flows exported    : 10
  Countries detected: 9
  Commodities       : 5
  Total trade value : US$ 275.0B
```

This supports downloaded Comtrade-style CSVs without requiring API credentials
yet. (Live Comtrade API ingestion is intentionally out of scope for now.)

---

## GDELT Event Risk Ingestion

AtlasGraph's third external signal is a **live event-risk layer** drawn from
global news/event data via the [GDELT DOC 2.0 API](https://api.gdeltproject.org/api/v2/doc/doc).
It complements the two structural signals already in the engine:

- **macro exposure** from World Bank indicators,
- **trade dependency** from Comtrade-style data, and now
- **event risk** from GDELT.

This is **not treated as ground truth** — it is a noisy public signal for
geopolitical and disruption-related risk (sanctions, conflict, export controls,
shipping disruption, semiconductor/energy/commodity stress, …). It is useful as
a near-real-time nudge on top of the slower-moving structural fundamentals.

### How it works

For each requested country, the importer issues one GDELT query combining the
country name with a fixed set of risk keywords:

```
sanctions, conflict, military, protest, strike, supply chain,
export controls, trade restrictions, shipping disruption,
semiconductor, energy, commodity
```

Countries are supplied as ISO3 codes and mapped to GDELT-friendly names:

| Code | Country                          | Code | Country        |
|------|----------------------------------|------|----------------|
| TWN  | Taiwan                           | USA  | United States  |
| CHN  | China                            | DEU  | Germany        |
| JPN  | Japan                            | SAU  | Saudi Arabia   |
| KOR  | South Korea                      | COD  | DR Congo       |
| IND  | India                            |      |                |

Each returned document is normalised into a stable `GDELTEventRecord`
(`country_code`, `country_name`, `title`, `url`, `source_country`, `domain`,
`published_at`, `tone`, `language`, `themes`, `risk_terms_matched`, `source`,
`fetched_at`). Fields the API does not provide in a given mode are left empty so
the schema never changes. Records are written to
`data/raw/gdelt/gdelt_events.json`.

The GDELT client lives behind a small `Fetcher` interface and an overridable
base URL, so it is fully testable from saved JSON fixtures (via `httptest`) —
the test suite never calls the live GDELT service. The CLI makes real HTTP calls
for actual use.

### Commands

```bash
go run ./cmd/atlas ingest gdelt --countries TWN,CHN,JPN,KOR,USA,DEU --days 7 --out data/raw/gdelt
go run ./cmd/atlas events risk --data data/raw/gdelt
go run ./cmd/atlas events risk --data data/raw/gdelt --output json
```

Ingestion reports how many documents were fetched, how many matched risk terms,
and the leading countries and risk terms:

```
GDELT EVENT INGESTION
----------------------------------------------------------------
  Countries              : TWN, CHN, JPN, KOR, USA, DEU
  Days                   : 7
  Records fetched        : 312
  Records with risk terms: 198
  Output                 : data/raw/gdelt/gdelt_events.json

  Top countries by event count:
    1. China                            74
    2. United States                    68
    3. Taiwan                           …

  Top matched risk terms:
    1. sanctions                        86
    2. semiconductor                    61
    3. conflict                        …
```

### Event risk scoring

`events risk` scores each country on a 0–100 scale from three components,
combined with calibrated weights (`internal/scoring/events`):

```
event_risk_score =
    0.45 * event_count_score        // volume of risk-relevant coverage
  + 0.35 * negative_tone_score      // how negative that coverage is
  + 0.20 * risk_term_density_score  // distinct risk themes per article
```

Each component is mapped onto 0–100 with absolute reference bands (so a
country's score does not depend on which other countries are in the panel), and
the final score falls into a qualitative band:

| Score   | Risk     |
|---------|----------|
| 0–30    | Low      |
| 30–60   | Medium   |
| 60–80   | High     |
| 80–100  | Critical |

```
EVENT RISK SCORES
----------------------------------------------------------------
  COUNTRY         EVENTS  AVG TONE  SCORE  RISK      TOP TERMS
  Taiwan          74      -6.8      71.4   High      sanctions, semiconductor, conflict
  China           68      -5.1      63.2   High      sanctions, export controls, conflict
  Japan           21      -1.2      28.7   Low       energy, supply chain

  Risk bands: Low 0-30 | Medium 30-60 | High 60-80 | Critical 80-100
  Note: a public event-risk signal from global news, not ground truth.
```

`--output json` emits the same scores (with per-component breakdowns, weights
and risk bands) as structured JSON, and `--save <file>` writes that JSON to disk.

---

## Generated Trade Graphs

This step converts trade-flow data into a dependency graph, so scenario shocks
are no longer limited to the manually seeded `data/sample` graph. The hand-seeded
sample dataset is left untouched; generated graphs are written to a separate
directory and consumed by the same `graph` / `shock` commands via `--data`. The
conversion lives in [`internal/tradegraph`](internal/tradegraph).

```bash
go run ./cmd/atlas ingest trade --file data/examples/trade_flows_sample.csv --out data/processed/trade
go run ./cmd/atlas graph build-trade --trade-data data/processed/trade --out data/generated/trade_graph
go run ./cmd/atlas graph summary --data data/generated/trade_graph
go run ./cmd/atlas shock --source Taiwan --commodity semiconductors --drop 30 --depth 3 --data data/generated/trade_graph --explain
```

### How records become a graph

Each normalised trade record is turned into typed graph entities and edges:

| Edge                                          | Relationship          | Weight                                                                 |
| --------------------------------------------- | --------------------- | ---------------------------------------------------------------------- |
| exporter country → commodity                  | `exports`             | exporter's share of that commodity's total export value (supplier importance) |
| commodity → importer country                  | `imports`             | importer's **top-supplier share**, with sourcing **HHI** as concentration |
| importer country → commodity-dependent sector | `industry_dependency` | coarse default dependency from the commodity→sector mapping            |

This preserves the supplier-dependency signal end to end: e.g. if the USA sources
62% of its semiconductors from Taiwan, the `Taiwan → semiconductors` edge carries
Taiwan's supplier importance and the `semiconductors → United States` edge carries
the 62% top-supplier share, so the `Taiwan → semiconductors → United States` path
weight reflects that concentration. Sectors are attached from a small, explicit
commodity→sector map (e.g. semiconductors → AI hardware, cloud infrastructure,
automotive electronics, consumer devices).

Generated scenario presets are emitted when their trigger flow is present in the
data — `taiwan_semiconductor_shock` (Taiwan exports semiconductors),
`lithium_battery_shock` (China exports lithium batteries) and
`crude_oil_supply_shock` (Saudi Arabia exports crude oil).

```
TRADE GRAPH BUILD
----------------------------------------------------------------
  Source trade data  : data/processed/trade
  Output             : data/generated/trade_graph
  Countries          : 9
  Commodities        : 5
  Sectors            : 8
  Dependencies       : 45
  Generated scenarios: 3
  Top generated dependency: DRC --exports--> cobalt ores (weight 1.00)
  Highest concentration import dependency: China <- cobalt ores (HHI 1.00, top DRC 100.0%)
```

The generated `entities.json`, `dependencies.json` and `scenarios.json` use
exactly the wire format the loader validates, so the standard `graph summary`,
`graph paths`, `graph dump` and `shock` commands all work against the output.

---

## Testing

The engine is covered by unit tests across the core packages:

- `internal/data` — JSON loading (embedded and on-disk), scenario presets,
  malformed/missing files, duplicate entities, unknown references and
  out-of-range weights.
- `internal/graph` — node/edge bookkeeping, neighbours, depth-bounded path
  enumeration, `PathsBetween`, degree/counts, `FindByName` resolution, cycle
  safety.
- `internal/scoring` — the fragility formula, clamping/capping, monotonicity,
  noisy-or dependency aggregation, concentration max.
- `internal/simulation` — input validation, initial-impact math, direct vs
  second-order classification, monotonicity in drop size, depth limiting, the
  "zero drop ⇒ no impact" invariant, **and the typed propagation rules**: a
  semiconductor shock not reaching crude oil/lithium/cobalt, `price_spike`
  travelling `price_exposure`, `route_disruption` travelling `route_exposure`,
  relationship-labelled paths, and unknown shock types failing cleanly.
- `internal/ingest/worldbank` — the World Bank client against an `httptest`
  server (success, pagination, non-200, malformed JSON, API error messages,
  empty results and context timeout), normalisation, save/load round-trips and
  per-country summary building. **No test touches the real network.**
- `internal/scoring/macro` — component normalisation and clamping, the weighted
  blend, risk-band assignment, missing-indicator fallback + weight
  renormalisation, year-lens selection and score ordering.
- `internal/ingest/trade` — CSV parsing (reordered/mixed-case headers), required
  column validation, malformed-row skipping with reasons, safe numeric parsing,
  save/load round-trips, summary aggregation, supplier-share and HHI
  concentration maths, and dependency/concentration risk bands.
- `internal/tradegraph` — converting trade records into country/commodity/sector
  entities, supplier-share export/import edge weighting, the
  exports/imports/industry_dependency edge set, generated scenario triggers, and
  a full round-trip proving the generated dataset loads and simulates.
- `internal/cli` — command dispatch, scenario list/run, graph summary/paths,
  risk leaderboard, JSON output shape (incl. profile/rules/blocked edges),
  labelled paths, `--explain` output, the `ingest`/`indicators`/`trade` commands,
  `graph build-trade` plus running `graph summary`/`shock` against the generated
  graph, and save-to-file behaviour.

```bash
go test ./...
```

---

## Roadmap

The engine is deliberately a clean, data-driven core. Planned expansion:

- **Phase 1 — Seeded graph engine** ✅ *(current)* — JSON-driven **typed** graph,
  shock profiles + rule-based propagation, fragility scoring, scenarios, CLI and
  tests.
- **Phase 2 — Real trade data ingestion** 🛠️ *(in progress)* — the World Bank
  macro ingestion module (`atlas ingest worldbank`) is the first real source, and
  `atlas score macro` already turns those indicators into an explainable macro
  exposure score per country. Next: pull production shares and trade flows from
  trade APIs and fold them into the graph's baseline weights, keeping the same
  interface so nothing downstream changes.
- **Phase 3 — Neo4j graph database** — persist the dependency graph and push
  traversal into the database for larger, real-world graphs.
- **Phase 4 — ClickHouse analytics layer** — store scenario runs and time series
  for large-scale querying and historical comparison.
- **Phase 5 — MLflow + LightGBM forecasting** — predict shock likelihood and
  forward-looking fragility from historical signals.
- **Phase 6 — Docker / AWS deployment** — reproducible, containerised stack and
  cloud deployment.

A dashboard, if it ever happens, sits on top of all of this — never in front of
it.
