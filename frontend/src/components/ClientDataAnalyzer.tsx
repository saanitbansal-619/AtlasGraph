import { useState } from 'react'
import { api, ApiRequestError } from '../lib/api'
import type { CustomDataAnalysisResponse } from '../types/api'
import { compactInt, fixed, pct, riskBadgeClass } from '../lib/format'
import { InlineError } from './States'
import { EmptyHint, Panel, Spinner } from './ui'

const SCHEMA_EXAMPLE = `importer,commodity,supplier,value_usd
United States,semiconductors,Taiwan,75000000
United States,semiconductors,Korea,25000000`

export function ClientDataAnalyzer({
  onAnalyzed,
}: {
  onAnalyzed?: (result: CustomDataAnalysisResponse | null) => void
} = {}) {
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<CustomDataAnalysisResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<{ message: string; hint?: string } | null>(null)

  const analyze = async () => {
    if (!file) return
    setLoading(true)
    setError(null)
    try {
      const analysis = await api.analyzeCustomData(file)
      setResult(analysis)
      onAnalyzed?.(analysis)
    } catch (e) {
      setResult(null)
      onAnalyzed?.(null)
      setError(
        e instanceof ApiRequestError
          ? { message: e.message, hint: e.hint }
          : { message: e instanceof Error ? e.message : 'Custom data analysis failed' },
      )
    } finally {
      setLoading(false)
    }
  }

  return (
    <Panel
      title="Client Data Analyzer"
      right={
        <span className="badge border-violet-500/40 bg-violet-500/10 text-violet-300">
          Custom CSV
        </span>
      }
    >
      <div className="space-y-4">
        <p className="text-xs leading-relaxed text-slate-400">
          Use this to test GFIP on client-specific supplier data. Uploaded data is normalized and
          scored using supplier concentration metrics.
        </p>

        <div className="grid gap-3 lg:grid-cols-[minmax(240px,0.8fr)_minmax(0,1.2fr)]">
          <div className="space-y-2">
            <label className="label block" htmlFor="client-data-csv">
              Supplier dependency CSV
            </label>
            <input
              id="client-data-csv"
              type="file"
              accept=".csv,text/csv"
              className="field file:mr-3 file:rounded file:border-0 file:bg-cyan-500/15 file:px-2 file:py-1 file:text-xs file:font-medium file:text-cyan-200"
              onChange={(event) => {
                setFile(event.target.files?.[0] ?? null)
                setResult(null)
                onAnalyzed?.(null)
                setError(null)
              }}
            />
            <button
              type="button"
              className="btn-primary w-full"
              disabled={!file || loading}
              onClick={() => void analyze()}
            >
              {loading ? (
                <>
                  <Spinner className="h-3.5 w-3.5" />
                  Analyzing…
                </>
              ) : (
                'Analyze Client Data'
              )}
            </button>
          </div>
          <div>
            <div className="label mb-1.5">Required CSV schema</div>
            <pre className="overflow-x-auto rounded border border-slate-800 bg-slate-950/70 p-3 text-[11px] leading-relaxed text-cyan-200">
              <code>{SCHEMA_EXAMPLE}</code>
            </pre>
            <p className="mt-1.5 text-[10px] text-slate-500">
              Aliases accepted: importer_name, supplier_name, exporter, trade_value_usd, value.
            </p>
          </div>
        </div>

        {error && <InlineError message={error.message} hint={error.hint} />}

        {!result && !error && !loading && (
          <EmptyHint>Select a CSV and analyze it to view supplier concentration results.</EmptyHint>
        )}

        {result && <AnalysisResults result={result} />}
      </div>
    </Panel>
  )
}

function AnalysisResults({ result }: { result: CustomDataAnalysisResponse }) {
  const summary = result.dataset_summary
  const metrics = [
    ['Rows processed', summary.rows_processed],
    ['Valid rows', summary.valid_rows],
    ['Invalid rows', summary.invalid_rows],
    ['Importers', summary.importers],
    ['Commodities', summary.commodities],
    ['Suppliers', summary.suppliers],
    ['Total value', formatUSD(summary.total_value_usd)],
  ]
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 xl:grid-cols-7">
        {metrics.map(([label, value]) => (
          <div key={String(label)} className="rounded border border-slate-800 bg-slate-900/30 px-3 py-2">
            <div className="text-[10px] uppercase tracking-wide text-slate-500">{label}</div>
            <div className="mt-1 font-mono text-lg font-semibold text-slate-100">
              {typeof value === 'number' ? compactInt(value) : value}
            </div>
          </div>
        ))}
      </div>

      {result.validation_errors.length > 0 && (
        <div className="rounded border border-amber-500/30 bg-amber-500/[0.06] px-3 py-2">
          <div className="text-xs font-semibold text-amber-200">
            {result.validation_errors.length} validation issue
            {result.validation_errors.length === 1 ? '' : 's'}
          </div>
          <ul className="mt-1 space-y-0.5 text-[11px] text-amber-200/70">
            {result.validation_errors.slice(0, 10).map((issue, index) => (
              <li key={`${issue.row}-${issue.field}-${index}`}>
                Row {issue.row} · {issue.field}: {issue.message}
              </li>
            ))}
          </ul>
        </div>
      )}

      {result.concentration_results.length === 0 ? (
        <EmptyHint>No valid supplier groups were available for concentration analysis.</EmptyHint>
      ) : (
        <div className="max-h-96 overflow-auto rounded border border-slate-800">
          <table className="w-full min-w-[920px] text-left text-xs">
            <thead className="sticky top-0 bg-slate-900/95 backdrop-blur">
              <tr className="border-b border-slate-800 text-slate-500">
                {['Importer', 'Commodity', 'Total value', 'Supplier count', 'Top supplier', 'Top supplier share', 'HHI', 'Risk'].map(
                  (label) => (
                    <th key={label} className="px-3 py-2 font-medium">
                      {label}
                    </th>
                  ),
                )}
              </tr>
            </thead>
            <tbody>
              {result.concentration_results.map((row) => (
                <tr
                  key={`${row.importer}-${row.commodity}`}
                  className="border-b border-slate-800/60"
                >
                  <td className="px-3 py-2 text-slate-200">{row.importer}</td>
                  <td className="px-3 py-2 text-amber-200">{row.commodity}</td>
                  <td className="px-3 py-2 font-mono text-slate-300">
                    {formatUSD(row.total_value_usd)}
                  </td>
                  <td className="px-3 py-2 font-mono text-slate-300">{row.supplier_count}</td>
                  <td className="px-3 py-2 text-slate-200">{row.top_supplier}</td>
                  <td className="px-3 py-2 font-mono text-slate-300">
                    {pct(row.top_supplier_share)}
                  </td>
                  <td className="px-3 py-2 font-mono text-slate-300">{fixed(row.hhi, 3)}</td>
                  <td className="px-3 py-2">
                    <span className={`badge ${riskBadgeClass(row.concentration_risk)}`}>
                      {row.concentration_risk}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function formatUSD(value: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(value)
}
