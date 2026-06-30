import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, ApiRequestError } from './lib/api'
import type {
  GraphSummaryResponse,
  HealthResponse,
  Scenario,
  ShockResponse,
} from './types/api'
import { Header } from './components/Header'
import { OverviewCards } from './components/OverviewCards'
import { ScenarioSelect } from './components/ScenarioSelect'
import { ShockSimulator, toRequest, type ShockForm } from './components/ShockSimulator'
import { ShockResults } from './components/ShockResults'
import { BackendDownNotice } from './components/States'

const DEFAULT_SCENARIO_ID = 'taiwan_semiconductor_shock'

interface UiError {
  message: string
  hint?: string
}

function toUiError(e: unknown): UiError {
  if (e instanceof ApiRequestError) return { message: e.message, hint: e.hint }
  return { message: e instanceof Error ? e.message : 'Unexpected error' }
}

const INITIAL_FORM: ShockForm = {
  source: 'Taiwan',
  commodity: 'semiconductors',
  shock_type: 'export_collapse',
  drop: 30,
  depth: 3,
  explain: true,
}

export default function App() {
  // Health
  const [health, setHealth] = useState<HealthResponse | null>(null)
  const [healthErr, setHealthErr] = useState<ApiRequestError | null>(null)
  const [healthLoading, setHealthLoading] = useState(true)

  // Graph summary
  const [summary, setSummary] = useState<GraphSummaryResponse | null>(null)
  const [summaryErr, setSummaryErr] = useState<UiError | null>(null)

  // Scenarios
  const [scenarios, setScenarios] = useState<Scenario[]>([])
  const [scenariosLoading, setScenariosLoading] = useState(true)
  const [selectedId, setSelectedId] = useState('')

  // Shock form + result
  const [form, setForm] = useState<ShockForm>(INITIAL_FORM)
  const [result, setResult] = useState<ShockResponse | null>(null)
  const [running, setRunning] = useState(false)
  const [runErr, setRunErr] = useState<UiError | null>(null)

  const applyScenario = useCallback((sc: Scenario) => {
    setForm({
      source: sc.source,
      commodity: sc.commodity,
      shock_type: sc.shock_type || 'export_collapse',
      drop: sc.shock_percent,
      depth: sc.depth || 3,
      explain: true,
    })
  }, [])

  const checkHealth = useCallback(async () => {
    try {
      const h = await api.health()
      setHealth(h)
      setHealthErr(null)
    } catch (e) {
      setHealth(null)
      setHealthErr(e instanceof ApiRequestError ? e : new ApiRequestError('Health check failed'))
    } finally {
      setHealthLoading(false)
    }
  }, [])

  const loadSummary = useCallback(async () => {
    try {
      setSummary(await api.graphSummary())
      setSummaryErr(null)
    } catch (e) {
      setSummary(null)
      setSummaryErr(toUiError(e))
    }
  }, [])

  const loadScenarios = useCallback(async () => {
    setScenariosLoading(true)
    try {
      const res = await api.scenarios()
      const list = res.scenarios ?? []
      setScenarios(list)
      if (list.length > 0) {
        const def = list.find((s) => s.id === DEFAULT_SCENARIO_ID) ?? list[0]
        setSelectedId(def.id)
        applyScenario(def)
      }
    } catch {
      setScenarios([])
    } finally {
      setScenariosLoading(false)
    }
  }, [applyScenario])

  const loadAll = useCallback(() => {
    setHealthLoading(true)
    void checkHealth()
    void loadSummary()
    void loadScenarios()
  }, [checkHealth, loadSummary, loadScenarios])

  // Initial load.
  useEffect(() => {
    loadAll()
  }, [loadAll])

  // Poll health so the status badge stays live.
  useEffect(() => {
    const id = setInterval(() => void checkHealth(), 15000)
    return () => clearInterval(id)
  }, [checkHealth])

  const onSelectScenario = useCallback(
    (id: string) => {
      setSelectedId(id)
      const sc = scenarios.find((s) => s.id === id)
      if (sc) applyScenario(sc)
    },
    [scenarios, applyScenario],
  )

  const runShock = useCallback(async () => {
    setRunning(true)
    setRunErr(null)
    try {
      const res = await api.runShock(toRequest(form))
      setResult(res)
      // A successful shock implies the API is up — refresh the badge.
      void checkHealth()
    } catch (e) {
      setRunErr(toUiError(e))
    } finally {
      setRunning(false)
    }
  }, [form, checkHealth])

  const backendDown = useMemo(
    () => !!healthErr && healthErr.unreachable && health === null,
    [healthErr, health],
  )

  return (
    <div className="min-h-screen">
      <Header health={health} error={!!healthErr} loading={healthLoading} />

      <main className="mx-auto max-w-7xl space-y-4 px-4 py-5">
        {backendDown && (
          <BackendDownNotice message={healthErr?.message} onRetry={loadAll} />
        )}

        <OverviewCards summary={summary} loading={healthLoading} error={summaryErr} />

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
          <div className="space-y-4 lg:col-span-1">
            <ScenarioSelect
              scenarios={scenarios}
              selectedId={selectedId}
              onSelect={onSelectScenario}
              loading={scenariosLoading}
            />
            <ShockSimulator form={form} setForm={setForm} onRun={runShock} running={running} />
          </div>

          <div className="lg:col-span-2">
            <ShockResults result={result} running={running} error={runErr} />
          </div>
        </div>

        <footer className="flex items-center justify-between border-t border-slate-800/80 pt-4 text-[11px] text-slate-600">
          <span>Global Fragility Intelligence Platform · Powered by AtlasGraph</span>
          <span className="font-mono">control-room demo</span>
        </footer>
      </main>
    </div>
  )
}
