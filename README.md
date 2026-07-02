# GFIP — Global Fragility Intelligence Platform

**Powered by AtlasGraph**

GFIP is a strategic supply-chain and infrastructure risk intelligence platform
that models how shocks to countries, commodities, and trade chokepoints propagate
through global dependency networks.

- **GFIP** — the user-facing intelligence platform (React dashboard for analyst-style exploration).
- **AtlasGraph** — the Go backend engine powering graph modeling, shock simulation, scoring, and API serving.

---

## What it does

In plain terms, GFIP:

1. **Builds a strategic dependency graph** — countries, commodities, sectors, and maritime routes linked by typed trade and industry relationships.
2. **Simulates shocks** — export collapses, supply cuts, route disruptions, and price spikes propagate along relationship-aware paths.
3. **Ranks impact** — affected countries, commodities, sectors, and dependency paths with fragility deltas.
4. **Compares crisis scenarios** — run multiple shocks side-by-side and rank systemic impact.
5. **Computes unified fragility scores** — explainable composite scores blending macro, trade, event, commodity, and graph signals.
6. **Provides a dashboard** — control-room UI for exploring graph size, fragility, shocks, paths, and scenario comparison.

---

## Key Features

- **Strategic global demo dataset** — curated synthetic graph for reproducible local demos (`data/strategic_global`)
- **Graph-based shock propagation engine** — typed shock profiles with relationship filtering and attenuation
- **Graph-guided custom shock controls** — only valid source → commodity → shock-type combinations are offered
- **Unified Fragility Score** — explainable country and commodity composite scores
- **Scenario Comparison Mode** — compare multiple shocks and rank by average/max fragility delta
- **Dependency path explanations** — readable paths with relationship labels, impact, and weight
- **Frontend analytics charts** — adaptive bar charts and compact ranking lists
- **REST API endpoints** — pure `net/http` JSON API with CORS for the dashboard
- **CLI support** — same engine via `atlas` commands
- **Tests and reproducible local workflow** — Go unit tests across core packages; frontend build validation

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
│   ├── strategic_global/   # Curated synthetic demo dataset
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

**Be explicit about what this demo uses.**

- `data/strategic_global` is a **synthetic but realistic strategic demo dataset** — designed for reproducible local demos and interviews, not live production intelligence.
- It includes **24 countries**, **20 commodities**, **20 sectors**, **8 routes**, and **193 dependencies**, plus 10 named shock scenarios.
- **World Bank macro ingestion** exists separately (`atlas ingest worldbank`) and can populate `data/raw/worldbank` from the real API.
- Bundled **trade**, **GDELT event**, and **commodity price** demo files are synthetic or fixture-based — they should **not** be presented as live global intelligence.
- **Future work** includes real UN Comtrade flows, a GDELT/news pipeline, World Bank Pink Sheet commodity prices, and geospatial datasets.

See also [`data/strategic_global/README.md`](data/strategic_global/README.md).

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

## Suggested Demo Flow

A ~5-minute interview walkthrough:

1. **Open the dashboard** — point out the strategic global graph size (24 countries, 193 dependencies) in the overview cards.
2. **Unified Fragility** — show top fragile countries and commodities charts; expand score breakdown if asked.
3. **Run Taiwan Semiconductor Export Collapse** — preset shock; highlight affected countries/sectors and top dependency paths (4 shown by default, expand for full list).
4. **Run Strait of Hormuz Crude Route Disruption** — contrast a route/chokepoint shock with a production shock.
5. **Scenario Comparison Mode** — select scenarios and compare average/max fragility deltas.
6. **Graph-guided custom shock** — switch to Custom Shock; show source → commodity → shock type cascade filtered by the graph.
7. **Technical depth** — mention Go backend, REST API, CLI parity, graph-guided validation, and `go test ./...` coverage.

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

- Real **UN Comtrade** ingestion and trade-graph generation
- Production **GDELT / news event** pipeline
- Real **World Bank Pink Sheet** commodity prices
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

See repository license file if present. Demo data is synthetic and for evaluation purposes only.
