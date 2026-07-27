import { useMemo } from 'react'
import type { CustomDataAnalysisResponse, ScenarioReportResponse, ShockResponse } from '../types/api'
import {
  buildMitigationInputs,
  mitigationRecommendationsForScenario,
  type MitigationConfidence,
  type MitigationDifficulty,
  type MitigationRecommendation,
} from '../lib/mitigationRecommendations'
import { fixed, riskBadgeClass } from '../lib/format'
import { EmptyHint, Panel } from './ui'

function priorityBadgeClass(priority: number): string {
  if (priority >= 75) return 'border-rose-500/50 bg-rose-500/15 text-rose-300'
  if (priority >= 55) return 'border-amber-500/50 bg-amber-500/15 text-amber-300'
  return 'border-cyan-500/40 bg-cyan-500/10 text-cyan-300'
}

function levelBadgeClass(level: MitigationDifficulty | MitigationConfidence): string {
  switch (level) {
    case 'High':
      return 'border-rose-500/40 bg-rose-500/10 text-rose-200'
    case 'Medium':
      return 'border-amber-500/40 bg-amber-500/10 text-amber-200'
    default:
      return 'border-emerald-500/40 bg-emerald-500/10 text-emerald-200'
  }
}

function RecommendationCard({ item, rank }: { item: MitigationRecommendation; rank: number }) {
  return (
    <div className="rounded border border-slate-800/80 bg-slate-900/30 p-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-[10px] text-slate-500">#{rank}</span>
            <h4 className="text-sm font-medium text-slate-100">{item.title}</h4>
          </div>
          <p className="mt-2 text-xs leading-relaxed text-slate-400">{item.reason}</p>
        </div>
        <span className={`badge shrink-0 ${priorityBadgeClass(item.priority)}`}>
          P{item.priority}
        </span>
      </div>

      <div className="mt-3 grid gap-2 text-xs sm:grid-cols-3">
        <div className="rounded border border-slate-800/70 bg-slate-950/40 px-2.5 py-2">
          <div className="text-[10px] uppercase tracking-wide text-slate-500">Expected impact</div>
          <p className="mt-1 leading-relaxed text-slate-300">{item.expectedImpact}</p>
        </div>
        <div className="rounded border border-slate-800/70 bg-slate-950/40 px-2.5 py-2">
          <div className="text-[10px] uppercase tracking-wide text-slate-500">Difficulty</div>
          <span className={`mt-1 inline-flex badge ${levelBadgeClass(item.difficulty)}`}>
            {item.difficulty}
          </span>
        </div>
        <div className="rounded border border-slate-800/70 bg-slate-950/40 px-2.5 py-2">
          <div className="text-[10px] uppercase tracking-wide text-slate-500">Confidence</div>
          <span className={`mt-1 inline-flex badge ${levelBadgeClass(item.confidence)}`}>
            {item.confidence}
          </span>
        </div>
      </div>
    </div>
  )
}

function InputSummary({ inputs }: { inputs: ReturnType<typeof buildMitigationInputs> }) {
  return (
    <div className="flex flex-wrap gap-1.5">
      <span className="rounded-full border border-slate-800 px-2 py-0.5 text-[10px] text-slate-400">
        HHI {inputs.hhi == null ? '—' : fixed(inputs.hhi, 3)}
      </span>
      <span className="rounded-full border border-slate-800 px-2 py-0.5 text-[10px] text-slate-400">
        Supplier share {inputs.supplierShare == null ? '—' : `${(inputs.supplierShare * 100).toFixed(0)}%`}
      </span>
      <span className={`rounded-full border px-2 py-0.5 text-[10px] ${riskBadgeClass(inputs.eventRisk >= 70 ? 'High' : inputs.eventRisk >= 45 ? 'Medium' : 'Low')}`}>
        Event risk {fixed(inputs.eventRisk, 0)}
      </span>
      <span className={`rounded-full border px-2 py-0.5 text-[10px] ${riskBadgeClass(inputs.macroRisk >= 70 ? 'High' : inputs.macroRisk >= 45 ? 'Medium' : 'Low')}`}>
        Macro risk {fixed(inputs.macroRisk, 0)}
      </span>
      <span className="rounded-full border border-slate-800 px-2 py-0.5 text-[10px] text-slate-400">
        {inputs.commodity} · {inputs.shockType.replace(/_/g, ' ')} · {fixed(inputs.dropPercent, 0)}% drop
      </span>
    </div>
  )
}

export function MitigationRecommendationsPanel({
  result,
  clientData,
  report,
}: {
  result: ShockResponse
  clientData?: CustomDataAnalysisResponse | null
  report?: ScenarioReportResponse | null
}) {
  const inputs = useMemo(
    () => buildMitigationInputs(result, clientData, report),
    [result, clientData, report],
  )
  const recommendations = useMemo(
    () => mitigationRecommendationsForScenario(result, clientData, report),
    [result, clientData, report],
  )

  return (
    <Panel title="Mitigation Recommendations">
      <div className="space-y-4">
        <div>
          <p className="text-xs leading-relaxed text-slate-500">
            Rule-based mitigation actions ranked by priority from concentration, supplier dependence,
            event and macro risk, commodity profile, shock type, and modeled drop severity.
          </p>
          <div className="mt-3">
            <InputSummary inputs={inputs} />
          </div>
        </div>

        {recommendations.length === 0 ? (
          <EmptyHint>
            No mitigation actions met the current rule thresholds for this scenario. Lower shock
            severity or upload client overlay data for concentration-driven recommendations.
          </EmptyHint>
        ) : (
          <div className="space-y-3">
            {recommendations.map((item, index) => (
              <RecommendationCard key={item.category} item={item} rank={index + 1} />
            ))}
          </div>
        )}
      </div>
    </Panel>
  )
}
