import type {
  GraphSummaryResponse,
  GraphEntitiesResponse,
  FragilitySummaryResponse,
  HealthResponse,
  DBHealthResponse,
  DBSummaryResponse,
  CustomDataAnalysisResponse,
  ScenariosResponse,
  ShockOptionsResponse,
  ShockValidOptionsResponse,
  ShockRequest,
  ShockResponse,
  CompareScenariosRequest,
  ScenarioCompareResponse,
  CommodityStressResponse,
  CommodityHistoryResponse,
  CommodityHistoryIndexResponse,
  EventRiskResponse,
  TradeSummaryResponse,
  TradeOptionsResponse,
  TradeDependencyResponse,
  TradeConcentrationResponse,
  ScenarioReportRequest,
  ScenarioReportResponse,
  ApiError,
} from '../types/api'

export const API_BASE =
  import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

// The exact command a user needs to start the backend — surfaced in error UI.
export const BACKEND_COMMAND =
  'go run ./cmd/atlas serve --data data/generated/trade_graph --trade-data data/processed/trade --macro-data data/raw/worldbank --event-data data/raw/gdelt --processed-event-data data/processed/events --port 8080'

// ApiRequestError carries the structured {error, hint} from the API, plus a
// flag distinguishing "backend unreachable" (network) from "request rejected".
export class ApiRequestError extends Error {
  hint?: string
  status?: number
  unreachable: boolean

  constructor(
    message: string,
    opts: { hint?: string; status?: number; unreachable?: boolean } = {},
  ) {
    super(message)
    this.name = 'ApiRequestError'
    this.hint = opts.hint
    this.status = opts.status
    this.unreachable = opts.unreachable ?? false
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(`${API_BASE}${path}`, {
      headers: { 'Content-Type': 'application/json' },
      ...init,
    })
  } catch {
    throw new ApiRequestError('Cannot reach the AtlasGraph API server.', {
      hint: `Start the backend, then retry:\n${BACKEND_COMMAND}`,
      unreachable: true,
    })
  }

  const text = await res.text()
  let data: unknown = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = null
    }
  }

  if (!res.ok) {
    const err = (data ?? {}) as ApiError
    throw new ApiRequestError(err.error || `Request failed (HTTP ${res.status})`, {
      hint: err.hint,
      status: res.status,
    })
  }

  return data as T
}

export const api = {
  health: () => request<HealthResponse>('/health'),
  dbHealth: () => request<DBHealthResponse>('/api/db/health'),
  dbSummary: () => request<DBSummaryResponse>('/api/db/summary'),
  analyzeCustomData: (file: File, datasetName?: string) => {
    const form = new FormData()
    form.append('file', file)
    if (datasetName?.trim()) form.append('dataset_name', datasetName.trim())
    return request<CustomDataAnalysisResponse>('/api/custom-data/analyze', {
      method: 'POST',
      body: form,
      headers: {},
    })
  },
  graphSummary: () => request<GraphSummaryResponse>('/api/graph/summary'),
  graphEntities: () => request<GraphEntitiesResponse>('/api/graph/entities'),
  scenarios: () => request<ScenariosResponse>('/api/scenarios'),
  shockOptions: () => request<ShockOptionsResponse>('/api/shock/options'),
  shockValidOptions: (source?: string) => {
    const q = source?.trim() ? `?source=${encodeURIComponent(source.trim())}` : ''
    return request<ShockValidOptionsResponse>(`/api/shock/valid-options${q}`)
  },
  fragilitySummary: () => request<FragilitySummaryResponse>('/api/fragility/summary'),
  commodityStress: () => request<CommodityStressResponse>('/api/commodities/stress'),
  commodityHistoryIndex: () => request<CommodityHistoryIndexResponse>('/api/commodities/history'),
  commodityHistory: (commodity: string) =>
    request<CommodityHistoryResponse>(
      `/api/commodities/history?commodity=${encodeURIComponent(commodity)}`,
    ),
  eventRisk: (country?: string) => {
    const q = country?.trim() ? `?country=${encodeURIComponent(country.trim())}` : ''
    return request<EventRiskResponse>(`/api/events/risk${q}`)
  },
  tradeSummary: () => request<TradeSummaryResponse>('/api/trade/summary'),
  tradeOptions: () => request<TradeOptionsResponse>('/api/trade/options'),
  tradeDependency: (importer: string, commodity: string) =>
    request<TradeDependencyResponse>(
      `/api/trade/dependency?importer=${encodeURIComponent(importer)}&commodity=${encodeURIComponent(commodity)}`,
    ),
  tradeConcentration: (importer: string, commodity: string) =>
    request<TradeConcentrationResponse>(
      `/api/trade/concentration?importer=${encodeURIComponent(importer)}&commodity=${encodeURIComponent(commodity)}`,
    ),
  runShock: (body: ShockRequest) =>
    request<ShockResponse>('/api/shock', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  compareScenarios: (body: CompareScenariosRequest) =>
    request<ScenarioCompareResponse>('/api/scenarios/compare', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  scenarioReport: (body: ScenarioReportRequest) =>
    request<ScenarioReportResponse>('/api/reports/scenario', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
}
