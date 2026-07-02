# AtlasGraph Technical Reference

Supplementary documentation for ingestion pipelines, scoring formulas, API details, and advanced CLI usage. The main project overview is in [README.md](../README.md).

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
  ingested here because the World Bank API does not serve it cleanly â€” it remains
  a first-class node in the seeded graph, just not in the macro panel.
- **Batched requests.** All requested countries are fetched in a single call per
  indicator (semicolon-separated), so a five-country panel is six HTTP requests,
  not thirty.
- **Robust client.** Context timeouts, non-200 handling, malformed-JSON
  detection, API error messages, pagination and genuinely missing values (kept as
  JSON `null`, never coerced to 0) are all handled with clear errors.
- **Output.** Normalised records are written to
  `data/raw/worldbank/worldbank_indicators.json` (the directory is created if
  needed). `data/raw/` is git-ignored â€” only a `.gitkeep` is tracked â€” so large
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
Macro Exposure Score** per country â€” the bridge between Phase 1's seeded graph
weights and real-world fundamentals. This is implemented in
[`internal/scoring/macro`](internal/scoring/macro).

The score is built from a **selected set of World Bank macroeconomic exposure
indicators** â€” trade exposure, manufacturing dependency, inflation stress,
high-tech export concentration and economic-buffer risk.

> **This is exposure/risk scoring, not forecasting.** It does not predict
> markets, prices or crises. It measures how *structurally exposed* an economy
> is â€” to trade, supply-chain, price and technology shocks â€” given its latest
> macro fundamentals, and shows exactly which factors drive that exposure.
>
> **This is not the final, complete AtlasGraph fragility score.** It covers
> macro exposure only. Full fragility scoring will later combine this with graph
> dependency / centrality, supplier concentration, event risk and commodity
> volatility.

### Components

Each country's score blends five normalised components (each 0â€“100):

| Component                    | Built from                                   | Higher means â€¦                          |
| ---------------------------- | -------------------------------------------- | --------------------------------------- |
| `trade_exposure`             | imports % GDP + exports % GDP                | more exposed to trade disruption        |
| `manufacturing_dependency`   | manufacturing value added % GDP              | more exposed to supply-chain shocks     |
| `inflation_stress`           | inflation, annual %                          | more macro price stress                 |
| `high_tech_concentration`    | high-tech exports Ã· GDP                      | more exposed to tech-trade disruption   |
| `economic_buffer_risk`       | GDP size (log scale), **inverse**            | smaller economy = less shock-absorbing  |

Components use **calibrated absolute reference bands**, not min-max over the
loaded panel, so a country's score is stable no matter which other countries are
present (and a single-country file still scores sensibly).

### Final score and risk bands

```
macro_exposure_score = 0.30Â·trade_exposure
                     + 0.25Â·manufacturing_dependency
                     + 0.20Â·inflation_stress
                     + 0.15Â·high_tech_concentration
                     + 0.10Â·economic_buffer_risk        â†’ 0..100
```

Weights sum to 1.0. When an indicator is missing, its component is dropped and
the remaining weights are **renormalised**, so gaps in the data never silently
deflate a score. Each component records the year it actually used (the latest
available at or before the requested `--year`).

| Score   | Risk level |
| ------- | ---------- |
| 0â€“30    | Low        |
| 30â€“60   | Medium     |
| 60â€“80   | High       |
| 80â€“100  | Critical   |

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
so real datasets â€” e.g. UN Comtrade exports â€” can later be dropped in unchanged.
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
and a per-supplier dependency band (Low <10% | Medium 10â€“40% | High â‰¥40%):

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
(HHI < 0.15 Low | 0.15â€“0.25 Medium | > 0.25 High):

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
exports**. This importer ingests those files directly â€” no API credentials are
required yet â€” and normalises them into the *same* `trade_flows.json` the rest
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
`electronic integrated circuits` / code `8542` â†’ **semiconductors**,
`lithium`/`batteries` â†’ **lithium batteries**, `cobalt` â†’ **cobalt ores**,
`petroleum oils`/`crude` â†’ **crude oil**, `rare earth` â†’ **rare earths**.
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

This is **not treated as ground truth** â€” it is a noisy public signal for
geopolitical and disruption-related risk (sanctions, conflict, export controls,
shipping disruption, semiconductor/energy/commodity stress, â€¦). It is useful as
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
base URL, so it is fully testable from saved JSON fixtures (via `httptest`) â€”
the test suite never calls the live GDELT service. The CLI makes real HTTP calls
for actual use.

