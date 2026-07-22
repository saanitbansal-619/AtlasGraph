import { useState, type ReactNode } from 'react'
import type { PipelineRunSummary, PipelineValidationCheck } from '../types/api'
import { compactInt } from '../lib/format'
import { InlineError } from './States'
import { Panel } from './ui'

const DESCRIPTION =
  'GFIP validates and normalizes public trade, macroeconomic, event-risk, commodity-price, and dependency graph data before loading analytics tables into PostgreSQL.'

const WARNING_HELPER =
  'Warnings usually reflect incomplete public-source coverage or records skipped during normalization, not application failure.'

const CHECK_LABELS: Record<string, string> = {
  'missing importer codes': 'Importer code gaps',
  'missing exporter codes': 'Exporter code gaps',
  'missing macro scores': 'Missing optional macro coverage',
  'missing commodity price series': 'Missing optional price series',
}

const SCALABILITY = [
  { stage: 'Current', detail: 'local Go/Python ETL + PostgreSQL' },
  { stage: 'Next scale', detail: 'Spark batch processing' },
  { stage: 'Warehouse-ready', detail: 'Snowflake/BigQuery-compatible analytics schema' },
] as const

function statusLabel(status: string) {
  switch (status.toLowerCase()) {
    case 'completed':
      return 'Completed'
    case 'warning':
      return 'Warning'
    case 'failed':
      return 'Failed'
    default:
      return status
  }
}

function statusBanner(status: string) {
  switch (status.toLowerCase()) {
    case 'warning':
      return {
        text: 'Pipeline completed with data-quality warnings. No critical failures detected.',
        className: 'border-amber-500/30 bg-amber-500/5 text-amber-100/90',
      }
    case 'failed':
      return {
        text: 'Pipeline run reported critical validation failures or missing core datasets.',
        className: 'border-rose-500/30 bg-rose-500/5 text-rose-100/90',
      }
    case 'completed':
      return {
        text: 'Pipeline completed successfully. Analytics tables are ready for use.',
        className: 'border-emerald-500/30 bg-emerald-500/5 text-emerald-100/90',
      }
    default:
      return null
  }
}

function formatCheckName(name: string) {
  return CHECK_LABELS[name.toLowerCase()] ?? name
}

