import type { LucideIcon } from 'lucide-react'
import {
  Activity,
  BarChart3,
  Database,
  History,
  LayoutDashboard,
  Users,
  Zap,
} from 'lucide-react'
import type { AppTab } from '../../types/navigation'

const NAV_ITEMS: Array<{ id: AppTab; label: string; icon: LucideIcon }> = [
  { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { id: 'shock', label: 'Shock Simulation', icon: Zap },
  { id: 'client', label: 'Client Analytics', icon: Users },
  { id: 'data-ops', label: 'Data Operations', icon: Database },
  { id: 'analytics', label: 'Analytics Explorer', icon: BarChart3 },
  { id: 'history', label: 'History', icon: History },
]

function GfipMark() {
  return (
    <div
      className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-cyan-500/30 bg-gradient-to-br from-slate-900 via-slate-950 to-slate-900"
      aria-hidden
    >
      <Activity className="h-5 w-5 text-cyan-300" strokeWidth={1.75} />
    </div>
  )
}

export function Sidebar({
  activeTab,
  onSelect,
  mobileOpen,
  onCloseMobile,
}: {
  activeTab: AppTab
  onSelect: (tab: AppTab) => void
  mobileOpen: boolean
  onCloseMobile: () => void
}) {
  return (
    <>
      {mobileOpen && (
        <button
          type="button"
          className="fixed inset-0 z-30 bg-slate-950/70 lg:hidden"
          aria-label="Close navigation"
          onClick={onCloseMobile}
        />
      )}

      <aside
        className={`fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-slate-800/80 bg-slate-950/95 backdrop-blur transition-transform lg:static lg:translate-x-0 ${
          mobileOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        <div className="border-b border-slate-800/80 px-4 py-4">
          <div className="flex items-start gap-3">
            <GfipMark />
            <div className="min-w-0">
              <div className="text-sm font-semibold tracking-tight text-slate-50">GFIP</div>
              <p className="mt-0.5 text-[11px] leading-snug text-slate-500">
                Global Fragility Intelligence Platform
              </p>
            </div>
          </div>
        </div>

        <nav className="flex-1 space-y-1 overflow-y-auto px-2 py-3" aria-label="Primary">
          {NAV_ITEMS.map(({ id, label, icon: Icon }) => {
            const active = activeTab === id
            return (
              <button
                key={id}
                type="button"
                onClick={() => {
                  onSelect(id)
                  onCloseMobile()
                }}
                className={`flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-left text-sm transition ${
                  active
                    ? 'border border-cyan-500/30 bg-cyan-500/10 text-cyan-100'
                    : 'border border-transparent text-slate-400 hover:bg-slate-900/70 hover:text-slate-200'
                }`}
              >
                <Icon className="h-4 w-4 shrink-0" strokeWidth={1.75} />
                <span className="truncate">{label}</span>
              </button>
            )
          })}
        </nav>

        <div className="border-t border-slate-800/80 px-4 py-3 text-[10px] uppercase tracking-[0.12em] text-slate-600">
          Powered by AtlasGraph
        </div>
      </aside>
    </>
  )
}