### Rate limiting and resilience

GDELT asks for **no more than one request every 5 seconds** and will otherwise
return `429 Too Many Requests`. Live ingestion can therefore be temporarily
rate-limited (especially from shared IPs or for heavy queries), so the importer
is built to be demo-safe and production-style:

- `--limit` caps results per country (default `25`) to keep queries light.
- `--delay-seconds` spaces per-country requests apart (default `6`, clamped up
  to a `5` second minimum).
- On `429`, the importer waits 10 seconds and retries up to **2** times per
  country.
- If a country still fails it is **skipped**, not fatal: successful countries are
  saved and the failed ones are reported (the command only fails when **every**
  country fails).

If every country fails, the importer points you at offline mode:

```
Live GDELT ingestion failed for all countries. Try again later or use --fixture data/examples/gdelt_events_sample.json for offline demo mode.
```

### Offline / reproducible demo mode

`--fixture` loads a **local synthetic fixture** instead of calling the API,
normalises it into the exact same `GDELTEventRecord` schema, and writes the same
`data/raw/gdelt/gdelt_events.json` â€” so every downstream command (`events risk`)
works identically offline.

> âš ï¸ `data/examples/gdelt_events_sample.json` is **synthetic, reproducible demo
> data â€” not real GDELT output**. The titles are plausible but invented and the
> URLs are `https://example.com/...` placeholders. Use it for offline demos and
> deterministic tests, never as a real-world event source.

### Commands

```bash
# Live ingestion (rate-limit aware): small per-country limit + 6s spacing.
go run ./cmd/atlas ingest gdelt --countries TWN,CHN,JPN,KOR,USA,DEU --days 7 --limit 25 --delay-seconds 6 --out data/raw/gdelt

# Offline reproducible demo (no network): load the synthetic fixture.
go run ./cmd/atlas ingest gdelt --fixture data/examples/gdelt_events_sample.json --out data/raw/gdelt

# Score event risk from whichever mode populated data/raw/gdelt.
go run ./cmd/atlas events risk --data data/raw/gdelt
go run ./cmd/atlas events risk --data data/raw/gdelt --output json
```

Live ingestion reports what was requested, the per-country success/failure
split, how many documents were fetched and matched risk terms, and the leading
countries and risk terms:

```
GDELT EVENT INGESTION
----------------------------------------------------------------
  Countries requested    : TWN, CHN, JPN, KOR, USA, DEU
  Days                   : 7
  Limit per country      : 25
  Delay seconds          : 6
  Countries succeeded    : TWN, CHN, JPN, KOR, USA, DEU
  Countries failed       : (none)
  Records fetched        : 132
  Records with risk terms: 98
  Output                 : data/raw/gdelt/gdelt_events.json

  Top countries by event count:
    1. China                            24
    2. United States                    23
    3. Taiwan                           â€¦

  Top matched risk terms:
    1. sanctions                        41
    2. semiconductor                    29
    3. conflict                        â€¦
```

Fixture ingestion prints the same leaderboards under a clearly labelled
**FIXTURE MODE** header so synthetic data is never mistaken for a live pull:

```
GDELT EVENT INGESTION â€” FIXTURE MODE
----------------------------------------------------------------
  Source fixture         : data/examples/gdelt_events_sample.json
  Output                 : data/raw/gdelt/gdelt_events.json
  Records loaded         : 16
  Countries              : TWN, CHN, USA, DEU, KOR, JPN, SAU, COD
  Records with risk terms: 16

  Top countries by event count:
    1. China                            2
    â€¦

  Note: synthetic, reproducible demo data â€” not real GDELT output.
```

### Event risk scoring

`events risk` scores each country on a 0â€“100 scale from three components,
combined with calibrated weights (`internal/scoring/events`):

```
event_risk_score =
    0.45 * event_count_score        // volume of risk-relevant coverage
  + 0.35 * negative_tone_score      // how negative that coverage is
  + 0.20 * risk_term_density_score  // distinct risk themes per article
```

