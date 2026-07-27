import { useMemo, useState } from 'react'
import type { CustomDataAnalysisResponse, ShockResponse } from '../../types/api'
import type { SavedShockScenario } from '../../types/savedScenario'
import {
  normalizeCommodityRankings,
  sortCountriesByScore,
} from '../../lib/commodityNormalize'
import { computeClientExposureOverlay } from '../../lib/clientExposure'
import { deltaClass, fixed, pct, signed } from '../../lib/format'
import { AdaptiveRankingChart } from '../charts/AdaptiveRankingChart'
import { EmptyHint, Panel } from '../ui'

function hhiForScenario(
  result: ShockResponse,
  clientData: CustomDataAnalysisResponse | null,
): number | null {
  const overlay = computeClientExposureOverlay(
    clientData,
    result.scenario.source,
    result.scenario.commodity,
  )
  const withHhi = overlay?.exposures.find((e) => e.hhi != null)
  return withHhi?.hhi ?? null
}

function metricCards(a: SavedShockScenario, b: SavedShockScenario, clientData: CustomDataAnalysisResponse | null) {
  const ar = a.result
  const br = b.result
  const as = ar.graph_impact_summary
  const bs = br.graph_impact_summary
  return [
    {
      label: 'Countries affected',
      left: String(as.affected_countries),
      right: String(bs.affected_countries),
    },
    {
      label: 'Commodities affected',
      left: String(as.affected_commodities),
      right: String(bs.affected_commodities),
    },
    {
      label: 'Avg fragility Δ',
      left: signed(as.avg_fragility_delta),
      right: signed(bs.avg_fragility_delta),
      leftClass: deltaClass(as.avg_fragility_delta),
      rightClass: deltaClass(bs.avg_fragility_delta),
    },
    {
      label: 'HHI (client overlay)',
      left: (() => {
        const v = hhiForScenario(ar, clientData)
        return v == null ? '—' : fixed(v, 3)
      })(),
      right: (() => {
        const v = hhiForScenario(br, clientData)
        return v == null ? '—' : fixed(v, 3)
      })(),
    },
    {
      label: 'Dependency paths',
      left: String(as.affected_paths),
      right: String(bs.affected_paths),
    },
    {
      label: 'Event risk',
      left: ar.data_fusion?.real_event_risk_used ? 'Active' : 'Inactive',
      right: br.data_fusion?.real_event_risk_used ? 'Active' : 'Inactive',
    },
    {
      label: 'Macro exposure',
      left: ar.data_fusion?.data_sources?.some((s) => /macro/i.test(s)) ? 'Active' : 'Inactive',
      right: br.data_fusion?.data_sources?.some((s) => /macro/i.test(s)) ? 'Active' : 'Inactive',
    },
  ]
}

