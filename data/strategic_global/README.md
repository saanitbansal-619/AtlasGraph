# Strategic Global Demo Dataset

This folder contains a **curated, synthetic strategic global risk dataset** for
AtlasGraph / GFIP. It is designed to expand the platform beyond the small
embedded sample graph (~9 countries, 45 dependencies) into a richer control-room
demo without requiring live data ingestion.

## What this is

- A hand-authored strategic dependency graph covering **24 countries**, **20
  commodities**, **20 sectors**, **8 maritime chokepoints**, **~190
  dependencies**, and **10 shock scenarios**.
- Realistic but **clearly synthetic** link strengths, concentrations, and
  descriptions — plausible for demos, not calibrated to real trade statistics.
- The same JSON wire format as `data/sample` and `data/generated/trade_graph`,
  validated by the standard AtlasGraph loader.

## What this is not

- **Not live trade data.** No UN Comtrade flows, no real-time prices, no GDELT
  event feeds are embedded here.
- **Not a replacement** for `data/generated/trade_graph`. That dataset is still
  produced by `atlas graph build-trade` from ingested trade panels.
- **Not authoritative risk intelligence.** Use it for reproducible local demos,
  integration tests, and API/CLI exercises.

## Future work

Real global expansion will come from ingested panels already supported elsewhere
in AtlasGraph:

- **UN Comtrade** trade flows → `atlas graph build-trade`
- **GDELT** event risk → `atlas ingest gdelt`
- **World Bank Pink Sheet** commodity prices → `atlas ingest commodity-prices`

This strategic dataset lets GFIP demonstrate multi-region shock propagation
*today* while those pipelines mature.

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

# Serve the API against this graph
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

## Files

| File | Purpose |
|------|---------|
| `entities.json` | Countries, commodities, sectors, routes |
| `dependencies.json` | Typed dependency edges with weights and concentrations |
| `scenarios.json` | Saved shock presets |

To regenerate from the maintainer tool (optional):

```bash
go run ./tools/generate_strategic_global
```
