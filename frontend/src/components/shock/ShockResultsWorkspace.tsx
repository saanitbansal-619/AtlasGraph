import { Fragment, useEffect, useMemo, useState } from 'react'
import type {
  AffectedPath,
  CustomDataAnalysisResponse,
  ScenarioReportResponse,
  ShockResponse,
} from '../../types/api'
import { ASSUMPTION_NOTE, type SubmittedScenario } from '../../types/scenario'
import {
  normalizeCommodityRankings,
  sortCountriesByScore,
} from '../../lib/commodityNormalize'
import { computeClientExposureOverlay } from '../../lib/clientExposure'
import {
  blockedEdgeCategory,
  deltaClass,
  fixed,
  formatRelationship,
  pct,
  signed,
} from '../../lib/format'
import { selectTopImpactedEntity } from '../../lib/shockEntities'
import { AdaptiveRankingChart } from '../charts/AdaptiveRankingChart'
import { ClientExposureOverlayPanel } from '../ClientExposureOverlay'
import { ExecutiveImpactBrief } from '../ExecutiveImpactBrief'
import { RecommendedActionsPanel } from '../MitigationRecommendations'
import { ScenarioIntelligenceReport } from '../ScenarioIntelligenceReport'
import { InlineError } from '../States'
import { EmptyHint, Panel, Spinner } from '../ui'

export function ShockResultsWorkspace({
  result,
  submitted,
  running,
  error,
  clientData,
  scenarioReport,
  reportLoading,
  reportErr,
  onGenerateReport,
  canGenerateReport,
  onSaveScenario,
  scenarioSaved,
}: {
  result: ShockResponse | null
  submitted?: SubmittedScenario | null
  running: boolean
  error?: { message: string; hint?: string } | null
  clientData?: CustomDataAnalysisResponse | null
  scenarioReport: ScenarioReportResponse | null
  reportLoading: boolean
  reportErr?: { message: string; hint?: string } | null
  onGenerateReport: () => void
  canGenerateReport: boolean
  onSaveScenario: () => void
  scenarioSaved: boolean
}) {
  if (error && !running) {
    return (
      <Panel title="Results">
        <InlineError message={error.message} hint={error.hint} />
      </Panel>
    )
  }

  if (running && !result) {
    return (
      <Panel title="Results">
        <div className="flex items-center gap-3 py-12 text-sm text-slate-400">
          <Spinner />
          Propagating shock through the dependency graph…
        </div>
      </Panel>
    )
  }

  if (!result) {
    return (
      <Panel title="Results">
        <EmptyHint>
          Run a scenario from Scenario Setup to view executive summary, risk cards, and
          propagation results here.
        </EmptyHint>
      </Panel>
    )
  }

  const s = result.graph_impact_summary
  const topImpacted = selectTopImpactedEntity(result)
  const clientOverlay = computeClientExposureOverlay(
    clientData,
    result.scenario.source,
    result.scenario.commodity,
  )

  const countryRows = sortCountriesByScore(
    (result.highest_risk_entities?.countries ?? []).map((it) => ({
      label: it.entity,
      value: it.delta,
    })),
  )
  const commodityRows = normalizeCommodityRankings(
    (result.highest_risk_entities?.commodities ?? []).map((it) => ({
      label: it.entity,
      value: it.delta,
    })),
  )

  return (
    <div className={`space-y-4 ${running ? 'opacity-60' : ''}`}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <ResultBanner result={result} submitted={submitted} />
        <button
          type="button"
          className="btn-primary w-auto shrink-0 px-3 py-1.5 text-xs"
          onClick={onSaveScenario}
          disabled={scenarioSaved}
        >
          {scenarioSaved ? 'Saved for comparison' : 'Save scenario'}
        </button>
      </div>

      <ExecutiveImpactBrief result={result} clientOverlay={clientOverlay} />

      <div className="grid grid-cols-2 gap-3 min-[480px]:grid-cols-3 xl:grid-cols-5">
        <MetricCard label="Affected nodes" value={String(s.affected_nodes)} />
        <MetricCard label="Affected paths" value={String(s.affected_paths)} />
        <MetricCard
          label="Avg fragility Δ"
          value={signed(s.avg_fragility_delta)}
          valueClassName={deltaClass(s.avg_fragility_delta)}
        />
        <MetricCard
          label="Largest estimated impact Δ"
          value={signed(s.largest_single_impact_delta)}
          valueClassName={deltaClass(s.largest_single_impact_delta)}
        />
        <MetricCard
          label="Top model-estimated exposure"
          value={topImpacted.label}
          valueClassName={topImpacted.isDirectCommodityFallback ? 'text-slate-400' : undefined}
          small
          wrapperClassName="col-span-2 min-[480px]:col-span-1"
        />
      </div>

      <RiskDriversPanel result={result} clientData={clientData ?? null} />

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <AdaptiveRankingChart
          title="Countries"
          subtitle="Δ fragility · descending"
          valueLabel="Δ fragility"
          valueDigits={1}
          valueSuffix=" Δ"
          data={countryRows}
          emptyLabel="No country-level estimated impacts were detected for this scenario."
          topN={8}
        />
        <AdaptiveRankingChart
          title="Commodities"
          subtitle="Δ fragility · normalized"
          valueLabel="Δ fragility"
          valueDigits={1}
          valueSuffix=" Δ"
          data={commodityRows}
          emptyLabel="No strategic commodity impacts were detected for this scenario."
          topN={8}
        />
      </div>

      <ClientExposureOverlayPanel
        clientData={clientData ?? null}
        source={result.scenario.source}
        commodity={result.scenario.commodity}
      />

      <PropagationGraphPanel paths={result.affected_paths} result={result} />

      <ScenarioIntelligenceReport
        report={scenarioReport}
        loading={reportLoading}
        error={reportErr}
        onGenerate={onGenerateReport}
        canGenerate={canGenerateReport}
        title="Executive Intelligence Report"
      />

      <RecommendedActionsPanel
        result={result}
        clientData={clientData}
        report={scenarioReport}
      />
    </div>
  )
}

