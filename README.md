# GFIP — Global Fragility Intelligence Platform

**Powered by AtlasGraph**

GFIP is a strategic supply-chain and infrastructure risk intelligence platform
that models how shocks to countries, commodities, and trade chokepoints propagate
through global dependency networks.

- **GFIP** — the user-facing intelligence platform (React dashboard for analyst-style exploration).
- **AtlasGraph** — the Go backend engine powering graph modeling, shock analysis, scoring, and API serving.

---

## What it does

In plain terms, GFIP:

1. **Builds a baseline dependency graph** — countries, commodities, sectors, and maritime routes linked by typed trade and industry relationships.
2. **Models shocks** — export collapses, supply cuts, route disruptions, and price spikes propagate along relationship-aware paths.
3. **Ranks estimated impact** — affected countries, commodities, sectors, and dependency paths with fragility deltas.
4. **Compares crisis scenarios** — run multiple shocks side-by-side and rank systemic estimated impact.
5. **Computes unified fragility scores** — explainable composite scores blending macro, trade, event, commodity, and graph signals.
6. **Provides a dashboard** — analyst UI for exploring graph size, fragility, shocks, paths, and scenario comparison.

---

## Key Features

- **Baseline dependency graph dataset** — curated graph for reproducible local runs (`data/strategic_global`)
- **Graph-based shock propagation engine** — typed shock profiles with relationship filtering and attenuation
- **Graph-guided custom shock controls** — only valid source → commodity → shock-type combinations are offered
- **Unified Fragility Score** — explainable country and commodity composite scores
- **Scenario Comparison Mode** — compare multiple shocks and rank by average/max fragility delta
- **Dependency path explanations** — readable paths with relationship labels, estimated impact, and weight
- **Frontend analytics charts** — adaptive bar charts and compact ranking lists
- **REST API endpoints** — pure `net/http` JSON API with CORS for the dashboard
- **CLI support** — same engine via `atlas` commands
- **Tests and reproducible local workflow** — Go unit tests across core packages; frontend build validation

**Observed data:** UN Comtrade trade flows, GDELT event-risk signals, and World Bank commodity prices.

**Model-derived outputs:** fragility scores, shock propagation, impact deltas, and graph centrality.

---

## Architecture

```
React / TypeScript GFIP Dashboard
        ↓
Go HTTP API Server
        ↓
AtlasGraph Engine
        ↓
Strategic Global Dataset + Processed Data
```

| Layer | Stack |
|-------|-------|
| Frontend | React, TypeScript, Vite, Tailwind, Recharts |
| Backend | Go 1.21+ |
| API | `net/http` JSON server (`atlas serve`) |
| Data | JSON strategic graph; processed trade/macro/event/commodity panels |
| Testing | `go test ./...`; `npm run build` for the frontend |

**Repository layout (high level)**

```
AtlasGraph/
├── cmd/atlas/              # CLI entry point
├── frontend/               # GFIP dashboard (React + Vite)
├── data/
│   ├── strategic_global/   # Baseline dependency graph dataset
│   ├── sample/             # Embedded small sample graph
│   └── examples/           # Sample CSV/JSON fixtures
├── internal/
│   ├── graph/              # In-memory directed graph
│   ├── simulation/         # Shock profiles and propagation
│   ├── scoring/            # Fragility, macro, events, commodities
│   ├── shockguide/         # Graph-valid shock option validation
│   └── cli/                # CLI + HTTP API handlers
└── docs/
    ├── TECHNICAL_REFERENCE.md
    └── screenshots/        # UI captures (placeholders)
```

---

## Data Note

**Be explicit about observed data vs model-derived outputs.**

- `data/strategic_global` is the **baseline dependency graph** — a curated graph for reproducible local analysis (countries, commodities, sectors, routes, and typed dependencies). Folder name is unchanged.
- It includes **24 countries**, **20 commodities**, **20 sectors**, **8 routes**, and **193 dependencies**, plus named shock scenarios.
- **Observed panels** (when present under `data/processed/`) drive exposure scoring:
  - **UN Comtrade** — trade flows, import dependencies, supplier concentration
  - **GDELT** — event-risk signals
  - **World Bank Pink Sheet** — commodity price history / stress
- **Model-derived outputs** include fragility scores, shock propagation, impact deltas, and graph centrality. These are estimates under stated model assumptions, not raw observed facts.
- **World Bank macro ingestion** (`atlas ingest worldbank`) can populate `data/raw/worldbank` from the real API.
- Bundled fixture files under `data/sample` / examples are for local development when processed panels are absent.

