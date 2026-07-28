# GFIP

**Global Fragility Intelligence Platform**

---

## Overview

Global supply chains depend on concentrated trade relationships, geographically distributed suppliers, and infrastructure corridors that are exposed to geopolitical, climatic, and economic disruption. A single export restriction, port closure, or supplier failure can propagate through downstream industries long before impacts appear in headline economic indicators. Organizations need tools that model these dependencies explicitly rather than relying on static risk spreadsheets.

GFIP (Global Fragility Intelligence Platform) addresses this by combining a curated supply-chain dependency graph with observed public datasets—trade flows, event-risk signals, macro indicators, and commodity prices—to estimate exposure and simulate how disruptions propagate through connected nodes. The platform produces model-derived fragility scores, shock propagation results, client-specific exposure overlays, and structured executive reports. Outputs are deterministic and explainable; they are estimates under stated assumptions, not forecasts of future events.

Geopolitical disruption modeling matters because supplier concentration, transit chokepoints, and cross-border manufacturing dependencies create non-linear risk. GFIP encodes these relationships as a graph and applies relationship-aware shock profiles so that, for example, a semiconductor export collapse propagates along permitted edge types without spilling into unrelated commodities.

The system is intended for supply chain analysts, risk and resilience teams, operations planners, and technical decision makers who need a reproducible analytical workflow—from raw public data through simulation to decision-support artifacts. GFIP ships as a Go analytics engine (AtlasGraph) and a React analyst workspace that share a JSON HTTP API.

---

## Features

### Risk Modeling

GFIP simulates four shock types—export collapse, supply cut, route disruption, and price spike—over a fused dependency graph. Each shock profile defines allowed relationship types, attenuation per hop, and recommended propagation depth. Operational assumptions (duration, recovery speed, substitute availability, inventory buffer) adjust impact multipliers. The engine returns direct and second-order exposure, affected paths, blocked edges, and fragility deltas for countries, commodities, and sectors.

Unified fragility scoring composites trade concentration, event risk, commodity price stress, macro exposure, and graph centrality into country- and commodity-level scores with component-level provenance.

### Supply Chain Analytics

Trade dependency and concentration APIs expose importer–commodity supplier rankings, Herfindahl-Hirschman Index (HHI) values, and concentration risk bands from UN Comtrade-derived panels. Client Analytics accepts uploaded supplier-dependency CSV files, validates rows, computes per-importer concentration metrics, and overlays matched exposure onto shock scenarios—including estimated exposed trade under a modeled drop percentage.

The Shock Simulation workspace supports saving scenarios to browser storage and comparing two saved runs side by side (countries, commodities, HHI, and impact metrics).

### Data Engineering

CLI ingest commands load UN Comtrade trade flows, GDELT event-risk data, World Bank macro indicators, and World Bank Pink Sheet commodity prices into normalized JSON panels under `data/processed/`. An optional PostgreSQL layer stores trade flows, event signals, macro scores, commodity prices, dependency edges, scenario runs, and data-quality checks. The Data Operations Monitor surfaces ETL validation results, pipeline health, and load summaries via `GET /api/pipeline/summary`.

### Executive Reporting

Scenario intelligence reports combine simulation output with trade evidence, GDELT event-risk context, World Bank macro context, and Pink Sheet commodity stress. Reports include executive summaries, key findings, exposure tables, model assumptions, and limitations. When client data is supplied, reports add a client exposure assessment and feed concentration metrics into the mitigation engine.

The mitigation recommendation engine applies deterministic rules (supplier share, HHI, event risk, macro exposure, shock type, drop severity) to produce ranked actions with priority, confidence, difficulty, and supporting metrics. An executive action plan aggregates recommendations with a priority distribution summary.

### Platform Features

The React frontend provides a multi-tab workspace: Dashboard, Shock Simulation (setup / results / comparison), Client Analytics, Data Operations Monitor, Analytics Explorer (event risk, trade signals, commodity stress, price history), and History (placeholder for persisted scenario review). The UI uses a dark theme with shared cards, ranking charts, and collapsible evidence panels. Frontend and backend test suites cover core rules, overlay logic, and report generation.

---

## System Architecture

GFIP follows a layered pipeline from public data ingestion through graph-based simulation to analyst-facing reports.

