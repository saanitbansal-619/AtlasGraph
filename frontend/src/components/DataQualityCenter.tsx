import type { DBHealthResponse, DBSummaryResponse } from '../types/api'
import { compactInt } from '../lib/format'
import { InlineError } from './States'
import { Panel } from './ui'

const DESCRIPTION =
  'GFIP stores normalized public trade, macroeconomic, event-risk, and commodity-price datasets in PostgreSQL to support reproducible analytics, scenario persistence, and data-quality monitoring.'

function StatusBadge({
  label,
  state,
}: {
  label: string
  state: 'active' | 'inactive' | 'error'
}) {
  const style =
    state === 'active'
      ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300'
      : state === 'error'
        ? 'border-rose-500/40 bg-rose-500/10 text-rose-300'
        : 'border-slate-600/60 bg-slate-800/40 text-slate-500'
  const dot =
    state === 'active' ? 'bg-emerald-400' : state === 'error' ? 'bg-rose-400' : 'bg-slate-600'
  return (
    <span className={`inline-flex items-center gap-1.5 rounded border px-2 py-0.5 text-[11px] ${style}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${dot}`} aria-hidden />
      {label}
    </span>
  )
}

function LoadingMetrics() {
  return (
    <div className="grid grid-cols-2 gap-2 md:grid-cols-4 xl:grid-cols-7">
      {Array.from({ length: 7 }, (_, index) => (
        <div key={index} className="rounded border border-slate-800 bg-slate-900/30 px-3 py-2.5">
          <div className="h-3 w-20 animate-pulse rounded bg-slate-800" />
          <div className="mt-2 h-6 w-12 animate-pulse rounded bg-slate-800" />
        </div>
      ))}
    </div>
  )
}

export function DataQualityCenter({
  health,
  summary,
  loading,
  error,
}: {
  health: DBHealthResponse | null
  summary: DBSummaryResponse | null
  loading: boolean
  error?: { message: string; hint?: string } | null
}) {
  const enabled = health?.enabled ?? false
  const healthy = enabled && health?.status === 'ok'
  const metrics = summary
    ? [
        ['Trade flows loaded', summary.trade_flows],
        ['Event-risk signals', summary.event_risk_signals],
        ['Macro scores', summary.macro_scores],
        ['Commodity price rows', summary.commodity_prices],
        ['Dependency edges', summary.dependency_edges],
        ['Scenario runs', summary.scenario_runs],
        ['Data quality checks', summary.data_quality_checks],
      ]
    : []
  const hasRows = metrics.some(([, value]) => Number(value) > 0)

  return (
    <Panel
      title="Data Quality Center"
      right={
        <div className="flex flex-wrap justify-end gap-1.5">
          <StatusBadge
            label={`PostgreSQL ${enabled ? 'enabled' : 'disabled'}`}
            state={enabled ? 'active' : 'inactive'}
          />
          <StatusBadge
            label={`DB status ${health?.status ?? (error ? 'error' : 'unknown')}`}
            state={error || health?.status === 'error' ? 'error' : healthy ? 'active' : 'inactive'}
          />
          <StatusBadge
            label="SQL-backed analytics active"
            state={healthy ? 'active' : 'inactive'}
          />
        </div>
      }
    >
      <p className="mb-3 text-xs leading-relaxed text-slate-400">{DESCRIPTION}</p>

      {loading && !health && <LoadingMetrics />}

      {error && !loading && <InlineError message={error.message} hint={error.hint} />}

      {!loading && !error && health && !health.enabled && (
        <div className="rounded border border-dashed border-slate-700 bg-slate-900/30 px-4 py-5 text-sm text-slate-400">
          PostgreSQL analytics is optional. The app is currently using file-backed analytics.
        </div>
      )}

      {!loading && !error && healthy && !summary && (
        <div className="rounded border border-dashed border-slate-700 px-4 py-5 text-sm text-slate-500">
          PostgreSQL is connected, but no analytics summary is available.
        </div>
      )}

      {summary && (
        <>
          <div className={`grid grid-cols-2 gap-2 md:grid-cols-4 xl:grid-cols-7 ${loading ? 'opacity-60' : ''}`}>
            {metrics.map(([label, value]) => (
              <div key={String(label)} className="rounded border border-slate-800 bg-slate-900/30 px-3 py-2.5">
                <div className="text-[10px] uppercase tracking-wide text-slate-500">{label}</div>
                <div className="mt-1 font-mono text-xl font-semibold text-cyan-200">
                  {compactInt(Number(value))}
                </div>
              </div>
            ))}
          </div>
          {!hasRows && (
            <p className="mt-3 text-xs text-slate-500">
              PostgreSQL is connected, but the analytics tables are empty. Run the database load command to populate them.
            </p>
          )}
        </>
      )}
    </Panel>
  )
}
