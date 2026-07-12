import type { GraphSummaryResponse } from '../types/api'
import { Panel } from './ui'

function sourceLabel(name: string, active: boolean) {
  return (
    <span
      className={`badge ${active ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300' : 'border-slate-600/60 bg-slate-800/40 text-slate-400'}`}
    >
      {name}
    </span>
  )
}

export function DataSourcesCard({
  summary,
  loading,
}: {
  summary: GraphSummaryResponse | null
  loading: boolean
}) {
  const sources = summary?.data_sources ?? ['Strategic demo graph']
  const has = (needle: string) => sources.some((s) => s.toLowerCase().includes(needle.toLowerCase()))

  return (
    <Panel
      title="Data Fusion / Data Sources"
      right={
        summary?.fusion_enabled ? (
          <span className="badge border-cyan-500/40 bg-cyan-500/10 text-cyan-300">Fusion active</span>
        ) : (
          <span className="badge border-slate-600/60 bg-slate-800/40 text-slate-400">Demo graph only</span>
        )
      }
    >
      <div className={`space-y-3 ${loading ? 'opacity-70' : ''}`}>
        <p className="text-xs text-slate-400">
          The strategic demo graph is augmented with local processed real-data panels when available.
          Not all graph edges are real yet.
        </p>
        <div className="flex flex-wrap gap-2">
          {sourceLabel('Base graph: Strategic demo graph', has('strategic'))}
          {sourceLabel('Trade: UN Comtrade', summary?.real_trade_edges_used ?? has('comtrade'))}
          {sourceLabel('Commodity prices: World Bank Pink Sheet', summary?.real_price_stress_used ?? has('world bank'))}
          {sourceLabel('Event risk: GDELT', summary?.real_event_risk_used ?? has('gdelt'))}
        </div>
        {summary && summary.fusion_enabled && (
          <p className="text-[11px] text-slate-500">
            Fused graph: {summary.fused_entities} entities · {summary.fused_dependencies} dependencies
            {summary.real_trade_edges != null && summary.real_trade_edges > 0 && ` · ${summary.real_trade_edges} real trade edges`}
          </p>
        )}
      </div>
    </Panel>
  )
}
