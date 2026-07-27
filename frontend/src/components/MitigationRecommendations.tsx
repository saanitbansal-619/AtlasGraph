import { useMemo } from 'react'
import type { CustomDataAnalysisResponse, MitigationRecommendation, ScenarioReportResponse, ShockResponse } from '../types/api'
import { executiveActionPlanForScenario } from '../lib/mitigationRecommendations'
import { fixed } from '../lib/format'
import { HorizontalBarChartCard } from './charts/HorizontalBarChartCard'
import { EmptyHint, Panel } from './ui'

function priorityBadgeClass(priority: string): string {
  switch (priority) {
    case 'Critical':
      return 'border-rose-500/60 bg-rose-500/20 text-rose-200'
    case 'High':
      return 'border-amber-500/50 bg-amber-500/15 text-amber-300'
    case 'Medium':
      return 'border-cyan-500/40 bg-cyan-500/10 text-cyan-300'
    default:
      return 'border-slate-600/40 bg-slate-600/10 text-slate-300'
  }
}

function levelBadgeClass(level: string): string {
  switch (level) {
    case 'High':
      return 'border-rose-500/40 bg-rose-500/10 text-rose-200'
    case 'Medium':
      return 'border-amber-500/40 bg-amber-500/10 text-amber-200'
    default:
      return 'border-emerald-500/40 bg-emerald-500/10 text-emerald-200'
  }
}

function formatMetricKey(key: string): string {
  return key.replace(/_/g, ' ')
}

function RecommendationCard({ item, rank }: { item: MitigationRecommendation; rank: number }) {
  const metrics = Object.entries(item.supporting_metrics ?? {}).slice(0, 4)
  return (
    <div className="rounded border border-slate-800/80 bg-slate-900/30 p-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-[10px] text-slate-500">#{rank}</span>
            <span className="rounded border border-slate-700 px-1.5 py-0.5 text-[10px] text-slate-400">
              {item.category}
            </span>
            <h4 className="text-sm font-medium text-slate-100">{item.title}</h4>
          </div>
          <p className="mt-2 text-xs leading-relaxed text-slate-400">{item.description}</p>
          <p className="mt-2 text-xs leading-relaxed text-slate-300">
            <span className="text-slate-500">Reason: </span>
            {item.reason}
          </p>
        </div>
        <span className={`badge shrink-0 ${priorityBadgeClass(item.priority)}`}>{item.priority}</span>
      </div>

      <div className="mt-3 grid gap-2 text-xs sm:grid-cols-3">
        <div className="rounded border border-slate-800/70 bg-slate-950/40 px-2.5 py-2">
          <div className="text-[10px] uppercase tracking-wide text-slate-500">Expected impact</div>
          <p className="mt-1 leading-relaxed text-slate-300">{item.expected_impact}</p>
        </div>
        <div className="rounded border border-slate-800/70 bg-slate-950/40 px-2.5 py-2">
          <div className="text-[10px] uppercase tracking-wide text-slate-500">Difficulty</div>
          <span className={`mt-1 inline-flex badge ${levelBadgeClass(item.implementation_difficulty)}`}>
            {item.implementation_difficulty}
          </span>
        </div>
        <div className="rounded border border-slate-800/70 bg-slate-950/40 px-2.5 py-2">
          <div className="text-[10px] uppercase tracking-wide text-slate-500">Confidence</div>
          <span className={`mt-1 inline-flex badge ${levelBadgeClass(item.confidence)}`}>
            {item.confidence}
          </span>
        </div>
      </div>

      {metrics.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {metrics.map(([key, value]) => (
            <span
              key={key}
              className="rounded-full border border-slate-800 px-2 py-0.5 font-mono text-[10px] text-slate-400"
            >
              {formatMetricKey(key)} {key.includes('share') || key.includes('percent') ? fixed(value, 0) : fixed(value, key === 'hhi' ? 3 : 1)}
              {key.includes('share') || key.includes('percent') ? '%' : ''}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}

function PriorityDistributionChart({
  distribution,
}: {
  distribution: { critical: number; high: number; medium: number; low: number }
}) {
  const data = [
    { label: 'Critical', value: distribution.critical, color: 'rgba(248,113,113,0.65)' },
    { label: 'High', value: distribution.high, color: 'rgba(251,191,36,0.60)' },
    { label: 'Medium', value: distribution.medium, color: 'rgba(34,211,238,0.55)' },
    { label: 'Low', value: distribution.low, color: 'rgba(148,163,184,0.50)' },
  ]
  const total = data.reduce((sum, row) => sum + row.value, 0)

  if (total === 0) {
    return (
      <div className="rounded border border-slate-800/80 p-3 text-xs text-slate-500">
        No recommendations to chart.
      </div>
    )
  }

  return (
    <HorizontalBarChartCard
      title="Recommendation Priority Distribution"
      subtitle="Count by urgency band"
      data={data}
      valueLabel="Recommendations"
      valueDigits={0}
      topN={4}
      height={160}
    />
  )
}

export function RecommendedActionsPanel({
  result,
  clientData,
  report,
}: {
  result: ShockResponse
  clientData?: CustomDataAnalysisResponse | null
  report?: ScenarioReportResponse | null
}) {
  const plan = useMemo(
    () => executiveActionPlanForScenario(result, clientData, report),
    [result, clientData, report],
  )

  return (
    <Panel title="Recommended Actions">
      <div className="space-y-4">
        <div className="rounded border border-cyan-900/40 bg-cyan-950/10 p-3">
          <div className="label mb-1">Executive Action Summary</div>
          <p className="text-sm leading-relaxed text-slate-300">{plan.summary}</p>
        </div>

        <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
          <div className="xl:col-span-2 space-y-3">
            <div className="label">Executive Recommendations</div>
            {plan.recommendations.length === 0 ? (
              <EmptyHint>
                No mitigation actions met the current rule thresholds for this scenario.
              </EmptyHint>
            ) : (
              plan.recommendations.map((item, index) => (
                <RecommendationCard key={item.id} item={item} rank={index + 1} />
              ))
            )}
          </div>

          <div className="space-y-3">
            <PriorityDistributionChart distribution={plan.priority_distribution} />

            <div className="rounded border border-slate-800/80 p-3">
              <div className="label mb-2">Estimated Business Impact</div>
              <p className="text-xs leading-relaxed text-slate-400">{plan.estimated_business_impact}</p>
            </div>

            <div className="rounded border border-slate-800/80 p-3">
              <div className="label mb-2">Supporting Evidence</div>
              <ul className="space-y-1.5 text-xs text-slate-400">
                {plan.supporting_evidence.map((line) => (
                  <li key={line} className="flex gap-2 leading-relaxed">
                    <span className="mt-1.5 h-1 w-1 shrink-0 rounded-full bg-slate-600" />
                    <span>{line}</span>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>
      </div>
    </Panel>
  )
}

// Backward-compatible export
export const MitigationRecommendationsPanel = RecommendedActionsPanel
