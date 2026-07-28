# GFIP
**Global Fragility Intelligence Platform**

GFIP models how supply chain disruptions propagate through global trade and dependency networks. It fuses a curated dependency graph with observed trade, event-risk, macro, and commodity-price data to estimate exposure, run shock scenarios, and produce analyst-ready outputs. The platform is built for supply chain analysts, risk teams, and decision makers who need deterministic, explainable impact analysis rather than black-box forecasts.

---

## Features

- **Graph-based shock simulation** — Propagates export collapses, supply cuts, route disruptions, and price spikes along relationship-aware graph edges.
- **Real-world data ingestion** — Ingests UN Comtrade, GDELT, World Bank macro, and Pink Sheet commodity prices into normalized analytics panels.
- **PostgreSQL analytics** — Optional persistence layer for trade flows, concentration metrics, scenario runs, and client upload storage.
- **Client supply chain analysis** — Uploads supplier dependency CSVs and computes importer-level exposure against simulated shocks.
- **Supplier concentration (HHI)** — Measures import concentration and supplier share per importer–commodity pair.
- **Executive intelligence reports** — Generates structured scenario reports with trade, event, macro, and commodity context.
- **Mitigation recommendation engine** — Produces ranked, rule-based mitigation actions with priority, confidence, and supporting metrics.
- **Pipeline monitoring and validation** — Surfaces ETL health, validation checks, and load-quality metrics in the Data Operations workspace.
- **Enterprise workspace frontend** — Multi-tab React application with dashboard, shock simulation, client analytics, analytics explorer, and history views.

---

## System Architecture

```
Public Data Sources (Comtrade, GDELT, World Bank)
        ↓
ETL Pipeline (ingest, validate, normalize)
        ↓
PostgreSQL / Processed JSON panels
        ↓
Dependency Graph + Data Fusion
        ↓
Shock Simulation Engine
        ↓
Business Impact Analysis (fragility, exposure, client overlay)
        ↓
Executive Reports + Mitigation Recommendations
        ↓
GFIP Frontend (React)
```

---

## Technology Stack

**Backend**
Go 1.21, `net/http` JSON API, shared CLI (`cmd/atlas`), graph fusion engine, simulation and scoring packages

**Frontend**
React 18, TypeScript, Vite, Tailwind CSS, Recharts, Vitest

**Database**
PostgreSQL 14+ via `pgx/v5`, SQL migrations

**Data Sources**
UN Comtrade, GDELT, World Bank macro indicators, World Bank Pink Sheet, baseline strategic dependency graph (`data/strategic_global`)

**Analytics**
Herfindahl-Hirschman Index (HHI), fragility scoring, event-risk composites, trade concentration, operational impact multipliers, deterministic recommendation rules

---

## Core Modules

**Shock Simulation**
Runs relationship-constrained propagation over the fused dependency graph. Supports configurable drop percentage, depth, shock type, and operational assumptions (duration, recovery, substitutes, inventory buffer).

**Client Analytics**
Parses client supplier CSV uploads, validates rows, and computes concentration results. Overlays matched importers onto shock scenarios with estimated exposed trade and supplier dependence.

**Business Impact Assessment**
Aggregates direct and second-order exposure, fragility deltas, propagation paths, and client-specific impact into executive briefs and risk driver panels.

**Data Pipeline**
Ingests raw public datasets, validates schema and row quality, normalizes entities, and loads analytics tables. Exposes pipeline summary and validation status via API.

**Executive Reporting**
Builds scenario intelligence reports from simulation output and observed context panels. Includes executive action plans, client exposure assessment, and mitigation recommendations when client data is provided.

---

## Repository Structure

```
cmd/atlas/          CLI entrypoint and HTTP server
frontend/           React/TypeScript dashboard
internal/
  cli/              API handlers and report builders
  simulation/       Shock propagation engine
  scoring/          Fragility, event, macro, commodity scoring
  ingest/           ETL loaders (trade, events, macro, prices)
  customdata/       Client CSV analysis
  clientoverlay/    Client exposure overlay service
  recommendations/  Mitigation recommendation engine
  pipeline/         ETL validation and summary
  db/               PostgreSQL repository layer
migrations/         PostgreSQL schema
data/               Baseline graph, raw inputs, processed panels
docs/               Technical reference
```

---

## Running the Project

**Prerequisites:** Go 1.21+, Node.js 18+, Docker (for PostgreSQL)

**1. Start PostgreSQL**

```bash
docker run -d --name atlasgraph-postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=atlasgraph \
  -p 5432:5432 postgres:14
```

**2. Configure database**

```bash
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/atlasgraph?sslmode=disable
go run ./cmd/atlas db migrate
```

**3. Start backend**

```bash
go run ./cmd/atlas serve \
  --data data/strategic_global \
  --trade-data data/processed/trade \
  --processed-event-data data/processed/events \
  --commodity-data data/processed/commodity_prices \
  --port 8080
```

API: `http://localhost:8080`

**4. Start frontend**

```bash
cd frontend && npm install && npm run dev
```

Dashboard: `http://localhost:5173`

**Tests**

```bash
go test ./...
cd frontend && npm run test && npm run build
```

---

## Screenshots

| Dashboard | Shock Simulation | Client Analytics |
|:---:|:---:|:---:|
| _[screenshot placeholder]_ | _[screenshot placeholder]_ | _[screenshot placeholder]_ |

---

## Future Enhancements

- Interactive dependency graph explorer with zoom and path highlighting
- PDF export for executive intelligence reports
- SQL-backed analytics explorer for ad hoc trade and concentration queries
- Expanded longitudinal scenario library with multi-period comparison

---

## Further Reading

- [`docs/TECHNICAL_REFERENCE.md`](docs/TECHNICAL_REFERENCE.md) — API reference, scoring formulas, ingestion details
- [`data/strategic_global/README.md`](data/strategic_global/README.md) — Baseline graph dataset notes