```mermaid
flowchart TD
    subgraph sources [Public Data Sources]
        A1[UN Comtrade]
        A2[GDELT]
        A3[World Bank Macro]
        A4[World Bank Pink Sheet]
        A5[Baseline Dependency Graph]
    end

    subgraph etl [ETL and Validation]
        B1[Ingest CLI]
        B2[Normalize and Validate]
        B3[Processed JSON Panels]
    end

    subgraph storage [Storage]
        C1[(PostgreSQL)]
        C2[File-backed Panels]
    end

    subgraph engine [Analytics Engine]
        D1[Graph Fusion]
        D2[Fragility Scoring]
        D3[Shock Simulation]
        D4[Client Overlay]
        D5[Recommendations]
    end

    subgraph output [Outputs]
        E1[REST API]
        E2[Scenario Reports]
        E3[GFIP Frontend]
    end

    sources --> B1
    B1 --> B2 --> B3
    B3 --> C1
    B3 --> C2
    C1 --> D1
    C2 --> D1
    A5 --> D1
    D1 --> D2 --> D3
    D3 --> D4
    D3 --> D5
    D3 --> E2
    D4 --> E2
    D5 --> E2
    D2 --> E1
    D3 --> E1
    E1 --> E3
    E2 --> E3
```

| Stage | Description |
|-------|-------------|
| **Public data sources** | Raw trade, event, macro, and price datasets plus a curated strategic dependency graph. |
| **ETL pipeline** | CLI ingest commands parse, validate, and write normalized records to `data/processed/`. |
| **Validation** | Pipeline checks verify file presence, row counts, schema conformance, and fusion readiness. |
| **PostgreSQL** | Optional persistence for analytics tables, scenario run storage, and client upload records. |
| **Dependency graph** | Baseline entities and edges fused with real trade dependencies when panels are available. |
| **Shock simulation** | Relationship-constrained propagation with operational impact adjustments. |
| **Business impact analysis** | Fragility deltas, exposure rankings, client overlay, and path-level impact. |
| **Executive intelligence** | Structured reports, mitigation plans, and API responses consumed by the frontend. |

---

## Project Architecture

```
AtlasGraph/
├── cmd/atlas/              Application entrypoint (CLI + HTTP server)
├── frontend/               React/TypeScript analyst workspace
├── internal/               Go packages (engine, ingest, API)
├── migrations/             PostgreSQL schema migrations
├── data/                   Baseline graph, examples, processed panels
├── docs/                   Technical reference and screenshot placeholders
├── tools/                  Dataset generation utilities
├── Makefile                Build, test, and developer shortcuts
├── go.mod
└── LICENSE
```

### `cmd/atlas/`

Thin `main` package that delegates to `internal/cli`. Subcommands include `serve` (HTTP API), `shock`, `scenario`, `graph`, `risk`, `ingest`, `db`, and `score`. The server and CLI share the same simulation and scoring engine.

### `frontend/`

Vite-powered React 18 application. Key areas:

| Path | Purpose |
|------|---------|
| `src/pages/` | Tab views: Dashboard, Shock Simulation, Client Analytics, Data Operations, Analytics Explorer, History |
| `src/components/` | Feature panels (shock results, reports, pipeline monitor, client overlay, recommendations) |
| `src/lib/` | API client, exposure overlay, mitigation rules, formatting utilities |
| `src/types/` | TypeScript interfaces mirroring API contracts |

### `internal/`

| Package | Purpose |
|---------|---------|
| `cli/` | HTTP handlers, scenario report builder, API request/response mapping |
| `simulation/` | Shock propagation engine, profiles, rules, scenario comparison |
| `scoring/` | Fragility, event, macro, and commodity scoring |
| `graph/` / `graphfusion/` | Graph loading and fusion of baseline + observed edges |
| `ingest/` | ETL loaders for trade, GDELT, macro, commodity prices |
| `pipeline/` | ETL validation and pipeline summary |
| `customdata/` | Client CSV parsing and concentration analysis |
| `clientoverlay/` | Client exposure overlay against shock parameters |
| `recommendations/` | Deterministic mitigation recommendation engine |
| `db/` | PostgreSQL repository and migration helpers |
| `operationalimpact/` | Duration, recovery, substitute, and inventory multipliers |
| `config/` | Defaults and build metadata |
| `data/` | Strategic graph loader |
| `models/` | Shared entity and relationship types |
| `shockguide/` | Graph-valid shock option discovery |

### `migrations/`

SQL migrations applied via `atlas db migrate`. Defines analytics and client-data tables (see Database Design).

### `data/`

