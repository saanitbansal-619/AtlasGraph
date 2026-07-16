import type { FragilitySummaryResponse, GraphSummaryResponse } from '../types/api'
import { Panel } from './ui'

function StatusBadge({ label, active }: { label: string; active: boolean }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded border px-2 py-0.5 text-[11px] font-medium ${
        active
          ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300'
          : 'border-slate-600/60 bg-slate-800/40 text-slate-500'
      }`}
      title={active ? 'Active in current session' : 'Not loaded for this session'}
    >
      <span
        className={`h-1.5 w-1.5 rounded-full ${active ? 'bg-emerald-400' : 'bg-slate-600'}`}
        aria-hidden
      />
      {label}
      <span className="text-[10px] opacity-70">{active ? 'active' : 'inactive'}</span>
    </span>
  )
}

export function DataSourcesCard({
  summary,
  fragility,
  loading,
  compact = false,
}: {
  summary: GraphSummaryResponse | null
  fragility?: FragilitySummaryResponse | null
  loading: boolean
  compact?: boolean
}) {
  const tradeActive = fragility?.real_trade_edges_used ?? summary?.real_trade_edges_used ?? false
  const eventActive = fragility?.real_event_risk_used ?? summary?.real_event_risk_used ?? false
  const priceActive = fragility?.real_price_stress_used ?? summary?.real_price_stress_used ?? false
  const macroActive =
    fragility?.data_sources?.includes('World Bank Macro') ??
    summary?.data_sources?.includes('World Bank Macro') ??
    false
  const fusionActive =
    fragility?.fusion_enabled ??
    summary?.fusion_enabled ??
    (tradeActive || eventActive || priceActive || macroActive)

  const tradeNote = fragility?.trade_concentration_note
  const tradeSource = fragility?.trade_concentration_source
  const showMeta =
    !!tradeSource ||
    !!tradeNote ||
    !!(summary?.fusion_enabled && summary.real_trade_edges != null && summary.real_trade_edges > 0)

  return (
    <Panel
      title="Data Fusion Status"
      className={compact ? 'h-full' : ''}
      noPad={compact}
      dense={compact}
      right={
        fusionActive ? (
          <span className="badge border-cyan-500/40 bg-cyan-500/10 text-cyan-300">Fusion active</span>
        ) : (
          <span className="badge border-slate-600/60 bg-slate-800/40 text-slate-400">
            Baseline graph only
          </span>
        )
      }
    >
      <div
        className={`${
          compact
            ? 'flex h-full flex-col justify-center gap-2 px-3 py-2.5'
            : 'flex flex-wrap items-center gap-x-6 gap-y-3 p-4'
        } ${loading ? 'opacity-70' : ''}`}
      >
        <p className="text-xs leading-snug text-slate-400">
          GFIP combines observed trade, event-risk, commodity-price, and macro data with a baseline
          dependency graph to estimate supply-chain exposure.
        </p>

        <div className="flex flex-wrap items-center gap-1.5">
          <StatusBadge label="Baseline dependency graph" active />
          <StatusBadge label="UN Comtrade" active={tradeActive} />
          <StatusBadge label="GDELT" active={eventActive} />
          <StatusBadge label="World Bank Pink Sheet" active={priceActive} />
          <StatusBadge label="World Bank Macro" active={macroActive} />
        </div>

        <p className="text-[11px] leading-snug text-slate-500">
          Observed data: UN Comtrade trade flows, GDELT event-risk signals, World Bank commodity
          prices{macroActive ? ', and World Bank macro indicators' : ''}.
          {macroActive && ' Country fragility includes World Bank macro indicators.'} Model-derived
          outputs: fragility scores, shock propagation, impact deltas, and graph centrality.
        </p>

        {showMeta && (
          <p className="text-[11px] leading-snug text-slate-500">
            {tradeSource && (
              <span>
                Trade concentration: {tradeSource}
                {tradeNote ? ` · ${tradeNote}` : ''}
              </span>
            )}
            {summary?.fusion_enabled &&
              summary.real_trade_edges != null &&
              summary.real_trade_edges > 0 && (
                <span className={tradeSource ? ' mt-0.5 block' : ''}>
                  Graph: {summary.fused_entities} entities · {summary.fused_dependencies} deps ·{' '}
                  {summary.real_trade_edges} real trade edges
                </span>
              )}
          </p>
        )}
      </div>
    </Panel>
  )
}