export function SavedScenarioComparison({
  saved,
  clientData,
  onClear,
}: {
  saved: SavedShockScenario[]
  clientData: CustomDataAnalysisResponse | null
  onClear?: () => void
}) {
  const [leftId, setLeftId] = useState('')
  const [rightId, setRightId] = useState('')

  const left = saved.find((s) => s.id === leftId) ?? null
  const right = saved.find((s) => s.id === rightId) ?? null
  const ready = !!left && !!right && left.id !== right.id

  const cards = useMemo(
    () => (ready && left && right ? metricCards(left, right, clientData) : []),
    [ready, left, right, clientData],
  )

  const countryChart = useMemo(() => {
    if (!ready || !left || !right) return []
    const map = new Map<string, { label: string; left: number; right: number }>()
    for (const row of sortCountriesByScore(
      (left.result.highest_risk_entities?.countries ?? []).map((it) => ({
        label: it.entity,
        value: it.delta,
      })),
    ).slice(0, 6)) {
      map.set(row.label, { label: row.label, left: row.value, right: 0 })
    }
    for (const row of sortCountriesByScore(
      (right.result.highest_risk_entities?.countries ?? []).map((it) => ({
        label: it.entity,
        value: it.delta,
      })),
    ).slice(0, 6)) {
      const existing = map.get(row.label)
      if (existing) existing.right = row.value
      else map.set(row.label, { label: row.label, left: 0, right: row.value })
    }
    return [...map.values()]
  }, [ready, left, right])

  if (saved.length < 2) {
    return (
      <Panel title="Comparison">
        <EmptyHint>Run and save at least two scenarios to compare.</EmptyHint>
      </Panel>
    )
  }

  return (
    <div className="space-y-4">
      <Panel
        title="Select scenarios"
        right={
          onClear ? (
            <button
              type="button"
              className="text-[11px] text-slate-500 hover:text-slate-300"
              onClick={onClear}
            >
              Clear saved
            </button>
          ) : undefined
        }
      >
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <div className="label mb-1.5">Scenario A</div>
            <select
              className="field"
              value={leftId}
              onChange={(e) => setLeftId(e.target.value)}
            >
              <option value="">Select scenario…</option>
              {saved.map((s) => (
                <option key={s.id} value={s.id} disabled={s.id === rightId}>
                  {s.title}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <div className="label mb-1.5">Scenario B</div>
            <select
              className="field"
              value={rightId}
              onChange={(e) => setRightId(e.target.value)}
            >
              <option value="">Select scenario…</option>
              {saved.map((s) => (
                <option key={s.id} value={s.id} disabled={s.id === leftId}>
                  {s.title}
                </option>
              ))}
            </select>
          </label>
        </div>
      </Panel>

      {!ready && (
        <EmptyHint>Select two different saved scenarios to open the comparison workspace.</EmptyHint>
      )}

      {ready && left && right && (
        <>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <ScenarioIdentityCard scenario={left} tone="A" />
            <ScenarioIdentityCard scenario={right} tone="B" />
          </div>

          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-4">
            {cards.map((card) => (
              <div key={card.label} className="panel px-3 py-2.5">
                <div className="label">{card.label}</div>
                <div className="mt-2 grid grid-cols-2 gap-2 text-center">
                  <div>
                    <div className="text-[10px] uppercase tracking-wide text-slate-600">A</div>
                    <div
                      className={`font-mono text-sm font-semibold ${
                        card.leftClass ?? 'text-cyan-200'
                      }`}
                    >
                      {card.left}
                    </div>
                  </div>
                  <div>
                    <div className="text-[10px] uppercase tracking-wide text-slate-600">B</div>
                    <div
                      className={`font-mono text-sm font-semibold ${
                        card.rightClass ?? 'text-amber-200'
                      }`}
                    >
                      {card.right}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <AdaptiveRankingChart
              title={`Countries · ${left.title}`}
              subtitle="Δ fragility"
              valueLabel="Δ fragility"
              valueDigits={1}
              valueSuffix=" Δ"
              data={sortCountriesByScore(
                (left.result.highest_risk_entities?.countries ?? []).map((it) => ({
                  label: it.entity,
                  value: it.delta,
                })),
              )}
              emptyLabel="No country impacts."
              topN={6}
            />
            <AdaptiveRankingChart
              title={`Countries · ${right.title}`}
              subtitle="Δ fragility"
              valueLabel="Δ fragility"
              valueDigits={1}
              valueSuffix=" Δ"
              data={sortCountriesByScore(
                (right.result.highest_risk_entities?.countries ?? []).map((it) => ({
                  label: it.entity,
                  value: it.delta,
                })),
              )}
              emptyLabel="No country impacts."
              topN={6}
            />
            <AdaptiveRankingChart
              title={`Commodities · ${left.title}`}
              subtitle="normalized"
              valueLabel="Δ fragility"
              valueDigits={1}
              valueSuffix=" Δ"
              data={normalizeCommodityRankings(
                (left.result.highest_risk_entities?.commodities ?? []).map((it) => ({
                  label: it.entity,
                  value: it.delta,
                })),
              )}
              emptyLabel="No commodity impacts."
              topN={6}
            />
            <AdaptiveRankingChart
              title={`Commodities · ${right.title}`}
              subtitle="normalized"
              valueLabel="Δ fragility"
              valueDigits={1}
              valueSuffix=" Δ"
              data={normalizeCommodityRankings(
                (right.result.highest_risk_entities?.commodities ?? []).map((it) => ({
                  label: it.entity,
                  value: it.delta,
                })),
              )}
              emptyLabel="No commodity impacts."
              topN={6}
            />
          </div>

          {countryChart.length > 0 && (
            <Panel title="Country fragility side-by-side" dense>
              <div className="overflow-x-auto">
                <table className="w-full min-w-[480px] text-left text-xs">
                  <thead className="text-slate-500">
                    <tr>
                      <th className="pb-2 pr-2 font-medium">Country</th>
                      <th className="pb-2 pr-2 text-right font-medium">A Δ</th>
                      <th className="pb-2 text-right font-medium">B Δ</th>
                    </tr>
                  </thead>
                  <tbody>
                    {countryChart.map((row) => (
                      <tr key={row.label} className="border-t border-slate-800/70">
                        <td className="py-1.5 pr-2 text-slate-200">{row.label}</td>
                        <td className={`py-1.5 pr-2 text-right font-mono ${deltaClass(row.left)}`}>
                          {row.left ? signed(row.left) : '—'}
                        </td>
                        <td className={`py-1.5 text-right font-mono ${deltaClass(row.right)}`}>
                          {row.right ? signed(row.right) : '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <p className="mt-2 text-[10px] text-slate-500">
                Path counts: A {left.result.affected_paths.length} · B{' '}
                {right.result.affected_paths.length}
                {' · '}
                Top path end-impact A{' '}
                {pct(Math.max(0, ...left.result.affected_paths.map((p) => p.end_impact), 0))} · B{' '}
                {pct(Math.max(0, ...right.result.affected_paths.map((p) => p.end_impact), 0))}
              </p>
            </Panel>
          )}
        </>
      )}
    </div>
  )
}

function ScenarioIdentityCard({
  scenario,
  tone,
}: {
  scenario: SavedShockScenario
  tone: 'A' | 'B'
}) {
  return (
    <div className="panel px-3 py-3">
      <div className="flex items-center justify-between gap-2">
        <span
          className={`badge ${
            tone === 'A'
              ? 'border-cyan-500/40 bg-cyan-500/10 text-cyan-200'
              : 'border-amber-500/40 bg-amber-500/10 text-amber-200'
          }`}
        >
          Scenario {tone}
        </span>
        <span className="font-mono text-[10px] text-slate-600">
          {new Date(scenario.savedAt).toLocaleString()}
        </span>
      </div>
      <div className="mt-2 text-sm font-medium text-slate-100">{scenario.title}</div>
      <div className="mt-1 font-mono text-[11px] text-slate-500">
        {scenario.source} → {scenario.commodity} · {scenario.shock_type} · {scenario.drop}% · d
        {scenario.depth}
      </div>
    </div>
  )
}