| Path | Contents |
|------|----------|
| `strategic_global/` | Baseline entities, dependencies, and bundled scenario presets |
| `processed/` | Normalized trade, event, macro, and commodity panels (created by ingest) |
| `raw/` | Downloaded source files (typically git-ignored) |
| `examples/` | Sample CSV/JSON files for testing ingest and client upload |

### `docs/`

`TECHNICAL_REFERENCE.md` — extended API, scoring, and ingestion documentation. `docs/screenshots/` holds placeholder assets for README screenshots.

### `tools/`

`generate_strategic_global` — utility for regenerating baseline graph artifacts.

---

## Technology Stack

### Backend

| Technology | Role |
|------------|------|
| **Go 1.21** | Single binary for CLI and API; strong concurrency and straightforward deployment |
| **`net/http`** | Lightweight JSON API without framework overhead; handlers colocated with CLI |
| **Standard library + targeted deps** | `pgx` for PostgreSQL, `excelize` for Pink Sheet XLSX parsing |

Go was chosen for performance on graph traversal, static typing across a large domain model, and a single deployable artifact that serves both batch ingest and the live API.

### Frontend

| Technology | Role |
|------------|------|
| **React 18 + TypeScript** | Component model with type-safe API contracts |
| **Vite** | Fast dev server and production bundling |
| **Tailwind CSS** | Consistent dark-theme utility styling |
| **Recharts** | Ranking bar charts and commodity price history |
| **Vitest + Testing Library** | Unit and component tests for rules and panels |

### Database

| Technology | Role |
|------------|------|
| **PostgreSQL 14+** | Durable analytics store, scenario persistence, client upload storage |
| **`pgx/v5`** | Native Go driver with connection pooling |

PostgreSQL is optional at runtime—the engine operates file-backed when `DATABASE_URL` is unset—but enables SQL-backed rankings, scenario history, and reproducible load-quality auditing.

### Analytics

| Component | Role |
|-----------|------|
| HHI and supplier share | Import concentration from trade and client data |
| Fragility composites | Weighted multi-signal country/commodity scores |
| Graph propagation | Hop-limited impact with shock-type edge filtering |
| Operational multipliers | Scenario-adjustable resilience parameters |
| Rule-based recommendations | Deterministic mitigation ranking (no LLM) |

### Data Sources

