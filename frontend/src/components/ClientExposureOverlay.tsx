import type { CustomDataAnalysisResponse } from '../types/api'
import {
  computeClientExposureOverlay,
  type ClientExposureOverlay,
} from '../lib/clientExposure'
import { fixed, pct, riskBadgeClass } from '../lib/format'
import { EmptyHint, Panel } from './ui'

function formatUSD(value: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(value)
}

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

function OverlaySummary({ overlay }: { overlay: ClientExposureOverlay }) {
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
        <SummaryCard label="Matched client exposures" value={String(overlay.matchedCount)} />
        <SummaryCard
          label="Top exposed importer"
          value={overlay.topImporter ?? '—'}
          tone="violet"
        />
        <SummaryCard
          label="Shocked supplier share"
          value={pct(overlay.topShare)}
          tone="amber"
        />
        <SummaryCard
          label="Concentration risk"
          value={overlay.topRisk ?? '—'}
          tone={overlay.topRisk === 'High' ? 'amber' : 'cyan'}
        />
      </div>

      <div className="max-h-72 overflow-auto rounded border border-slate-800">
        <table className="w-full min-w-[820px] text-left text-xs">
          <thead className="sticky top-0 bg-slate-900/95 backdrop-blur">
            <tr className="border-b border-slate-800 text-slate-500">
              {[
                'Importer',
                'Commodity',
                'Shocked supplier',
                'Supplier share',
                'Supplier value',
                'Total value',
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
                  {formatUSD(row.supplier_value_usd)}
                </td>
                <td className="px-3 py-2 font-mono text-slate-300">
                  {formatUSD(row.total_import_value_usd)}
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
}: {
  clientData: CustomDataAnalysisResponse | null
  source: string
  commodity: string
}) {
  const overlay = computeClientExposureOverlay(clientData, source, commodity)

  return (
    <Panel
      title="Client Exposure Overlay"
      dense
      right={
        <span className="badge border-violet-500/40 bg-violet-500/10 text-violet-300">
          Client CSV
        </span>
      }
    >
      {!clientData && (
        <EmptyHint>
          Upload client supplier data to compare public scenario exposure against client-specific
          dependencies.
        </EmptyHint>
      )}

      {clientData && overlay && overlay.matchedCount === 0 && (
        <EmptyHint>
          No client-specific exposure matched this shock source and commodity.
        </EmptyHint>
      )}

      {clientData && overlay && overlay.matchedCount > 0 && <OverlaySummary overlay={overlay} />}
    </Panel>
  )
}
