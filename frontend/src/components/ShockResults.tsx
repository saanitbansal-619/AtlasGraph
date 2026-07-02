import { Fragment, useMemo, useState } from 'react'
import type { BlockedEdge, ExposureItem, ShockResponse } from '../types/api'
import { ASSUMPTION_NOTE, type SubmittedScenario } from '../types/scenario'
import {
  blockedEdgeCategory,
  deltaClass,
  fixed,
  formatRelationship,
  pct,
  signed,
} from '../lib/format'
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

      <div className="grid grid-cols-2 gap-3 min-[480px]:grid-cols-3 xl:grid-cols-5">
        <MetricCard label="Affected nodes" value={String(s.affected_nodes)} />
        <MetricCard label="Affected paths" value={String(s.affected_paths)} />
        <MetricCard
          label="Avg fragility Δ"
          value={signed(s.avg_fragility_delta)}
          valueClassName={deltaClass(s.avg_fragility_delta)}
        />
        <MetricCard
          label="Largest impact Δ"
          value={signed(s.largest_single_impact_delta)}
          valueClassName={deltaClass(s.largest_single_impact_delta)}
        />
        <MetricCard
          label="Top impacted"
          value={s.largest_single_impact_entity || '—'}
          small
          wrapperClassName="col-span-2 min-[480px]:col-span-1"
        />
      </div>

      <div className="grid grid-cols-1 gap-4 2xl:grid-cols-2">
        <Panel title="Direct Exposure" noPad className="min-w-0">
          <ExposureTable items={result.direct_exposure} emptyLabel="No direct exposure within depth." />
        </Panel>
        <Panel title="Second-Order Exposure" noPad className="min-w-0">
          <ExposureTable
            items={result.second_order_exposure}
            emptyLabel="No second-order exposure within depth."
            sparseNote="Only a few nodes were affected at second order — most impact remained in direct exposure."
          />
        </Panel>
      </div>

      <Panel title={`Affected Dependency Paths · ${result.affected_paths.length}`} noPad>
        <PathList result={result} />
      </Panel>

      {result.blocked_edges && result.blocked_edges.length > 0 && (
        <BlockedEdgesPanel result={result} />
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
  const sc = result.scenario
  const meta = submitted?.meta
  const title =
    submitted?.title?.trim() || sc.name || `${sc.source} · ${result.shock_profile.type}`
  const warnings = result.warnings ?? []

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

      {warnings.length > 0 && (
        <div className="space-y-1.5">
          {warnings.map((w, i) => (
            <div
              key={i}
              className="flex items-start gap-2 rounded border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs leading-relaxed text-amber-200/90"
            >
              <span aria-hidden className="mt-0.5 text-amber-400">
                ⚠
              </span>
              <span>{w}</span>
            </div>
          ))}
        </div>
      )}

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
  valueClassName = '',
  wrapperClassName = '',
  small = false,
}: {
  label: string
  value: string
  valueClassName?: string
  wrapperClassName?: string
  small?: boolean
}) {
  return (
    <div className={`panel min-w-0 px-3 py-2.5 ${wrapperClassName}`}>
      <div className="label">{label}</div>
      <div
        className={`mt-1 font-mono font-semibold tabular-nums ${
          small ? 'text-sm leading-snug' : 'text-2xl'
        } ${valueClassName || 'text-slate-50'}`}
        title={value}
      >
        {small ? <span className="block break-words">{value}</span> : value}
      </div>
    </div>
  )
}

