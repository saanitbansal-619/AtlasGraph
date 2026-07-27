import {
  BarChart3,
  Building2,
  Database,
  Globe2,
  Package,
  ShieldAlert,
} from 'lucide-react'
import type {
  CommodityHistoryIndexResponse,
  CommodityHistoryResponse,
  CommodityStressResponse,
  EventRiskResponse,
  TradeConcentrationResponse,
  TradeDependencyResponse,
  TradeOptionsResponse,
  TradeSummaryResponse,
} from '../types/api'
import { CommodityStressPanel } from '../components/CommodityStressPanel'
import { CommodityPriceHistory } from '../components/CommodityPriceHistory'
import { EventRiskPanel } from '../components/EventRiskPanel'
import { TradeSignalsPanel } from '../components/TradeSignalsPanel'
import { SectionCard } from '../components/shared/SectionCard'

const EXPLORERS = [
  {
    title: 'Commodity Explorer',
    detail: 'Inspect commodity fragility, price stress, and supplier concentration pathways.',
    icon: Package,
  },
  {
    title: 'Country Explorer',
    detail: 'Compare country-level macro, event-risk, and dependency exposure profiles.',
    icon: Globe2,
  },
  {
    title: 'Supplier Rankings',
    detail: 'Rank critical suppliers by trade share, concentration risk, and shock sensitivity.',
    icon: Building2,
  },
  {
    title: 'Event Risk Rankings',
    detail: 'Browse GDELT-derived geopolitical risk signals across countries and corridors.',
    icon: ShieldAlert,
  },
  {
    title: 'SQL Analytics',
    detail: 'Query PostgreSQL-backed analytics tables for warehouse-style investigations.',
    icon: Database,
    soon: true,
  },
] as const

export function AnalyticsExplorerPage({
  eventRisk,
  eventRiskLoading,
  eventRiskErr,
  tradeSummary,
  tradeLoading,
  tradeErr,
  tradeOptions,
  tradeOptionsLoading,
  tradeOptionsErr,
  fetchTradeDependency,
  fetchTradeConcentration,
  commodityStress,
  commodityStressLoading,
  commodityStressErr,
  commodityHistoryIndex,
  commodityHistoryLoading,
  commodityHistoryErr,
  fetchCommodityHistory,
}: {
  eventRisk: EventRiskResponse | null
  eventRiskLoading: boolean
  eventRiskErr?: { message: string; hint?: string } | null
  tradeSummary: TradeSummaryResponse | null
  tradeLoading: boolean
  tradeErr?: { message: string; hint?: string } | null
  tradeOptions: TradeOptionsResponse | null
  tradeOptionsLoading: boolean
  tradeOptionsErr?: { message: string; hint?: string } | null
  fetchTradeDependency: (importer: string, commodity: string) => Promise<TradeDependencyResponse>
  fetchTradeConcentration: (
    importer: string,
    commodity: string,
  ) => Promise<TradeConcentrationResponse>
  commodityStress: CommodityStressResponse | null
  commodityStressLoading: boolean
  commodityStressErr?: { message: string; hint?: string } | null
  commodityHistoryIndex: CommodityHistoryIndexResponse | null
  commodityHistoryLoading: boolean
  commodityHistoryErr?: { message: string; hint?: string } | null
  fetchCommodityHistory: (commodity: string) => Promise<CommodityHistoryResponse>
}) {
  return (
    <div className="space-y-4">
      <SectionCard title="Planned analytics workspaces" dense>
        <p className="mb-3 text-xs leading-relaxed text-slate-400">
          Future drill-down workspaces for commodities, countries, suppliers, and SQL analytics.
        </p>
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {EXPLORERS.map(({ title, detail, icon: Icon, ...rest }) => {
            const soon = 'soon' in rest && rest.soon
            return (
              <div
                key={title}
                className="rounded border border-slate-800 bg-slate-900/30 px-3 py-3"
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <Icon className="h-4 w-4 text-cyan-300" strokeWidth={1.75} />
                    <div className="text-sm font-medium text-slate-200">{title}</div>
                  </div>
                  {soon ? (
                    <span className="badge border-amber-500/30 bg-amber-500/10 text-amber-200">
                      Coming Soon
                    </span>
                  ) : (
                    <span className="badge border-slate-600/60 bg-slate-800/40 text-slate-400">
                      Planned
                    </span>
                  )}
                </div>
                <p className="mt-2 text-xs leading-relaxed text-slate-500">{detail}</p>
              </div>
            )
          })}
        </div>
      </SectionCard>

      <SectionCard title="Available now" dense>
        <div className="mb-3 flex items-start gap-2 text-xs text-slate-400">
          <BarChart3 className="mt-0.5 h-4 w-4 shrink-0 text-slate-500" strokeWidth={1.75} />
          <p>
            Live event-risk, trade, and commodity panels from the current GFIP analytics APIs.
          </p>
        </div>
      </SectionCard>

      <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-2">
        <div className="min-w-0">
          <EventRiskPanel data={eventRisk} loading={eventRiskLoading} error={eventRiskErr} />
        </div>
        <div className="min-w-0">
          <TradeSignalsPanel
            summary={tradeSummary}
            summaryLoading={tradeLoading}
            summaryError={tradeErr}
            options={tradeOptions}
            optionsLoading={tradeOptionsLoading}
            optionsError={tradeOptionsErr}
            fetchDependency={fetchTradeDependency}
            fetchConcentration={fetchTradeConcentration}
          />
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <CommodityStressPanel
          data={commodityStress}
          loading={commodityStressLoading}
          error={commodityStressErr}
        />
        <CommodityPriceHistory
          index={commodityHistoryIndex}
          loadingIndex={commodityHistoryLoading}
          indexError={commodityHistoryErr}
          fetchHistory={fetchCommodityHistory}
        />
      </div>
    </div>
  )
}
