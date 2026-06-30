import type {
  GraphSummaryResponse,
  GraphEntitiesResponse,
  HealthResponse,
  ScenariosResponse,
  ShockOptionsResponse,
  ShockRequest,
  ShockResponse,
  ApiError,
} from '../types/api'

export const API_BASE =
  import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

// The exact command a user needs to start the backend — surfaced in error UI.
export const BACKEND_COMMAND =
  'go run ./cmd/atlas serve --data data/generated/trade_graph --trade-data data/processed/trade --macro-data data/raw/worldbank --event-data data/raw/gdelt --port 8080'

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
  graphSummary: () => request<GraphSummaryResponse>('/api/graph/summary'),
  graphEntities: () => request<GraphEntitiesResponse>('/api/graph/entities'),
  scenarios: () => request<ScenariosResponse>('/api/scenarios'),
  shockOptions: () => request<ShockOptionsResponse>('/api/shock/options'),
  runShock: (body: ShockRequest) =>
    request<ShockResponse>('/api/shock', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
}
