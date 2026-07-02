import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, ApiRequestError } from '../lib/api'
import type {
  CompareScenarioInput,
  RecommendedScenario,
  ScenarioCompareResponse,
  ShockOptionsResponse,
} from '../types/api'
import { deltaClass, signed } from '../lib/format'
import { AdaptiveRankingChart } from './charts/AdaptiveRankingChart'
import { EmptyHint, Panel, Spinner } from './ui'
import { InlineError } from './States'

function scenarioKey(rs: RecommendedScenario): string {
  return `${rs.label}|${rs.source}|${rs.commodity}|${rs.shock_type}`
}

function toCompareInput(rs: RecommendedScenario): CompareScenarioInput {
  return {
    label: rs.label,
    source: rs.source,
    commodity: rs.commodity,
    shock_type: rs.shock_type,
    drop: rs.drop,
    depth: rs.depth || 3,
    explain: true,
  }
}

export function ScenarioComparison({
  options,
}: {
  options: ShockOptionsResponse | null
}) {
  const recommended = options?.recommended_scenarios ?? []
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [comparing, setComparing] = useState(false)
  const [result, setResult] = useState<ScenarioCompareResponse | null>(null)
  const [error, setError] = useState<{ message: string; hint?: string } | null>(null)

  useEffect(() => {
    if (recommended.length === 0) return
    setSelected(new Set(recommended.map(scenarioKey)))
  }, [recommended])

  const toggle = useCallback((rs: RecommendedScenario) => {
    const key = scenarioKey(rs)
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }, [])

  const selectedScenarios = useMemo(
    () => recommended.filter((rs) => selected.has(scenarioKey(rs))),
    [recommended, selected],
  )

  const runCompare = useCallback(async () => {
    if (selectedScenarios.length === 0) return
    setComparing(true)
    setError(null)
    try {
      const res = await api.compareScenarios({
        scenarios: selectedScenarios.map(toCompareInput),
      })
      setResult(res)
    } catch (e) {
      setResult(null)
      if (e instanceof ApiRequestError) {
        setError({ message: e.message, hint: e.hint })
      } else {
        setError({ message: e instanceof Error ? e.message : 'Comparison failed' })
      }
    } finally {
      setComparing(false)
    }
  }, [selectedScenarios])

  return (
    <div className="space-y-4">
      <Panel
        title="Scenario Comparison"
        right={
          <span className="text-[10px] font-mono uppercase tracking-wider text-slate-500">
            rank systemic impact
          </span>
        }
      >
        {options === null ? (
          <div className="flex items-center gap-2 py-6 text-sm text-slate-400">
            <Spinner />
            Loading recommended scenarios…
          </div>
        ) : recommended.length === 0 ? (
          <EmptyHint>No graph-validated scenarios available for comparison.</EmptyHint>
        ) : (
          <div className="space-y-4">
            <p className="text-xs text-slate-400">
              Select shocks to run side-by-side. Results are ranked by average fragility
              delta, then max delta and affected nodes.
            </p>
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {recommended.map((rs) => {
                const key = scenarioKey(rs)
                const on = selected.has(key)
                return (
                  <label
                    key={key}
                    className={`cursor-pointer rounded border p-3 transition ${
                      on
                        ? 'border-cyan-500/50 bg-cyan-500/10'
                        : 'border-slate-700/70 bg-slate-900/40 hover:border-slate-600'
                    }`}
                  >
                    <div className="flex items-start gap-2">
                      <input
                        type="checkbox"
                        className="mt-0.5 accent-cyan-400"
                        checked={on}
                        onChange={() => toggle(rs)}
                        disabled={comparing}
                      />
                      <div className="min-w-0 flex-1">
                        <div className="text-sm font-medium text-slate-100">{rs.label}</div>
                        <div className="mt-1 font-mono text-[10px] text-slate-500">
                          {rs.source} · {rs.commodity} · {rs.shock_type} · {rs.drop}% · d
                          {rs.depth}
                        </div>
                      </div>
                    </div>
                  </label>
                )
              })}
            </div>
            <button
              type="button"
              className="btn-primary w-full sm:w-auto"
              disabled={comparing || selectedScenarios.length === 0}
              onClick={() => void runCompare()}
            >
              {comparing ? 'Comparing…' : 'Compare Selected Scenarios'}
            </button>
          </div>
        )}
      </Panel>

      {error && !comparing && (
        <Panel title="Comparison Results">
          <InlineError message={error.message} hint={error.hint} />
        </Panel>
      )}

      {comparing && !result && (
        <Panel title="Comparison Results">
          <div className="flex items-center gap-3 py-10 text-sm text-slate-400">
            <Spinner />
            Running {selectedScenarios.length} shock scenario
            {selectedScenarios.length === 1 ? '' : 's'}…
          </div>
        </Panel>
      )}

      {result && (
        <CompareResults data={result} dimmed={comparing} />
      )}
    </div>
  )
}

