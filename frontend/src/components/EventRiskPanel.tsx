import type { EventRiskResponse } from '../types/api'
import { fixed, riskBadgeClass } from '../lib/format'
import { Panel, Spinner } from './ui'
import { InlineError } from './States'
import { AdaptiveRankingChart } from './charts/AdaptiveRankingChart'

export function EventRiskPanel({
  data,
  loading,
  error,
}: {
  data: EventRiskResponse | null
  loading: boolean
  error?: { message: string; hint?: string } | null
}) {
  if (error && !data) {
    return (
      <Panel title="Event Risk Signals">
        <InlineError message={error.message} hint={error.hint} />
      </Panel>
    )
  }

  const badge = data?.real_event_data ? (
    <span className="badge border-emerald-500/40 bg-emerald-500/10 text-emerald-300">
      Real event data
    </span>
  ) : (
    <span className="badge border-slate-600/60 bg-slate-800/40 text-slate-400">
      Demo event data
    </span>
  )

  const topScores = data?.scores.slice(0, 8) ?? []
  const chartData = data?.scores.slice(0, 5).map((s) => ({
    label: s.country_name,
    value: s.event_risk_score,
    meta: { risk: s.risk_level },
  })) ?? []

  return (
    <Panel
      title="Event Risk Signals"
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
          Loading event risk signals…
        </div>
      ) : topScores.length === 0 ? (
        <div className="px-4 py-5 text-sm text-slate-500">No event risk scores available.</div>
      ) : (
        <div className={`space-y-4 ${loading ? 'opacity-70' : ''}`}>
          <p className="border-b border-slate-800/60 px-4 py-2 text-[11px] text-slate-500">
            Source: {data.source || 'unknown'}
            {data.date_from && data.date_to ? ` · ${data.date_from} to ${data.date_to}` : ''}
          </p>

          <div className="grid grid-cols-1 gap-4 px-4 pb-4 xl:grid-cols-2">
            <div className="min-w-0">
              <div className="max-h-64 overflow-y-auto rounded border border-slate-800/60">
                <table className="w-full border-collapse text-sm">
                  <thead className="sticky top-0 z-10 bg-slate-900/95 backdrop-blur">
                    <tr className="border-b border-slate-800">
                      <th className="th text-left">Country</th>
                      <th className="th text-right">Score</th>
                      <th className="th text-center">Risk</th>
                      <th className="th text-right">Recent</th>
                      <th className="th text-right">Tone</th>
                    </tr>
                  </thead>
                  <tbody>
                    {topScores.map((s) => (
                      <tr
                        key={s.country_code || s.country_name}
                        className="border-b border-slate-800/60 hover:bg-slate-800/30"
                      >
                        <td className="td font-medium text-slate-100">{s.country_name}</td>
                        <td className="td text-right font-mono text-xs font-semibold text-cyan-300">
                          {fixed(s.event_risk_score, 1)}
                        </td>
                        <td className="td text-center">
                          <span className={`badge ${riskBadgeClass(s.risk_level)}`}>
                            {s.risk_level}
                          </span>
                        </td>
                        <td className="td text-right font-mono text-xs text-slate-400">
                          {s.recent_event_count ?? '—'}
                        </td>
                        <td className="td text-right font-mono text-xs text-slate-400">
                          {fixed(s.average_tone ?? s.avg_tone, 1)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {topScores[0]?.top_event_types && topScores[0].top_event_types.length > 0 && (
                <p className="mt-2 text-[11px] text-slate-500">
                  Top types (leader): {topScores[0].top_event_types.join(', ')}
                </p>
              )}
            </div>

            <AdaptiveRankingChart
              title="Top Event-Risk Countries"
              subtitle="0–100 score"
              valueLabel="Event risk"
              valueDigits={1}
              valueSuffix=""
              data={chartData}
              emptyLabel="No event risk scores to chart."
              topN={5}
              rankingLimit={5}
            />
          </div>
        </div>
      )}
    </Panel>
  )
}