See also [`data/strategic_global/README.md`](data/strategic_global/README.md) and [`data/raw/worldbank_pinksheet/README.md`](data/raw/worldbank_pinksheet/README.md).

---

## Real Commodity Price Pipeline

GFIP can ingest **World Bank Commodity Markets / Pink Sheet** monthly historical prices from a local XLSX file.

### Download and place the file

1. Download **CMO-Historical-Data-Monthly.xlsx** from the [World Bank Commodity Markets](https://www.worldbank.org/en/research/commodity-markets) page.
2. Place it at:

```
data/raw/worldbank_pinksheet/CMO-Historical-Data-Monthly.xlsx
```

The XLSX is **not committed** to this repository (large, updated monthly).

### Ingest

```bash
go run ./cmd/atlas ingest commodity-prices \
  --file data/raw/worldbank_pinksheet/CMO-Historical-Data-Monthly.xlsx \
  --out data/processed/commodity_prices \
  --source worldbank-pinksheet
```

`--source` is optional for `.xlsx` files (auto-detected). CSV ingest of `data/examples/commodity_prices_sample.csv` still works for offline demos.

### API

```bash
curl http://localhost:8080/api/commodities/history
curl "http://localhost:8080/api/commodities/history?commodity=crude%20oil"
curl http://localhost:8080/api/commodities/stress
```

`GET /api/commodities/stress` includes `data_source` and `real_price_data` so the dashboard can show **Real price data** vs **Sample price data**.

### Data note

Pink Sheet data is **real public monthly historical nominal USD prices**, not live streaming market data. Some strategic graph commodities (semiconductors, rare earths, etc.) may not exist in Pink Sheet; ingest reports them as missing rather than failing.

---

## Real Event Risk Pipeline

GFIP can ingest **GDELT-style public event/news CSV or JSON** files from local downloads and score country-level event risk.

### Raw data folder

Place downloaded GDELT event exports here:

```
data/raw/gdelt_events/
```

See [`data/raw/gdelt_events/README.md`](data/raw/gdelt_events/README.md) for expected fields.

### Ingest

```bash
go run ./cmd/atlas ingest events \
  --file data/raw/gdelt_events/events.csv \
  --out data/processed/events \
  --source gdelt
```

For offline testing without a full GDELT download:

```bash
go run ./cmd/atlas ingest events \
  --file data/examples/gdelt_events_sample.csv \
  --out data/processed/events \
  --source gdelt
```

Output: `data/processed/events/event_risk.json`

### API

```bash
curl http://localhost:8080/api/events/risk
curl "http://localhost:8080/api/events/risk?country=Ukraine"
```

Responses include `source` and `real_event_data` so the dashboard can show **Real event data** vs **Sample event data**.

When no processed file exists, the API falls back to legacy demo GDELT fixture data from `--event-data` (typically `data/raw/gdelt`).

### Serve with processed event risk

```bash
go run ./cmd/atlas serve \
  --data data/strategic_global \
  --processed-event-data data/processed/events \
  --event-data data/raw/gdelt \
  --port 8080
```

### Data note

GDELT/event data in v1 is **manually refreshed** — download new files and re-run ingest. When real files are provided, GFIP treats the signals as **real public event/news-derived data**, not live intelligence or ground truth.

The existing offline demo path still works:

```bash
go run ./cmd/atlas ingest gdelt --fixture data/examples/gdelt_events_sample.json --out data/raw/gdelt
```

---

## Real Trade Data Pipeline

GFIP can ingest **manually downloaded UN Comtrade CSV exports** and compute supplier dependency shares for strategic commodities.

### Raw data folder

Place downloaded Comtrade CSV files here:

```
data/raw/un_comtrade/
```

See [`data/raw/un_comtrade/README.md`](data/raw/un_comtrade/README.md).

### Ingest

```bash
go run ./cmd/atlas ingest trade \
  --dir data/raw/un_comtrade \
  --out data/processed/trade \
  --source un-comtrade
```

Single file:

```bash
go run ./cmd/atlas ingest trade \
  --file data/raw/un_comtrade/usa_wheat_1001_2024.csv \
  --out data/processed/trade \
  --source un-comtrade
```

For offline testing:

```bash
go run ./cmd/atlas ingest trade \
  --file data/examples/un_comtrade_sample.csv \
  --out data/processed/trade \
  --source un-comtrade
```

Output: `data/processed/trade/trade_dependencies.json`

### API

```bash
curl http://localhost:8080/api/trade/summary
curl "http://localhost:8080/api/trade/dependency?importer=United%20States&commodity=wheat"
curl "http://localhost:8080/api/trade/concentration?importer=USA&commodity=semiconductors"
```

Responses include `source` and `real_trade_data`. Importer aliases such as `USA` and `United States` are supported.

When no processed dependency file exists, endpoints fall back to legacy demo `trade_flows.json`.

### Data note

v1 uses **manually downloaded** UN Comtrade CSV exports for **USA 2024 imports** across selected HS-coded commodities. This is not a live Comtrade API integration — refresh by downloading new exports and re-running ingest.

---

## Real Data Graph Fusion

v1 **augments** the baseline dependency graph (`data/strategic_global`) with local processed real-data panels. The base graph is preserved; fusion adds edges and vulnerability metadata when processed files exist.

| Signal | Source | Fusion effect |
|--------|--------|---------------|
| Trade dependencies | `data/processed/trade/trade_dependencies.json` | `real_exports` and `real_import_dependency` edges weighted by supplier share |
| Commodity price stress | `data/processed/commodity_prices` | Commodity vulnerability multiplier during shock propagation (capped at 1.20×) |
| Event risk | `data/processed/events/event_risk.json` | Country vulnerability multiplier during shock propagation (capped at 1.25×) |

**Important:** Not all graph edges are observed trade. Baseline dependency edges remain; real UN Comtrade edges are additive and marked `real_data: true`.

When processed data is missing, behaviour matches the pre-fusion baseline dependency graph.

### Fusion CLI

```bash
go run ./cmd/atlas graph summary \
  --data data/strategic_global \
  --trade-data data/processed/trade \
  --event-data data/processed/events \
  --commodity-data data/processed/commodity_prices
```

Shows fused entity/dependency counts and data sources when real panels are present.

### Fusion API transparency

`GET /api/graph/summary`, `GET /api/fragility/summary`, and `POST /api/shock` include fields such as:

- `fusion_enabled`, `real_trade_edges_used`, `real_event_risk_used`, `real_price_stress_used`
- `data_sources`: e.g. `["Baseline dependency graph", "UN Comtrade", "World Bank Pink Sheet", "GDELT"]`

Shock results may include `data_fusion.propagation_note`, e.g. `Real-data-backed model propagation: trade + commodity prices + event risk`.

**Observed data:** UN Comtrade trade flows, GDELT event-risk signals, and World Bank commodity prices.

**Model-derived outputs:** fragility scores, shock propagation, impact deltas, and graph centrality.

---

## Quickstart

### Backend validation

```bash
go mod tidy
go vet ./...
go test ./...
```

### Run backend (strategic global dataset)

```bash
go run ./cmd/atlas serve \
  --data data/strategic_global \
  --trade-data data/processed/trade \
  --macro-data data/raw/worldbank \
  --event-data data/raw/gdelt \
  --processed-event-data data/processed/events \
  --commodity-data data/processed/commodity_prices \
  --port 8080
```

> Missing data paths disable only the matching endpoints — the server still starts.
> For a minimal demo, only `--data data/strategic_global` is required.

### Run frontend

```bash
cd frontend
npm install
npm run dev
```

Open **http://localhost:5173**

The header shows an **API Online** badge when `GET /health` succeeds. If the API is down, the UI displays the backend command to run.

---

## CLI Examples

```bash
# Graph overview
go run ./cmd/atlas graph summary --data data/strategic_global

# List scenario presets
go run ./cmd/atlas scenario list --data data/strategic_global

# Run a preset with propagation explanation
go run ./cmd/atlas scenario run taiwan_semiconductor_shock --data data/strategic_global --explain

# Custom route disruption shock
go run ./cmd/atlas shock \
  --source "Strait of Hormuz" \
  --commodity "crude oil" \
  --drop 35 \
  --depth 3 \
  --type route_disruption \
  --data data/strategic_global \
  --explain

# Unified fragility scores (uses all available signal panels)
go run ./cmd/atlas score fragility \
  --graph-data data/strategic_global \
  --trade-data data/processed/trade \
  --macro-data data/raw/worldbank \
  --event-data data/raw/gdelt \
  --commodity-data data/processed/commodity_prices

# Compare recommended scenarios side-by-side
go run ./cmd/atlas scenario compare --data data/strategic_global
```

---

## API Examples

Major endpoints (base URL `http://localhost:8080`):

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness probe |
| `GET` | `/api/graph/summary` | Entity counts and high-degree nodes |
| `GET` | `/api/graph/entities` | Entities grouped by type |
| `GET` | `/api/scenarios` | Saved scenario presets |
| `GET` | `/api/shock/options` | Shock types, guidance, recommended scenarios |
| `GET` | `/api/shock/valid-options` | Graph-valid source → commodity → shock_type combos |
| `POST` | `/api/shock` | Run a shock simulation |
| `POST` | `/api/scenarios/compare` | Compare multiple scenarios |
| `GET` | `/api/fragility/summary` | Top countries and commodities by unified score |
| `GET` | `/api/commodities/stress` | Commodity price-stress scores |
| `GET` | `/api/commodities/history` | Available commodities with price history |
| `GET` | `/api/commodities/history?commodity=crude%20oil` | Monthly price history for one commodity |
| `GET` | `/api/events/risk` | Event-risk scores |
| `GET` | `/api/macro/scores` | Macro exposure scores |

**Sample `POST /api/shock` body:**

```json
{
  "source": "Taiwan",
  "commodity": "semiconductors",
  "drop": 30,
  "depth": 3,
  "shock_type": "export_collapse",
  "explain": true
}
```

```bash
curl http://localhost:8080/health
curl http://localhost:8080/api/graph/summary
curl -X POST http://localhost:8080/api/shock \
  -H "Content-Type: application/json" \
  -d '{"source":"Taiwan","commodity":"semiconductors","drop":30,"depth":3,"shock_type":"export_collapse"}'
```

Full API documentation, error shapes, and additional trade endpoints:
[`docs/TECHNICAL_REFERENCE.md`](docs/TECHNICAL_REFERENCE.md#http-api-server).

---

## Suggested Walkthrough

A ~5-minute walkthrough:

1. **Open the dashboard** — point out the baseline dependency graph size (24 countries, 193 dependencies) in the overview cards.
2. **Data Fusion Status** — show observed sources (UN Comtrade, GDELT, World Bank Pink Sheet) vs model-derived outputs.
3. **Unified Fragility** — show top fragile countries and commodities charts; expand score breakdown if asked.
4. **Run Taiwan Semiconductor Export Collapse** — preset shock; highlight estimated impact on countries/sectors and top dependency paths.
5. **Run Strait of Hormuz Crude Route Disruption** — contrast a route/chokepoint shock with a production shock.
6. **Scenario Comparison Mode** — select scenarios and compare average/max fragility deltas.
7. **Graph-guided custom shock** — switch to Custom Shock; show source → commodity → shock type cascade filtered by the graph.

---

## Engineering Highlights

- **Go backend engine** with shared logic across CLI and HTTP API
- **Graph traversal and shock propagation profiles** — `export_collapse`, `supply_cut`, `price_spike`, `route_disruption`
- **Relationship filtering by shock type** — shocks only travel along permitted edge types; blocked branches are explainable
- **Explainable scoring** — component breakdowns for macro, event, trade, commodity, and unified fragility
- **Graph-guided validation** — `valid-options` prevents invalid source/commodity/shock combinations in the UI
- **Reusable frontend components** — adaptive ranking lists vs bar charts, collapsible path/detail sections
- **Reproducible test-driven workflow** — unit tests across graph, simulation, scoring, ingest, and API handlers

---

## Screenshots

Screenshots should be added after final UI capture. Placeholders:

| Screenshot | Path |
|------------|------|
| Dashboard overview | `docs/screenshots/dashboard-overview.png` |
| Shock simulator | `docs/screenshots/shock-simulator.png` |
| Scenario comparison | `docs/screenshots/scenario-comparison.png` |

---

## Future Work

- Broader **UN Comtrade** coverage and refresh automation
- Production **GDELT / news event** refresh pipeline
- Automated **World Bank Pink Sheet** refresh
- **Geospatial map view** of routes and chokepoints
- **Scenario report export** (PDF/JSON briefings)
- **Analyst briefing generation** from shock results
- **Cloud deployment** (Docker / AWS)
- **User-selectable datasets** in the dashboard

---

## Testing

```bash
go test ./...

cd frontend
npm run build
```

Core packages under test: `internal/graph`, `internal/simulation`, `internal/scoring`, `internal/data`, `internal/shockguide`, `internal/ingest/*`, `internal/cli` (including API handler tests).

---

## Further Reading

Detailed technical documentation is preserved in [`docs/TECHNICAL_REFERENCE.md`](docs/TECHNICAL_REFERENCE.md), including:

- World Bank macro ingestion and macro exposure scoring
- Trade dependency ingestion and UN Comtrade-style CSV import
- GDELT event-risk ingestion (live + offline fixture mode)
- Generated trade graphs (`graph build-trade`)
- Commodity price stress scoring
- Unified fragility formula and component weights
- Full HTTP API reference with curl examples
- Extended testing notes and roadmap

---

## License

See repository license file if present. Baseline graph and fixture datasets are for evaluation and local analysis; model outputs are estimates under stated assumptions.
