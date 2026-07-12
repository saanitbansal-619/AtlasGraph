import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, ApiRequestError } from './lib/api'
import type {
  FragilitySummaryResponse,
  GraphSummaryResponse,
  HealthResponse,
  RecommendedScenario,
  Scenario,
  ShockOptionsResponse,
  ShockValidOptionsResponse,
  ShockResponse,
  CommodityStressResponse,
  CommodityHistoryIndexResponse,
  EventRiskResponse,
  TradeSummaryResponse,
} from './types/api'
import {
  DEFAULT_META,
  DEFAULT_SCENARIO_NAME,
  type ScenarioMeta,
  type ShockMode,
  type SubmittedScenario,
} from './types/scenario'
import { Header } from './components/Header'
import { OverviewCards } from './components/OverviewCards'
import { DataSourcesCard } from './components/DataSourcesCard'
import { CommodityStressPanel } from './components/CommodityStressPanel'
import { CommodityPriceHistory } from './components/CommodityPriceHistory'
import { EventRiskPanel } from './components/EventRiskPanel'
import { TradeSignalsPanel } from './components/TradeSignalsPanel'
import { UnifiedFragility } from './components/UnifiedFragility'
import { ShockSimulator, toRequest, type ShockForm } from './components/ShockSimulator'
import { ShockResults } from './components/ShockResults'
import { ScenarioComparison } from './components/ScenarioComparison'
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

// True when the form still matches the preset exactly on the fields that drive
// propagation (source, commodity, shock type, drop, depth).
function presetMatches(form: ShockForm, p: Scenario): boolean {
  return (
    form.source.trim() === p.source &&
    form.commodity.trim() === p.commodity &&
    form.shock_type === p.shock_type &&
    form.drop === p.shock_percent &&
    form.depth === p.depth
  )
}

function titleCase(s: string): string {
  return s.trim().replace(/\b\w/g, (c) => c.toUpperCase())
}

