import type { ExposureItem, ShockResponse } from '../types/api'
import { ASSUMPTION_NOTE, type SubmittedScenario } from '../types/scenario'
import { deltaClass, fixed, pct, prettyPath, signed } from '../lib/format'
import { EmptyHint, Panel, Spinner, TypeBadge } from './ui'
import { InlineError } from './States'

export function ShockResults({
  result,
  submitted,
  running,
  error,
}: {
  result: ShockResponse | null
  submitted?: SubmittedScenario | null
  running: boolean
  error?: { message: string; hint?: string } | null
}) {
  if (error && !running) {
    return (
      <Panel title="Shock Results">
        <InlineError message={error.message} hint={error.hint} />
      </Panel>
    )
  }

  if (running && !result) {
    return (
      <Panel title="Shock Results">
        <div className="flex items-center gap-3 py-12 text-sm text-slate-400">
          <Spinner />
          Propagating shock through the dependency graph…
        </div>
      </Panel>
    )
  }

  if (!result) {
    return (
      <Panel title="Shock Results">
        <EmptyHint>
          Configure a scenario and run a simulation to trace cascading exposure
          across the global dependency graph.
        </EmptyHint>
      </Panel>
    )
  }

  const s = result.graph_impact_summary

  return (
    <div className={`space-y-4 ${running ? 'opacity-60' : ''}`}>
      <ResultBanner result={result} submitted={submitted} />

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <MetricCard label="Affected nodes" value={String(s.affected_nodes)} />
        <MetricCard label="Affected paths" value={String(s.affected_paths)} />
        <MetricCard
          label="Avg fragility Δ"
          value={signed(s.avg_fragility_delta)}
          className={deltaClass(s.avg_fragility_delta)}
        />
        <MetricCard
          label="Largest impact Δ"
          value={signed(s.largest_single_impact_delta)}
          className={deltaClass(s.largest_single_impact_delta)}
        />
        <MetricCard
          label="Top impacted"
          value={s.largest_single_impact_entity || '—'}
          small
        />
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <Panel title="Direct Exposure" noPad>
          <ExposureTable items={result.direct_exposure} emptyLabel="No direct exposure within depth." />
        </Panel>
        <Panel title="Second-Order Exposure" noPad>
          <ExposureTable
            items={result.second_order_exposure}
            emptyLabel="No second-order exposure within depth."
          />
        </Panel>
      </div>

      <Panel title={`Affected Dependency Paths · ${result.affected_paths.length}`} noPad>
        <PathList result={result} />
      </Panel>

      {result.blocked_edges && (
        <Panel title={`Blocked Edges · ${result.blocked_edges.length}`} noPad>
          <BlockedList result={result} />
        </Panel>
      )}
    </div>
  )
}

function ResultBanner({
  result,
  submitted,
}: {
  result: ShockResponse
  submitted?: SubmittedScenario | null
}) {
  // The header LINE always reflects the actual submitted shock (backend echo).
  const sc = result.scenario
  // The TITLE reflects the resolved scenario name captured at submit time.
  const meta = submitted?.meta
  const title =
    submitted?.title?.trim() || sc.name || `${sc.source} · ${result.shock_profile.type}`

  return (
    <div className="panel space-y-2.5 px-4 py-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="text-sm font-semibold text-slate-50">{title}</h3>
          {submitted?.modifiedPreset && (
            <span className="badge border-amber-500/40 bg-amber-500/10 text-amber-300">
              Modified preset
            </span>
          )}
        </div>
        <span className="badge border-cyan-500/40 bg-cyan-500/10 text-cyan-300">
          {sc.shock_type || result.shock_profile.type}
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
        <span className="flex items-center gap-2">
          <span className="font-semibold text-slate-100">{sc.source}</span>
          <span className="text-slate-600">→</span>
          <span className="font-semibold text-amber-300">{sc.commodity}</span>
        </span>
        <span className="flex flex-wrap gap-x-4 font-mono text-xs text-slate-400">
          <span>
            drop <span className="text-slate-200">{fixed(sc.shock_percent, 0)}%</span>
          </span>
          <span>
            depth <span className="text-slate-200">{sc.depth}</span>
          </span>
          <span>
            initial impact <span className="text-slate-200">{pct(sc.initial_impact)}</span>
          </span>
          <span>
            attenuation <span className="text-slate-200">{fixed(result.shock_profile.attenuation, 2)}</span>
          </span>
        </span>
      </div>

      {meta && (
        <div className="flex flex-wrap gap-1.5">
          <AssumptionChip label="duration" value={meta.assumptions.duration} />
          <AssumptionChip label="recovery" value={meta.assumptions.recovery} />
          <AssumptionChip label="substitute" value={meta.assumptions.substitute} />
          <AssumptionChip label="inventory" value={meta.assumptions.inventory} />
        </div>
      )}

      {meta?.notes?.trim() && (
        <div className="rounded border border-slate-800 bg-slate-950/50 px-3 py-2 text-xs leading-relaxed text-slate-300">
          <span className="text-slate-500">Hypothesis: </span>
          {meta.notes.trim()}
        </div>
      )}

      <p className="text-[11px] italic text-slate-500">{ASSUMPTION_NOTE}</p>
    </div>
  )
}

