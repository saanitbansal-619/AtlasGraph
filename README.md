# GFIP
Global Fragility Intelligence Platform

[![CI](https://github.com/saanitbansal-619/AtlasGraph/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/saanitbansal-619/AtlasGraph/actions/workflows/ci.yml)

GFIP models supply-chain fragility and disruption propagation across a fused dependency graph built from public trade, event, macro, and commodity datasets. It addresses concentrated supplier risk and cross-border exposure by simulating shocks (export collapse, supply cut, route disruption, price spike) and quantifying downstream impact. The stack is a Go analytics engine (AtlasGraph) with a PostgreSQL-backed ETL layer and a React/TypeScript analyst workspace. Users ingest datasets, run shock scenarios, overlay client supplier portfolios, generate executive intelligence reports, and receive deterministic mitigation recommendations.

---

## Features

### Analytics

- Graph-based supply chain shock simulation (4 shock types, hop-limited propagation)
- Relationship-constrained dependency propagation engine
- Unified fragility scoring (trade, events, macro, commodity, graph centrality)
- Business impact assessment with operational multipliers
- Scenario intelligence reports and executive action plans
- Deterministic mitigation recommendation engine (10 rule categories, priority ranking)

### Data Engineering

- PostgreSQL analytics database (optional; file-backed fallback)
- CLI ETL pipeline (`atlas ingest` for trade, GDELT, macro, commodity prices)
- Data quality validation and pipeline health monitoring
- Real-world dataset ingestion (UN Comtrade, World Bank, GDELT, Pink Sheet)
- Graph fusion of baseline dependencies with observed trade edges

### Client Analytics

- Supplier concentration analysis from uploaded CSV
- HHI calculation and concentration risk bands
- Client exposure overlay on shock scenarios
- Estimated exposed trade under modeled drop percentage
- Portfolio risk assessment and importer ranking

### Platform

- Guided analyst workflow (client portfolio → shock → dollars at risk → actions)
- Enterprise React dashboard (multi-tab analyst workspace)
- Saved scenario comparison (browser localStorage)
- Shock workspace navigation (setup / results / comparison)
- Data Operations pipeline monitor
- Reproducible case study: [Taiwan semiconductors](docs/CASE_STUDY_TAIWAN_SEMICONDUCTORS.md)
- Analytics Explorer (event risk, trade signals, commodity stress)

---

## Architecture

```mermaid
flowchart TD
    A[Data Sources] --> B[ETL]
    B --> C[PostgreSQL / File Panels]
    C --> D[Graph Fusion]
    D --> E[Shock Engine]
    E --> F[Business Impact]
    F --> G[Executive Reporting]
    G --> H[React Dashboard]
```

---

## Tech Stack

| Category | Technologies |
|----------|--------------|
| Backend | Go 1.21, `net/http`, `pgx/v5`, `excelize` |
| Frontend | React 18, TypeScript, Vite, Tailwind CSS, Recharts, Vitest |
| Database | PostgreSQL 14+ |
| Data Sources | UN Comtrade, GDELT, World Bank Macro, World Bank Pink Sheet |
| Visualization | Recharts, custom ranking charts, Recharts commodity history |

---

## Project Structure

```
AtlasGraph/
├── cmd/atlas/              CLI entrypoint and HTTP server
├── frontend/               React analyst workspace
│   └── src/
│       ├── pages/          Tab views (Dashboard, Shock, Client Analytics, etc.)
│       ├── components/     Feature panels, charts, layout
│       ├── lib/            API client, overlay logic, mitigation rules
│       └── types/          TypeScript API contracts
├── internal/
│   ├── cli/                HTTP handlers and scenario report builder
│   ├── simulation/         Shock propagation engine
│   ├── scoring/            Fragility, event, macro, commodity scoring
│   ├── ingest/             ETL loaders (trade, GDELT, macro, prices)
│   ├── graph/              Graph loading and traversal
│   ├── graphfusion/        Baseline + observed edge fusion
│   ├── pipeline/           ETL validation and summary
│   ├── customdata/         Client CSV parsing and concentration
│   ├── clientoverlay/      Client exposure overlay service
│   ├── recommendations/    Deterministic mitigation engine
│   ├── db/                 PostgreSQL repository
│   └── operationalimpact/  Scenario resilience multipliers
├── migrations/             PostgreSQL schema (001 init, 002 client data)
├── data/
│   ├── strategic_global/   Baseline entities, dependencies, scenarios
│   ├── processed/          Normalized panels (created by ingest)
│   ├── examples/           Sample CSV/JSON for testing
│   └── raw/                Downloaded source files
├── docs/                   Technical reference and screenshot assets
├── tools/                  Dataset generation utilities
├── go.mod
├── Makefile
└── LICENSE
```

| Folder | Description |
|--------|-------------|
| `cmd/atlas/` | Thin `main` delegating to `internal/cli` subcommands (`serve`, `shock`, `scenario`, `ingest`, `db`, `score`). |
| `frontend/` | Vite React app with tabbed workspace, shock simulation UI, and API integration. |
| `internal/` | Go domain packages: simulation, scoring, ingest, graph fusion, recommendations, and HTTP API. |
| `migrations/` | SQL migrations applied via `atlas db migrate`. |
| `data/` | Baseline dependency graph, processed analytics panels, and sample files. |
| `docs/` | Extended technical reference (`TECHNICAL_REFERENCE.md`) and screenshot placeholders. |
| `tools/` | Utilities for regenerating baseline graph artifacts and expanded GDELT fixtures. |

---

## Core Components

| Component | Purpose |
|-----------|---------|
| Shock Simulation Engine | Propagates typed disruptions through the dependency graph with attenuation, edge filtering, and operational multipliers. |
| Client Analytics | Parses supplier CSV uploads, computes HHI and concentration bands, and ranks portfolio exposure. |
| Client Exposure Overlay | Matches client suppliers and commodities to shock parameters; estimates exposed trade value. |
| Business Impact Engine | Aggregates fragility deltas, exposure rankings, and risk drivers into executive impact briefs. |
| Data Pipeline | Ingests, validates, and loads trade, event, macro, and commodity panels into PostgreSQL or file storage. |
| Recommendation Engine | Applies deterministic rules to shock and concentration inputs; outputs prioritized mitigation actions. |
| Executive Reporting | Assembles scenario intelligence reports with findings, evidence, assumptions, and action plans. |

---

## Data Sources

| Dataset | Purpose |
|---------|---------|
| UN Comtrade | Bilateral trade flows, import dependencies, supplier rankings, HHI |
| World Bank Macro | Country structural indicators for macro exposure scoring |
| World Bank Pink Sheet | Commodity price history and stress signals |
| GDELT | Country-level geopolitical event-risk frequency and severity |
| Baseline dependency graph | Curated entity relationships (`data/strategic_global`) |

---

## Quick Start

### Docker

```bash
docker run -d --name atlasgraph-postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=atlasgraph \
  -p 5432:5432 \
  postgres:14
```

### Backend

```bash
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/atlasgraph?sslmode=disable

go run ./cmd/atlas db migrate

go run ./cmd/atlas db load \
  --trade-data data/processed/trade \
  --macro-data data/processed/macro \
  --event-data data/processed/events \
  --commodity-data data/processed/commodity_prices \
  --graph-data data/strategic_global

go run ./cmd/atlas serve \
  --data data/strategic_global \
  --trade-data data/processed/trade \
  --processed-macro-data data/processed/macro \
  --processed-event-data data/processed/events \
  --commodity-data data/processed/commodity_prices \
  --port 8080
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

### Verify

```bash
go test ./...
cd frontend && npm run test && npm run build
```

---

## Screenshots

| Dashboard | Shock Simulation |
|:---:|:---:|
| ![Dashboard](docs/screenshots/dashboard.png) | ![Shock Simulation](docs/screenshots/shock-simulation.png) |

| Client Analytics | Pipeline Monitor |
|:---:|:---:|
| ![Client Analytics](docs/screenshots/client-analytics.png) | ![Pipeline Monitor](docs/screenshots/pipeline-monitor.png) |

---

## Future Work

- Interactive dependency graph explorer with path highlighting
- Server-backed scenario history (replace History tab placeholder)
- SQL analytics explorer over PostgreSQL trade and concentration tables
- PDF export for executive intelligence reports
- Expanded Analytics Explorer (country, supplier, commodity drill-down)

---

## License

MIT — see [LICENSE](LICENSE).