export default function App() {
  // Health
  const [health, setHealth] = useState<HealthResponse | null>(null)
  const [healthErr, setHealthErr] = useState<ApiRequestError | null>(null)
  const [healthLoading, setHealthLoading] = useState(true)

  // Graph summary
  const [summary, setSummary] = useState<GraphSummaryResponse | null>(null)
  const [summaryErr, setSummaryErr] = useState<UiError | null>(null)

  // Unified fragility summary
  const [fragility, setFragility] = useState<FragilitySummaryResponse | null>(null)
  const [fragilityErr, setFragilityErr] = useState<UiError | null>(null)
  const [fragilityLoading, setFragilityLoading] = useState(true)

  // Commodity stress + price history
  const [commodityStress, setCommodityStress] = useState<CommodityStressResponse | null>(null)
  const [commodityStressErr, setCommodityStressErr] = useState<UiError | null>(null)
  const [commodityStressLoading, setCommodityStressLoading] = useState(true)
  const [commodityHistoryIndex, setCommodityHistoryIndex] =
    useState<CommodityHistoryIndexResponse | null>(null)
  const [commodityHistoryErr, setCommodityHistoryErr] = useState<UiError | null>(null)
  const [commodityHistoryLoading, setCommodityHistoryLoading] = useState(true)

  // Event risk signals
  const [eventRisk, setEventRisk] = useState<EventRiskResponse | null>(null)
  const [eventRiskErr, setEventRiskErr] = useState<UiError | null>(null)
  const [eventRiskLoading, setEventRiskLoading] = useState(true)

  // Trade dependency signals
  const [tradeSummary, setTradeSummary] = useState<TradeSummaryResponse | null>(null)
  const [tradeErr, setTradeErr] = useState<UiError | null>(null)
  const [tradeLoading, setTradeLoading] = useState(true)

  // Scenarios
  const [scenarios, setScenarios] = useState<Scenario[]>([])
  const [scenariosLoading, setScenariosLoading] = useState(true)
  const [selectedId, setSelectedId] = useState('')

  // Graph-aware guidance: shock options + valid custom-shock combos.
  const [options, setOptions] = useState<ShockOptionsResponse | null>(null)
  const [validOptions, setValidOptions] = useState<ShockValidOptionsResponse | null>(null)

  // Shock form + scenario metadata (metadata is frontend-only).
  const [mode, setMode] = useState<ShockMode>('preset')
  const [form, setForm] = useState<ShockForm>(INITIAL_FORM)
  const [meta, setMeta] = useState<ScenarioMeta>(DEFAULT_META)

  // Shock result + the scenario snapshot captured when it was run.
  const [result, setResult] = useState<ShockResponse | null>(null)
  const [submitted, setSubmitted] = useState<SubmittedScenario | null>(null)
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
    // Carry the preset's name into the metadata; keep the analyst's assumptions.
    setMeta((m) => ({ ...m, name: sc.name || sc.id, notes: '' }))
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

  const loadFragility = useCallback(async () => {
    setFragilityLoading(true)
    try {
      setFragility(await api.fragilitySummary())
      setFragilityErr(null)
    } catch (e) {
      setFragility(null)
      setFragilityErr(toUiError(e))
    } finally {
      setFragilityLoading(false)
    }
  }, [])

  const loadCommodityStress = useCallback(async () => {
    setCommodityStressLoading(true)
    try {
      setCommodityStress(await api.commodityStress())
      setCommodityStressErr(null)
    } catch (e) {
      setCommodityStress(null)
      setCommodityStressErr(toUiError(e))
    } finally {
      setCommodityStressLoading(false)
    }
  }, [])

  const loadCommodityHistoryIndex = useCallback(async () => {
    setCommodityHistoryLoading(true)
    try {
      setCommodityHistoryIndex(await api.commodityHistoryIndex())
      setCommodityHistoryErr(null)
    } catch (e) {
      setCommodityHistoryIndex(null)
      setCommodityHistoryErr(toUiError(e))
    } finally {
      setCommodityHistoryLoading(false)
    }
  }, [])

  const loadEventRisk = useCallback(async () => {
    setEventRiskLoading(true)
    try {
      setEventRisk(await api.eventRisk())
      setEventRiskErr(null)
    } catch (e) {
      setEventRisk(null)
      setEventRiskErr(toUiError(e))
    } finally {
      setEventRiskLoading(false)
    }
  }, [])

  const loadTradeSummary = useCallback(async () => {
    setTradeLoading(true)
    try {
      setTradeSummary(await api.tradeSummary())
      setTradeErr(null)
    } catch (e) {
      setTradeSummary(null)
      setTradeErr(toUiError(e))
    } finally {
      setTradeLoading(false)
    }
  }, [])

  const fetchTradeDependency = useCallback(
    (importer: string, commodity: string) => api.tradeDependency(importer, commodity),
    [],
  )

  const fetchTradeConcentration = useCallback(
    (importer: string, commodity: string) => api.tradeConcentration(importer, commodity),
    [],
  )

  const fetchCommodityHistory = useCallback(
    (commodity: string) => api.commodityHistory(commodity),
    [],
  )

  const loadGuidance = useCallback(async () => {
    try {
      setOptions(await api.shockOptions())
    } catch {
      setOptions(null)
    }
    try {
      setValidOptions(await api.shockValidOptions())
    } catch {
      setValidOptions(null)
    }
  }, [])

  const loadAll = useCallback(() => {
    setHealthLoading(true)
    void checkHealth()
    void loadSummary()
    void loadFragility()
    void loadEventRisk()
    void loadTradeSummary()
    void loadCommodityStress()
    void loadCommodityHistoryIndex()
    void loadScenarios()
    void loadGuidance()
  }, [checkHealth, loadSummary, loadFragility, loadEventRisk, loadTradeSummary, loadCommodityStress, loadCommodityHistoryIndex, loadScenarios, loadGuidance])

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

  const onReset = useCallback(() => {
    setForm(INITIAL_FORM)
    setMeta({ ...DEFAULT_META, assumptions: { ...DEFAULT_META.assumptions } })
  }, [])

  // Clicking a recommended scenario drops into custom mode pre-filled with a
  // combination known to make sense for the current graph.
  const onApplyRecommended = useCallback((rs: RecommendedScenario) => {
    setMode('custom')
    setForm({
      source: rs.source,
      commodity: rs.commodity,
      shock_type: rs.shock_type,
      drop: rs.drop,
      depth: rs.depth || 3,
      explain: true,
    })
    setMeta((m) => ({ ...m, name: rs.label, notes: '' }))
  }, [])

  const runShock = useCallback(async () => {
    setRunning(true)
    setRunErr(null)

    // Build a snapshot of exactly what is being submitted, and resolve the title
    // from the current form/preset relationship so it never goes stale.
    const preset = scenarios.find((s) => s.id === selectedId)
    const modifiedPreset = mode === 'preset' && !!preset && !presetMatches(form, preset)
    let title: string
    if (mode === 'custom') {
      title = meta.name.trim() || DEFAULT_SCENARIO_NAME
    } else if (preset && !modifiedPreset) {
      title = preset.name || preset.id
    } else {
      title = `${titleCase(form.source)} ${titleCase(form.commodity)} Shock`
    }
    const snapshot: SubmittedScenario = {
      title,
      mode,
      modifiedPreset,
      meta: { ...meta, assumptions: { ...meta.assumptions } },
    }

    try {
      const res = await api.runShock(toRequest(form))
      setResult(res)
      setSubmitted(snapshot)
      void checkHealth()
    } catch (e) {
      setRunErr(toUiError(e))
    } finally {
      setRunning(false)
    }
  }, [form, meta, mode, scenarios, selectedId, checkHealth])

  const backendDown = useMemo(
    () => !!healthErr && healthErr.unreachable && health === null,
    [healthErr, health],
  )

  return (
    <div className="min-h-screen">
      <Header health={health} error={!!healthErr} loading={healthLoading} />

      <main className="dashboard-shell space-y-4 py-5">
        {backendDown && (
          <BackendDownNotice message={healthErr?.message} onRetry={loadAll} />
        )}

        <div className="grid grid-cols-1 gap-3 lg:grid-cols-2 lg:items-stretch">
          <div className="min-w-0">
            <OverviewCards summary={summary} loading={healthLoading} error={summaryErr} />
          </div>
          <div className="min-w-0 lg:h-full">
            <DataSourcesCard
              summary={summary}
              fragility={fragility}
              loading={healthLoading || fragilityLoading}
              compact
            />
          </div>
        </div>

        <UnifiedFragility summary={fragility} loading={fragilityLoading} error={fragilityErr} />

        <EventRiskPanel data={eventRisk} loading={eventRiskLoading} error={eventRiskErr} />

        <TradeSignalsPanel
          summary={tradeSummary}
          summaryLoading={tradeLoading}
          summaryError={tradeErr}
          fetchDependency={fetchTradeDependency}
          fetchConcentration={fetchTradeConcentration}
        />

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

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(280px,30%)_minmax(0,1fr)] xl:grid-cols-[minmax(300px,28%)_minmax(0,1fr)]">
          <div className="min-w-0">
            <ShockSimulator
              mode={mode}
              setMode={setMode}
              form={form}
              setForm={setForm}
              meta={meta}
              setMeta={setMeta}
              scenarios={scenarios}
              selectedId={selectedId}
              onSelectScenario={onSelectScenario}
              scenariosLoading={scenariosLoading}
              options={options}
              validOptions={validOptions}
              onApplyRecommended={onApplyRecommended}
              onRun={runShock}
              onReset={onReset}
              running={running}
            />
          </div>

          <div className="min-w-0">
            <ShockResults result={result} submitted={submitted} running={running} error={runErr} />
          </div>
        </div>

        <ScenarioComparison options={options} />

        <footer className="flex flex-col gap-1 border-t border-slate-800/80 pt-4 text-[11px] text-slate-600 sm:flex-row sm:items-center sm:justify-between">
          <span>
            GFIP · Global Fragility Intelligence Platform · Powered by AtlasGraph
          </span>
          <span className="font-mono">control-room demo</span>
        </footer>
      </main>
    </div>
  )
}
