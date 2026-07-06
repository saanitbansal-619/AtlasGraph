import { useEffect, useMemo, useState } from 'react'
import type {
  TradeConcentrationResponse,
  TradeDependencyResponse,
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

export function TradeSignalsPanel({
  summary,
  summaryLoading,
  summaryError,
  fetchDependency,
  fetchConcentration,
}: {
  summary: TradeSummaryResponse | null
  summaryLoading: boolean
  summaryError?: { message: string; hint?: string } | null
  fetchDependency: (importer: string, commodity: string) => Promise<TradeDependencyResponse>
  fetchConcentration: (importer: string, commodity: string) => Promise<TradeConcentrationResponse>
}) {
  const [selectedCommodity, setSelectedCommodity] = useState('')
  const [dependency, setDependency] = useState<TradeDependencyResponse | null>(null)
  const [concentration, setConcentration] = useState<TradeConcentrationResponse | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState(false)

  const commodityOptions = useMemo(() => {
    const fromAvailable = summary?.available_commodities?.map((c) => c.trim()).filter(Boolean) ?? []
    if (fromAvailable.length > 0) {
      return [...new Set(fromAvailable)]
    }
    const items = summary?.top_commodities ?? []
    const names = items.map((c) => (c.name || c.code).trim()).filter(Boolean)
    return [...new Set(names)]
  }, [summary])

  useEffect(() => {
    if (selectedCommodity || commodityOptions.length === 0) return
    const preferred =
      commodityOptions.find((c) => c.toLowerCase() === DEFAULT_COMMODITY) ?? commodityOptions[0]
    setSelectedCommodity(preferred)
  }, [commodityOptions, selectedCommodity])

  useEffect(() => {
    setExpanded(false)
  }, [selectedCommodity])

  useEffect(() => {
    if (!selectedCommodity) return

    let cancelled = false
    setDetailLoading(true)
    setDetailError(null)
    setDependency(null)
    setConcentration(null)

    void Promise.all([
      fetchDependency(DEFAULT_IMPORTER, selectedCommodity),
      fetchConcentration(DEFAULT_IMPORTER, selectedCommodity),
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
  }, [selectedCommodity, fetchDependency, fetchConcentration])

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

  if (summaryError && !summary) {
    return (
      <Panel title="Trade Dependency Signals">
        <InlineError message={summaryError.message} hint={summaryError.hint} />
      </Panel>
    )
  }

  const real =
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
      Demo trade data
    </span>
  )

  return (
    <Panel
      title="Trade Dependency Signals"
      right={
        <div className="flex items-center gap-2">
          {badge}
          {(summaryLoading || detailLoading) && <Spinner className="h-3 w-3" />}
        </div>
      }
      noPad
    >
      {summaryLoading && !summary ? (
        <div className="flex items-center gap-3 px-4 py-6 text-sm text-slate-400">
          <Spinner />
          Loading trade dependency signals…
        </div>
      ) : !summary ? (
        <p className="px-4 py-5 text-sm text-slate-500">No trade summary available.</p>
      ) : commodityOptions.length === 0 ? (
        <p className="px-4 py-5 text-sm text-slate-500">
          No trade commodities available yet. Ingest UN Comtrade data on the backend.
        </p>
      ) : (
        <div className={`space-y-3 ${summaryLoading || detailLoading ? 'opacity-70' : ''}`}>
          <p className="border-b border-slate-800/60 px-4 py-2 text-[11px] text-slate-500">
            Source: {summary.source || 'unknown'} · {summary.records} flows · {summary.commodities}{' '}
            commodities
          </p>

          <div className="space-y-2 px-4 pb-4">
            <label className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
              <span className="shrink-0 text-xs text-slate-400">Commodity</span>
              <select
                className="field min-w-0 flex-1 sm:max-w-xs"
                value={selectedCommodity}
                onChange={(e) => setSelectedCommodity(e.target.value)}
                disabled={detailLoading}
              >
                {commodityOptions.map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
            </label>

            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-sm text-slate-300">
                <span className="font-medium text-slate-100">
                  {DEFAULT_IMPORTER} imports {selectedCommodity}
                </span>
                {concentration && !detailLoading && (
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

            {detailLoading && !dependency ? (
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
                No supplier dependency data for {selectedCommodity} yet.
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
