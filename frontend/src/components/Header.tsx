import type { HealthResponse } from '../types/api'
import { API_BASE } from '../lib/api'
import { Dot, Spinner } from './ui'

type HealthStatus = 'loading' | 'online' | 'offline'

function statusOf(health: HealthResponse | null, error: boolean, loading: boolean): HealthStatus {
  if (loading && !health && !error) return 'loading'
  if (health && !error) return 'online'
  return 'offline'
}

function GfipLogo() {
  return (
    <div
      className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-cyan-500/35 bg-slate-900/80 shadow-[inset_0_0_14px_rgba(34,211,238,0.07)]"
      aria-hidden
    >
      <svg viewBox="0 0 32 32" className="h-6 w-6" fill="none">
        <circle cx="16" cy="16" r="11" stroke="rgba(34,211,238,0.45)" strokeWidth="0.75" />
        <ellipse cx="16" cy="16" rx="5" ry="11" stroke="rgba(34,211,238,0.22)" strokeWidth="0.5" />
        <path d="M5 16h22" stroke="rgba(34,211,238,0.18)" strokeWidth="0.5" />
        <path d="M16 5v22" stroke="rgba(34,211,238,0.12)" strokeWidth="0.5" />
        <circle cx="16" cy="10.5" r="2" fill="rgba(34,211,238,0.72)" />
        <circle cx="22.5" cy="18" r="1.5" fill="rgba(56,189,248,0.55)" />
        <circle cx="10.5" cy="19" r="1.25" fill="rgba(129,140,248,0.48)" />
      </svg>
    </div>
  )
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
      <div className="dashboard-shell flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
        <div className="flex min-w-0 items-start gap-3">
          <GfipLogo />
          <div className="min-w-0 leading-tight">
            <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
              <span className="font-mono text-[11px] font-bold tracking-[0.22em] text-cyan-400/90">
                GFIP
              </span>
              <h1 className="text-sm font-semibold tracking-tight text-slate-50 sm:text-[15px]">
                Global Fragility Intelligence Platform
              </h1>
            </div>
            <p className="mt-1 max-w-xl text-[11px] leading-snug text-slate-400 sm:text-xs">
              Strategic supply-chain and infrastructure risk intelligence
            </p>
            <p className="mt-1.5 text-[10px] uppercase tracking-[0.14em] text-slate-600">
              Powered by <span className="font-medium text-slate-500">AtlasGraph</span>
            </p>
          </div>
        </div>

        <div className="shrink-0 self-start sm:self-center">
          <ApiStatusBadge status={status} version={health?.version} />
        </div>
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
      <div className="flex flex-col items-start gap-0.5 sm:items-end">
        <div className="badge border-rose-500/40 bg-rose-500/10 text-rose-300">
          <Dot className="bg-rose-400" />
          API Offline
        </div>
        <span className="font-mono text-[10px] text-slate-600">{API_BASE}</span>
      </div>
    )
  }
  return (
    <div className="flex flex-col items-start gap-0.5 sm:items-end">
      <div className="badge border-emerald-500/40 bg-emerald-500/10 text-emerald-300">
        <Dot className="bg-emerald-400 shadow-[0_0_8px_1px_rgba(52,211,153,0.6)]" />
        API Online
      </div>
      <span className="font-mono text-[10px] text-slate-600">
        {API_BASE}
        {version ? ` · ${version}` : ''}
      </span>
    </div>
  )
}