function CompareResults({
  data,
  dimmed,
}: {
  data: ScenarioCompareResponse
  dimmed: boolean
}) {
  const s = data.summary
  return (
    <div className={`space-y-4 ${dimmed ? 'opacity-60' : ''}`}>
      <Panel title="Comparison Summary">
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <SummaryChip label="Worst overall" value={s.worst_overall_scenario} accent />
          <SummaryChip label="Most countries" value={s.most_countries_affected} />
          <SummaryChip label="Most sectors" value={s.most_sectors_affected} />
          <SummaryChip label="Highest avg Δ" value={s.highest_average_fragility_delta} />
          <SummaryChip label="Highest max Δ" value={s.highest_max_fragility_delta} />
        </div>
      </Panel>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <AdaptiveRankingChart
          title="Scenario Avg Fragility Delta"
          subtitle="Δ fragility"
          valueLabel="Δ fragility"
          valueDigits={1}
          valueSuffix=" Δ"
          data={data.results.map((r) => ({
            label: r.label,
            value: r.average_fragility_delta,
          }))}
          emptyLabel="No comparison results available."
          topN={10}
          forceRanking={data.results.length <= 3}
        />
        <AdaptiveRankingChart
          title="Scenario Max Fragility Delta"
          subtitle="Δ fragility"
          valueLabel="Δ fragility"
          valueDigits={1}
          valueSuffix=" Δ"
          data={data.results.map((r) => ({
            label: r.label,
            value: r.max_fragility_delta,
          }))}
          emptyLabel="No comparison results available."
          topN={10}
          forceRanking={data.results.length <= 3}
        />
      </div>

      <div className="space-y-3">
        {data.results.map((r, i) => (
          <Panel
            key={`${r.label}-${i}`}
            title={`#${i + 1} · ${r.label}`}
            right={
              <span className="font-mono text-[10px] text-slate-500">
                {r.source} · {r.commodity}
              </span>
            }
          >
            <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
              <Metric label="Affected nodes" value={String(r.affected_nodes_count)} />
              <Metric label="Affected paths" value={String(r.affected_paths_count)} />
              <Metric
                label="Avg fragility Δ"
                value={signed(r.average_fragility_delta)}
                className={deltaClass(r.average_fragility_delta)}
              />
              <Metric
                label="Max fragility Δ"
                value={signed(r.max_fragility_delta)}
                className={deltaClass(r.max_fragility_delta)}
              />
            </div>

            {(r.top_affected_countries.length > 0 || r.top_affected_sectors.length > 0) && (
              <div className="mt-4 grid gap-4 md:grid-cols-2">
                {r.top_affected_countries.length > 0 && (
                  <TopList title="Top countries" items={r.top_affected_countries} />
                )}
                {r.top_affected_sectors.length > 0 && (
                  <TopList title="Top sectors" items={r.top_affected_sectors} />
                )}
              </div>
            )}

            {r.warnings && r.warnings.length > 0 && (
              <div className="mt-4 rounded border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-xs text-amber-200/90">
                {r.warnings.map((w) => (
                  <div key={w}>{w}</div>
                ))}
              </div>
            )}
          </Panel>
        ))}
      </div>
    </div>
  )
}

function SummaryChip({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return (
    <div className="rounded border border-slate-800 bg-slate-950/50 px-3 py-2">
      <div className="text-[10px] uppercase tracking-wider text-slate-500">{label}</div>
      <div className={`mt-0.5 truncate text-sm ${accent ? 'text-cyan-200' : 'text-slate-200'}`}>
        {value || '—'}
      </div>
    </div>
  )
}

function Metric({
  label,
  value,
  className = '',
}: {
  label: string
  value: string
  className?: string
}) {
  return (
    <div className="rounded border border-slate-800/80 bg-slate-950/40 px-3 py-2">
      <div className="text-[10px] uppercase tracking-wider text-slate-500">{label}</div>
      <div className={`mt-0.5 font-mono text-sm text-slate-100 ${className}`}>{value}</div>
    </div>
  )
}

function TopList({
  title,
  items,
}: {
  title: string
  items: { entity: string; type: string; delta: number }[]
}) {
  return (
    <div>
      <div className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-slate-500">
        {title}
      </div>
      <ul className="space-y-1 text-xs">
        {items.map((it) => (
          <li key={it.entity} className="flex justify-between gap-2 text-slate-300">
            <span className="truncate">{it.entity}</span>
            <span className={`shrink-0 font-mono ${deltaClass(it.delta)}`}>
              {signed(it.delta)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}
