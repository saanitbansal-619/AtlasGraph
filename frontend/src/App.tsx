import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, ApiRequestError } from './lib/api'
import type {
  FragilitySummaryResponse,
  GraphSummaryResponse,
  HealthResponse,
  DBHealthResponse,
  DBSummaryResponse,
  PipelineRunSummary,
  CustomDataAnalysisResponse,
  RecommendedScenario,
  Scenario,
  ShockOptionsResponse,
  ShockValidOptionsResponse,
  ShockResponse,
  CommodityStressResponse,
  CommodityHistoryIndexResponse,
  EventRiskResponse,
  TradeSummaryResponse,
  TradeOptionsResponse,
  ScenarioReportResponse,
} from './types/api'
import {
  DEFAULT_META,
  DEFAULT_SCENARIO_NAME,
  type ScenarioMeta,
  type ShockMode,
  type SubmittedScenario,
  operationalRequestFields,
} from './types/scenario'
import { type AppTab, TAB_META } from './types/navigation'
import { Header } from './components/layout/Header'
import { Sidebar } from './components/layout/Sidebar'
import { BackendDownNotice } from './components/States'
import { toRequest, type ShockForm } from './components/ShockSimulator'
import { DashboardPage } from './pages/Dashboard'
import { ShockSimulationPage } from './pages/ShockSimulation'
import { ClientAnalyticsPage } from './pages/ClientAnalytics'
import { DataOperationsPage } from './pages/DataOperations'
import { AnalyticsExplorerPage } from './pages/AnalyticsExplorer'
import { HistoryPage } from './pages/History'

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
  const [activeTab, setActiveTab] = useState<AppTab>('dashboard')
  const [sidebarOpen, setSidebarOpen] = useState(false)

  const [health, setHealth] = useState<HealthResponse | null>(null)
  const [healthErr, setHealthErr] = useState<ApiRequestError | null>(null)
  const [healthLoading, setHealthLoading] = useState(true)

  const [dbHealth, setDBHealth] = useState<DBHealthResponse | null>(null)
  const [dbSummary, setDBSummary] = useState<DBSummaryResponse | null>(null)
  const [dbErr, setDBErr] = useState<UiError | null>(null)
  const [dbLoading, setDBLoading] = useState(true)

  const [pipelineSummary, setPipelineSummary] = useState<PipelineRunSummary | null>(null)
  const [pipelineErr, setPipelineErr] = useState<UiError | null>(null)
  const [pipelineLoading, setPipelineLoading] = useState(true)

  const [summary, setSummary] = useState<GraphSummaryResponse | null>(null)
  const [summaryErr, setSummaryErr] = useState<UiError | null>(null)

  const [fragility, setFragility] = useState<FragilitySummaryResponse | null>(null)
  const [fragilityErr, setFragilityErr] = useState<UiError | null>(null)
  const [fragilityLoading, setFragilityLoading] = useState(true)

  const [commodityStress, setCommodityStress] = useState<CommodityStressResponse | null>(null)
  const [commodityStressErr, setCommodityStressErr] = useState<UiError | null>(null)
  const [commodityStressLoading, setCommodityStressLoading] = useState(true)
  const [commodityHistoryIndex, setCommodityHistoryIndex] =
    useState<CommodityHistoryIndexResponse | null>(null)
  const [commodityHistoryErr, setCommodityHistoryErr] = useState<UiError | null>(null)
  const [commodityHistoryLoading, setCommodityHistoryLoading] = useState(true)

  const [eventRisk, setEventRisk] = useState<EventRiskResponse | null>(null)
  const [eventRiskErr, setEventRiskErr] = useState<UiError | null>(null)
  const [eventRiskLoading, setEventRiskLoading] = useState(true)

  const [tradeSummary, setTradeSummary] = useState<TradeSummaryResponse | null>(null)
  const [tradeOptions, setTradeOptions] = useState<TradeOptionsResponse | null>(null)
  const [tradeErr, setTradeErr] = useState<UiError | null>(null)
  const [tradeOptionsErr, setTradeOptionsErr] = useState<UiError | null>(null)
  const [tradeLoading, setTradeLoading] = useState(true)
  const [tradeOptionsLoading, setTradeOptionsLoading] = useState(true)

  const [scenarios, setScenarios] = useState<Scenario[]>([])
  const [scenariosLoading, setScenariosLoading] = useState(true)
  const [selectedId, setSelectedId] = useState('')

  const [options, setOptions] = useState<ShockOptionsResponse | null>(null)
  const [validOptions, setValidOptions] = useState<ShockValidOptionsResponse | null>(null)

  const [mode, setMode] = useState<ShockMode>('preset')
  const [form, setForm] = useState<ShockForm>(INITIAL_FORM)
  const [meta, setMeta] = useState<ScenarioMeta>(DEFAULT_META)

  const [result, setResult] = useState<ShockResponse | null>(null)
  const [submitted, setSubmitted] = useState<SubmittedScenario | null>(null)
  const [running, setRunning] = useState(false)
  const [runErr, setRunErr] = useState<UiError | null>(null)

  const [clientAnalysis, setClientAnalysis] = useState<CustomDataAnalysisResponse | null>(null)

  const [scenarioReport, setScenarioReport] = useState<ScenarioReportResponse | null>(null)
  const [reportLoading, setReportLoading] = useState(false)
  const [reportErr, setReportErr] = useState<UiError | null>(null)

  const applyScenario = useCallback((sc: Scenario) => {
    setForm({
      source: sc.source,
      commodity: sc.commodity,
      shock_type: sc.shock_type || 'export_collapse',
      drop: sc.shock_percent,
      depth: sc.depth || 3,
      explain: true,
    })
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

  const loadTradeOptions = useCallback(async () => {
    setTradeOptionsLoading(true)
    try {
      setTradeOptions(await api.tradeOptions())
      setTradeOptionsErr(null)
    } catch (e) {
      setTradeOptions(null)
      setTradeOptionsErr(toUiError(e))
    } finally {
      setTradeOptionsLoading(false)
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

  const loadDBAnalytics = useCallback(async () => {
    setDBLoading(true)
    setDBErr(null)
    try {
      const dbStatus = await api.dbHealth()
      setDBHealth(dbStatus)
      if (!dbStatus.enabled) {
        setDBSummary(null)
        return
      }
      setDBSummary(await api.dbSummary())
    } catch (e) {
      setDBSummary(null)
      setDBErr(toUiError(e))
    } finally {
      setDBLoading(false)
    }
  }, [])

  const loadPipelineSummary = useCallback(async () => {
    setPipelineLoading(true)
    setPipelineErr(null)
    try {
      setPipelineSummary(await api.pipelineSummary())
    } catch (e) {
      setPipelineSummary(null)
      setPipelineErr(toUiError(e))
    } finally {
      setPipelineLoading(false)
    }
  }, [])

  const loadAll = useCallback(() => {
    setHealthLoading(true)
    void checkHealth()
    void loadSummary()
    void loadFragility()
    void loadEventRisk()
    void loadTradeSummary()
    void loadTradeOptions()
    void loadCommodityStress()
    void loadCommodityHistoryIndex()
    void loadScenarios()
    void loadGuidance()
    void loadDBAnalytics()
    void loadPipelineSummary()
  }, [
    checkHealth,
    loadSummary,
    loadFragility,
    loadEventRisk,
    loadTradeSummary,
    loadTradeOptions,
    loadCommodityStress,
    loadCommodityHistoryIndex,
    loadScenarios,
    loadGuidance,
    loadDBAnalytics,
    loadPipelineSummary,
  ])

  useEffect(() => {
    loadAll()
  }, [loadAll])

  useEffect(() => {
    setScenarioReport(null)
    setReportErr(null)
  }, [form, meta.assumptions])

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
      const res = await api.runShock(toRequest(form, meta.assumptions))
      setResult(res)
      setSubmitted(snapshot)
      void checkHealth()
    } catch (e) {
      setRunErr(toUiError(e))
    } finally {
      setRunning(false)
    }
  }, [form, meta, mode, scenarios, selectedId, checkHealth])

  const generateScenarioReport = useCallback(async () => {
    setReportLoading(true)
    setReportErr(null)
    try {
      const res = await api.scenarioReport({
        source: form.source,
        commodity: form.commodity,
        shock_type: form.shock_type,
        drop_percent: form.drop,
        depth: form.depth,
        ...operationalRequestFields(meta.assumptions),
        ...(clientAnalysis
          ? {
              client_data: {
                concentration_results: clientAnalysis.concentration_results ?? [],
                normalized_rows: clientAnalysis.normalized_rows ?? [],
              },
            }
          : {}),
      })
      setScenarioReport(res)
    } catch (e) {
      setReportErr(toUiError(e))
    } finally {
      setReportLoading(false)
    }
  }, [form, meta.assumptions, clientAnalysis])

  const backendDown = useMemo(
    () => !!healthErr && healthErr.unreachable && health === null,
    [healthErr, health],
  )

  const tab = TAB_META[activeTab]
  const canGenerateReport = !backendDown && !!form.source.trim() && !!form.commodity.trim()

  return (
    <div className="flex h-screen min-h-0 overflow-hidden">
      <Sidebar
        activeTab={activeTab}
        onSelect={setActiveTab}
        mobileOpen={sidebarOpen}
        onCloseMobile={() => setSidebarOpen(false)}
      />

      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <Header
          title={tab.title}
          description={tab.description}
          health={health}
          error={!!healthErr}
          loading={healthLoading}
          onOpenSidebar={() => setSidebarOpen(true)}
        />

        <main className="min-h-0 flex-1 overflow-y-auto">
          <div className="space-y-4 px-4 py-4 sm:px-5 lg:px-6">
            {backendDown && (
              <BackendDownNotice message={healthErr?.message} onRetry={loadAll} />
            )}

            {activeTab === 'dashboard' && (
              <DashboardPage
                summary={summary}
                summaryLoading={healthLoading}
                summaryErr={summaryErr}
                fragility={fragility}
                fragilityLoading={fragilityLoading}
                fragilityErr={fragilityErr}
                result={result}
                clientAnalysis={clientAnalysis}
                scenarioReport={scenarioReport}
                reportLoading={reportLoading}
                reportErr={reportErr}
                onGenerateReport={generateScenarioReport}
                canGenerateReport={canGenerateReport}
                onOpenShock={() => setActiveTab('shock')}
              />
            )}

            {activeTab === 'shock' && (
              <ShockSimulationPage
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
                result={result}
                submitted={submitted}
                runErr={runErr}
                clientAnalysis={clientAnalysis}
                scenarioReport={scenarioReport}
                reportLoading={reportLoading}
                reportErr={reportErr}
                onGenerateReport={generateScenarioReport}
                canGenerateReport={canGenerateReport}
              />
            )}

            {activeTab === 'client' && (
              <ClientAnalyticsPage
                clientAnalysis={clientAnalysis}
                onAnalyzed={setClientAnalysis}
              />
            )}

            {activeTab === 'data-ops' && (
              <DataOperationsPage
                dbHealth={dbHealth}
                dbSummary={dbSummary}
                dbLoading={dbLoading}
                dbErr={dbErr}
                pipelineSummary={pipelineSummary}
                pipelineLoading={pipelineLoading}
                pipelineErr={pipelineErr}
              />
            )}

            {activeTab === 'analytics' && (
              <AnalyticsExplorerPage
                eventRisk={eventRisk}
                eventRiskLoading={eventRiskLoading}
                eventRiskErr={eventRiskErr}
                tradeSummary={tradeSummary}
                tradeLoading={tradeLoading}
                tradeErr={tradeErr}
                tradeOptions={tradeOptions}
                tradeOptionsLoading={tradeOptionsLoading}
                tradeOptionsErr={tradeOptionsErr}
                fetchTradeDependency={fetchTradeDependency}
                fetchTradeConcentration={fetchTradeConcentration}
                commodityStress={commodityStress}
                commodityStressLoading={commodityStressLoading}
                commodityStressErr={commodityStressErr}
                commodityHistoryIndex={commodityHistoryIndex}
                commodityHistoryLoading={commodityHistoryLoading}
                commodityHistoryErr={commodityHistoryErr}
                fetchCommodityHistory={fetchCommodityHistory}
              />
            )}

            {activeTab === 'history' && <HistoryPage />}

            <footer className="flex flex-col gap-1 border-t border-slate-800/80 pt-4 text-[11px] text-slate-600 sm:flex-row sm:items-center sm:justify-between">
              <span>GFIP · Global Fragility Intelligence Platform · Powered by AtlasGraph</span>
              <span className="font-mono">analyst workspace</span>
            </footer>
          </div>
        </main>
      </div>
    </div>
  )
}