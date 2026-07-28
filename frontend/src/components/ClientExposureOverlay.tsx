import type { CustomDataAnalysisResponse } from '../types/api'
import {
  buildClientExposureAssessment,
  computeClientExposureOverlay,
  formatCompactUSD,
  type ClientExposureOverlay,
} from '../lib/clientExposure'
import { fixed, pct, riskBadgeClass } from '../lib/format'
import { HorizontalBarChartCard } from './charts/HorizontalBarChartCard'
import { EmptyHint, Panel } from './ui'

function SummaryCard({
  label,
  value,
  tone,
}: {
  label: string
  value: string
  tone?: 'cyan' | 'amber' | 'violet'
}) {
  const valueClass =
    tone === 'amber'
      ? 'text-amber-200'
      : tone === 'violet'
        ? 'text-violet-200'
        : 'text-cyan-200'
  return (
    <div className="rounded border border-slate-800 bg-slate-900/30 px-3 py-2">
      <div className="text-[10px] uppercase tracking-wide text-slate-500">{label}</div>
      <div className={`mt-1 font-mono text-lg font-semibold ${valueClass}`}>{value}</div>
    </div>
  )
}

function ExposureChart({ overlay }: { overlay: ClientExposureOverlay }) {
  const data = overlay.exposures.slice(0, 8).map((row) => ({
    label: row.importer,
    value: row.estimated_exposed_trade,
    color: 'rgba(167,139,250,0.55)',
  }))
  return (
    <HorizontalBarChartCard
      title="Top Client Exposure by Importer"
      subtitle="Estimated exposed trade"
      data={data}
      valueLabel="Exposed trade"
      valueDigits={1}
      topN={8}
      height={Math.max(160, data.length * 36)}
      valueSuffix=""
    />
  )
}

function OverlayContent({ overlay }: { overlay: ClientExposureOverlay }) {
  const assessment = overlay.assessment ?? buildClientExposureAssessment(overlay)

  return (
    <div className="space-y-4">
      {assessment && (
        <div className="rounded border border-violet-900/40 bg-violet-950/10 p-3">
          <div className="label mb-1">Client Exposure Assessment</div>
          <p className="text-sm leading-relaxed text-slate-300">{assessment}</p>
        </div>
      )}

      <div className="grid grid-cols-2 gap-2 xl:grid-cols-5">
        <SummaryCard label="Matched importers" value={String(overlay.matchedCount)} />
        <SummaryCard
          label="Estimated exposed trade"
          value={formatCompactUSD(overlay.totalEstimatedExposedTrade)}
          tone="amber"
        />
        <SummaryCard label="Highest supplier share" value={pct(overlay.topShare)} tone="violet" />
        <SummaryCard
          label="Highest HHI"
          value={overlay.highestHHI == null ? '—' : fixed(overlay.highestHHI, 3)}
        />
        <SummaryCard
          label="Average concentration risk"
          value={overlay.averageConcentrationRisk ?? '—'}
          tone={overlay.averageConcentrationRisk === 'High' ? 'amber' : 'cyan'}
        />
      </div>

      <ExposureChart overlay={overlay} />

      <div className="max-h-80 overflow-auto rounded border border-slate-800">
        <table className="w-full min-w-[980px] text-left text-xs">
          <thead className="sticky top-0 bg-slate-900/95 backdrop-blur">
            <tr className="border-b border-slate-800 text-slate-500">
              {[
                'Importer',
                'Commodity',
                'Supplier',
                'Supplier share',
                'Supplier value',
                'Estimated exposed trade',
                'HHI',
                'Risk',
              ].map((label) => (
                <th key={label} className="px-3 py-2 font-medium">
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {overlay.exposures.map((row) => (
              <tr
                key={`${row.importer}-${row.commodity}-${row.shocked_supplier}`}
                className="border-b border-slate-800/60"
              >
                <td className="px-3 py-2 text-slate-200">{row.importer}</td>
                <td className="px-3 py-2 text-amber-200">{row.commodity}</td>
                <td className="px-3 py-2 text-slate-200">{row.shocked_supplier}</td>
                <td className="px-3 py-2 font-mono text-slate-300">{pct(row.supplier_share)}</td>
                <td className="px-3 py-2 font-mono text-slate-300">
                  {formatCompactUSD(row.supplier_value_usd)}
                </td>
                <td className="px-3 py-2 font-mono text-amber-200">
                  {formatCompactUSD(row.estimated_exposed_trade)}
                </td>
                <td className="px-3 py-2 font-mono text-slate-300">
                  {row.hhi == null ? '—' : fixed(row.hhi, 3)}
                </td>
                <td className="px-3 py-2">
                  {row.concentration_risk ? (
                    <span className={`badge ${riskBadgeClass(row.concentration_risk)}`}>
                      {row.concentration_risk}
                    </span>
                  ) : (
                    <span className="text-slate-500">—</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export function ClientExposureOverlayPanel({
  clientData,
  source,
  commodity,
  dropPercent = 0,
}: {
  clientData: CustomDataAnalysisResponse | null
  source: string
  commodity: string
  dropPercent?: number
}) {
  const overlay = computeClientExposureOverlay(clientData, source, commodity, dropPercent)

  return (
    <Panel
      title="Client Exposure"
      dense
      right={
        clientData ? (
          <span className="badge border-violet-500/40 bg-violet-500/10 text-violet-300">
            Client CSV
          </span>
        ) : undefined
      }
    >
      {!clientData && (
        <EmptyHint>
          Upload client supplier data to assess organization-specific exposure.
        </EmptyHint>
      )}

      {clientData && overlay && overlay.matchedCount === 0 && (
        <EmptyHint>
          No client-specific exposure matched this shock source and commodity.
        </EmptyHint>
      )}

      {clientData && overlay && overlay.matchedCount > 0 && <OverlayContent overlay={overlay} />}
    </Panel>
  )
}
