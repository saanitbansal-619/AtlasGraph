import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react'
import type {
  CustomDataAnalysisResponse,
  RecommendedScenario,
  Scenario,
  ScenarioReportResponse,
  ShockOptionsResponse,
  ShockResponse,
  ShockValidOptionsResponse,
} from '../types/api'
import type { ScenarioMeta, ShockMode, SubmittedScenario } from '../types/scenario'
import {
  createSavedScenario,
  loadSavedScenarios,
  persistSavedScenarios,
  type SavedShockScenario,
  type ShockWorkspaceTab,
} from '../types/savedScenario'
import { ShockSimulator, type ShockForm } from '../components/ShockSimulator'
import { ShockWorkspaceNav } from '../components/shock/ShockWorkspaceNav'
import { ShockResultsWorkspace } from '../components/shock/ShockResultsWorkspace'
import { SavedScenarioComparison } from '../components/shock/SavedScenarioComparison'

export function ShockSimulationPage({
  mode,
  setMode,
  form,
  setForm,
  meta,
  setMeta,
  scenarios,
  selectedId,
  onSelectScenario,
  scenariosLoading,
  options,
  validOptions,
  onApplyRecommended,
  onRun,
  onReset,
  running,
  result,
  submitted,
  runErr,
  clientAnalysis,
  scenarioReport,
  reportLoading,
  reportErr,
  onGenerateReport,
  canGenerateReport,
}: {
  mode: ShockMode
  setMode: Dispatch<SetStateAction<ShockMode>>
  form: ShockForm
  setForm: Dispatch<SetStateAction<ShockForm>>
  meta: ScenarioMeta
  setMeta: Dispatch<SetStateAction<ScenarioMeta>>
  scenarios: Scenario[]
  selectedId: string
  onSelectScenario: (id: string) => void
  scenariosLoading: boolean
  options: ShockOptionsResponse | null
  validOptions: ShockValidOptionsResponse | null
  onApplyRecommended: (rs: RecommendedScenario) => void
  onRun: () => void
  onReset: () => void
  running: boolean
  result: ShockResponse | null
  submitted: SubmittedScenario | null
  runErr?: { message: string; hint?: string } | null
  clientAnalysis: CustomDataAnalysisResponse | null
  scenarioReport: ScenarioReportResponse | null
  reportLoading: boolean
  reportErr?: { message: string; hint?: string } | null
  onGenerateReport: () => void
  canGenerateReport: boolean
}) {
  const [workspaceTab, setWorkspaceTab] = useState<ShockWorkspaceTab>('setup')
  const [saved, setSaved] = useState<SavedShockScenario[]>(() => loadSavedScenarios())
  const prevRunning = useRef(false)

  useEffect(() => {
    if (prevRunning.current && !running && result && !runErr) {
      setWorkspaceTab('results')
    }
    prevRunning.current = running
  }, [running, result, runErr])

  useEffect(() => {
    persistSavedScenarios(saved)
  }, [saved])

  const alreadySaved =
    !!result &&
    saved.some(
      (s) =>
        s.result.scenario.source === result.scenario.source &&
        s.result.scenario.commodity === result.scenario.commodity &&
        s.result.scenario.shock_type === result.scenario.shock_type &&
        s.result.scenario.shock_percent === result.scenario.shock_percent &&
        s.result.scenario.depth === result.scenario.depth &&
        s.title === (submitted?.title?.trim() || result.scenario.name || s.title),
    )

  const onSaveScenario = () => {
    if (!result || alreadySaved) return
    setSaved((prev) => [createSavedScenario({ result, submitted }), ...prev].slice(0, 20))
  }

  return (
    <div className="space-y-4">
      <ShockWorkspaceNav active={workspaceTab} onChange={setWorkspaceTab} />

      {workspaceTab === 'setup' && (
        <div className="mx-auto max-w-xl">
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
            onRun={onRun}
            onReset={onReset}
            running={running}
          />
        </div>
      )}

      {workspaceTab === 'results' && (
        <ShockResultsWorkspace
          result={result}
          submitted={submitted}
          running={running}
          error={runErr}
          clientData={clientAnalysis}
          scenarioReport={scenarioReport}
          reportLoading={reportLoading}
          reportErr={reportErr}
          onGenerateReport={onGenerateReport}
          canGenerateReport={canGenerateReport}
          onSaveScenario={onSaveScenario}
          scenarioSaved={alreadySaved}
        />
      )}

      {workspaceTab === 'comparison' && (
        <SavedScenarioComparison
          saved={saved}
          clientData={clientAnalysis}
          onClear={() => setSaved([])}
        />
      )}
    </div>
  )
}