function formatTimestamp(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function formatDetails(details?: PipelineValidationCheck['details']) {
  if (details == null) return '—'
  if (typeof details === 'string') return details
  if (typeof details.note === 'string') return details.note
  if (typeof details.error === 'string') return details.error
  if (typeof details.count === 'number') return `${compactInt(details.count)} records`
  return JSON.stringify(details)
}

function checkStatusClass(status: string) {
  switch (status.toLowerCase()) {
    case 'passed':
      return 'text-emerald-300'
    case 'warning':
      return 'text-amber-300'
    case 'failed':
      return 'text-rose-300'
    default:
      return 'text-slate-300'
  }
}

function SummaryCard({
  label,
  value,
  tone = 'default',
}: {
  label: string
  value: string
  tone?: 'default' | 'good' | 'warn' | 'bad'
}) {
  const valueClass =
    tone === 'good'
      ? 'text-emerald-300'
      : tone === 'warn'
        ? 'text-amber-300'
        : tone === 'bad'
          ? 'text-rose-300'
          : 'text-cyan-200'
  return (
    <div className="rounded border border-slate-800 bg-slate-900/30 px-2.5 py-2">
      <div className="text-[10px] uppercase tracking-wide text-slate-500">{label}</div>
      <div className={`mt-0.5 font-mono text-sm font-semibold ${valueClass}`}>{value}</div>
    </div>
  )
}

function CollapsibleSection({
  title,
  open,
  onToggle,
  children,
  hint,
}: {
  title: string
  open: boolean
  onToggle: () => void
  children: ReactNode
  hint?: string
}) {
  return (
    <div className="rounded border border-slate-800/80">
      <button
        type="button"
        className="flex w-full items-center justify-between gap-2 px-2.5 py-2 text-left hover:bg-slate-900/40"
        onClick={onToggle}
        aria-expanded={open}
      >
        <div>
          <div className="text-xs font-medium text-slate-200">{title}</div>
          {hint && !open && <div className="text-[10px] text-slate-500">{hint}</div>}
        </div>
        <span className="shrink-0 font-mono text-[11px] text-slate-500">{open ? '−' : '+'}</span>
      </button>
      {open && <div className="border-t border-slate-800/80 px-2.5 py-2">{children}</div>}
    </div>
  )
}

function LoadingSkeleton() {
  return (
    <div className="space-y-2">
      <div className="grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-6">
        {Array.from({ length: 6 }, (_, index) => (
          <div key={index} className="rounded border border-slate-800 bg-slate-900/30 px-2.5 py-2">
            <div className="h-3 w-16 animate-pulse rounded bg-slate-800" />
            <div className="mt-1.5 h-5 w-10 animate-pulse rounded bg-slate-800" />
          </div>
        ))}
      </div>
      <div className="h-16 animate-pulse rounded border border-slate-800 bg-slate-900/30" />
    </div>
  )
}

export function DataPipelineMonitor({
  summary,
  loading,
  error,
}: {
  summary: PipelineRunSummary | null
  loading: boolean
  error?: { message: string; hint?: string } | null
}) {
  const [validationOpen, setValidationOpen] = useState(true)
  const [sourcesOpen, setSourcesOpen] = useState(false)
  const [tablesOpen, setTablesOpen] = useState(false)
  const [scalabilityOpen, setScalabilityOpen] = useState(false)

  const banner = summary ? statusBanner(summary.status) : null
  const statusTone =
    summary?.status === 'completed'
      ? 'good'
      : summary?.status === 'warning'
        ? 'warn'
        : summary?.status === 'failed'
          ? 'bad'
          : 'default'

  return (
    <Panel title="Data Operations Monitor">
      <p className="mb-2 text-xs leading-relaxed text-slate-400">{DESCRIPTION}</p>

      {loading && !summary && <LoadingSkeleton />}

      {error && !loading && <InlineError message={error.message} hint={error.hint} />}

      {!loading && !error && summary?.status === 'idle' && (
        <div className="rounded border border-dashed border-slate-700 bg-slate-900/30 px-3 py-4 text-sm text-slate-400">
          No processed pipeline data is available yet. Run ingest and database load commands to
          populate the analytics layer.
        </div>
      )}

      {summary && summary.status !== 'idle' && (
        <div className="space-y-2.5">
          {banner && (
            <div className={`rounded border px-3 py-2 text-xs leading-relaxed ${banner.className}`}>
              {banner.text}
            </div>
          )}

          <div className="grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-6">
            <SummaryCard
              label="Pipeline Status"
              value={statusLabel(summary.status)}
              tone={statusTone}
            />
            <SummaryCard label="Rows Loaded" value={compactInt(summary.total_rows_loaded)} />
            <SummaryCard
              label="Data Sources"
              value={compactInt(summary.sources_processed.length)}
            />
            <SummaryCard
              label="Checks Passed"
              value={compactInt(summary.validation_checks_passed)}
              tone="good"
            />
            <SummaryCard
              label="Warnings"
              value={compactInt(summary.validation_checks_warnings)}
              tone={summary.validation_checks_warnings > 0 ? 'warn' : 'default'}
            />
            <SummaryCard
              label="Failed Checks"
              value={compactInt(summary.validation_checks_failed)}
              tone={summary.validation_checks_failed > 0 ? 'bad' : 'default'}
            />
          </div>

          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[10px] text-slate-500">
            <span>
              <span className="text-slate-400">Last run:</span> {formatTimestamp(summary.completed_at)}
            </span>
            <span>
              <span className="text-slate-400">Skipped / flagged rows:</span>{' '}
              {compactInt(summary.invalid_rows)}
            </span>
            <span>
              <span className="text-slate-400">Processed:</span>{' '}
              {compactInt(summary.total_rows_processed)}
            </span>
          </div>

          {(summary.validation_checks_warnings > 0 || summary.invalid_rows > 0) && (
            <p className="text-[11px] leading-relaxed text-slate-500">{WARNING_HELPER}</p>
          )}

          {summary.validation_checks.length > 0 && (
            <CollapsibleSection
              title="Validation Details"
              open={validationOpen}
              onToggle={() => setValidationOpen((v) => !v)}
              hint={`${summary.validation_checks.length} checks`}
            >
              <div className="overflow-x-auto">
                <table className="min-w-full text-left text-[11px]">
                  <thead className="text-slate-500">
                    <tr>
                      <th className="pb-1.5 pr-2 font-medium">Check</th>
                      <th className="pb-1.5 pr-2 font-medium">Source</th>
                      <th className="pb-1.5 pr-2 font-medium">Status</th>
                      <th className="pb-1.5 pr-2 text-right font-medium">Metric</th>
                      <th className="pb-1.5 font-medium">Details</th>
                    </tr>
                  </thead>
                  <tbody>
                    {summary.validation_checks.map((check) => (
                      <tr key={`${check.check_name}-${check.source}`} className="border-t border-slate-800/70">
                        <td className="py-1.5 pr-2 text-slate-200">{formatCheckName(check.check_name)}</td>
                        <td className="py-1.5 pr-2 text-slate-400">{check.source}</td>
                        <td className={`py-1.5 pr-2 capitalize ${checkStatusClass(check.status)}`}>
                          {check.status}
                        </td>
                        <td className="py-1.5 pr-2 text-right font-mono text-slate-300">
                          {compactInt(check.metric_value)}
                        </td>
                        <td className="py-1.5 text-slate-500">{formatDetails(check.details)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </CollapsibleSection>
          )}

          <CollapsibleSection
            title="Rows by Source"
            open={sourcesOpen}
            onToggle={() => setSourcesOpen((v) => !v)}
            hint={`${summary.sources_processed.length} sources`}
          >
            <div className="overflow-x-auto">
              <table className="min-w-full text-left text-[11px]">
                <thead className="text-slate-500">
                  <tr>
                    <th className="pb-1.5 pr-2 font-medium">Source</th>
                    <th className="pb-1.5 pr-2 font-medium">Dataset</th>
                    <th className="pb-1.5 pr-2 text-right font-medium">Processed</th>
                    <th className="pb-1.5 text-right font-medium">Loaded</th>
                  </tr>
                </thead>
                <tbody>
                  {summary.sources_processed.map((row) => (
                    <tr key={row.name} className="border-t border-slate-800/70">
                      <td className="py-1.5 pr-2 text-slate-200">{row.name}</td>
                      <td className="py-1.5 pr-2 text-slate-400">{row.source}</td>
                      <td className="py-1.5 pr-2 text-right font-mono text-cyan-200">
                        {compactInt(row.rows_processed)}
                      </td>
                      <td className="py-1.5 text-right font-mono text-slate-300">
                        {row.rows_loaded > 0 ? compactInt(row.rows_loaded) : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CollapsibleSection>

          {summary.output_tables.length > 0 && (
            <CollapsibleSection
              title="Output Tables"
              open={tablesOpen}
              onToggle={() => setTablesOpen((v) => !v)}
              hint={`${summary.output_tables.length} tables`}
            >
              <div className="flex flex-wrap gap-1">
                {summary.output_tables.map((table) => (
                  <span
                    key={table}
                    className="rounded border border-slate-700 bg-slate-900/50 px-1.5 py-0.5 font-mono text-[10px] text-slate-300"
                  >
                    {table}
                  </span>
                ))}
              </div>
            </CollapsibleSection>
          )}

          <CollapsibleSection
            title="Scalability Path"
            open={scalabilityOpen}
            onToggle={() => setScalabilityOpen((v) => !v)}
            hint="Current and future scale"
          >
            <div className="grid gap-1.5 sm:grid-cols-3">
              {SCALABILITY.map((item) => (
                <div key={item.stage} className="rounded border border-slate-800/80 bg-slate-900/20 px-2 py-1.5">
                  <div className="text-[10px] uppercase tracking-wide text-slate-500">{item.stage}</div>
                  <div className="mt-0.5 text-[11px] text-slate-300">{item.detail}</div>
                </div>
              ))}
            </div>
          </CollapsibleSection>

          {summary.notes && summary.notes.length > 0 && (
            <div className="rounded border border-slate-800/60 bg-slate-900/20 px-2.5 py-1.5 text-[10px] text-slate-500">
              {summary.notes.map((note) => (
                <p key={note}>{note}</p>
              ))}
            </div>
          )}
        </div>
      )}
    </Panel>
  )
}
