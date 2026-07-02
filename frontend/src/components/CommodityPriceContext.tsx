import { useEffect, useState } from 'react'
import type { CommodityHistoryResponse } from '../types/api'
import { api, ApiRequestError } from '../lib/api'
import { CommodityPriceLineChart } from './charts/CommodityPriceLineChart'
import { Panel, Spinner } from './ui'

const SOURCE_LABEL = 'World Bank Pink Sheet'
const SOURCE_SUBTITLE = 'Monthly nominal price history · World Bank Pink Sheet'

export function CommodityPriceContext({
  commodity,
  resetKey,
}: {
  commodity: string
  resetKey?: unknown
}) {
  const [history, setHistory] = useState<CommodityHistoryResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [missing, setMissing] = useState(false)
  const [expanded, setExpanded] = useState(false)

  useEffect(() => {
    setExpanded(false)
  }, [resetKey])

  useEffect(() => {
    const name = commodity.trim()
    if (!name) {
      setHistory(null)
      setMissing(true)
      return
    }

    let cancelled = false
    setLoading(true)
    setMissing(false)
    setHistory(null)

    void api
      .commodityHistory(name)
      .then((res) => {
        if (!cancelled) {
          setHistory(res)
          setMissing(res.points.length === 0)
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setHistory(null)
          setMissing(e instanceof ApiRequestError && e.status === 404)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [commodity])

  const displayCommodity = history?.commodity || commodity.trim()
  const hasRealData = Boolean(history && history.points.length > 0 && !missing)
  const headerTitle = hasRealData
    ? `Commodity Price Context · ${displayCommodity} · ${SOURCE_LABEL}`
    : `Commodity Price Context · ${displayCommodity}`

  const toggleButton = (
    <button
      type="button"
      onClick={() => setExpanded((v) => !v)}
      className="rounded border border-slate-700/60 bg-slate-950/40 px-2.5 py-1 text-[11px] font-medium text-slate-400 transition hover:border-slate-600 hover:text-slate-300"
      aria-expanded={expanded}
    >
      {expanded ? 'Hide price history' : 'Show price history'}
    </button>
  )

  return (
    <Panel title={headerTitle} right={toggleButton} noPad className="bg-slate-950/25">
      {expanded && (
        <div className="space-y-2 border-t border-slate-800/60 px-4 py-3">
          {hasRealData && (
            <p className="text-[10px] font-mono uppercase tracking-wider text-slate-500">
              {SOURCE_SUBTITLE}
            </p>
          )}

          {loading ? (
            <div className="flex items-center gap-3 py-6 text-sm text-slate-400">
              <Spinner />
              Loading monthly prices…
            </div>
          ) : missing || !history || history.points.length === 0 ? (
            <p className="rounded border border-slate-800/80 bg-slate-950/40 px-3 py-4 text-sm text-slate-500">
              No real price history available for {displayCommodity} yet.
            </p>
          ) : (
            <CommodityPriceLineChart points={history.points} height={180} />
          )}
        </div>
      )}
    </Panel>
  )
}
