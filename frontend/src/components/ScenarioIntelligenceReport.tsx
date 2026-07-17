import { useMemo, useState } from 'react'
import type {
  ReportContextItem,
  ReportExposureItem,
  ReportTradeEvidence,
  ScenarioReportResponse,
} from '../types/api'
import { deltaClass, fixed, pct, riskBadgeClass, signed } from '../lib/format'
import { EmptyHint, Panel, Spinner, TypeBadge } from './ui'
import { InlineError } from './States'

function reportToMarkdown(report: ScenarioReportResponse): string {
  const lines: string[] = []
  lines.push(`# ${report.title}`, '')
  lines.push('## Executive Summary', '', report.executive_summary, '')
  lines.push('## Key Findings', '')
  for (const f of report.key_findings ?? []) {
    lines.push(`- ${f}`)
  }
  lines.push('')

  const table = (title: string, rows: ReportExposureItem[]) => {
    lines.push(`## ${title}`, '')
    if (!rows?.length) {
      lines.push('_None_', '')
      return
    }
    lines.push('| Entity | Type | Est. impact | Fragility Δ | Provenance |')
    lines.push('| --- | --- | --- | --- | --- |')
    for (const r of rows) {
      lines.push(
        `| ${r.entity} | ${r.type} | ${fixed(r.estimated_impact, 3)} | ${signed(r.fragility_delta)} | ${r.data_provenance} |`,
      )
    }
    lines.push('')
  }

  table('Impacted Countries', report.most_exposed_countries ?? [])
  table('Impacted Commodities', report.most_exposed_commodities ?? [])
  table('Impacted Sectors', report.most_exposed_sectors ?? [])

  lines.push('## Evidence', '')
  if ((report.trade_evidence ?? []).length) {
    lines.push('### Trade concentration (UN Comtrade)', '')
    for (const t of report.trade_evidence) {
      lines.push(`- ${t.summary}`)
    }
    lines.push('')
  }
  const ctxBlock = (heading: string, items: ReportContextItem[]) => {
    if (!items?.length) return
    lines.push(`### ${heading}`, '')
    for (const c of items) {
      lines.push(`- **${c.entity}**: ${c.summary}`)
    }
    lines.push('')
  }
  ctxBlock('Event-risk context (GDELT)', report.event_risk_context ?? [])
  ctxBlock('Macro context (World Bank Macro)', report.macro_context ?? [])
  ctxBlock('Commodity fragility (World Bank Pink Sheet)', report.commodity_fragility_context ?? [])

  lines.push('## Model Assumptions', '')
  for (const a of report.model_assumptions ?? []) lines.push(`- ${a}`)
  lines.push('', '## Limitations', '')
  for (const l of report.limitations ?? []) lines.push(`- ${l}`)
  lines.push('', '## Data Sources', '')
  for (const s of report.data_sources ?? []) lines.push(`- ${s}`)
  lines.push('')
  return lines.join('\n')
}

