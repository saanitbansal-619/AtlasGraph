import type { ReactNode } from 'react'
import type { Scenario, ShockRequest } from '../types/api'
import {
  ASSUMPTION_NOTE,
  DURATION_OPTIONS,
  INVENTORY_OPTIONS,
  RECOVERY_OPTIONS,
  SUBSTITUTE_OPTIONS,
  type ScenarioAssumptions,
  type ScenarioMeta,
  type ShockMode,
} from '../types/scenario'
import { Panel, Spinner } from './ui'

const SHOCK_TYPES = ['export_collapse', 'supply_cut', 'price_spike', 'route_disruption']

const SHOCK_TYPE_DESC: Record<string, string> = {
  export_collapse: 'Producer exports fall, affecting importers and downstream sectors.',
  supply_cut: 'Source supply availability falls.',
  price_spike: 'Commodity price pressure rises across exposed sectors.',
  route_disruption: 'Logistics route disruption affects flows.',
}

const SOURCE_EXAMPLES = ['Taiwan', 'China', 'Saudi Arabia', 'United States', 'Japan']
const COMMODITY_EXAMPLES = [
  'semiconductors',
  'crude oil',
  'lithium batteries',
  'cobalt ores',
  'rare earths',
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
  mode,
  setMode,
  form,
  setForm,
  meta,
  setMeta,
  scenarios,
  selectedId,
  onSelectScenario,
  scenariosLoading,
  onRun,
  onReset,
  running,
}: {
  mode: ShockMode
  setMode: (m: ShockMode) => void
  form: ShockForm
  setForm: (next: ShockForm) => void
  meta: ScenarioMeta
  setMeta: (next: ScenarioMeta) => void
  scenarios: Scenario[]
  selectedId: string
  onSelectScenario: (id: string) => void
  scenariosLoading: boolean
  onRun: () => void
  onReset: () => void
  running: boolean
}) {
  const update = <K extends keyof ShockForm>(key: K, value: ShockForm[K]) =>
    setForm({ ...form, [key]: value })

  const updateAssumption = <K extends keyof ScenarioAssumptions>(
    key: K,
    value: ScenarioAssumptions[K],
  ) => setMeta({ ...meta, assumptions: { ...meta.assumptions, [key]: value } })

  const canRun = form.source.trim() !== '' && form.commodity.trim() !== '' && !running
  const selected = scenarios.find((s) => s.id === selectedId)

  return (
    <Panel title="Shock Simulator" right={<ModeToggle mode={mode} setMode={setMode} />}>
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault()
          if (canRun) onRun()
        }}
      >
        {mode === 'preset' ? (
          <div>
            <div className="label mb-1.5 flex items-center gap-2">
              Scenario preset
              {scenariosLoading && <Spinner className="h-3 w-3" />}
            </div>
            <select
              className="field"
              value={selectedId}
              onChange={(e) => onSelectScenario(e.target.value)}
              disabled={scenariosLoading || scenarios.length === 0}
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
          </div>
        ) : (
          <div className="space-y-3">
            <Field label="Scenario name">
              <input
                className="field"
                value={meta.name}
                onChange={(e) => setMeta({ ...meta, name: e.target.value })}
                placeholder="Custom Shock Scenario"
              />
            </Field>
            <Field label="Notes / hypothesis">
              <textarea
                className="field min-h-[68px] resize-y"
                value={meta.notes}
                onChange={(e) => setMeta({ ...meta, notes: e.target.value })}
                placeholder="What are you testing? Example: What happens if Taiwan semiconductor exports fall by 50%?"
              />
            </Field>
          </div>
        )}

        <Divider label="Shock parameters" />

        <div className="space-y-1.5">
          <Field label="Source">
            <input
              className="field"
              value={form.source}
              onChange={(e) => update('source', e.target.value)}
              placeholder="Taiwan"
            />
          </Field>
          <ExampleChips
            examples={SOURCE_EXAMPLES}
            active={form.source}
            onPick={(v) => update('source', v)}
          />
        </div>

        <div className="space-y-1.5">
          <Field label="Commodity">
            <input
              className="field"
              value={form.commodity}
              onChange={(e) => update('commodity', e.target.value)}
              placeholder="semiconductors"
            />
          </Field>
          <ExampleChips
            examples={COMMODITY_EXAMPLES}
            active={form.commodity}
            onPick={(v) => update('commodity', v)}
          />
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
          <p className="mt-1.5 text-xs leading-relaxed text-slate-400">
            {SHOCK_TYPE_DESC[form.shock_type]}
          </p>
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

        <label className="flex cursor-pointer items-center gap-2 text-sm text-slate-300">
          <input
            type="checkbox"
            checked={form.explain}
            onChange={(e) => update('explain', e.target.checked)}
            className="h-4 w-4 rounded border-slate-600 bg-slate-950 accent-cyan-400"
          />
          Explain (include blocked edges &amp; propagation rules)
        </label>

        <Divider label="Scenario assumptions" />

        <div className="grid grid-cols-2 gap-3">
          <Field label="Duration">
            <Select
              value={meta.assumptions.duration}
              options={DURATION_OPTIONS}
              onChange={(v) => updateAssumption('duration', v as ScenarioAssumptions['duration'])}
            />
          </Field>
          <Field label="Recovery speed">
            <Select
              value={meta.assumptions.recovery}
              options={RECOVERY_OPTIONS}
              onChange={(v) => updateAssumption('recovery', v as ScenarioAssumptions['recovery'])}
            />
          </Field>
          <Field label="Substitute availability">
            <Select
              value={meta.assumptions.substitute}
              options={SUBSTITUTE_OPTIONS}
              onChange={(v) =>
                updateAssumption('substitute', v as ScenarioAssumptions['substitute'])
              }
            />
          </Field>
          <Field label="Inventory buffer">
            <Select
              value={meta.assumptions.inventory}
              options={INVENTORY_OPTIONS}
              onChange={(v) => updateAssumption('inventory', v as ScenarioAssumptions['inventory'])}
            />
          </Field>
        </div>
        <p className="text-[11px] italic leading-relaxed text-slate-500">{ASSUMPTION_NOTE}</p>

        <div className="flex gap-2 pt-1">
          <button type="submit" className="btn-primary flex-1" disabled={!canRun}>
            {running ? (
              <>
                <Spinner className="h-4 w-4" />
                Running…
              </>
            ) : mode === 'custom' ? (
              'Run Custom Shock'
            ) : (
              'Run Shock Simulation'
            )}
          </button>
          <button
            type="button"
            onClick={onReset}
            disabled={running}
            className="rounded border border-slate-700 px-3 py-2 text-sm font-semibold text-slate-300 transition hover:border-slate-500 hover:text-slate-100 disabled:opacity-50"
          >
            Reset
          </button>
        </div>
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

