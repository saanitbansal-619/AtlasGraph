import type { FragilitySummaryResponse } from '../types/api'
import { fixed, riskBadgeClass } from '../lib/format'
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

  return (
    <Panel title="Unified Fragility" right={<span className="text-[11px] text-slate-500">top 5 by score</span>}>
      <div className={`grid grid-cols-1 gap-4 lg:grid-cols-2 ${loading ? 'opacity-70' : ''}`}>
        <FragilityTable
          label="Countries"
          rows={summary.countries.map((c) => ({
            name: c.country_name,
            score: c.score,
            risk: c.risk_level,
            drivers: c.top_drivers,
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
          }))}
          empty="No commodity fragility scores available."
        />
      </div>
    </Panel>
  )
}

function FragilityTable({
  label,
  rows,
  empty,
}: {
  label: string
  rows: Array<{ name: string; score: number; risk: string; drivers: string[] }>
  empty: string
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
                <td className="td font-medium text-slate-100">{row.name}</td>
                <td className="td text-right font-mono tabular-nums text-slate-200">{fixed(row.score, 1)}</td>
                <td className="td text-center">
                  <span className={`badge ${riskBadgeClass(row.risk)}`}>{row.risk}</span>
                </td>
                <td className="td text-xs text-slate-400">{row.drivers.join(', ') || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
