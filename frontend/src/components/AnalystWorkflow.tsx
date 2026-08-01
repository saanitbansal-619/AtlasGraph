import type { AppTab } from '../types/navigation'
import type { CustomDataAnalysisResponse, ScenarioReportResponse, ShockResponse } from '../types/api'
import { computeClientExposureOverlay, formatCompactUSD } from '../lib/clientExposure'
import { SectionCard } from './shared/SectionCard'

export type WorkflowStepId = 'client' | 'shock' | 'exposure' | 'report'

type StepStatus = 'done' | 'current' | 'todo'

function stepStatus(done: boolean, current: boolean): StepStatus {
  if (done) return 'done'
  if (current) return 'current'
  return 'todo'
}

function statusClass(status: StepStatus): string {
  switch (status) {
    case 'done':
      return 'border-emerald-500/40 bg-emerald-500/10 text-emerald-200'
    case 'current':
      return 'border-cyan-500/40 bg-cyan-500/10 text-cyan-100'
    default:
      return 'border-slate-700/70 bg-slate-900/40 text-slate-400'
  }
}

/**
 * Guided path: upload client data → run shock → review exposure → generate report/actions.
 * Navigates existing tabs rather than duplicating their UIs.
 */
export function AnalystWorkflow({
  clientAnalysis,
  result,
  scenarioReport,
  onNavigate,
  onLoadCaseStudy,
}: {
  clientAnalysis: CustomDataAnalysisResponse | null
  result: ShockResponse | null
  scenarioReport: ScenarioReportResponse | null
  onNavigate: (tab: AppTab, opts?: { shockTab?: 'setup' | 'results' | 'comparison' }) => void
  onLoadCaseStudy?: () => void
}) {
  const overlay =
    result != null
      ? computeClientExposureOverlay(
          clientAnalysis,
          result.scenario.source,
          result.scenario.commodity,
          result.scenario.shock_percent,
        )
      : null

  const hasClient = !!clientAnalysis
  const hasShock = !!result
  const hasExposure = !!overlay && overlay.matchedCount > 0
  const hasReport = !!scenarioReport

  const steps: Array<{
    id: WorkflowStepId
    label: string
    detail: string
    status: StepStatus
    actionLabel: string
    onAction: () => void
  }> = [
    {
      id: 'client',
      label: '1. Client portfolio',
      detail: hasClient
        ? `${clientAnalysis!.dataset_summary.valid_rows} valid rows · ${clientAnalysis!.concentration_results.length} concentration groups`
        : 'Upload supplier-dependency CSV (optional but recommended).',
      status: stepStatus(hasClient, !hasClient),
      actionLabel: hasClient ? 'Review client data' : 'Upload client data',
      onAction: () => onNavigate('client'),
    },
    {
      id: 'shock',
      label: '2. Run shock',
      detail: hasShock
        ? `${result!.scenario.source} → ${result!.scenario.commodity} · ${result!.scenario.shock_percent}%`
        : 'Configure and run a geopolitical / supply shock.',
      status: stepStatus(hasShock, hasClient && !hasShock),
      actionLabel: hasShock ? 'Open shock results' : 'Configure shock',
      onAction: () => onNavigate('shock', { shockTab: hasShock ? 'results' : 'setup' }),
    },
    {
      id: 'exposure',
      label: '3. Client dollars at risk',
      detail: hasExposure
        ? `${formatCompactUSD(overlay!.totalEstimatedExposedTrade)} estimated exposed · ${overlay!.matchedCount} matched importers`
        : hasShock && hasClient
          ? 'No client rows matched this shock source/commodity.'
          : 'Requires client data + a completed shock.',
      status: stepStatus(hasExposure, hasShock && hasClient && !hasExposure),
      actionLabel: hasShock ? 'View exposure' : 'Run shock first',
      onAction: () => onNavigate('shock', { shockTab: 'results' }),
    },
    {
      id: 'report',
      label: '4. Report & actions',
      detail: hasReport
        ? 'Executive intelligence report ready with mitigation recommendations.'
        : 'Generate the scenario report and review ranked actions.',
      status: stepStatus(hasReport, hasShock && !hasReport),
      actionLabel: hasReport ? 'View report' : 'Generate report',
      onAction: () => onNavigate('shock', { shockTab: 'results' }),
    },
  ]

  return (
    <SectionCard
      title="Analyst Workflow"
      dense
      right={
        onLoadCaseStudy ? (
          <button
            type="button"
            className="text-[11px] font-medium text-cyan-300 hover:underline"
            onClick={onLoadCaseStudy}
          >
            Load Taiwan case study
          </button>
        ) : undefined
      }
    >
      <p className="mb-3 text-xs leading-relaxed text-slate-400">
        Purpose path: portfolio → shock → dollars at risk → explainable actions. Outputs are
        model-derived estimates under stated assumptions, not forecasts.
      </p>
      <ol className="grid gap-2 md:grid-cols-2 xl:grid-cols-4">
        {steps.map((step) => (
          <li
            key={step.id}
            className={`flex flex-col rounded border px-3 py-2.5 ${statusClass(step.status)}`}
          >
            <div className="text-xs font-semibold">{step.label}</div>
            <p className="mt-1 flex-1 text-[11px] leading-snug opacity-90">{step.detail}</p>
            <button
              type="button"
              className="mt-2 self-start text-[11px] font-medium underline-offset-2 hover:underline"
              onClick={step.onAction}
              disabled={step.id === 'exposure' && !hasShock}
            >
              {step.actionLabel}
            </button>
          </li>
        ))}
      </ol>
    </SectionCard>
  )
}

export function ModelDisclaimer({ compact = false }: { compact?: boolean }) {
  return (
    <div
      className={`rounded border border-amber-500/25 bg-amber-500/[0.06] text-amber-100/90 ${
        compact ? 'px-2.5 py-1.5 text-[10px]' : 'px-3 py-2 text-[11px]'
      }`}
    >
      <span className="font-semibold text-amber-200">Model disclaimer: </span>
      GFIP produces deterministic estimates under stated assumptions and observed data panels. It
      does not predict future events, account for unmodeled substitutes, or replace human judgment.
    </div>
  )
}