function ModeToggle({ mode, setMode }: { mode: ShockMode; setMode: (m: ShockMode) => void }) {
  return (
    <div className="inline-flex rounded border border-slate-700 bg-slate-950/60 p-0.5 text-[11px] font-semibold">
      {(['preset', 'custom'] as const).map((m) => (
        <button
          key={m}
          type="button"
          onClick={() => setMode(m)}
          className={`rounded px-2.5 py-1 transition ${
            mode === m
              ? 'bg-cyan-500/20 text-cyan-200'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          {m === 'preset' ? 'Preset' : 'Custom Shock'}
        </button>
      ))}
    </div>
  )
}

function ExampleChips({
  examples,
  active,
  onPick,
}: {
  examples: string[]
  active: string
  onPick: (v: string) => void
}) {
  return (
    <div className="flex flex-wrap items-center gap-1">
      <span className="text-[10px] uppercase tracking-wider text-slate-600">e.g.</span>
      {examples.map((x) => {
        const on = active.trim().toLowerCase() === x.toLowerCase()
        return (
          <button
            key={x}
            type="button"
            onClick={() => onPick(x)}
            className={`rounded border px-1.5 py-0.5 text-[10px] transition ${
              on
                ? 'border-cyan-500/50 bg-cyan-500/10 text-cyan-200'
                : 'border-slate-700/70 bg-slate-800/40 text-slate-400 hover:border-cyan-500/50 hover:text-cyan-200'
            }`}
          >
            {x}
          </button>
        )
      })}
    </div>
  )
}

function Select({
  value,
  options,
  onChange,
}: {
  value: string
  options: readonly string[]
  onChange: (v: string) => void
}) {
  return (
    <select className="field" value={value} onChange={(e) => onChange(e.target.value)}>
      {options.map((o) => (
        <option key={o} value={o}>
          {o}
        </option>
      ))}
    </select>
  )
}

function Divider({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-2 pt-1">
      <span className="label whitespace-nowrap">{label}</span>
      <span className="h-px flex-1 bg-slate-800" />
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <div className="label mb-1.5">{label}</div>
      {children}
    </div>
  )
}