function ExposureTable({ rows }: { rows: ReportExposureItem[] }) {
  if (!rows.length) {
    return <p className="text-xs text-slate-500">No model-derived exposure in this band.</p>
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[480px] text-left text-xs">
        <thead>
          <tr className="border-b border-slate-800 text-slate-500">
            <th className="th py-1.5 pr-2 font-medium">Entity</th>
            <th className="th py-1.5 pr-2 font-medium">Type</th>
            <th className="th py-1.5 pr-2 font-medium text-right">Est. impact</th>
            <th className="th py-1.5 pr-2 font-medium text-right">Fragility Δ</th>
            <th className="th py-1.5 font-medium">Provenance</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={`${r.type}-${r.entity}`} className="border-b border-slate-800/60">
              <td className="td py-1.5 pr-2 text-slate-200">{r.entity}</td>
              <td className="td py-1.5 pr-2">
                <TypeBadge type={r.type} />
              </td>
              <td className="td py-1.5 pr-2 text-right font-mono text-slate-300">
                {fixed(r.estimated_impact, 3)}
              </td>
              <td className={`td py-1.5 pr-2 text-right font-mono ${deltaClass(r.fragility_delta)}`}>
                {signed(r.fragility_delta)}
              </td>
              <td className="td py-1.5 text-slate-500">{r.data_provenance}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function EvidenceList({
  trade,
  events,
  macro,
  commodities,
}: {
  trade: ReportTradeEvidence[]
  events: ReportContextItem[]
  macro: ReportContextItem[]
  commodities: ReportContextItem[]
}) {
  const empty =
    trade.length === 0 && events.length === 0 && macro.length === 0 && commodities.length === 0
  if (empty) {
    return (
      <p className="text-xs text-slate-500">
        No supporting observed panels matched this scenario for the current data load.
      </p>
    )
  }
  return (
    <div className="space-y-3 text-sm text-slate-300">
      {trade.length > 0 && (
        <div>
          <div className="mb-1 text-[11px] uppercase tracking-wide text-slate-500">
            Trade concentration · UN Comtrade
          </div>
          <ul className="space-y-1.5">
            {trade.map((t) => (
              <li key={`${t.importer}-${t.commodity}`} className="leading-relaxed">
                {t.summary}
                {t.top_supplier_code ? (
                  <span className="ml-1 font-mono text-[11px] text-slate-500">
                    ({t.top_supplier_code}, HHI {fixed(t.hhi, 2)}, top share {pct(t.top_supplier_share)})
                  </span>
                ) : null}
              </li>
            ))}
          </ul>
        </div>
      )}
      {events.length > 0 && (
        <div>
          <div className="mb-1 text-[11px] uppercase tracking-wide text-slate-500">
            Event-risk · GDELT
          </div>
          <ul className="space-y-1.5">
            {events.map((e) => (
              <li key={e.entity} className="leading-relaxed">
                <span className="text-slate-200">{e.entity}</span>
                <span className={`ml-2 badge ${riskBadgeClass(e.risk_level)}`}>{e.risk_level}</span>
                <span className="ml-2 font-mono text-xs text-slate-400">{fixed(e.score, 1)}</span>
                <div className="text-xs text-slate-500">{e.summary}</div>
              </li>
            ))}
          </ul>
        </div>
      )}
      {macro.length > 0 && (
        <div>
          <div className="mb-1 text-[11px] uppercase tracking-wide text-slate-500">
            Macro · World Bank Macro
          </div>
          <ul className="space-y-1.5">
            {macro.map((m) => (
              <li key={m.entity} className="leading-relaxed">
                <span className="text-slate-200">{m.entity}</span>
                <span className={`ml-2 badge ${riskBadgeClass(m.risk_level)}`}>{m.risk_level}</span>
                <span className="ml-2 font-mono text-xs text-slate-400">{fixed(m.score, 1)}</span>
                <div className="text-xs text-slate-500">{m.summary}</div>
              </li>
            ))}
          </ul>
        </div>
      )}
      {commodities.length > 0 && (
        <div>
          <div className="mb-1 text-[11px] uppercase tracking-wide text-slate-500">
            Commodity fragility · World Bank Pink Sheet
          </div>
          <ul className="space-y-1.5">
            {commodities.map((c) => (
              <li key={c.entity} className="leading-relaxed">
                <span className="text-slate-200">{c.entity}</span>
                <span className={`ml-2 badge ${riskBadgeClass(c.risk_level)}`}>{c.risk_level}</span>
                <span className="ml-2 font-mono text-xs text-slate-400">{fixed(c.score, 1)}</span>
                <div className="text-xs text-slate-500">{c.summary}</div>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

export function ScenarioIntelligenceReport({
  report,
  loading,
  error,
  onGenerate,
  canGenerate,
}: {
  report: ScenarioReportResponse | null
  loading: boolean
  error?: { message: string; hint?: string } | null
  onGenerate: () => void
  canGenerate: boolean
}) {
  const [copied, setCopied] = useState(false)
  const [assumptionsOpen, setAssumptionsOpen] = useState(false)

  const markdown = useMemo(() => (report ? reportToMarkdown(report) : ''), [report])

  const copyReport = async () => {
    if (!markdown) return
    try {
      await navigator.clipboard.writeText(markdown)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      setCopied(false)
    }
  }

  return (
    <Panel
      title="Scenario Intelligence Report"
      right={
        <div className="flex items-center gap-2">
          {report && (
            <button
              type="button"
              className="rounded border border-slate-700 px-2.5 py-1 text-[11px] text-slate-300 hover:border-cyan-700 hover:text-cyan-200"
              onClick={() => void copyReport()}
            >
              {copied ? 'Copied' : 'Copy Report'}
            </button>
          )}
          <button
            type="button"
            className="btn-primary px-2.5 py-1 text-[11px]"
            disabled={!canGenerate || loading}
            onClick={onGenerate}
          >
            {loading ? 'Generating…' : 'Generate Report'}
          </button>
        </div>
      }
    >
      {error && !loading && <InlineError message={error.message} hint={error.hint} />}

      {loading && !report && (
        <div className="flex items-center gap-3 py-10 text-sm text-slate-400">
          <Spinner />
          Building analyst report from shock propagation and observed panels…
        </div>
      )}

      {!loading && !report && !error && (
        <EmptyHint>
          Generate an analyst-style scenario report from the current shock controls. The report
          combines model-derived exposure with observed trade, event-risk, and macro context.
        </EmptyHint>
      )}

      {report && (
        <div className={`space-y-4 ${loading ? 'opacity-60' : ''}`}>
          <div>
            <h3 className="text-sm font-medium text-slate-100">{report.title}</h3>
            <p className="mt-1 text-[11px] text-slate-500">
              Data sources: {(report.data_sources ?? []).join(' · ') || 'Baseline dependency graph'}
            </p>
          </div>

          <div className="rounded border border-slate-800 bg-slate-900/40 p-3">
            <div className="label mb-1">Executive Summary</div>
            <p className="text-sm leading-relaxed text-slate-300">{report.executive_summary}</p>
          </div>

          <div>
            <div className="label mb-2">Key Findings</div>
            <ul className="space-y-1.5 text-sm text-slate-300">
              {(report.key_findings ?? []).map((f) => (
                <li key={f} className="flex gap-2 leading-relaxed">
                  <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-cyan-500/80" />
                  <span>{f}</span>
                </li>
              ))}
            </ul>
          </div>

          <div className="grid grid-cols-1 gap-3 xl:grid-cols-3">
            <div className="rounded border border-slate-800/80 p-3">
              <div className="label mb-2">Impacted Countries</div>
              <ExposureTable rows={report.most_exposed_countries ?? []} />
            </div>
            <div className="rounded border border-slate-800/80 p-3">
              <div className="label mb-2">Impacted Commodities</div>
              <ExposureTable rows={report.most_exposed_commodities ?? []} />
            </div>
            <div className="rounded border border-slate-800/80 p-3">
              <div className="label mb-2">Impacted Sectors</div>
              <ExposureTable rows={report.most_exposed_sectors ?? []} />
            </div>
          </div>

          <div className="rounded border border-slate-800/80 p-3">
            <div className="label mb-2">Evidence</div>
            <EvidenceList
              trade={report.trade_evidence ?? []}
              events={report.event_risk_context ?? []}
              macro={report.macro_context ?? []}
              commodities={report.commodity_fragility_context ?? []}
            />
          </div>

          <div className="rounded border border-slate-800/80">
            <button
              type="button"
              className="flex w-full items-center justify-between px-3 py-2 text-left text-xs text-slate-300 hover:bg-slate-900/50"
              onClick={() => setAssumptionsOpen((v) => !v)}
              aria-expanded={assumptionsOpen}
            >
              <span className="font-medium">Model Assumptions / Limitations</span>
              <span className="text-slate-500">{assumptionsOpen ? 'Hide' : 'Show'}</span>
            </button>
            {assumptionsOpen && (
              <div className="grid gap-3 border-t border-slate-800 px-3 py-3 text-xs text-slate-400 sm:grid-cols-2">
                <div>
                  <div className="mb-1 text-[11px] uppercase tracking-wide text-slate-500">
                    Assumptions
                  </div>
                  <ul className="space-y-1.5">
                    {(report.model_assumptions ?? []).map((a) => (
                      <li key={a}>• {a}</li>
                    ))}
                  </ul>
                </div>
                <div>
                  <div className="mb-1 text-[11px] uppercase tracking-wide text-slate-500">
                    Limitations
                  </div>
                  <ul className="space-y-1.5">
                    {(report.limitations ?? []).map((l) => (
                      <li key={l}>• {l}</li>
                    ))}
                  </ul>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </Panel>
  )
}
