import type { CommodityStressResponse } from '../types/api'
import { fixed, riskBadgeClass } from '../lib/format'
import { Panel, Spinner } from './ui'
import { InlineError } from './States'

export function CommodityStressPanel({
  data,
  loading,
  error,
}: {
  data: CommodityStressResponse | null
  loading: boolean
  error?: { message: string; hint?: string } | null
}) {
  if (error && !data) {
    return (
      <Panel title="Commodity Price Stress">
        <InlineError message={error.message} hint={error.hint} />
      </Panel>
    )
  }

  const badge = data?.real_price_data ? (
    <span className="badge border-emerald-500/40 bg-emerald-500/10 text-emerald-300">
      Real price data
    </span>
  ) : (
    <span className="badge border-slate-600/60 bg-slate-800/40 text-slate-400">
      Sample price data
    </span>
  )

  return (
    <Panel
      title="Commodity Price Stress"
      right={
        <div className="flex items-center gap-2">
          {badge}
          {loading && <Spinner className="h-3 w-3" />}
        </div>
      }
      noPad
    >
      {!data ? (
        <div className="flex items-center gap-3 px-4 py-6 text-sm text-slate-400">
          {loading && <Spinner />}
          Loading commodity stress scores…
        </div>
      ) : data.scores.length === 0 ? (
        <div className="px-4 py-5 text-sm text-slate-500">No commodity stress scores available.</div>
      ) : (
        <div className={`${loading ? 'opacity-70' : ''}`}>
          <p className="border-b border-slate-800/60 px-4 py-2 text-[11px] text-slate-500">
            Source: {data.data_source || 'unknown'}
          </p>
          <div className="max-h-72 overflow-y-auto">
            <table className="w-full border-collapse text-sm">
              <thead className="sticky top-0 z-10 bg-slate-900/95 backdrop-blur">
                <tr className="border-b border-slate-800">
                  <th className="th text-left">Commodity</th>
                  <th className="th text-right">Latest</th>
                  <th className="th text-right">3M</th>
                  <th className="th text-right">Score</th>
                  <th className="th text-center">Risk</th>
                </tr>
              </thead>
              <tbody>
                {data.scores.slice(0, 8).map((s) => (
                  <tr
                    key={s.commodity_code}
                    className="border-b border-slate-800/60 hover:bg-slate-800/30"
                  >
                    <td className="td font-medium text-slate-100">{s.commodity_name}</td>
                    <td className="td text-right font-mono text-xs text-slate-300">
                      {fixed(s.latest_price_usd, 1)}
                    </td>
                    <td className="td text-right font-mono text-xs text-slate-400">
                      {s.change_3m_pct != null ? `${fixed(s.change_3m_pct, 1)}%` : 'n/a'}
                    </td>
                    <td className="td text-right font-mono text-xs font-semibold text-cyan-300">
                      {fixed(s.commodity_stress_score, 1)}
                    </td>
                    <td className="td text-center">
                      <span className={`badge ${riskBadgeClass(s.risk_level)}`}>{s.risk_level}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </Panel>
  )
}
