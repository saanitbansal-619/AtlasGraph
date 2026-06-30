import type { Scenario } from '../types/api'
import { Panel } from './ui'
import { Spinner } from './ui'

export function ScenarioSelect({
  scenarios,
  selectedId,
  onSelect,
  loading,
}: {
  scenarios: Scenario[]
  selectedId: string
  onSelect: (id: string) => void
  loading: boolean
}) {
  const selected = scenarios.find((s) => s.id === selectedId)

  return (
    <Panel title="Scenario Preset" right={loading ? <Spinner className="h-3.5 w-3.5" /> : undefined}>
      <label className="label" htmlFor="scenario">
        Load a saved shock preset
      </label>
      <select
        id="scenario"
        className="field mt-1.5"
        value={selectedId}
        onChange={(e) => onSelect(e.target.value)}
        disabled={loading || scenarios.length === 0}
      >
        {scenarios.length === 0 && <option value="">No scenarios available</option>}
        {scenarios.map((s) => (
          <option key={s.id} value={s.id}>
            {s.name || s.id}
          </option>
        ))}
      </select>

      {selected?.description && (
        <p className="mt-2 text-xs leading-relaxed text-slate-400">{selected.description}</p>
      )}
      {selected && (
        <div className="mt-3 flex flex-wrap gap-1.5 font-mono text-[11px] text-slate-400">
          <Tag>{selected.source}</Tag>
          <Tag>{selected.commodity}</Tag>
          <Tag>{selected.shock_type}</Tag>
          <Tag>{selected.shock_percent}% drop</Tag>
          <Tag>depth {selected.depth}</Tag>
        </div>
      )}
    </Panel>
  )
}

function Tag({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded border border-slate-700/70 bg-slate-800/40 px-1.5 py-0.5">{children}</span>
  )
}
