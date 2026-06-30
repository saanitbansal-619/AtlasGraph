import type { ShockRequest } from '../types/api'
import { Panel, Spinner } from './ui'

const SHOCK_TYPES = [
  'export_collapse',
  'supply_cut',
  'price_spike',
  'route_disruption',
]

export interface ShockForm {
  source: string
  commodity: string
  shock_type: string
  drop: number
  depth: number
  explain: boolean
}

export function ShockSimulator({
  form,
  setForm,
  onRun,
  running,
}: {
  form: ShockForm
  setForm: (next: ShockForm) => void
  onRun: () => void
  running: boolean
}) {
  const update = <K extends keyof ShockForm>(key: K, value: ShockForm[K]) =>
    setForm({ ...form, [key]: value })

  const canRun = form.source.trim() !== '' && form.commodity.trim() !== '' && !running

  return (
    <Panel title="Shock Simulator">
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault()
          if (canRun) onRun()
        }}
      >
        <div className="grid grid-cols-2 gap-3">
          <Field label="Source">
            <input
              className="field"
              value={form.source}
              onChange={(e) => update('source', e.target.value)}
              placeholder="Taiwan"
            />
          </Field>
          <Field label="Commodity">
            <input
              className="field"
              value={form.commodity}
              onChange={(e) => update('commodity', e.target.value)}
              placeholder="semiconductors"
            />
          </Field>
        </div>

        <Field label="Shock type">
          <select
            className="field"
            value={form.shock_type}
            onChange={(e) => update('shock_type', e.target.value)}
          >
            {SHOCK_TYPES.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label={`Drop  ·  ${form.drop}%`}>
            <input
              type="range"
              min={0}
              max={100}
              step={5}
              value={form.drop}
              onChange={(e) => update('drop', Number(e.target.value))}
              className="mt-2 w-full accent-cyan-400"
            />
          </Field>
          <Field label="Depth (hops)">
            <input
              type="number"
              min={1}
              max={8}
              className="field"
              value={form.depth}
              onChange={(e) => update('depth', Math.max(1, Number(e.target.value) || 1))}
            />
          </Field>
        </div>

        <label className="flex cursor-pointer items-center gap-2 pt-1 text-sm text-slate-300">
          <input
            type="checkbox"
            checked={form.explain}
            onChange={(e) => update('explain', e.target.checked)}
            className="h-4 w-4 rounded border-slate-600 bg-slate-950 accent-cyan-400"
          />
          Explain (include blocked edges &amp; propagation rules)
        </label>

        <button type="submit" className="btn-primary" disabled={!canRun}>
          {running ? (
            <>
              <Spinner className="h-4 w-4" />
              Running simulation…
            </>
          ) : (
            'Run Shock Simulation'
          )}
        </button>
      </form>
    </Panel>
  )
}

export function toRequest(form: ShockForm): ShockRequest {
  return {
    source: form.source.trim(),
    commodity: form.commodity.trim(),
    shock_type: form.shock_type,
    drop: form.drop,
    depth: form.depth,
    explain: form.explain,
  }
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="label mb-1.5">{label}</div>
      {children}
    </div>
  )
}
