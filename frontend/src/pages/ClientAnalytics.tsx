import type { CustomDataAnalysisResponse } from '../types/api'
import { ClientDataAnalyzer } from '../components/ClientDataAnalyzer'
import { SectionCard } from '../components/shared/SectionCard'
import { StatCard } from '../components/shared/StatCard'
import { compactInt } from '../lib/format'

export function ClientAnalyticsPage({
  clientAnalysis,
  onAnalyzed,
}: {
  clientAnalysis: CustomDataAnalysisResponse | null
  onAnalyzed: (result: CustomDataAnalysisResponse | null) => void
}) {
  const summary = clientAnalysis?.dataset_summary

  return (
    <div className="space-y-4">
      {summary && (
        <div className="grid grid-cols-2 gap-2 md:grid-cols-4 xl:grid-cols-6">
          <StatCard label="Valid rows" value={compactInt(summary.valid_rows)} compact />
          <StatCard label="Importers" value={compactInt(summary.importers)} compact />
          <StatCard label="Commodities" value={compactInt(summary.commodities)} compact />
          <StatCard label="Suppliers" value={compactInt(summary.suppliers)} compact />
          <StatCard
            label="Concentration groups"
            value={compactInt(clientAnalysis?.concentration_results.length ?? 0)}
            compact
          />
          <StatCard
            label="High-risk groups"
            value={compactInt(
              clientAnalysis?.concentration_results.filter((r) => r.concentration_risk === 'High')
                .length ?? 0,
            )}
            accent
            compact
          />
        </div>
      )}

      <ClientDataAnalyzer onAnalyzed={onAnalyzed} result={clientAnalysis} />

      {!clientAnalysis && (
        <SectionCard title="Client exposure statistics" dense>
          <p className="text-xs leading-relaxed text-slate-400">
            After a successful upload, supplier concentration results and HHI metrics appear above.
            Matched exposures also feed the Client Exposure Overlay inside Shock Simulation.
          </p>
        </SectionCard>
      )}
    </div>
  )
}
