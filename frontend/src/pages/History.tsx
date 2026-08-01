import { useEffect, useState } from 'react'
import { History, Database, HardDrive } from 'lucide-react'
import { api, ApiRequestError } from '../lib/api'
import type { DBHealthResponse, DBScenarioRun } from '../types/api'
import { loadSavedScenarios, type SavedShockScenario } from '../types/savedScenario'
import { SectionCard } from '../components/shared/SectionCard'
import { EmptyHint, Spinner } from '../components/ui'
import { InlineError } from '../components/States'
import { ModelDisclaimer } from '../components/AnalystWorkflow'

/**
 * Scenario history: PostgreSQL scenario_runs when DB is enabled; otherwise
 * browser-local saved shocks from Shock Simulation comparison.
 */
export function HistoryPage({
  dbHealth,
}: {
  dbHealth: DBHealthResponse | null
}) {
  const [serverRuns, setServerRuns] = useState<DBScenarioRun[] | null>(null)
  const [localSaved, setLocalSaved] = useState<SavedShockScenario[]>(() => loadSavedScenarios())
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<{ message: string; hint?: string } | null>(null)

  const dbEnabled = !!dbHealth?.enabled && dbHealth.status === 'ok'

  useEffect(() => {
    setLocalSaved(loadSavedScenarios())
  }, [])

  useEffect(() => {
    if (!dbEnabled) {
      setServerRuns(null)
      setError(null)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    void api
      .dbRecentScenarios()
      .then((res) => {
        if (!cancelled) setServerRuns(res.scenarios ?? [])
      })
      .catch((e) => {
        if (cancelled) return
        setServerRuns(null)
        setError(
          e instanceof ApiRequestError
            ? { message: e.message, hint: e.hint }
            : { message: e instanceof Error ? e.message : 'Failed to load scenario history' },
        )
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [dbEnabled])

  return (
    <div className="space-y-4">
      <ModelDisclaimer compact />

      <SectionCard
        title="Scenario history"
        dense
        right={
          <span className="inline-flex items-center gap-1.5 text-[10px] uppercase tracking-wide text-slate-500">
            {dbEnabled ? (
              <>
                <Database className="h-3 w-3" /> PostgreSQL
              </>
            ) : (
              <>
                <HardDrive className="h-3 w-3" /> Local browser
              </>
            )}
          </span>
        }
      >
        {dbEnabled ? (
          <>
            {loading && (
              <div className="flex items-center gap-2 py-6 text-sm text-slate-400">
                <Spinner />
                Loading persisted scenario runs…
              </div>
            )}
            {error && !loading && <InlineError message={error.message} hint={error.hint} />}
            {!loading && !error && serverRuns && serverRuns.length === 0 && (
              <EmptyHint>
                <div className="flex flex-col items-center gap-2">
                  <History className="h-5 w-5 text-slate-600" strokeWidth={1.75} />
                  <span>No scenario runs stored yet.</span>
                  <span className="text-[11px] text-slate-600">
                    Generate a scenario intelligence report with PostgreSQL enabled to persist runs.
                  </span>
                </div>
              </EmptyHint>
            )}
            {!loading && serverRuns && serverRuns.length > 0 && (
              <div className="overflow-x-auto rounded border border-slate-800">
                <table className="w-full min-w-[720px] text-left text-xs">
                  <thead className="sticky top-0 bg-slate-900/95 text-slate-500">
                    <tr className="border-b border-slate-800">
                      {['When', 'Scenario', 'Source', 'Commodity', 'Shock', 'Drop', 'Depth'].map(
                        (h) => (
                          <th key={h} className="px-3 py-2 font-medium">
                            {h}
                          </th>
                        ),
                      )}
                    </tr>
                  </thead>
                  <tbody>
                    {serverRuns.map((run) => (
                      <tr
                        key={`${run.scenario_id}-${run.created_at}`}
                        className="border-b border-slate-800/60"
                      >
                        <td className="px-3 py-2 font-mono text-slate-400">
                          {formatWhen(run.created_at)}
                        </td>
                        <td className="px-3 py-2 text-slate-200">{run.scenario_id}</td>
                        <td className="px-3 py-2 text-slate-300">{run.source}</td>
                        <td className="px-3 py-2 text-amber-200">{run.commodity}</td>
                        <td className="px-3 py-2 text-slate-400">{run.shock_type}</td>
                        <td className="px-3 py-2 font-mono text-slate-300">{run.drop_percent}%</td>
                        <td className="px-3 py-2 font-mono text-slate-300">{run.depth}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        ) : (
          <>
            <p className="mb-3 text-xs text-slate-500">
              PostgreSQL is disabled. Showing shocks saved locally for comparison in Shock
              Simulation. Set <span className="font-mono">DATABASE_URL</span> and generate a
              report to persist server-side history.
            </p>
            {localSaved.length === 0 ? (
              <EmptyHint>
                <div className="flex flex-col items-center gap-2">
                  <History className="h-5 w-5 text-slate-600" strokeWidth={1.75} />
                  <span>No local saved scenarios yet.</span>
                  <span className="text-[11px] text-slate-600">
                    Run a shock and click “Save scenario” on the Results tab.
                  </span>
                </div>
              </EmptyHint>
            ) : (
              <div className="overflow-x-auto rounded border border-slate-800">
                <table className="w-full min-w-[720px] text-left text-xs">
                  <thead className="sticky top-0 bg-slate-900/95 text-slate-500">
                    <tr className="border-b border-slate-800">
                      {['Saved', 'Title', 'Source', 'Commodity', 'Shock', 'Drop', 'Depth'].map(
                        (h) => (
                          <th key={h} className="px-3 py-2 font-medium">
                            {h}
                          </th>
                        ),
                      )}
                    </tr>
                  </thead>
                  <tbody>
                    {localSaved.map((s) => (
                      <tr key={s.id} className="border-b border-slate-800/60">
                        <td className="px-3 py-2 font-mono text-slate-400">
                          {formatWhen(s.savedAt)}
                        </td>
                        <td className="px-3 py-2 text-slate-200">{s.title}</td>
                        <td className="px-3 py-2 text-slate-300">{s.source}</td>
                        <td className="px-3 py-2 text-amber-200">{s.commodity}</td>
                        <td className="px-3 py-2 text-slate-400">{s.shock_type}</td>
                        <td className="px-3 py-2 font-mono text-slate-300">{s.drop}%</td>
                        <td className="px-3 py-2 font-mono text-slate-300">{s.depth}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </SectionCard>
    </div>
  )
}

function formatWhen(value: string): string {
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}