function ExposureTable({
  items,
  emptyLabel,
  sparseNote,
}: {
  items: ExposureItem[]
  emptyLabel: string
  sparseNote?: string
}) {
  if (items.length === 0) {
    return <div className="px-4 py-5 text-sm text-slate-500">{emptyLabel}</div>
  }

  const scrollable = items.length > 8

  return (
    <div className={scrollable ? 'max-h-80 overflow-y-auto' : ''}>
      <table className="w-full table-fixed border-collapse">
        <colgroup>
          <col className="w-[36%]" />
          <col className="w-[14%]" />
          <col className="w-[13%]" />
          <col className="w-[25%]" />
          <col className="w-[12%]" />
        </colgroup>
        <thead className={scrollable ? 'sticky top-0 z-10 bg-slate-900/95 backdrop-blur' : ''}>
          <tr className="border-b border-slate-800">
            <th className="th">Entity</th>
            <th className="th w-[14%]">Type</th>
            <th className="th text-right">Impact</th>
            <th className="th text-right">Base → Shock</th>
            <th className="th text-right">Δ</th>
          </tr>
        </thead>
        <tbody>
          {items.map((it, i) => (
            <tr key={`${it.entity}-${i}`} className="border-b border-slate-800/60 hover:bg-slate-800/30">
              <td className="exposure-td-entity">{it.entity}</td>
              <td className="exposure-td-type">
                <TypeBadge type={it.type} />
              </td>
              <td className="exposure-td-mono text-right">{pct(it.impact)}</td>
              <td className="exposure-td-mono text-right text-slate-400">
                {fixed(it.base_fragility)} →{' '}
                <span className="text-slate-200">{fixed(it.shock_fragility)}</span>
              </td>
              <td className={`exposure-td-mono text-right font-semibold ${deltaClass(it.delta)}`}>
                {signed(it.delta)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {sparseNote && items.length > 0 && items.length <= 2 && (
        <p className="border-t border-slate-800/60 px-4 py-2.5 text-xs leading-relaxed text-slate-500">
          {sparseNote}
        </p>
      )}
    </div>
  )
}

function PathList({ result }: { result: ShockResponse }) {
  if (result.affected_paths.length === 0) {
    return <div className="px-4 py-5 text-sm text-slate-500">No affected paths within depth.</div>
  }
  return (
    <ul className="divide-y divide-slate-800/60">
      {result.affected_paths.map((p, i) => (
        <li
          key={i}
          className="flex flex-col gap-2.5 px-4 py-3 sm:flex-row sm:items-start sm:justify-between sm:gap-4"
        >
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-x-1.5 gap-y-1 text-sm leading-relaxed">
              {p.path.map((node, j) => (
                <Fragment key={`${node}-${j}`}>
                  {j > 0 && <span className="text-slate-600" aria-hidden>→</span>}
                  <span className="font-medium text-slate-100">{node}</span>
                </Fragment>
              ))}
            </div>
            {p.relationships.length > 0 && (
              <div className="mt-1.5 flex flex-wrap items-center gap-1">
                {p.relationships.map((rel, j) => (
                  <Fragment key={`${rel}-${j}`}>
                    {j > 0 && <span className="text-[10px] text-slate-600">·</span>}
                    <span className="rounded border border-slate-700/50 bg-slate-800/30 px-1.5 py-0.5 text-[10px] text-slate-500">
                      {formatRelationship(rel)}
                    </span>
                  </Fragment>
                ))}
              </div>
            )}
          </div>
          <div className="flex shrink-0 flex-wrap gap-2">
            <CompactMetricBadge label="impact" value={pct(p.end_impact)} accent />
            <CompactMetricBadge label="weight" value={fixed(p.path_weight, 2)} />
          </div>
        </li>
      ))}
    </ul>
  )
}

function CompactMetricBadge({
  label,
  value,
  accent,
}: {
  label: string
  value: string
  accent?: boolean
}) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded border border-slate-700/60 bg-slate-950/50 px-2 py-1 font-mono text-[11px] tabular-nums">
      <span className="text-slate-500">{label}</span>
      <span className={accent ? 'font-semibold text-cyan-300' : 'text-slate-200'}>{value}</span>
    </span>
  )
}

function BlockedEdgesPanel({ result }: { result: ShockResponse }) {
  const [open, setOpen] = useState(false)
  const blocked = result.blocked_edges ?? []
  const commodity = result.scenario.commodity

  const grouped = useMemo(() => {
    const map = new Map<string, BlockedEdge[]>()
    for (const b of blocked) {
      const cat = blockedEdgeCategory(b.reason)
      const list = map.get(cat) ?? []
      list.push(b)
      map.set(cat, list)
    }
    return [...map.entries()].sort((a, b) => b[1].length - a[1].length)
  }, [blocked])

  const shockType = result.scenario.shock_type || result.shock_profile.type
  const focusLabel = shockType.includes('route')
    ? `${commodity} route propagation`
    : `${commodity} supply routes`

  return (
    <Panel title="Propagation focus" noPad>
      <div className="space-y-3 px-4 py-3">
        <p className="text-sm leading-relaxed text-slate-300">
          <span className="font-semibold text-slate-100">{blocked.length}</span> unrelated
          branch{blocked.length === 1 ? '' : 'es'} were ignored to keep this shock focused on{' '}
          <span className="text-amber-300">{focusLabel}</span>.
        </p>

        {grouped.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {grouped.map(([cat, items]) => (
              <span
                key={cat}
                className="rounded border border-slate-700/60 bg-slate-800/30 px-2 py-1 text-[11px] text-slate-400"
              >
                <span className="font-mono font-semibold text-slate-200">{items.length}</span>{' '}
                {cat.toLowerCase()}
              </span>
            ))}
          </div>
        )}

        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="flex w-full items-center justify-between rounded border border-slate-700/60 bg-slate-950/40 px-3 py-2 text-left text-xs text-slate-400 transition hover:border-slate-600 hover:text-slate-300"
          aria-expanded={open}
        >
          <span className="font-semibold uppercase tracking-wider">Advanced / Debug Details</span>
          <span className="font-mono text-slate-500">{open ? '−' : '+'}</span>
        </button>

        {open && <BlockedEdgesDebug grouped={grouped} />}
      </div>
    </Panel>
  )
}

function BlockedEdgesDebug({ grouped }: { grouped: [string, BlockedEdge[]][] }) {
  return (
    <div className="space-y-3 rounded border border-slate-800/80 bg-slate-950/30">
      {grouped.map(([cat, items]) => (
        <div key={cat}>
          <div className="border-b border-slate-800/60 px-3 py-2 text-[10px] font-semibold uppercase tracking-wider text-slate-500">
            {cat} · {items.length}
          </div>
          <ul className="divide-y divide-slate-800/40">
            {items.map((b, i) => (
              <li key={i} className="px-3 py-2">
                <div className="text-sm text-slate-200">
                  <span className="font-medium">{b.from}</span>
                  <span className="mx-1.5 text-slate-600">→</span>
                  <span className="font-medium">{b.to}</span>
                  <span className="ml-2 rounded border border-slate-700/50 px-1 py-0.5 text-[10px] text-slate-500">
                    {formatRelationship(b.relationship_type)}
                  </span>
                </div>
                <div className="mt-0.5 flex flex-wrap gap-2 text-[11px] text-slate-500">
                  <span className="text-rose-300/80">{b.reason}</span>
                  {b.commodity && <span className="font-mono">commodity: {b.commodity}</span>}
                </div>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  )
}
