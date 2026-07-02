import { useEffect, useState } from 'react'
import type { CommodityHistoryIndexResponse, CommodityHistoryResponse } from '../types/api'
import { CommodityPriceLineChart } from './charts/CommodityPriceLineChart'
import { Panel, Spinner } from './ui'
import { InlineError } from './States'

export function CommodityPriceHistory({
  index,
  loadingIndex,
  indexError,
  fetchHistory,
}: {
  index: CommodityHistoryIndexResponse | null
  loadingIndex: boolean
  indexError?: { message: string; hint?: string } | null
  fetchHistory: (commodity: string) => Promise<CommodityHistoryResponse>
}) {
  const commodities = index?.commodities ?? []
  const [selected, setSelected] = useState('')
  const [history, setHistory] = useState<CommodityHistoryResponse | null>(null)
  const [loadingHistory, setLoadingHistory] = useState(false)
  const [historyError, setHistoryError] = useState<string | null>(null)

  useEffect(() => {
    if (!selected && commodities.length > 0) {
      const preferred =
        commodities.find((c) => c.toLowerCase() === 'crude oil') ?? commodities[0]
      setSelected(preferred)
    }
  }, [commodities, selected])

  useEffect(() => {
    if (!selected) return
    let cancelled = false
    setLoadingHistory(true)
    setHistoryError(null)
    void fetchHistory(selected)
      .then((res) => {
        if (!cancelled) setHistory(res)
      })
      .catch((e) => {
        if (!cancelled) {
          setHistory(null)
          setHistoryError(e instanceof Error ? e.message : 'Failed to load price history')
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingHistory(false)
      })
    return () => {
      cancelled = true
    }
  }, [selected, fetchHistory])

  if (indexError && !index) {
    return (
      <Panel title="Commodity Price History">
        <InlineError message={indexError.message} hint={indexError.hint} />
      </Panel>
    )
  }

  return (
    <Panel
      title="Commodity Price History"
      right={
        <span className="text-[10px] font-mono uppercase tracking-wider text-slate-500">
          Monthly nominal price history
        </span>
      }
      noPad
    >
      <div className="space-y-3 px-4 py-3">
        {loadingIndex && !index ? (
          <div className="flex items-center gap-3 py-6 text-sm text-slate-400">
            <Spinner />
            Loading available commodities…
          </div>
        ) : commodities.length === 0 ? (
          <p className="py-4 text-sm text-slate-500">
            No commodity price history is available yet. Ingest World Bank Pink Sheet data on the
            backend.
          </p>
        ) : (
          <>
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <label className="flex min-w-0 flex-1 items-center gap-2 text-xs text-slate-400">
                <span className="shrink-0">Commodity</span>
                <select
                  className="field min-w-0 flex-1"
                  value={selected}
                  onChange={(e) => setSelected(e.target.value)}
                  disabled={loadingHistory}
                >
                  {commodities.map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
                </select>
              </label>
              <p className="text-[11px] text-slate-500">
                Source: {history?.source || index?.source || 'World Bank Pink Sheet'}
              </p>
            </div>

            {loadingHistory && !history ? (
              <div className="flex items-center gap-3 py-10 text-sm text-slate-400">
                <Spinner />
                Loading monthly prices…
              </div>
            ) : historyError || !history || history.points.length === 0 ? (
              <p className="rounded border border-slate-800/80 bg-slate-950/40 px-3 py-4 text-sm text-slate-500">
                No real price history available for this commodity yet.
              </p>
            ) : (
              <CommodityPriceLineChart points={history.points} height={220} />
            )}
          </>
        )}
      </div>
    </Panel>
  )
}
