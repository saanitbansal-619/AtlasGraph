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
      className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg border border-cyan-500/30 bg-gradient-to-br from-slate-900 via-slate-950 to-slate-900 shadow-[inset_0_0_18px_rgba(34,211,238,0.08)]"
      title="GFIP"
    >
      <svg viewBox="0 0 48 48" className="h-10 w-10" fill="none" role="img" aria-label="GFIP">
        {/* Concentric rings only */}
        <circle cx="24" cy="24" r="19.5" stroke="rgba(34,211,238,0.32)" strokeWidth="1" />
        <circle cx="24" cy="24" r="13.25" stroke="rgba(34,211,238,0.16)" strokeWidth="0.75" />

        {/* Initials: G top · F left · I right · P bottom */}
        <text
          x="24"
          y="12.5"
          textAnchor="middle"
          dominantBaseline="central"
          fill="rgba(34,211,238,0.95)"
          fontFamily="ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
          fontSize="9"
          fontWeight="700"
        >
          G
        </text>
        <text
          x="12.5"
          y="24.35"
          textAnchor="middle"
          dominantBaseline="central"
          fill="rgba(56,189,248,0.94)"
          fontFamily="ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
          fontSize="9"
          fontWeight="700"
        >
          F
        </text>
        <text
          x="35.5"
          y="24.35"
          textAnchor="middle"
          dominantBaseline="central"
          fill="rgba(56,189,248,0.94)"
          fontFamily="ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
          fontSize="9"
          fontWeight="700"
        >
          I
        </text>
        <text
          x="24"
          y="36.1"
          textAnchor="middle"
          dominantBaseline="central"
          fill="rgba(34,211,238,0.95)"
          fontFamily="ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
          fontSize="9"
          fontWeight="700"
        >
          P
        </text>
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
        <div className="flex min-w-0 items-center gap-3.5">
          <GfipLogo />
          <div className="min-w-0 leading-tight">
            <h1 className="text-sm font-semibold tracking-tight text-slate-50 sm:text-[15px]">
              Global Fragility Intelligence Platform
            </h1>
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
