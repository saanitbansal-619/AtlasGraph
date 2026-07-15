# Baseline Dependency Graph Dataset

This folder contains the **baseline dependency graph** for AtlasGraph / GFIP
(`data/strategic_global`). It expands beyond the small embedded sample graph
(~9 countries, 45 dependencies) into a fuller country–commodity–sector–route
graph for reproducible local analysis. Folder path is unchanged.

## What this is

- A curated baseline dependency graph covering **24 countries**, **20
  commodities**, **20 sectors**, **8 maritime chokepoints**, **~190
  dependencies**, and named shock scenarios.
- Typed relationships used for model propagation (fragility deltas, shock
  paths, centrality). Link strengths are curated for analysis workflows — not
  raw observed trade statistics by themselves.
- The same JSON wire format as `data/sample` and `data/generated/trade_graph`,
  validated by the standard AtlasGraph loader.

## Observed data vs this graph

Observed panels fuse **on top of** this baseline when present under
`data/processed/`:

- **UN Comtrade** — trade flows, import dependencies, supplier concentration
- **GDELT** — event-risk signals
- **World Bank Pink Sheet** — commodity price history / stress

**Model-derived outputs** (fragility scores, shock propagation, impact deltas,
graph centrality) estimate exposure under stated model assumptions. They are not
raw observed facts.

## What this is not

- **Not a replacement** for `data/generated/trade_graph`. That dataset is still
  produced by `atlas graph build-trade` from ingested trade panels.
- **Not live streaming** of Comtrade / GDELT / Pink Sheet — those require local
  ingest into `data/processed` (or equivalent paths passed to `serve`).

## Quick commands

```bash
# Graph overview
go run ./cmd/atlas graph summary --data data/strategic_global

# List shock presets
go run ./cmd/atlas scenario list --data data/strategic_global

# Run a preset
go run ./cmd/atlas scenario run taiwan_semiconductor_shock --data data/strategic_global --explain

# Custom shock
go run ./cmd/atlas shock --source Taiwan --commodity semiconductors --drop 30 --depth 3 --data data/strategic_global --explain

# Compare recommended scenarios
go run ./cmd/atlas scenario compare --data data/strategic_global

# Serve the API against this graph (+ real panels when present)
go run ./cmd/atlas serve \
  --data data/strategic_global \
  --trade-data data/processed/trade \
  --macro-data data/raw/worldbank \
  --event-data data/raw/gdelt \
  --commodity-data data/processed/commodity_prices \
  --port 8080
```

## Scenario presets

| ID | Description |
|----|-------------|
| `taiwan_semiconductor_shock` | Taiwan semiconductor export collapse |
| `china_rare_earth_export_control` | China rare earth export control |
| `saudi_crude_oil_supply_cut` | Saudi crude oil supply cut |
| `russia_natural_gas_disruption` | Russia natural gas disruption |
| `ukraine_wheat_export_disruption` | Ukraine wheat export disruption |
| `drc_cobalt_supply_disruption` | DRC cobalt supply disruption |
| `chile_lithium_export_disruption` | Chile lithium export disruption |
| `hormuz_crude_route_disruption` | Strait of Hormuz crude route disruption |
| `panama_canal_shipping_disruption` | Panama Canal shipping disruption |
| `south_china_sea_electronics_disruption` | South China Sea electronics disruption |
