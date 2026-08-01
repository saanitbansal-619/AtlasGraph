# Case Study: Taiwan Semiconductor Export Collapse

Reproducible demonstration of GFIP’s portfolio-aware shock workflow.

## Purpose

Answer a concrete decision question:

> If Taiwan semiconductor exports fall by 30%, which importers and downstream sectors are most exposed, and what mitigation actions follow from concentration and evidence panels?

## Fixed inputs

| Parameter | Value |
|-----------|--------|
| Scenario ID | `taiwan_semiconductor_shock` |
| Source | Taiwan |
| Commodity | semiconductors |
| Shock type | `export_collapse` |
| Drop | 30% |
| Depth | 3 hops |
| Graph dataset | `data/strategic_global` |
| Optional client CSV | `data/client_overlay_test.csv` |

Operational assumptions (defaults unless changed in UI):

- Duration: short
- Recovery: medium
- Substitute availability: limited
- Inventory buffer: low

## How to reproduce

```bash
# Backend (example)
go run ./cmd/atlas serve --data data/strategic_global --port 8080

# Frontend
cd frontend && npm run dev
```

In the UI:

1. Dashboard → **Load Taiwan case study** (or Shock Simulation → select preset `Taiwan Semiconductor Export Collapse`).
2. Optional: Client Analytics → upload `data/client_overlay_test.csv`.
3. Run shock.
4. Generate Scenario Intelligence Report.
5. Review Recommended Actions and client exposure dollars at risk.

CLI equivalent:

```bash
go run ./cmd/atlas scenario run taiwan_semiconductor_shock --data data/strategic_global --explain
```

## Expected qualitative outputs

Exact numeric ranks can shift when fused trade/event/price panels are loaded. Under the baseline strategic graph you should observe:

- Direct exposure concentrated on **semiconductors** and Taiwan-linked export edges.
- Downstream pressure on compute / electronics / AI-adjacent sectors via `used_by` / industry dependencies.
- Affected paths listing Taiwan → semiconductors → downstream entities.
- Fragility deltas positive for impacted countries/commodities (higher = more stressed under the model).

With `client_overlay_test.csv` loaded and supplier/commodity matched to Taiwan / semiconductors:

- Client overlay reports matched importers and **estimated exposed trade** = supplier value × drop%.
- Recommendations may include supplier diversification / concentration mitigation when HHI or top-supplier share crosses rule thresholds.

## Decision artifacts

| Artifact | Location |
|----------|----------|
| Propagation paths & blocked edges | Shock Simulation → Results |
| Client dollars at risk | Results KPI row + Client Exposure panel |
| Executive report | Scenario Intelligence Report |
| Mitigation actions | Recommended Actions (deterministic rules) |

## What the model cannot claim

- It does **not** forecast whether a Taiwan export restriction will occur.
- It does **not** model firm-level inventory, contracts, dual-sourcing already in place, or classified supply links.
- Missing public panels (trade, GDELT, macro, Pink Sheet) degrade evidence; provenance badges show what contributed.
- Mitigation recommendations are **rule-based**, not optimized portfolio advice or legal/compliance guidance.

## Validation notes

- Path structure should match dependency semantics: target depends on source; export collapse propagates along profile-allowed relationships.
- Cross-commodity branches are blocked by default for this shock profile—blocked edges in explain mode are expected, not failures.
- Re-run with identical parameters should yield identical scores (deterministic engine).

## Hiring / review checkpoint

If reviewing this project, the credibility test is whether you can:

1. Reproduce the scenario from fixed inputs.
2. See assumptions and limitations next to the executive summary.
3. Trace client dollars at risk to uploaded rows × modeled drop.
4. Explain why a recommended action fired (supporting metrics on the card).
