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
| `simulation` | Orchestrates a scenario: inject → propagate → score → summarise.    |
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
atlas indicators country <ISO3>          [--data dir]
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
- `internal/cli` — command dispatch, scenario list/run, graph summary/paths,
  risk leaderboard, JSON output shape (incl. profile/rules/blocked edges),
  labelled paths, `--explain` output, the `ingest`/`indicators` commands and
  save-to-file behaviour.

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
  macro ingestion module (`atlas ingest worldbank`) is the first real source.
  Next: pull production shares and trade flows from trade APIs and feed them into
  baseline fragility, keeping the same graph interface so nothing downstream
  changes.
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
