import { useMemo, type ReactNode } from 'react'
import type {
  RecommendedScenario,
  Scenario,
  ShockOptionsResponse,
  ShockRequest,
  ShockValidOptionsResponse,
  ValidCommodityOption,
  ValidSourceOption,
} from '../types/api'
import {
  ASSUMPTION_NOTE,
  DEFAULT_ASSUMPTIONS,
  DURATION_OPTIONS,
  INVENTORY_OPTIONS,
  RECOVERY_OPTIONS,
  SUBSTITUTE_OPTIONS,
  type ScenarioAssumptions,
  type ScenarioMeta,
  type ShockMode,
  operationalRequestFields,
} from '../types/scenario'
import { Panel, Spinner } from './ui'

const SHOCK_TYPE_DESC: Record<string, string> = {
  export_collapse: 'Producer exports fall, affecting importers and downstream sectors.',
  supply_cut: 'Source supply availability falls.',
  price_spike: 'Commodity price pressure rises across exposed sectors.',
  route_disruption: 'Logistics route disruption affects flows.',
}

const eq = (a: string, b: string) => a.trim().toLowerCase() === b.trim().toLowerCase()

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
  options,
  validOptions,
  onApplyRecommended,
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
  options: ShockOptionsResponse | null
  validOptions: ShockValidOptionsResponse | null
  onApplyRecommended: (rs: RecommendedScenario) => void
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

  const selected = scenarios.find((s) => s.id === selectedId)
  const validSources = validOptions?.sources ?? []

  const selectedSource = useMemo(
    () => findValidSource(validOptions, form.source),
    [validOptions, form.source],
  )

  const commodityOptions = selectedSource?.commodities ?? []

  const selectedCommodity = useMemo(
    () => findValidCommodity(selectedSource, form.commodity),
    [selectedSource, form.commodity],
  )

  const shockTypeOptions = selectedCommodity?.shock_types ?? []

  const guidedValid = useMemo(
    () => isGuidedValid(form, validOptions),
    [form, validOptions],
  )

  const canRun =
    !running &&
    (mode === 'preset'
      ? form.source.trim() !== '' && form.commodity.trim() !== ''
      : guidedValid)

  const shockTypeName = (type: string) =>
    options?.shock_types?.find((s) => s.type === type)?.name || type.replace(/_/g, ' ')

  const shockDescription = useMemo(() => {
    const opt = options?.shock_types?.find((s) => s.type === form.shock_type)
    return opt?.description || SHOCK_TYPE_DESC[form.shock_type] || ''
  }, [options, form.shock_type])

  const guidedMessage = useMemo(() => {
    if (mode !== 'custom') return null
    if (!validOptions) return 'Loading graph-valid shock combinations…'
    if (!form.source.trim()) return 'Select a source to see connected commodities.'
    if (!selectedSource) return 'Select a source to see connected commodities.'
    if (commodityOptions.length === 0) {
      return 'No valid commodities found for this source in the current graph.'
    }
    if (!form.commodity.trim() || !selectedCommodity) {
      return 'Select a connected commodity for this source.'
    }
    if (!shockTypeOptions.includes(form.shock_type)) {
      return 'This source and commodity do not support that shock type.'
    }
    return null
  }, [
    mode,
    validOptions,
    form.source,
    form.commodity,
    form.shock_type,
    selectedSource,
    commodityOptions.length,
    selectedCommodity,
    shockTypeOptions,
  ])

  const onSourceInput = (source: string) => {
    const src = findValidSource(validOptions, source)
    if (!src) {
      setForm({ ...form, source })
      return
    }
    const commodities = src.commodities
    let commodity = form.commodity
    let shock_type = form.shock_type
    const com = findValidCommodity(src, commodity)
    if (!com) {
      commodity = commodities[0]?.commodity ?? ''
      shock_type = commodities[0]?.shock_types[0] ?? form.shock_type
    } else if (!com.shock_types.includes(shock_type)) {
      shock_type = com.shock_types[0] ?? shock_type
    }
    setForm({ ...form, source: src.source, commodity, shock_type })
  }

  const onCommodityChange = (commodity: string) => {
    const com = findValidCommodity(selectedSource, commodity)
    let shock_type = form.shock_type
    if (com && !com.shock_types.includes(shock_type)) {
      shock_type = com.shock_types[0] ?? shock_type
    }
    setForm({ ...form, commodity, shock_type })
  }

  const switchMode = (next: ShockMode) => {
    if (next === 'custom' && validOptions && !isGuidedValid(form, validOptions)) {
      const first = validOptions.sources[0]
      const com = first?.commodities[0]
      if (first && com) {
        setForm({
          ...form,
          source: first.source,
          commodity: com.commodity,
          shock_type: com.shock_types[0] ?? form.shock_type,
        })
      }
    }
    setMode(next)
  }

  const recommended = options?.recommended_scenarios ?? []

  return (
    <Panel title="Shock Simulator" right={<ModeToggle mode={mode} setMode={switchMode} />}>
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

        {recommended.length > 0 && (
          <RecommendedScenarios items={recommended} onPick={onApplyRecommended} disabled={running} />
        )}

        {mode === 'preset' ? (
          <>
            <Divider label="Scenario details" />
            <div className="space-y-2 rounded border border-slate-800/80 bg-slate-950/40 px-3 py-2.5">
              <ReadOnlyRow label="Source" value={form.source} />
              <ReadOnlyRow label="Commodity" value={form.commodity} accent />
              <ReadOnlyRow label="Shock type" value={shockTypeName(form.shock_type)} />
              <p className="pt-1 text-[11px] text-slate-500">
                Preset propagation targets are fixed. Adjust drop and depth below.
              </p>
            </div>
            <Divider label="Scenario parameters" />
          </>
        ) : (
          <>
            <Divider label="Guided custom shock" />
            <p className="text-xs leading-relaxed text-slate-400">
              Source {'->'} connected commodity {'->'} valid shock type. Only graph-linked combinations
              are shown.
            </p>

            <Field label="Source">
              {validOptions === null ? (
                <div className="flex items-center gap-2 py-2 text-sm text-slate-400">
                  <Spinner className="h-3.5 w-3.5" />
                  Loading valid sources…
                </div>
              ) : (
                <>
                  <input
                    className="field"
                    list="gfip-valid-sources"
                    value={form.source}
                    onChange={(e) => onSourceInput(e.target.value)}
                    placeholder="Search countries / routes…"
                    autoComplete="off"
                  />
                  <datalist id="gfip-valid-sources">
                    {validSources.map((s) => (
                      <option key={s.source} value={s.source} />
                    ))}
                  </datalist>
                  {selectedSource && (
                    <p className="mt-1 text-[10px] uppercase tracking-wider text-slate-500">
                      {selectedSource.type}
                    </p>
                  )}
                </>
              )}
            </Field>

            <Field label="Connected commodity">
              <select
                className="field"
                value={form.commodity}
                onChange={(e) => onCommodityChange(e.target.value)}
                disabled={!selectedSource || commodityOptions.length === 0}
              >
                {commodityOptions.length === 0 && (
                  <option value="">No commodities for this source</option>
                )}
                {commodityOptions.map((c) => (
                  <option key={c.commodity} value={c.commodity}>
                    {c.commodity}
                  </option>
                ))}
              </select>
              {selectedCommodity && selectedCommodity.relationships.length > 0 && (
                <p className="mt-1.5 text-[11px] text-slate-500">
                  via {selectedCommodity.relationships.map((r) => r.replace(/_/g, ' ')).join(', ')}
                </p>
              )}
            </Field>

            <Field label="Valid shock type">
              <select
                className="field"
                value={form.shock_type}
                onChange={(e) => update('shock_type', e.target.value)}
                disabled={shockTypeOptions.length === 0}
              >
                {shockTypeOptions.length === 0 && (
                  <option value="">Select source and commodity first</option>
                )}
                {shockTypeOptions.map((t) => (
                  <option key={t} value={t}>
                    {shockTypeName(t)}
                  </option>
                ))}
              </select>
              <p className="mt-1.5 text-xs leading-relaxed text-slate-400">{shockDescription}</p>
            </Field>

            {guidedMessage && (
              <p className="rounded border border-slate-800/80 bg-slate-950/40 px-3 py-2 text-xs text-slate-400">
                {guidedMessage}
              </p>
            )}
          </>
        )}

        {mode === 'preset' && (
          <Field label="Shock type description">
            <p className="text-xs leading-relaxed text-slate-400">{shockDescription}</p>
          </Field>
        )}

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
          Show technical propagation details
        </label>

        <Divider label="Model assumptions" />

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
              'Run Shock'
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

function findValidSource(
  valid: ShockValidOptionsResponse | null,
  source: string,
): ValidSourceOption | undefined {
  if (!valid || !source.trim()) return undefined
  return valid.sources.find((s) => eq(s.source, source))
}

function findValidCommodity(
  src: ValidSourceOption | undefined,
  commodity: string,
): ValidCommodityOption | undefined {
  if (!src || !commodity.trim()) return undefined
  return src.commodities.find((c) => eq(c.commodity, commodity))
}

function isGuidedValid(form: ShockForm, valid: ShockValidOptionsResponse | null): boolean {
  const src = findValidSource(valid, form.source)
  const com = findValidCommodity(src, form.commodity)
  if (!src || !com) return false
  return com.shock_types.includes(form.shock_type)
}

export function toRequest(
  form: ShockForm,
  assumptions: ScenarioAssumptions = DEFAULT_ASSUMPTIONS,
): ShockRequest {
  return {
    source: form.source.trim(),
    commodity: form.commodity.trim(),
    shock_type: form.shock_type,
    drop: form.drop,
    depth: form.depth,
    explain: form.explain,
    ...operationalRequestFields(assumptions),
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

function RecommendedScenarios({
  items,
  onPick,
  disabled,
}: {
  items: RecommendedScenario[]
  onPick: (rs: RecommendedScenario) => void
  disabled: boolean
}) {
  return (
    <div className="space-y-1.5">
      <div className="label">Recommended scenarios</div>
      <div className="flex flex-wrap gap-1.5">
        {items.map((rs) => (
          <button
            key={rs.label}
            type="button"
            disabled={disabled}
            onClick={() => onPick(rs)}
            title={`${rs.source} -> ${rs.commodity} · ${rs.shock_type} · drop ${rs.drop}% · depth ${rs.depth}`}
            className="rounded border border-slate-700/70 bg-slate-800/40 px-2 py-1 text-[11px] text-slate-300 transition hover:border-cyan-500/50 hover:text-cyan-200 disabled:opacity-50"
          >
            {rs.label}
          </button>
        ))}
      </div>
    </div>
  )
}

function ReadOnlyRow({
  label,
  value,
  accent,
}: {
  label: string
  value: string
  accent?: boolean
}) {
  return (
    <div className="flex items-baseline justify-between gap-3 text-sm">
      <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-500">
        {label}
      </span>
      <span className={`text-right font-medium ${accent ? 'text-amber-300' : 'text-slate-100'}`}>
        {value || '—'}
      </span>
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