See [Data Sources](#data-sources) below.

### Infrastructure

| Component | Role |
|-----------|------|
| **Docker** | Local PostgreSQL container for development |
| **File-backed panels** | Default mode without database configuration |
| **CORS-enabled dev server** | Frontend on `:5173`, API on `:8080` |

---

## Core Components

### Shock Simulation Engine

**Inputs:** source entity, commodity, shock type, drop percentage, propagation depth, and optional operational parameters (duration, recovery speed, substitute availability, inventory buffer days).

**Propagation:** The engine locates the source and commodity nodes, injects an initial disruption, and walks the dependency graph to a configured depth. Each shock profile restricts which relationship types propagate (e.g., exports, used_by). Cross-commodity branches can be blocked per profile rules. Attenuation reduces impact per hop. Blocked edges are recorded with reasons.

**Risk calculation:** Affected nodes receive updated fragility scores. Direct exposure (distance 2) and second-order exposure (distance 3) are classified. Operational impact multipliers adjust deltas based on scenario assumptions. Data fusion may apply event-risk and price-stress weighting when panels are loaded.

**Outputs:** JSON response with scenario metadata, exposure lists, affected paths, graph impact summary, highest-risk entity rankings, warnings, data-fusion provenance, and operational assumption details.

### Client Analytics

**Supplier concentration:** Uploaded CSV rows (importer, commodity, supplier, value_usd) are validated and aggregated. Per importer–commodity group, the engine computes total import value, supplier count, top supplier share, HHI, and a concentration risk band (Low / Medium / High).

**HHI:** Herfindahl-Hirschman Index on supplier value shares; thresholds align with `internal/customdata` risk bands (≥ 0.25 High, ≥ 0.15 Medium).

**Trade dependency:** Client overlay matches shock source to supplier and shock commodity to client commodity (with alias normalization, e.g., LNG ↔ natural gas). Estimated exposed trade = supplier value × (drop / 100).

**Portfolio analysis:** Matched importers are ranked by estimated exposed trade. Summary metrics include highest supplier share, highest HHI, average concentration risk, and a narrative client exposure assessment.

### Data Pipeline

**ETL:** `atlas ingest` subcommands load Comtrade CSVs, GDELT event files, World Bank macro (API or CSV), and Pink Sheet XLSX into `data/processed/{trade,events,macro,commodity_prices}`.

**Validation:** `internal/pipeline` checks processed file presence, record counts, schema fields, and graph-fusion readiness. Results surface in the Data Operations Monitor.

**Loading:** `atlas db load` replaces PostgreSQL analytics tables from the current processed snapshot and writes `data_quality_checks` rows.

**PostgreSQL:** When enabled, trade and signal data are queryable via `/api/db/*` endpoints; scenario reports persist to `scenario_runs` on successful `POST /api/reports/scenario`.

### Business Impact Assessment

**Fragility scoring:** Country and commodity scores blend available signals (trade concentration, GDELT event risk, Pink Sheet stress, macro indicators, graph degree) with documented weights. Missing panels degrade gracefully with provenance flags.

**Country and sector ranking:** Shock results rank entities by fragility delta and estimated impact. Charts normalize commodity aliases and filter to strategic commodities.

**Risk aggregation:** Executive impact briefs synthesize top downstream entities, fusion drivers, operational notes, and client overlay sentences. Risk driver panels collapse evidence sources, operational assumptions, client exposure, and propagation notes.

### Executive Reporting

**Summary generation:** `buildScenarioReport` assembles title, executive summary, and key findings from simulation output plus observed context panels.

**Key findings:** Capped list covering top exposure, trade concentration, event/macro context, client overlay (when provided), operational adjustments, and data provenance.

**Decision support:** Reports include exposure tables, trade evidence, context items, model assumptions, limitations, an executive action plan (mitigation recommendations), and optional client exposure assessment. Reports can be copied as Markdown from the frontend.

---

## Data Sources

| Source | Purpose | Examples |
|--------|---------|----------|
| **UN Comtrade** | Bilateral trade flows, import dependencies, supplier rankings, HHI | Semiconductor imports by importer; exporter concentration |
| **World Bank Macro** | Structural country indicators for macro exposure scoring | GDP, trade openness, manufacturing share, inflation |
| **World Bank Pink Sheet** | Commodity price history and stress signals | Crude oil, natural gas/LNG, metals, agricultural commodities |
| **GDELT** | Country-level event-risk frequency and severity | Geopolitical event counts mapped to country risk scores |
| **Baseline dependency graph** | Curated entity relationships when observed trade is absent | `data/strategic_global` entities and dependencies |

Observed panels fuse into the baseline graph as additive real-trade edges. The baseline graph remains authoritative for entities and relationships not covered by ingested trade data.

---

## Database Design

PostgreSQL schema is defined in `migrations/`. The engine runs without a database when `DATABASE_URL` is unset.

| Table | Purpose |
|-------|---------|
| `trade_flows` | Normalized bilateral trade records (importer, exporter, commodity, HS code, value, year, source) |
| `commodity_prices` | Monthly commodity price observations from Pink Sheet |
| `event_risk_signals` | Per-country GDELT-derived risk scores and levels |
| `macro_scores` | Per-country World Bank macro composite scores |
| `dependency_edges` | Fused graph edges with relationship type, weight, and provenance |
| `scenario_runs` | Persisted scenario reports (parameters, top affected JSON, full `report_json`) |
| `data_quality_checks` | ETL load metrics and validation outcomes |
| `custom_trade_flows` | Client-uploaded supplier dependency rows |
| `custom_concentration_results` | Client-uploaded concentration analytics per importer–commodity |

Indexes support lookups by importer/commodity, country code, dataset name, and scenario recency.

---

## Running the Project

### Prerequisites

- Go 1.21+
- Node.js 18+
- Docker (for local PostgreSQL)
- Processed data panels under `data/processed/` (or run ingest first; see `docs/TECHNICAL_REFERENCE.md`)

### 1. Start PostgreSQL (Docker)

```bash
docker run -d --name atlasgraph-postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=atlasgraph \
  -p 5432:5432 \
  postgres:14
```

### 2. Configure environment

```bash
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/atlasgraph?sslmode=disable
```

On Windows (Command Prompt):

```bat
set DATABASE_URL=postgres://postgres:postgres@localhost:5432/atlasgraph?sslmode=disable
```

### 3. Run migrations

```bash
go run ./cmd/atlas db migrate
```

### 4. Load analytics tables (optional)

```bash
go run ./cmd/atlas db load \
  --trade-data data/processed/trade \
  --macro-data data/processed/macro \
  --event-data data/processed/events \
  --commodity-data data/processed/commodity_prices \
  --graph-data data/strategic_global
```

### 5. Start backend

```bash
go run ./cmd/atlas serve \
  --data data/strategic_global \
  --trade-data data/processed/trade \
  --processed-macro-data data/processed/macro \
  --processed-event-data data/processed/events \
  --commodity-data data/processed/commodity_prices \
  --port 8080
```

API: `http://localhost:8080` — Health check at `/health`.

Without `DATABASE_URL`, the server logs that Postgres is disabled and continues with file-backed analytics.

### 6. Start frontend

```bash
cd frontend
npm install
npm run dev
```

Dashboard: `http://localhost:5173`

### 7. Verify

```bash
go test ./...
cd frontend && npm run test && npm run build
```

---

## Example Workflow

1. **Ingest datasets** — Run `atlas ingest` for trade, events, macro, and commodity prices (if panels are not already present).
2. **Validate pipeline** — Open the Data Operations Monitor tab or call `GET /api/pipeline/summary` to confirm validation status.
3. **Upload client data (optional)** — On Client Analytics, upload a supplier-dependency CSV and review concentration results.
4. **Configure shock** — On Shock Simulation → Scenario Setup, select source, commodity, shock type, drop %, depth, and operational assumptions.
5. **Run simulation** — Execute the shock and review propagation results, risk cards, country/commodity rankings, and client exposure overlay.
6. **Analyze impacts** — Inspect affected paths, blocked edges, executive impact brief, and recommended mitigation actions.
7. **Generate executive report** — Produce a scenario intelligence report (client data is included when uploaded). Review findings, evidence panels, and executive action plan.
8. **Compare scenarios (optional)** — Save scenarios locally and use the Comparison tab for side-by-side metrics.

---

## Screenshots

| Dashboard | Shock Simulation |
|:---:|:---:|
| ![Dashboard](docs/screenshots/dashboard.png) | ![Shock Simulation](docs/screenshots/shock-simulation.png) |

| Client Analytics | Pipeline Monitor |
|:---:|:---:|
| ![Client Analytics](docs/screenshots/client-analytics.png) | ![Pipeline Monitor](docs/screenshots/pipeline-monitor.png) |

> Screenshot files are placeholders. Add captures to `docs/screenshots/` to populate the images above.

---

## Design Decisions

**Why Go** — Graph propagation and ingest pipelines are CPU-bound and benefit from compiled performance. A single language covers CLI tooling, the HTTP API, and shared domain types without a separate batch runtime.

**Why PostgreSQL** — Optional but valuable for durable analytics, SQL-backed supplier rankings, scenario run history, and auditable load-quality records. File-backed mode preserves functionality when no database is configured.

**Why graph propagation** — Supply chain risk is inherently relational. Encoding dependencies as typed edges with shock-specific propagation rules produces explainable paths and blocked-branch diagnostics that scalar models cannot provide.

**Why deterministic recommendations** — Mitigation actions are generated from explicit thresholds on concentration, event risk, macro exposure, and shock parameters. This keeps outputs reproducible, testable, and suitable for analyst review without non-deterministic AI components.

**Why enterprise workspace UI** — Risk analysis spans multiple concerns (data health, simulation, client overlay, reporting). A tabbed workspace with consistent cards and provenance badges reduces context switching and mirrors how analyst teams partition responsibilities.

---

## Future Work

| Area | Description |
|------|-------------|
| **Interactive dependency graph** | Pan/zoom graph explorer with path highlighting beyond the current propagation path list |
| **Persistent scenario history** | Server-backed History tab replacing the current placeholder; extend beyond browser-local saved scenarios |
| **SQL analytics explorer** | Ad hoc query workspace over PostgreSQL trade and concentration tables |
| **PDF report export** | Downloadable executive intelligence and action-plan documents |
| **Expanded Analytics Explorer** | Full commodity, country, and supplier explorer views beyond the current live panels |

---

## License

MIT License — see [LICENSE](LICENSE).

Public data sources remain subject to their respective terms. Model outputs are estimates under stated assumptions and are not predictions of future events.

---

## Further Reading

- [docs/TECHNICAL_REFERENCE.md](docs/TECHNICAL_REFERENCE.md) — API endpoints, scoring formulas, ingest commands
- [data/strategic_global/README.md](data/strategic_global/README.md) — Baseline graph dataset documentation
