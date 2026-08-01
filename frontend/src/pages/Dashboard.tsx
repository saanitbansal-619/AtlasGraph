import type {
  CustomDataAnalysisResponse,
  FragilitySummaryResponse,
  GraphSummaryResponse,
  ScenarioReportResponse,
  ShockResponse,
} from '../types/api'
import type { AppTab } from '../types/navigation'
import { compactInt } from '../lib/format'
import { DataSourcesCard } from '../components/DataSourcesCard'
import { OverviewCards } from '../components/OverviewCards'
import { ExecutiveImpactBrief } from '../components/ExecutiveImpactBrief'
import { ScenarioIntelligenceReport } from '../components/ScenarioIntelligenceReport'
import { UnifiedFragility } from '../components/UnifiedFragility'
import { AnalystWorkflow, ModelDisclaimer } from '../components/AnalystWorkflow'
import { computeClientExposureOverlay, formatCompactUSD } from '../lib/clientExposure'
import { SectionCard } from '../components/shared/SectionCard'
import { StatCard } from '../components/shared/StatCard'
import { EmptyHint } from '../components/ui'

export function DashboardPage({
  summary,
  summaryLoading,
  summaryErr,
  fragility,
  fragilityLoading,
  fragilityErr,
  result,
  clientAnalysis,
  scenarioReport,
  reportLoading,
  reportErr,
  onGenerateReport,
  canGenerateReport,
  onNavigate,
  onLoadCaseStudy,
}: {
  summary: GraphSummaryResponse | null
  summaryLoading: boolean
  summaryErr?: { message: string; hint?: string } | null
  fragility: FragilitySummaryResponse | null
  fragilityLoading: boolean
  fragilityErr?: { message: string; hint?: string } | null
  result: ShockResponse | null
  clientAnalysis: CustomDataAnalysisResponse | null
  scenarioReport: ScenarioReportResponse | null
  reportLoading: boolean
  reportErr?: { message: string; hint?: string } | null
  onGenerateReport: () => void
  canGenerateReport: boolean
  onNavigate: (tab: AppTab, opts?: { shockTab?: 'setup' | 'results' | 'comparison' }) => void
  onLoadCaseStudy: () => void
}) {
  const clientOverlay = result
    ? computeClientExposureOverlay(
        clientAnalysis,
        result.scenario.source,
        result.scenario.commodity,
        result.scenario.shock_percent,
      )
    : null

  const topCountries = fragility?.countries?.slice(0, 3) ?? []
  const topCommodities = fragility?.commodities?.slice(0, 3) ?? []
  const exposedTrade =
    clientOverlay && clientOverlay.matchedCount > 0
      ? formatCompactUSD(clientOverlay.totalEstimatedExposedTrade)
      : '—'

  return (
    <div className="space-y-4">
      <AnalystWorkflow
        clientAnalysis={clientAnalysis}
        result={result}
        scenarioReport={scenarioReport}
        onNavigate={onNavigate}
        onLoadCaseStudy={onLoadCaseStudy}
      />

      <ModelDisclaimer />

      <div className="grid grid-cols-2 gap-2 md:grid-cols-5">
        <StatCard
          label="Graph nodes"
          value={summary ? compactInt(summary.nodes) : '—'}
          compact
        />
        <StatCard
          label="Dependencies"
          value={summary ? compactInt(summary.dependencies) : '—'}
          accent
          compact
        />
        <StatCard
          label="Fragile countries"
          value={fragility ? compactInt(fragility.countries?.length ?? 0) : '—'}
          compact
        />
        <StatCard label="Client datasets" value={clientAnalysis ? 'Loaded' : 'None'} compact />
        <StatCard label="Client $ at risk" value={exposedTrade} accent compact />
      </div>

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2 lg:items-stretch">
        <div className="min-w-0 lg:h-full">
          <DataSourcesCard
            summary={summary}
            fragility={fragility}
            loading={summaryLoading || fragilityLoading}
            compact
          />
        </div>
        <div className="min-w-0">
          <OverviewCards summary={summary} loading={summaryLoading} error={summaryErr} />
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        {result ? (
          <ExecutiveImpactBrief result={result} clientOverlay={clientOverlay} />
        ) : (
          <SectionCard title="Executive Summary" dense>
            <EmptyHint>
              Follow the Analyst Workflow above, or{' '}
              <button
                type="button"
                className="text-cyan-300 underline-offset-2 hover:underline"
                onClick={() => onNavigate('shock')}
              >
                open Shock Simulation
              </button>
              .
            </EmptyHint>
          </SectionCard>
        )}

        <SectionCard title="Top Risk Indicators" dense>
          {fragilityErr && !fragility ? (
            <EmptyHint>{fragilityErr.message}</EmptyHint>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2">
              <div>
                <div className="label mb-2">Countries</div>
                <ul className="space-y-1.5 text-xs text-slate-300">
                  {topCountries.length === 0 && (
                    <li className="text-slate-500">No country fragility scores loaded.</li>
                  )}
                  {topCountries.map((row) => (
                    <li key={row.country_name} className="flex justify-between gap-2">
                      <span>{row.country_name}</span>
                      <span className="font-mono text-cyan-200">{row.score.toFixed(1)}</span>
                    </li>
                  ))}
                </ul>
              </div>
              <div>
                <div className="label mb-2">Commodities</div>
                <ul className="space-y-1.5 text-xs text-slate-300">
                  {topCommodities.length === 0 && (
                    <li className="text-slate-500">No commodity fragility scores loaded.</li>
                  )}
                  {topCommodities.map((row) => (
                    <li key={row.commodity_name} className="flex justify-between gap-2">
                      <span>{row.commodity_name}</span>
                      <span className="font-mono text-amber-200">{row.score.toFixed(1)}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          )}
        </SectionCard>
      </div>

      <UnifiedFragility summary={fragility} loading={fragilityLoading} error={fragilityErr} />

      <ScenarioIntelligenceReport
        report={scenarioReport}
        loading={reportLoading}
        error={reportErr}
        onGenerate={onGenerateReport}
        canGenerate={canGenerateReport}
      />
    </div>
  )
}
