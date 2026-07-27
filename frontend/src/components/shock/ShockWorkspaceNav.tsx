import type { ShockWorkspaceTab } from '../../types/savedScenario'

const TABS: Array<{ id: ShockWorkspaceTab; label: string }> = [
  { id: 'setup', label: 'Scenario Setup' },
  { id: 'results', label: 'Results' },
  { id: 'comparison', label: 'Comparison' },
]

export function ShockWorkspaceNav({
  active,
  onChange,
}: {
  active: ShockWorkspaceTab
  onChange: (tab: ShockWorkspaceTab) => void
}) {
  return (
    <div className="flex flex-wrap gap-1 rounded-md border border-slate-800/80 bg-slate-950/50 p-1">
      {TABS.map((tab) => {
        const on = active === tab.id
        return (
          <button
            key={tab.id}
            type="button"
            onClick={() => onChange(tab.id)}
            className={`rounded px-3 py-1.5 text-xs font-medium transition sm:text-sm ${
              on
                ? 'bg-cyan-500/15 text-cyan-100 ring-1 ring-cyan-500/40'
                : 'text-slate-400 hover:bg-slate-900/80 hover:text-slate-200'
            }`}
          >
            {tab.label}
          </button>
        )
      })}
    </div>
  )
}
