import { useEffect, useMemo, useState } from 'react'
import type {
  TradeConcentrationResponse,
  TradeDependencyResponse,
  TradeImportOption,
  TradeOptionsResponse,
  TradeSummaryResponse,
} from '../types/api'
import { fixed, pct, riskBadgeClass } from '../lib/format'
import { Panel, Spinner } from './ui'
import { InlineError } from './States'

const SUPPLIER_PREVIEW = 8
const DEFAULT_IMPORTER = 'United States'
const DEFAULT_COMMODITY = 'semiconductors'

function supplierShare(s: TradeDependencyResponse['suppliers'][number]): number {
  return s.share_pct ?? s.share * 100
}

function pickDefaultImporter(options: TradeImportOption[]): TradeImportOption | null {
  if (options.length === 0) return null
  return (
    options.find((c) => c.name.toLowerCase() === DEFAULT_IMPORTER.toLowerCase()) ?? options[0]
  )
}

function pickCommodity(options: string[], preferred: string): string {
  if (options.length === 0) return ''
  if (preferred && options.includes(preferred)) return preferred
  const semi = options.find((c) => c.toLowerCase() === DEFAULT_COMMODITY)
  return semi ?? options[0]
}

export function TradeSignalsPanel({
  summary,
  summaryLoading,
  summaryError,
  options,
  optionsLoading,
  optionsError,
  fetchDependency,
  fetchConcentration,
}: {
  summary: TradeSummaryResponse | null
  summaryLoading: boolean
  summaryError?: { message: string; hint?: string } | null
  options: TradeOptionsResponse | null
  optionsLoading: boolean
  optionsError?: { message: string; hint?: string } | null
  fetchDependency: (importer: string, commodity: string) => Promise<TradeDependencyResponse>
  fetchConcentration: (importer: string, commodity: string) => Promise<TradeConcentrationResponse>
}) {
  const [selectedImporter, setSelectedImporter] = useState('')
  const [selectedCommodity, setSelectedCommodity] = useState('')
  const [dependency, setDependency] = useState<TradeDependencyResponse | null>(null)
  const [concentration, setConcentration] = useState<TradeConcentrationResponse | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState(false)

  const importerEntries = useMemo(() => {
    if (options?.importers?.length) {
      return options.importers.filter((im) => im.name.trim() && im.commodities.length > 0)
    }
    // Fallback from summary when options endpoint is unavailable.
    const fromAvailable = summary?.available_importers?.map((c) => c.trim()).filter(Boolean) ?? []
    const names =
      fromAvailable.length > 0
        ? fromAvailable
        : (summary?.top_importers ?? []).map((c) => (c.name || c.code).trim()).filter(Boolean)
    const commodities =
      summary?.available_commodities?.map((c) => c.trim()).filter(Boolean) ??
      (summary?.top_commodities ?? []).map((c) => (c.name || c.code).trim()).filter(Boolean)
    return [...new Set(names)].map((name) => ({
      name,
      code: '',
      commodities: [...new Set(commodities)],
    }))
  }, [options, summary])

  const selectedEntry = useMemo(
    () => importerEntries.find((im) => im.name === selectedImporter) ?? null,
    [importerEntries, selectedImporter],
  )

  const commodityOptions = selectedEntry?.commodities ?? []

  const pairValid =
    Boolean(selectedImporter) &&
    Boolean(selectedCommodity) &&
    commodityOptions.includes(selectedCommodity)

  useEffect(() => {
    if (importerEntries.length === 0) {
      if (selectedImporter) setSelectedImporter('')
      return
    }
    if (!selectedImporter || !importerEntries.some((im) => im.name === selectedImporter)) {
      const next = pickDefaultImporter(importerEntries)
      setSelectedImporter(next?.name ?? '')
    }
  }, [importerEntries, selectedImporter])

  useEffect(() => {
    if (!selectedEntry) {
      if (selectedCommodity) setSelectedCommodity('')
      return
    }
    const next = pickCommodity(selectedEntry.commodities, selectedCommodity)
    if (next !== selectedCommodity) {
      setSelectedCommodity(next)
    }
  }, [selectedEntry, selectedCommodity])

  useEffect(() => {
    setExpanded(false)
  }, [selectedImporter, selectedCommodity])

  useEffect(() => {
    if (!pairValid) {
      setDependency(null)
      setConcentration(null)
      setDetailError(null)
      setDetailLoading(false)
      return
    }

    let cancelled = false
    setDetailLoading(true)
    setDetailError(null)
    setDependency(null)
    setConcentration(null)

    void Promise.all([
      fetchDependency(selectedImporter, selectedCommodity),
      fetchConcentration(selectedImporter, selectedCommodity),
    ])
      .then(([dep, con]) => {
        if (!cancelled) {
          setDependency(dep)
          setConcentration(con)
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setDependency(null)
          setConcentration(null)
          setDetailError(e instanceof Error ? e.message : 'Failed to load trade dependency data')
        }
      })
      .finally(() => {
        if (!cancelled) setDetailLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [pairValid, selectedImporter, selectedCommodity, fetchDependency, fetchConcentration])

  const sortedSuppliers = useMemo(() => {
    if (!dependency) return []
    return [...dependency.suppliers].sort((a, b) => {
      const diff = supplierShare(b) - supplierShare(a)
      if (diff !== 0) return diff
      return b.value_usd - a.value_usd
    })
  }, [dependency])

  const total = sortedSuppliers.length
  const canExpand = total > SUPPLIER_PREVIEW
  const visible =
    expanded || !canExpand ? sortedSuppliers : sortedSuppliers.slice(0, SUPPLIER_PREVIEW)
  const showing = visible.length

  const panelError = optionsError ?? summaryError
  const panelLoading = optionsLoading || summaryLoading

  if (panelError && !options && !summary) {
    return (
      <Panel title="Trade Dependency Signals" className="h-full min-w-0" dense>
        <InlineError message={panelError.message} hint={panelError.hint} />
      </Panel>
    )
  }

  const real =
    options?.real_trade_data ??
    summary?.real_trade_data ??
    dependency?.real_trade_data ??
    concentration?.real_trade_data ??
    false
  const badge = real ? (
    <span className="badge border-emerald-500/40 bg-emerald-500/10 text-emerald-300">
      Real trade data
    </span>
  ) : (
    <span className="badge border-slate-600/60 bg-slate-800/40 text-slate-400">
      Sample trade data
    </span>
  )

  const sourceLabel = options?.source || summary?.source || 'unknown'

  return (
    <Panel
      title="Trade Dependency Signals"
      className="h-full min-w-0"
      dense
      right={
        <div className="flex items-center gap-2">
          {badge}
          {(panelLoading || detailLoading) && <Spinner className="h-3 w-3" />}
        </div>
      }
      noPad
    >
      {panelLoading && !options && !summary ? (
        <div className="flex items-center gap-3 px-3 py-5 text-sm text-slate-400">
          <Spinner />
          Loading trade dependency signals…
        </div>
      ) : importerEntries.length === 0 ? (
        <p className="px-3 py-4 text-sm text-slate-500">
          No importer trade dependency options available yet. Ingest UN Comtrade import data on the
          backend.
        </p>
      ) : (
        <div className={`space-y-3 ${panelLoading || detailLoading ? 'opacity-70' : ''}`}>
          <p className="border-b border-slate-800/60 px-3 py-1.5 text-[11px] text-slate-500">
            Source: {sourceLabel}
            {summary ? (
              <>
                {' '}
                · {summary.records} flows · {summary.commodities} commodities
              </>
            ) : null}
          </p>

          <div className="space-y-2 px-3 pb-3">
            <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-end">
              <label className="flex min-w-0 flex-1 flex-col gap-1.5">
                <span className="text-xs text-slate-400">Importer</span>
                <select
                  className="field min-w-0"
                  value={selectedImporter}
                  onChange={(e) => setSelectedImporter(e.target.value)}
                  disabled={detailLoading}
                >
                  {importerEntries.map((im) => (
                    <option key={im.code || im.name} value={im.name}>
                      {im.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="flex min-w-0 flex-1 flex-col gap-1.5">
                <span className="text-xs text-slate-400">Commodity</span>
                <select
                  className="field min-w-0"
                  value={selectedCommodity}
                  onChange={(e) => setSelectedCommodity(e.target.value)}
                  disabled={detailLoading || commodityOptions.length === 0}
                >
                  {commodityOptions.map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-sm text-slate-300">
                <span className="font-medium text-slate-100">
                  {selectedImporter} imports {selectedCommodity || '…'}
                </span>
                {concentration && !detailLoading && sortedSuppliers.length > 0 && (
                  <span className="ml-2">
                    <span className={`badge ${riskBadgeClass(concentration.concentration_risk)}`}>
                      {concentration.concentration_risk} concentration
                    </span>
                  </span>
                )}
              </p>
              {canExpand && !detailLoading && !detailError && dependency && (
                <button
                  type="button"
                  onClick={() => setExpanded((v) => !v)}
                  className="rounded border border-slate-700/60 bg-slate-950/40 px-2.5 py-1 text-[11px] font-medium text-slate-400 transition hover:border-slate-600 hover:text-slate-300"
                  aria-expanded={expanded}
                >
                  {expanded ? 'Show fewer' : 'Show all suppliers'}
                </button>
              )}
            </div>

            {!pairValid ? (
              <p className="rounded border border-slate-800/80 bg-slate-950/40 px-3 py-4 text-sm text-slate-500">
                No supplier data available for this importer and commodity.
              </p>
            ) : detailLoading && !dependency ? (
              <div className="flex items-center gap-3 py-8 text-sm text-slate-400">
                <Spinner />
                Loading supplier breakdown…
              </div>
            ) : detailError ? (
              <p className="rounded border border-slate-800/80 bg-slate-950/40 px-3 py-4 text-sm text-slate-500">
                {detailError}
              </p>
            ) : !dependency || sortedSuppliers.length === 0 ? (
              <p className="rounded border border-slate-800/80 bg-slate-950/40 px-3 py-4 text-sm text-slate-500">
                No supplier data available for this importer and commodity.
              </p>
            ) : (
              <>
                <p className="text-[11px] text-slate-500">
                  Top suppliers · showing {showing} of {total}
                </p>

                <div className="rounded border border-slate-800/60">
                  <table className="w-full border-collapse text-sm">
                    <thead>
                      <tr className="border-b border-slate-800">
                        <th className="th text-left">Supplier</th>
                        <th className="th text-right">Share</th>
                        <th className="th text-right">Value</th>
                      </tr>
                    </thead>
                    <tbody>
                      {visible.map((s) => (
                        <tr
                          key={s.exporter_code || s.exporter_name}
                          className="border-b border-slate-800/60 last:border-b-0 hover:bg-slate-800/30"
                        >
                          <td className="td font-medium text-slate-100">{s.exporter_name}</td>
                          <td className="td text-right font-mono text-xs text-cyan-300">
                            {s.share_pct != null ? `${fixed(s.share_pct, 1)}%` : pct(s.share, 1)}
                          </td>
                          <td className="td text-right font-mono text-xs text-slate-400">
                            {fixed(s.value_usd / 1e6, 1)}M
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </Panel>
  )
}
