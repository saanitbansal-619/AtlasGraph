# GFIP — Global Fragility Intelligence Platform

**Powered by AtlasGraph**

## Project Overview

GFIP is a full-stack global fragility intelligence platform powered by AtlasGraph, a Go backend engine that combines observed trade, event-risk, and commodity-price data with a baseline dependency graph to estimate supply-chain exposure.

- **GFIP** — React/TypeScript analyst dashboard for exploring exposure, fragility, and shock results
- **AtlasGraph** — Go engine for graph modeling, scoring, data fusion, and the HTTP API

Scores are model-derived estimates, not raw factual predictions.

---

## Observed Data Sources

| Source | Role |
|--------|------|
| **UN Comtrade** | Trade flows, import dependencies, supplier concentration, trade concentration |
| **GDELT** | Country-level event-risk signals |
| **World Bank Pink Sheet** | Commodity price history and stress |
| **Baseline dependency graph** | Curated supply-chain relationships and sector dependencies (`data/strategic_global`) |

Observed panels live under `data/processed/` and fuse into the baseline graph when present. Baseline edges remain; real trade edges are additive and marked as real-data.

---

## Model-Derived Outputs

These are estimates under stated model assumptions, driven by the observed panels above:

- **Fragility scores** — explainable country and commodity composites
- **Shock propagation** — relationship-aware cascade along graph edges
- **Estimated impact deltas** — before/after fragility and exposure changes
- **Graph centrality** — structural importance in the dependency network
- **Executive impact briefs** — concise narrative summaries of estimated exposure

---

## Current Data Coverage

Snapshot with processed panels fused into the baseline graph:

| Metric | Value |
|--------|------:|
| UN Comtrade records | 1,669 |
| Countries (trade panel) | 197 |
| Commodities (trade panel) | 8 |
| Trade value represented | US$ 2.64T+ |
| Total dependencies (fused) | 2,039 |
| Real trade edges | 1,846 |

Coverage reflects locally ingested public datasets for selected commodities and years — not a live global feed for every HS code.

---

## Key Features

- **Shock simulation** — model export collapses, supply cuts, route disruptions, and price spikes
- **Unified fragility scoring** — composite scores blending trade, event, commodity, and graph signals
- **Trade dependency signals** — importer/commodity concentration and supplier shares from Comtrade
- **Event risk signals** — GDELT-derived country risk indicators
- **Commodity price history** — World Bank Pink Sheet time series and stress views
- **Executive impact brief** — short estimated-exposure narrative for each shock run
- **Scenario comparison** — side-by-side ranking by average/max fragility delta
- **Data fusion status** — transparent badges for baseline graph, Comtrade, GDELT, and Pink Sheet

---

## Tech Stack

| Layer | Stack |
|-------|-------|
| Backend | Go (`net/http` JSON API, shared CLI + server engine) |
| Frontend | React, TypeScript, Vite |
| UI | Tailwind CSS, Recharts |
| Ingestion | UN Comtrade pipeline, GDELT event-risk pipeline, World Bank commodity-price pipeline |

```
React / TypeScript dashboard
        ↓
Go HTTP API (`atlas serve`)
        ↓
AtlasGraph engine + data fusion
        ↓
Baseline graph + processed trade / events / prices
```

---

## Run Locally

### Prerequisites

- Go 1.21+
- Node.js 18+

### 1. Ingest observed data (if needed)

Skip if `data/processed/{trade,events,commodity_prices}` already exists.

```bash
# UN Comtrade (place CSVs under data/raw/un_comtrade/)
go run ./cmd/atlas ingest trade \
  --dir data/raw/un_comtrade \
  --out data/processed/trade \
  --source un-comtrade

# GDELT / event risk
go run ./cmd/atlas ingest events \
  --file data/raw/gdelt_events/events.csv \
  --out data/processed/events \
  --source gdelt

# World Bank Pink Sheet (download CMO-Historical-Data-Monthly.xlsx first)
go run ./cmd/atlas ingest commodity-prices \
  --file data/raw/worldbank_pinksheet/CMO-Historical-Data-Monthly.xlsx \
  --out data/processed/commodity_prices \
  --source worldbank-pinksheet
```

### 2. Backend

```bash
go mod tidy
go test ./...

go run ./cmd/atlas serve \
  --data data/strategic_global \
  --trade-data data/processed/trade \
  --macro-data data/raw/worldbank \
  --event-data data/raw/gdelt \
  --processed-event-data data/processed/events \
  --commodity-data data/processed/commodity_prices \
  --port 8080
```

API: **http://localhost:8080**

Missing processed paths disable only matching signals; the server still starts with the baseline graph.

### 3. Frontend

```bash
cd frontend
npm install
npm run dev
```

Dashboard: **http://localhost:5173**

---

## Useful CLI

```bash
go run ./cmd/atlas graph summary \
  --data data/strategic_global \
  --trade-data data/processed/trade \
  --event-data data/processed/events \
  --commodity-data data/processed/commodity_prices

go run ./cmd/atlas scenario list --data data/strategic_global
go run ./cmd/atlas scenario run taiwan_semiconductor_shock --data data/strategic_global --explain
go run ./cmd/atlas scenario compare --data data/strategic_global
```

---

## Testing

```bash
go test ./...

cd frontend
npm run build
```

---

## Further Reading

Detailed API reference, scoring formulas, and ingestion notes:
[`docs/TECHNICAL_REFERENCE.md`](docs/TECHNICAL_REFERENCE.md)

Dataset notes:
- [`data/strategic_global/README.md`](data/strategic_global/README.md)
- [`data/raw/un_comtrade/README.md`](data/raw/un_comtrade/README.md)
- [`data/raw/gdelt_events/README.md`](data/raw/gdelt_events/README.md)
- [`data/raw/worldbank_pinksheet/README.md`](data/raw/worldbank_pinksheet/README.md)

---

## License

See repository license file if present. Observed panels use public data sources under their respective terms. Model outputs are estimates under stated assumptions.
