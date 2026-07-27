import { Menu } from 'lucide-react'
import type { HealthResponse } from '../../types/api'
import { API_BASE } from '../../lib/api'
import { Dot, Spinner } from '../ui'

type HealthStatus = 'loading' | 'online' | 'offline'

function statusOf(health: HealthResponse | null, error: boolean, loading: boolean): HealthStatus {
  if (loading && !health && !error) return 'loading'
  if (health && !error) return 'online'
  return 'offline'
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
        <span className="font-mono text-[10px] text-slate-600">{API_BASE}</span>
      </div>
    )
  }
  return (
    <div className="flex flex-col items-end gap-0.5">
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

export function Header({
  title,
  description,
  health,
  error,
  loading,
  onOpenSidebar,
}: {
  title: string
  description: string
  health: HealthResponse | null
  error: boolean
  loading: boolean
  onOpenSidebar: () => void
}) {
  const status = statusOf(health, error, loading)

  return (
    <header className="sticky top-0 z-20 border-b border-slate-800/80 bg-slate-950/85 backdrop-blur">
      <div className="flex items-start justify-between gap-3 px-4 py-3 sm:px-5 lg:px-6">
        <div className="flex min-w-0 items-start gap-3">
          <button
            type="button"
            className="mt-0.5 inline-flex rounded border border-slate-700/80 bg-slate-900/60 p-1.5 text-slate-300 lg:hidden"
            onClick={onOpenSidebar}
            aria-label="Open navigation"
          >
            <Menu className="h-4 w-4" />
          </button>
          <div className="min-w-0">
            <h1 className="text-base font-semibold tracking-tight text-slate-50 sm:text-lg">
              {title}
            </h1>
            <p className="mt-1 max-w-3xl text-xs leading-relaxed text-slate-400">{description}</p>
          </div>
        </div>
        <div className="shrink-0">
          <ApiStatusBadge status={status} version={health?.version} />
        </div>
      </div>
    </header>
  )
}