Each component is mapped onto 0â€“100 with absolute reference bands (so a
country's score does not depend on which other countries are in the panel), and
the final score falls into a qualitative band:

| Score   | Risk     |
|---------|----------|
| 0â€“30    | Low      |
| 30â€“60   | Medium   |
| 60â€“80   | High     |
| 80â€“100  | Critical |

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
| exporter country â†’ commodity                  | `exports`             | exporter's share of that commodity's total export value (supplier importance) |
| commodity â†’ importer country                  | `imports`             | importer's **top-supplier share**, with sourcing **HHI** as concentration |
| importer country â†’ commodity-dependent sector | `industry_dependency` | coarse default dependency from the commodityâ†’sector mapping            |

This preserves the supplier-dependency signal end to end: e.g. if the USA sources
62% of its semiconductors from Taiwan, the `Taiwan â†’ semiconductors` edge carries
Taiwan's supplier importance and the `semiconductors â†’ United States` edge carries
the 62% top-supplier share, so the `Taiwan â†’ semiconductors â†’ United States` path
weight reflects that concentration. Sectors are attached from a small, explicit
commodityâ†’sector map (e.g. semiconductors â†’ AI hardware, cloud infrastructure,
automotive electronics, consumer devices).

Generated scenario presets are emitted when their trigger flow is present in the
data â€” `taiwan_semiconductor_shock` (Taiwan exports semiconductors),
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

## Strategic Global Demo Dataset

`data/strategic_global` is a **curated synthetic strategic dataset** for larger
control-room demos: **24 countries**, **20 commodities**, **20 sectors**, **8
maritime chokepoints**, **~190 dependencies**, and **10 shock scenarios**. It uses
the same JSON schema as `data/sample` and does **not** replace the trade-graph
builder output under `data/generated/trade_graph`.

This is reproducible local demo data â€” **not** live UN Comtrade, GDELT, or World
Bank prices. See [`data/strategic_global/README.md`](data/strategic_global/README.md)
for scope and future-ingestion notes.

```bash
# Graph overview
go run ./cmd/atlas graph summary --data data/strategic_global

# Scenario presets (Taiwan semiconductors, Hormuz crude, Panama shipping, â€¦)
go run ./cmd/atlas scenario list --data data/strategic_global
go run ./cmd/atlas scenario run taiwan_semiconductor_shock --data data/strategic_global --explain

# Custom shock
go run ./cmd/atlas shock --source Taiwan --commodity semiconductors --drop 30 --depth 3 --data data/strategic_global --explain

# Compare scenarios on the larger graph
go run ./cmd/atlas scenario compare --data data/strategic_global

# Serve the API against this dataset
go run ./cmd/atlas serve \
  --data data/strategic_global \
  --trade-data data/processed/trade \
  --macro-data data/raw/worldbank \
  --event-data data/raw/gdelt \
  --commodity-data data/processed/commodity_prices \
  --port 8080
```

---

## Commodity Price Stress

Knowing that a country *depends* on a commodity is only half the picture â€”
AtlasGraph also tracks whether that commodity is under recent **price stress or
volatility**. A local CSV importer ingests commodity price time series modelled
on **World Bank "Pink Sheet" style** monthly prices, and a separate scorer turns
each series into an explainable 0â€“100 stress score. The importer lives in
[`internal/ingest/commodityprices`](internal/ingest/commodityprices) and the
scorer in [`internal/scoring/commodities`](internal/scoring/commodities). No
external APIs are called.

> âš ï¸ **The bundled `data/examples/commodity_prices_sample.csv` is synthetic,
> reproducible demo data â€” not real World Bank prices.** It contains plausible
> monthly prices for 10 commodities (crude oil, natural gas, copper, aluminum,
> lithium carbonate, cobalt, nickel, wheat, corn, rice) across 24 months
> (2023-01 â†’ 2024-12) so the demo is fully offline and deterministic.

### Input CSV format

```
date,commodity_code,commodity_name,price_usd,unit,source
2024-01,crude_oil,crude oil,82.4,USD/barrel,synthetic_world_bank_pink_sheet_style
```

Dates may be `YYYY-MM` or `YYYY-MM-DD` (normalised to `YYYY-MM`); commodity codes
are lower-cased with spaces/hyphens collapsed to underscores; prices tolerate
thousands separators and a leading `$`. Malformed rows (bad date, non-positive
price, missing code/name) are skipped with a reason rather than aborting the
file. Each record is normalised to:

```json
{
  "date": "2024-01",
  "commodity_code": "crude_oil",
  "commodity_name": "crude oil",
  "price_usd": 82.4,
  "unit": "USD/barrel",
  "source": "synthetic_world_bank_pink_sheet_style"
}
```

### Commodity Stress Score

Each commodity is scored on three transparent, individually-weighted components:

- `recent_change_score` â€” magnitude of the **% change over the last 3 months**
- `volatility_score` â€” **standard deviation of monthly returns** over the last 12 months
- `momentum_score` â€” magnitude of the **% change over the last 12 months**

```
Commodity Stress Score = 0.40 * recent_change_score
                       + 0.40 * volatility_score
                       + 0.20 * momentum_score
```

Risk bands: **Low 0â€“30 | Medium 30â€“60 | High 60â€“80 | Critical 80â€“100**.

> This is a commodity price **stress** score, **not a prediction of future
> prices**. It summarises recent movement and volatility from historical monthly
> data only.

### Commands

```bash
# Ingest (synthetic demo data) â†’ data/processed/commodity_prices/commodity_prices.json
go run ./cmd/atlas ingest commodity-prices --file data/examples/commodity_prices_sample.csv --out data/processed/commodity_prices

# Score price stress per commodity
go run ./cmd/atlas score commodities --data data/processed/commodity_prices

# Document the formula, components, bands and limitations (no data needed)
go run ./cmd/atlas score commodities --data data/processed/commodity_prices --explain-formula

# Machine-readable output
go run ./cmd/atlas score commodities --data data/processed/commodity_prices --output json
```

Ingestion report:

```
COMMODITY PRICE INGESTION
----------------------------------------------------------------
  Source file  : data/examples/commodity_prices_sample.csv
  Output       : data/processed/commodity_prices/commodity_prices.json
  Rows         : 240
  Valid rows   : 240
  Skipped rows : 0
  Commodities  : 10
  Date range   : 2023-01 to 2024-12
  Latest month : 2024-12
```

Stress scores:

```
COMMODITY STRESS SCORES
----------------------------------------------------------------
  COMMODITY          LATEST PRICE           3M CHANGE  12M CHANGE  VOLATILITY  SCORE  RISK
  natural gas        3.40 USD/mmbtu         +47.8%     +36.0%      15.8%       89.0   Critical
  lithium carbonate  9,800 USD/metric ton   -6.7%      -32.4%      8.7%        37.9   Medium
  nickel             15,600 USD/metric ton  -1.3%      -5.5%       6.4%        19.8   Low
  cobalt             24,000 USD/metric ton  -1.2%      -25.0%      3.5%        16.7   Low
  crude oil          73.00 USD/barrel       -1.4%      -1.4%       4.0%        12.3   Low
  ...
```

(`12M CHANGE` shows `n/a` for commodities with fewer than 13 months of data.)

---

## Unified Fragility Score

AtlasGraph combines existing GFIP signals into an **explainable composite
fragility score** for countries and commodities. This is a composite risk score,
not a prediction â€” it summarises structural exposure from signals the engine
already computes.

### Country Fragility Score

| Component | Weight | Source |
|-----------|--------|--------|
| `macro_exposure_score` | 0.30 | World Bank macro exposure scorer |
| `event_risk_score` | 0.25 | GDELT event-risk scorer |
| `trade_concentration_score` | 0.25 | Average supplier HHI across imported commodities |
| `shock_exposure_score` | 0.20 | Default scenario shock impact on the country |

### Commodity Fragility Score

| Component | Weight | Source |
|-----------|--------|--------|
| `commodity_stress_score` | 0.35 | Commodity price stress scorer |
| `supplier_concentration_score` | 0.30 | Average importer-side HHI across trade flows |
| `event_exposure_score` | 0.20 | Average event risk of exporter countries |
| `graph_centrality_score` | 0.15 | Commodity node degree relative to the graph |

**Risk bands** (both country and commodity): Low 0â€“30, Medium 30â€“60, High 60â€“80,
Critical 80â€“100.

Missing components are listed in `missing_components` and excluded from the
blend; remaining weights are renormalised so partial data still produces a
meaningful score.

### Commands

```bash
# Text table (countries + commodities)
go run ./cmd/atlas score fragility \
  --graph-data data/generated/trade_graph \
  --trade-data data/processed/trade \
  --macro-data data/raw/worldbank \
  --event-data data/raw/gdelt \
  --commodity-data data/processed/commodity_prices

# JSON output
go run ./cmd/atlas score fragility \
  --graph-data data/generated/trade_graph \
  --trade-data data/processed/trade \
  --macro-data data/raw/worldbank \
  --event-data data/raw/gdelt \
  --commodity-data data/processed/commodity_prices \
  --output json

# Formula explanation (no data required)
go run ./cmd/atlas score fragility --explain-formula
```

Example text output:

```
UNIFIED FRAGILITY SCORES
----------------------------------------------------------------
COUNTRIES
COUNTRY        SCORE  RISK    TOP DRIVERS
Taiwan         62.4   High    macro exposure, event risk
United States  48.1   Medium  trade concentration, macro exposure
...

COMMODITIES
COMMODITY      SCORE  RISK    TOP DRIVERS
semiconductors 55.2   Medium  graph centrality, supplier concentration
crude oil      41.0   Medium  commodity stress, event exposure
...
```

### API endpoints

| Method & path | Description |
|---------------|-------------|
| `GET /api/fragility/countries` | All country unified fragility scores |
| `GET /api/fragility/commodities` | All commodity unified fragility scores |
| `GET /api/fragility/summary` | Top 5 countries and top 5 commodities by score |

If some data paths are missing at server startup, these endpoints still return
partial scores with `missing_components` populated â€” they do not crash.

```bash
curl http://localhost:8080/api/fragility/summary
curl http://localhost:8080/api/fragility/countries
curl http://localhost:8080/api/fragility/commodities
```

The GFIP frontend overview page shows the top 5 countries and commodities from
`GET /api/fragility/summary` with score, risk badge, and top drivers.

---

## HTTP API Server

AtlasGraph ships a lightweight, pureâ€“`net/http` JSON API so the same engine that
powers the CLI can back a future web frontend (a Vite app on `:5173` is already
allowed via CORS). It adds **no new dependencies** and reuses the exact internal
logic and JSON shapes the CLI uses â€” `/api/shock`, for example, returns the same
structure as `atlas shock --output json`.

### Start the server

```bash
go run ./cmd/atlas serve \
  --data data/generated/trade_graph \
  --trade-data data/processed/trade \
  --macro-data data/raw/worldbank \
  --event-data data/raw/gdelt \
  --commodity-data data/processed/commodity_prices \
  --port 8080
```

All data flags are optional and loaded **lazily, per request**, so the server
always starts. If a data path is missing or empty, only the affected endpoint
returns a helpful JSON error â€” every other endpoint keeps working. Pass
`--data ""` (empty) to serve the **embedded sample** graph with no files on disk.

Startup prints the port, each data path, and the available endpoints:

```
ATLASGRAPH API SERVER
----------------------------------------------------------------
  Port        : 8080
  Graph data  : data/generated/trade_graph
  Trade data  : data/processed/trade
  Macro data  : data/raw/worldbank
  Event data  : data/raw/gdelt
  Commodity data: data/processed/commodity_prices

  Endpoints:
    GET  /health
    GET  /api/graph/summary
    GET  /api/graph/entities
    GET  /api/scenarios
    GET  /api/shock/options
    POST /api/shock
    POST /api/scenarios/compare
    GET  /api/trade/summary
    GET  /api/trade/dependency?importer=USA&commodity=semiconductors
    GET  /api/trade/concentration?importer=USA&commodity=semiconductors
    GET  /api/macro/scores
    GET  /api/events/risk
    GET  /api/commodities/stress
    GET  /api/fragility/countries
    GET  /api/fragility/commodities
    GET  /api/fragility/summary

  Listening on http://localhost:8080
```

### Endpoints

| Method & path | Description |
|---------------|-------------|
| `GET  /health` | Liveness probe (`{"status":"ok",...}`) |
| `GET  /api/graph/summary` | Entity counts and highest-degree nodes |
| `GET  /api/graph/entities` | Graph entities grouped by type (countries, commodities, sectors, routes, companies) |
| `GET  /api/scenarios` | Saved shock scenario presets |
| `GET  /api/shock/options` | Graph-aware shock guidance: valid sources/commodities, shock-type descriptions, and recommended scenarios |
| `POST /api/shock` | Run a shock simulation (body below) |
| `POST /api/scenarios/compare` | Run multiple shock scenarios and rank systemic impact |
| `GET  /api/trade/summary` | Ingested trade-panel digest |
| `GET  /api/trade/dependency?importer=&commodity=` | Supplier dependency breakdown |
| `GET  /api/trade/concentration?importer=&commodity=` | Supplier HHI concentration |
| `GET  /api/macro/scores` | Macro exposure scores |
| `GET  /api/events/risk` | GDELT event-risk scores |
| `GET  /api/commodities/stress` | Commodity price-stress scores |
| `GET  /api/fragility/countries` | Unified country fragility scores |
| `GET  /api/fragility/commodities` | Unified commodity fragility scores |
| `GET  /api/fragility/summary` | Top 5 countries and commodities by fragility |

### `POST /api/shock`

Request body (`drop`, `depth` and `shock_type` are optional and fall back to
engine defaults â€” `30`, `3`, `export_collapse`):

```json
{
  "source": "Taiwan",
  "commodity": "semiconductors",
  "drop": 30,
  "depth": 3,
  "shock_type": "export_collapse"
}
```

The response matches `atlas shock --output json` (scenario, exposures, affected
paths, highest-risk entities, graph impact summary). Add `"explain": true` to
include the `blocked_edges` breakdown.

Valid-looking but suboptimal combinations are **not** rejected. Instead the
response may include a non-fatal `warnings` array, e.g. a `route_disruption` on a
graph with no route nodes, or a shock type that does not travel along the
relationship linking the chosen source to the commodity:

```json
{
  "warnings": [
    "route_disruption works best with route nodes, but the current graph has no routes.",
    "No direct exports edge found from China to crude oil in this graph."
  ]
}
```

`GET /api/shock/options` exposes the same guidance up front: graph-validated
`sources`/`commodities` for dropdowns, per-shock-type descriptions, and
`recommended_scenarios` that only include combinations that make sense for the
loaded graph.

### Scenario Comparison

Compare multiple shock scenarios side-by-side and rank which causes the most
systemic impact. Each scenario is run independently through the same shock
engine as `POST /api/shock`; scoring formulas and propagation logic are
unchanged.

**CLI** â€” by default compares graph-validated recommended scenarios (Taiwan
semiconductor export collapse, China lithium battery supply cut, Saudi crude oil
supply cut when present in the graph):

```bash
go run ./cmd/atlas scenario compare --data data/generated/trade_graph
go run ./cmd/atlas scenario compare --data data/generated/trade_graph --output json
```

**API** â€” `POST /api/scenarios/compare`:

```json
{
  "scenarios": [
    {
      "label": "Taiwan semiconductor export collapse",
      "source": "Taiwan",
      "commodity": "semiconductors",
      "shock_type": "export_collapse",
      "drop": 30,
      "depth": 3,
      "explain": true
    },
    {
      "label": "China lithium battery supply cut",
      "source": "China",
      "commodity": "lithium batteries",
      "shock_type": "supply_cut",
      "drop": 35,
      "depth": 3,
      "explain": true
    },
    {
      "label": "Saudi crude oil supply cut",
      "source": "Saudi Arabia",
      "commodity": "crude oil",
      "shock_type": "supply_cut",
      "drop": 25,
      "depth": 3,
      "explain": true
    }
  ]
}
```

Response shape:

```json
{
  "summary": {
    "worst_overall_scenario": "Taiwan semiconductor export collapse",
    "most_countries_affected": "Taiwan semiconductor export collapse",
    "most_sectors_affected": "Taiwan semiconductor export collapse",
    "highest_average_fragility_delta": "Taiwan semiconductor export collapse",
    "highest_max_fragility_delta": "Taiwan semiconductor export collapse"
  },
  "results": [
    {
      "label": "Taiwan semiconductor export collapse",
      "source": "Taiwan",
      "commodity": "semiconductors",
      "shock_type": "export_collapse",
      "drop": 30,
      "depth": 3,
      "affected_nodes_count": 12,
      "affected_paths_count": 8,
      "average_fragility_delta": 9.42,
      "max_fragility_delta": 16.3,
      "top_affected_entities": [],
      "top_affected_countries": [],
      "top_affected_sectors": [],
      "warnings": []
    }
  ]
}
```

Results are ranked worst-first (by average fragility delta, then max delta, then
affected nodes). Invalid scenarios are included with a `warnings` entry describing
the failure; the comparison as a whole still returns HTTP 200.

```bash
curl -X POST http://localhost:8080/api/scenarios/compare \
  -H "Content-Type: application/json" \
  -d '{"scenarios":[{"label":"Taiwan semiconductor export collapse","source":"Taiwan","commodity":"semiconductors","shock_type":"export_collapse","drop":30,"depth":3}]}'
```

### Error shape

Every failure returns a consistent JSON envelope (the `hint` is optional):

```json
{
  "error": "importer and commodity query parameters are required",
  "hint": "example: /api/trade/dependency?importer=USA&commodity=semiconductors"
}
```

### curl examples

```bash
# Health check
curl http://localhost:8080/health

# Graph + scenarios
curl http://localhost:8080/api/graph/summary
curl http://localhost:8080/api/graph/entities
curl http://localhost:8080/api/scenarios

# Graph-aware shock guidance (valid sources/commodities, recommended scenarios)
curl http://localhost:8080/api/shock/options

# Run a shock
curl -X POST http://localhost:8080/api/shock \
  -H "Content-Type: application/json" \
  -d '{"source":"Taiwan","commodity":"semiconductors","drop":30,"depth":3,"shock_type":"export_collapse"}'

# Compare scenarios
curl -X POST http://localhost:8080/api/scenarios/compare \
  -H "Content-Type: application/json" \
  -d '{"scenarios":[{"label":"Taiwan semiconductor export collapse","source":"Taiwan","commodity":"semiconductors","shock_type":"export_collapse","drop":30,"depth":3}]}'

# Trade analysis
curl http://localhost:8080/api/trade/summary
curl "http://localhost:8080/api/trade/dependency?importer=USA&commodity=semiconductors"
curl "http://localhost:8080/api/trade/concentration?importer=USA&commodity=semiconductors"

# Scores
curl http://localhost:8080/api/macro/scores
curl http://localhost:8080/api/events/risk
curl http://localhost:8080/api/commodities/stress
curl http://localhost:8080/api/fragility/summary
```

### CORS

`http://localhost:5173` and `http://127.0.0.1:5173` are pre-approved so the Vite
dev frontend (see below) can call the API directly; preflight `OPTIONS` requests
are answered with `204 No Content`.

---

## Web Frontend (control room)

A **React + TypeScript + Vite + Tailwind** frontend lives in [`frontend/`](frontend).
It is a dark, Bloomberg/Palantir-style **control-room UI** for the platform â€”
**Global Fragility Intelligence Platform**, *Powered by AtlasGraph* â€” and talks
to the Go API over HTTP. It is intentionally dependency-light (no Next.js, no
state library, no UI kit) so it stays easy to read and demo.

This milestone's screen includes:

- a header with a **live API status badge** (polls `GET /health`),
- **overview cards** (nodes / countries / commodities / sectors / dependencies)
  from `GET /api/graph/summary`,
- a **Unified Fragility** panel (top 5 countries and commodities by score,
  with risk badges and top drivers) from `GET /api/fragility/summary`,
- a **scenario preset** dropdown from `GET /api/scenarios` (defaults to
  `taiwan_semiconductor_shock` when present),
- a **graph-aware Shock Simulator**: source/commodity inputs are searchable
  dropdowns backed by `GET /api/graph/entities`, shock-type descriptions and
  **recommended-scenario** quick-buttons come from `GET /api/shock/options`, and
  weak combinations raise an amber advisory before you run (you can still run),
- a **Shock Simulator** panel that `POST`s to `/api/shock`, pre-filled from the
  selected scenario, and
- **shock results**: impact metrics, direct & second-order exposure tables, the
  affected dependency paths, any backend `warnings`, and (with *Explain* on) the
  blocked edges.

### Run it

The frontend needs the API running first. **In one terminal, start the backend:**

```bash
go run ./cmd/atlas serve \
  --data data/generated/trade_graph \
  --trade-data data/processed/trade \
  --macro-data data/raw/worldbank \
  --event-data data/raw/gdelt \
  --commodity-data data/processed/commodity_prices \
  --port 8080
```

> First time? Generate a graph and (optionally) ingest data first, e.g.
> `atlas ingest trade --file data/examples/comtrade_sample.csv --out data/processed/trade`
> then `atlas graph build-trade --trade-data data/processed/trade --out data/generated/trade_graph`,
> and `atlas ingest gdelt --fixture data/examples/gdelt_events_sample.json --out data/raw/gdelt`.
> Any missing path only disables the matching endpoint â€” the server still starts.

**In a second terminal, start the frontend:**

```bash
cd frontend
npm install
npm run dev
```

Then open the printed URL (http://localhost:5173). If the API is down, the UI
shows an "API unavailable" notice with the exact backend command to run.

### Configuration

The API base URL defaults to `http://localhost:8080` and can be overridden with
an env var (see [`frontend/.env.example`](frontend/.env.example)):

```bash
# frontend/.env
VITE_API_BASE_URL=http://localhost:8080
```

### Frontend layout

```
frontend/
â”œâ”€â”€ index.html
â”œâ”€â”€ package.json
â”œâ”€â”€ vite.config.ts          # dev server on :5173
â”œâ”€â”€ tailwind.config.js
â””â”€â”€ src/
    â”œâ”€â”€ App.tsx             # data fetching + layout
    â”œâ”€â”€ lib/api.ts          # typed API client (VITE_API_BASE_URL)
    â”œâ”€â”€ types/api.ts        # response/request types
    â””â”€â”€ components/         # Header, OverviewCards, ScenarioSelect,
                            # ShockSimulator, ShockResults, â€¦
```

---

## Testing

The engine is covered by unit tests across the core packages:

- `internal/data` â€” JSON loading (embedded and on-disk), scenario presets,
  malformed/missing files, duplicate entities, unknown references and
  out-of-range weights.
- `internal/graph` â€” node/edge bookkeeping, neighbours, depth-bounded path
  enumeration, `PathsBetween`, degree/counts, `FindByName` resolution, cycle
  safety.
- `internal/scoring` â€” the fragility formula, clamping/capping, monotonicity,
  noisy-or dependency aggregation, concentration max.
- `internal/simulation` â€” input validation, initial-impact math, direct vs
  second-order classification, monotonicity in drop size, depth limiting, the
  "zero drop â‡’ no impact" invariant, **and the typed propagation rules**: a
  semiconductor shock not reaching crude oil/lithium/cobalt, `price_spike`
  travelling `price_exposure`, `route_disruption` travelling `route_exposure`,
  relationship-labelled paths, and unknown shock types failing cleanly.
- `internal/ingest/worldbank` â€” the World Bank client against an `httptest`
  server (success, pagination, non-200, malformed JSON, API error messages,
  empty results and context timeout), normalisation, save/load round-trips and
  per-country summary building. **No test touches the real network.**
- `internal/scoring/macro` â€” component normalisation and clamping, the weighted
  blend, risk-band assignment, missing-indicator fallback + weight
  renormalisation, year-lens selection and score ordering.
- `internal/ingest/trade` â€” CSV parsing (reordered/mixed-case headers), required
  column validation, malformed-row skipping with reasons, safe numeric parsing,
  save/load round-trips, summary aggregation, supplier-share and HHI
  concentration maths, and dependency/concentration risk bands.
- `internal/tradegraph` â€” converting trade records into country/commodity/sector
  entities, supplier-share export/import edge weighting, the
  exports/imports/industry_dependency edge set, generated scenario triggers, and
  a full round-trip proving the generated dataset loads and simulates.
- `internal/cli` â€” command dispatch, scenario list/run, graph summary/paths,
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

- **Phase 1 â€” Seeded graph engine** âœ… *(current)* â€” JSON-driven **typed** graph,
  shock profiles + rule-based propagation, fragility scoring, scenarios, CLI and
  tests.
- **Phase 2 â€” Real trade data ingestion** ðŸ› ï¸ *(in progress)* â€” the World Bank
  macro ingestion module (`atlas ingest worldbank`) is the first real source, and
  `atlas score macro` already turns those indicators into an explainable macro
  exposure score per country. Next: pull production shares and trade flows from
  trade APIs and fold them into the graph's baseline weights, keeping the same
  interface so nothing downstream changes.
- **Phase 3 â€” Neo4j graph database** â€” persist the dependency graph and push
  traversal into the database for larger, real-world graphs.
- **Phase 4 â€” ClickHouse analytics layer** â€” store scenario runs and time series
  for large-scale querying and historical comparison.
- **Phase 5 â€” MLflow + LightGBM forecasting** â€” predict shock likelihood and
  forward-looking fragility from historical signals.
- **Phase 6 â€” Docker / AWS deployment** â€” reproducible, containerised stack and
  cloud deployment.

A dashboard, if it ever happens, sits on top of all of this â€” never in front of
it.
