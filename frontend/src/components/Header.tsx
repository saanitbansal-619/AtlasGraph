import type { HealthResponse } from '../types/api'
import { API_BASE } from '../lib/api'
import { Dot, Spinner } from './ui'

type HealthStatus = 'loading' | 'online' | 'offline'

function statusOf(health: HealthResponse | null, error: boolean, loading: boolean): HealthStatus {
  if (loading && !health && !error) return 'loading'
  if (health && !error) return 'online'
  return 'offline'
}

export function Header({
  health,
  error,
  loading,
}: {
  health: HealthResponse | null
  error: boolean
  loading: boolean
}) {
  const status = statusOf(health, error, loading)

  return (
    <header className="sticky top-0 z-20 border-b border-slate-800/80 bg-slate-950/80 backdrop-blur">
      <div className="dashboard-shell flex items-center justify-between gap-4 py-3">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded border border-cyan-500/40 bg-cyan-500/10 font-mono text-sm font-bold text-cyan-300">
            AG
          </div>
          <div className="leading-tight">
            <h1 className="text-sm font-semibold tracking-tight text-slate-50 sm:text-base">
              Global Fragility Intelligence Platform
            </h1>
            <p className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
              Powered by AtlasGraph
            </p>
          </div>
        </div>

        <ApiStatusBadge status={status} version={health?.version} />
      </div>
    </header>
  )
}

function ApiStatusBadge({ status, version }: { status: HealthStatus; version?: string }) {
  if (status === 'loading') {
    return (
      <div className="badge border-amber-500/40 bg-amber-500/10 text-amber-300">
        <Spinner className="h-3 w-3" />
        Connecting
      </div>
    )
  }
  if (status === 'offline') {
    return (
      <div className="flex flex-col items-end gap-0.5">
        <div className="badge border-rose-500/40 bg-rose-500/10 text-rose-300">
          <Dot className="bg-rose-400" />
          API Offline
        </div>
        <span className="hidden font-mono text-[10px] text-slate-600 sm:block">{API_BASE}</span>
      </div>
    )
  }
  return (
    <div className="flex flex-col items-end gap-0.5">
      <div className="badge border-emerald-500/40 bg-emerald-500/10 text-emerald-300">
        <Dot className="bg-emerald-400 shadow-[0_0_8px_1px_rgba(52,211,153,0.6)]" />
        API Online
      </div>
      <span className="hidden font-mono text-[10px] text-slate-600 sm:block">
        {API_BASE}
        {version ? ` · ${version}` : ''}
      </span>
    </div>
  )
}