function AssumptionChip({ label, value }: { label: string; value: string }) {
  return (
    <span className="rounded border border-slate-700/70 bg-slate-800/40 px-2 py-0.5 font-mono text-[11px] text-slate-300">
      <span className="text-slate-500">{label} </span>
      {value}
    </span>
  )
}

function MetricCard({
  label,
  value,
  className = '',
  small = false,
}: {
  label: string
  value: string
  className?: string
  small?: boolean
}) {
  return (
    <div className="panel px-3 py-2.5">
      <div className="label">{label}</div>
      <div
        className={`mt-1 font-mono font-semibold tabular-nums ${small ? 'text-sm' : 'text-2xl'} ${
          className || 'text-slate-50'
        }`}
        title={value}
      >
        {small ? <span className="block truncate">{value}</span> : value}
      </div>
    </div>
  )
}

function ExposureTable({ items, emptyLabel }: { items: ExposureItem[]; emptyLabel: string }) {
  if (items.length === 0) {
    return <div className="px-4 py-6 text-sm text-slate-500">{emptyLabel}</div>
  }
  return (
    <div className="max-h-72 overflow-auto">
      <table className="w-full border-collapse">
        <thead className="sticky top-0 bg-slate-900/90 backdrop-blur">
          <tr className="border-b border-slate-800">
            <th className="th">Entity</th>
            <th className="th">Type</th>
            <th className="th text-right">Impact</th>
            <th className="th text-right">Base → Shock</th>
            <th className="th text-right">Δ</th>
          </tr>
        </thead>
        <tbody>
          {items.map((it, i) => (
            <tr key={`${it.entity}-${i}`} className="border-b border-slate-800/60 hover:bg-slate-800/30">
              <td className="td font-medium text-slate-100">{it.entity}</td>
              <td className="td">
                <TypeBadge type={it.type} />
              </td>
              <td className="td text-right font-mono tabular-nums">{pct(it.impact)}</td>
              <td className="td text-right font-mono tabular-nums text-slate-400">
                {fixed(it.base_fragility)} → <span className="text-slate-200">{fixed(it.shock_fragility)}</span>
              </td>
              <td className={`td text-right font-mono font-semibold tabular-nums ${deltaClass(it.delta)}`}>
                {signed(it.delta)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function PathList({ result }: { result: ShockResponse }) {
  if (result.affected_paths.length === 0) {
    return <div className="px-4 py-6 text-sm text-slate-500">No affected paths within depth.</div>
  }
  return (
    <ul className="divide-y divide-slate-800/60">
      {result.affected_paths.map((p, i) => (
        <li key={i} className="flex items-center justify-between gap-3 px-4 py-2.5">
          <code className="truncate font-mono text-[13px] text-slate-200">{prettyPath(p.labeled_path)}</code>
          <div className="flex shrink-0 gap-3 font-mono text-[11px] text-slate-500">
            <span title="impact reaching the endpoint">
              impact <span className="text-cyan-300">{pct(p.end_impact)}</span>
            </span>
            <span title="product of edge weights">
              w <span className="text-slate-300">{fixed(p.path_weight, 2)}</span>
            </span>
          </div>
        </li>
      ))}
    </ul>
  )
}

function BlockedList({ result }: { result: ShockResponse }) {
  const blocked = result.blocked_edges ?? []
  if (blocked.length === 0) {
    return (
      <div className="px-4 py-6 text-sm text-slate-500">
        No branches were blocked — every relationship was permitted by this shock type.
      </div>
    )
  }
  return (
    <ul className="divide-y divide-slate-800/60">
      {blocked.map((b, i) => (
        <li key={i} className="px-4 py-2.5">
          <code className="font-mono text-[13px] text-slate-300">
            {b.from} <span className="text-rose-400/80">--{b.relationship_type}--&gt;</span> {b.to}
          </code>
          <div className="mt-0.5 flex flex-wrap gap-2 text-[11px] text-slate-500">
            <span className="text-rose-300/80">{b.reason}</span>
            {b.commodity && <span className="font-mono">commodity: {b.commodity}</span>}
          </div>
        </li>
      ))}
    </ul>
  )
}