function RiskDriversPanel({
  result,
  clientData,
}: {
  result: ShockResponse
  clientData: CustomDataAnalysisResponse | null
}) {
  const [openKey, setOpenKey] = useState<string | null>(null)
  const drivers = useMemo(() => {
    const rows: Array<{ key: string; label: string; summary: string; details: string[] }> = []
    const fusion = result.data_fusion
    if (fusion?.data_sources?.length) {
      rows.push({
        key: 'fusion',
        label: 'Evidence sources',
        summary: `${fusion.data_sources.length} active evidence sources`,
        details: fusion.data_sources,
      })
    }
    if (result.operational_assumptions) {
      const o = result.operational_assumptions
      rows.push({
        key: 'operational',
        label: 'Operational assumptions',
        summary: o.explanation,
        details: [
          `Duration ${o.duration_days}d (${fixed(o.duration_factor, 2)}×)`,
          `Recovery ${o.recovery_speed} (${fixed(o.recovery_factor, 2)}×)`,
          `Substitutes ${o.substitute_availability} (${fixed(o.substitute_factor, 2)}×)`,
          `Inventory ${o.inventory_buffer_days}d (${fixed(o.inventory_factor, 2)}×)`,
        ],
      })
    }
    const overlay = computeClientExposureOverlay(
      clientData,
      result.scenario.source,
      result.scenario.commodity,
    )
    if (overlay && overlay.matchedCount > 0) {
      rows.push({
        key: 'client',
        label: 'Client supplier exposure',
        summary: `${overlay.matchedCount} matched importer groups`,
        details: overlay.exposures.slice(0, 5).map(
          (e) =>
            `${e.importer} · ${e.commodity} · share ${(e.supplier_share * 100).toFixed(0)}%` +
            (e.concentration_risk ? ` · risk ${e.concentration_risk}` : ''),
        ),
      })
    }
    if (fusion?.propagation_note) {
      rows.push({
        key: 'propagation',
        label: 'Propagation note',
        summary: fusion.propagation_note,
        details: [
          fusion.real_trade_edges_used ? 'UN Comtrade trade edges used' : 'Baseline trade edges',
          fusion.real_event_risk_used ? 'GDELT event-risk used' : 'No GDELT overlay',
          fusion.real_price_stress_used ? 'Pink Sheet price stress used' : 'No price stress overlay',
        ],
      })
    }
    return rows
  }, [result, clientData])

  if (drivers.length === 0) return null

  return (
    <Panel title="Risk Drivers" dense>
      <div className="divide-y divide-slate-800/80">
        {drivers.map((row) => {
          const open = openKey === row.key
          return (
            <div key={row.key}>
              <button
                type="button"
                className="flex w-full items-start justify-between gap-3 px-1 py-2.5 text-left hover:bg-slate-900/40"
                onClick={() => setOpenKey(open ? null : row.key)}
                aria-expanded={open}
              >
                <div className="min-w-0">
                  <div className="text-xs font-medium text-slate-200">{row.label}</div>
                  <div className="mt-0.5 text-[11px] leading-relaxed text-slate-500">
                    {row.summary}
                  </div>
                </div>
                <span className="shrink-0 font-mono text-[11px] text-slate-500">
                  {open ? '−' : '+'}
                </span>
              </button>
              {open && (
                <ul className="space-y-1 pb-2.5 pl-1 text-[11px] text-slate-400">
                  {row.details.map((detail) => (
                    <li key={detail} className="flex gap-2">
                      <span className="mt-1.5 h-1 w-1 shrink-0 rounded-full bg-slate-600" />
                      <span>{detail}</span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )
        })}
      </div>
    </Panel>
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
  const title =
    submitted?.title?.trim() || sc.name || `${sc.source} · ${result.shock_profile.type}`
  const warnings = result.warnings ?? []

  return (
    <div className="panel min-w-0 flex-1 space-y-2 px-4 py-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-slate-50">{title}</h3>
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
        </span>
      </div>
      {warnings.map((w, i) => (
        <div
          key={i}
          className="rounded border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-200/90"
        >
          {w}
        </div>
      ))}
      <p className="text-[11px] italic text-slate-500">{ASSUMPTION_NOTE}</p>
    </div>
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

const PATH_PREVIEW_LIMIT = 4

function PropagationGraphPanel({
  paths,
  result,
}: {
  paths: AffectedPath[]
  result: ShockResponse
}) {
  const [expanded, setExpanded] = useState(false)
  const sorted = useMemo(
    () =>
      [...paths].sort((a, b) => {
        if (b.end_impact !== a.end_impact) return b.end_impact - a.end_impact
        if (b.path_weight !== a.path_weight) return b.path_weight - a.path_weight
        return a.path.length - b.path.length
      }),
    [paths],
  )
  const total = sorted.length
  const canExpand = total > PATH_PREVIEW_LIMIT
  const visible = expanded || !canExpand ? sorted : sorted.slice(0, PATH_PREVIEW_LIMIT)

  useEffect(() => {
    setExpanded(false)
  }, [result])

  return (
    <Panel
      title={total === 0 ? 'Propagation Graph' : `Propagation Graph · ${visible.length} of ${total}`}
      noPad
      right={
        canExpand ? (
          <button
            type="button"
            className="text-[11px] text-cyan-300 hover:underline"
            onClick={() => setExpanded((v) => !v)}
          >
            {expanded ? 'Show fewer' : 'Show all paths'}
          </button>
        ) : undefined
      }
    >
      {total === 0 ? (
        <div className="px-4 py-5 text-sm text-slate-500">No affected dependency paths.</div>
      ) : (
        <div className="divide-y divide-slate-800/70">
          {visible.map((p, index) => (
            <div key={`${p.labeled_path}-${index}`} className="px-4 py-3">
              <div className="text-xs leading-relaxed text-slate-200">
                {(p.path ?? []).map((node, i) => (
                  <Fragment key={`${node}-${i}`}>
                    {i > 0 && <span className="mx-1.5 text-slate-600">→</span>}
                    <span>{node}</span>
                  </Fragment>
                ))}
              </div>
              <div className="mt-1 flex flex-wrap gap-x-3 font-mono text-[10px] text-slate-500">
                <span>end impact {pct(p.end_impact)}</span>
                <span>weight {fixed(p.path_weight, 2)}</span>
                {(p.relationships ?? []).length > 0 && (
                  <span>
                    {(p.relationships ?? []).map(formatRelationship).join(' · ')}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {result.blocked_edges && result.blocked_edges.length > 0 && (
        <div className="border-t border-slate-800 px-4 py-3">
          <div className="label mb-2">Blocked branches</div>
          <ul className="space-y-1 text-[11px] text-slate-500">
            {result.blocked_edges.slice(0, 6).map((edge, i) => (
              <li key={`${edge.from}-${edge.to}-${i}`}>
                {edge.from} → {edge.to} · {formatRelationship(edge.relationship_type)} ·{' '}
                {blockedEdgeCategory(edge.reason)}
              </li>
            ))}
          </ul>
        </div>
      )}
    </Panel>
  )
}
