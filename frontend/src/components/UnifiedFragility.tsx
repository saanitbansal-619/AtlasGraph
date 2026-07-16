import { useState } from 'react'
import type { FragilityComponent, FragilitySummaryResponse } from '../types/api'
import { fixed, riskBadgeClass } from '../lib/format'
import { HorizontalBarChartCard } from './charts/HorizontalBarChartCard'
import { Panel, Spinner } from './ui'
import { InlineError } from './States'

export function UnifiedFragility({
  summary,
  loading,
  error,
}: {
  summary: FragilitySummaryResponse | null
  loading: boolean
  error?: { message: string; hint?: string } | null
}) {
  const [showDetails, setShowDetails] = useState(false)

  if (error && !summary) {
    return (
      <Panel title="Unified Fragility">
        <InlineError message={error.message} hint={error.hint} />
      </Panel>
    )
  }

  if (!summary) {
    return (
      <Panel title="Unified Fragility">
        <div className="flex items-center gap-3 py-6 text-sm text-slate-400">
          {loading && <Spinner />}
          Loading unified fragility scores…
        </div>
      </Panel>
    )
  }

  const macroSourceActive = summary.data_sources?.includes('World Bank Macro') ?? false

  return (
    <Panel title="Unified Fragility" right={<span className="text-[11px] text-slate-500">top 5 by score</span>}>
      <div className={`space-y-3 ${loading ? 'opacity-70' : ''}`}>
        <p className="text-xs text-slate-400">
          Observed trade, event-risk, commodity-price{macroSourceActive ? ', and World Bank macro' : ''}{' '}
          signals combine with baseline graph structure to produce model-derived fragility scores.
          {macroSourceActive && (
            <span className="text-slate-500"> Country fragility includes World Bank macro indicators.</span>
          )}
        </p>

        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          <HorizontalBarChartCard
            title="Top Fragile Countries"
            subtitle="fragility score"
            valueLabel="Fragility score"
            valueDigits={1}
            data={summary.countries.map((c) => ({
              label: c.country_name,
              value: c.score,
              meta: { risk: c.risk_level },
            }))}
            emptyLabel="No country fragility scores available."
            topN={5}
            showValueLabels
            valueSuffix=" score"
          />
          <HorizontalBarChartCard
            title="Top Fragile Commodities"
            subtitle="fragility score"
            valueLabel="Fragility score"
            valueDigits={1}
            data={summary.commodities.map((c) => ({
              label: c.commodity_name,
              value: c.score,
              meta: {
                risk: c.risk_level,
                drivers: (c.top_drivers ?? []).slice(0, 3).join(', '),
              },
            }))}
            emptyLabel="No commodity fragility scores available."
            topN={5}
            showValueLabels
            valueSuffix=" score"
          />
        </div>

        <button
          type="button"
          onClick={() => setShowDetails((v) => !v)}
          className="rounded border border-slate-700/60 bg-slate-950/40 px-3 py-1.5 text-[11px] font-medium text-slate-400 transition hover:border-slate-600 hover:text-slate-300"
        >
          {showDetails ? 'Hide detailed score breakdown' : 'Show detailed score breakdown'}
        </button>

        {showDetails && (
          <div className="space-y-3 border-t border-slate-800/60 pt-3">
            {(summary.trade_concentration_source || summary.trade_concentration_note) && (
              <p className="text-[11px] text-slate-500">
                {summary.trade_concentration_source && (
                  <span>Trade concentration source: {summary.trade_concentration_source}</span>
                )}
                {summary.trade_concentration_note && (
                  <span>
                    {summary.trade_concentration_source ? ' · ' : ''}
                    {summary.trade_concentration_note}
                  </span>
                )}
              </p>
            )}
            <div className="text-[10px] font-semibold uppercase tracking-wider text-slate-500">
              Score Breakdown
            </div>
            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
              <FragilityTable
                label="Countries"
                macroSourceActive={macroSourceActive}
                rows={summary.countries.map((c) => ({
                  name: c.country_name,
                  score: c.score,
                  risk: c.risk_level,
                  drivers: c.top_drivers,
                  components: c.components,
                }))}
                empty="No country fragility scores available."
              />
              <FragilityTable
                label="Commodities"
                rows={summary.commodities.map((c) => ({
                  name: c.commodity_name,
                  score: c.score,
                  risk: c.risk_level,
                  drivers: c.top_drivers,
                  components: c.components,
                }))}
                empty="No commodity fragility scores available."
              />
            </div>
          </div>
        )}
      </div>
    </Panel>
  )
}

function FragilityTable({
  label,
  rows,
  empty,
  macroSourceActive = false,
}: {
  label: string
  rows: Array<{
    name: string
    score: number
    risk: string
    drivers: string[]
    components?: FragilityComponent[]
  }>
  empty: string
  macroSourceActive?: boolean
}) {
  if (rows.length === 0) {
    return <p className="text-sm text-slate-500">{empty}</p>
  }

  return (
    <div>
      <div className="label mb-2">{label}</div>
      <div className="overflow-hidden rounded border border-slate-800">
        <table className="w-full border-collapse text-sm">
          <thead className="bg-slate-900/80">
            <tr className="border-b border-slate-800">
              <th className="th text-left">{label.slice(0, -1)}</th>
              <th className="th text-right">Score</th>
              <th className="th text-center">Risk</th>
              <th className="th text-left">Top drivers</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.name} className="border-b border-slate-800/60 hover:bg-slate-800/30">
                <td className="td align-top">
                  <div className="font-medium text-slate-100">{row.name}</div>
                  <ComponentProvenance
                    components={row.components}
                    macroSourceActive={macroSourceActive}
                  />
                </td>
                <td className="td align-top text-right font-mono tabular-nums text-slate-200">
                  {fixed(row.score, 1)}
                </td>
                <td className="td align-top text-center">
                  <span className={`badge ${riskBadgeClass(row.risk)}`}>{row.risk}</span>
                </td>
                <td className="td align-top text-xs text-slate-400">{row.drivers.join(', ') || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function ComponentProvenance({
  components,
  macroSourceActive = false,
}: {
  components?: FragilityComponent[]
  macroSourceActive?: boolean
}) {
  if (!components?.length) return null

  const labeled = components.filter(
    (c) =>
      c.available &&
      (c.source ||
        c.note ||
        (macroSourceActive && c.key === 'macro_exposure_score')),
  )
  if (labeled.length === 0) return null

  return (
    <div className="mt-1.5 space-y-0.5">
      {labeled.map((c) => {
        const source =
          c.source ||
          (macroSourceActive && c.key === 'macro_exposure_score'
            ? 'World Bank Macro'
            : undefined)
        return (
          <div key={c.key} className="text-[10px] leading-snug text-slate-500">
            <span className="text-slate-400">{c.name}</span>
            {source && <span> · source: {source}</span>}
            {c.note && <span> · note: {c.note}</span>}
          </div>
        )
      })}
    </div>
  )
}
