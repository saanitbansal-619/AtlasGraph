import type { GraphSummaryResponse } from '../types/api'
import { compactInt } from '../lib/format'
import { Stat } from './ui'
import { InlineError } from './States'

const SKELETON = ['Nodes', 'Countries', 'Commodities', 'Sectors', 'Dependencies']

export function OverviewCards({
  summary,
  loading,
  error,
}: {
  summary: GraphSummaryResponse | null
  loading: boolean
  error?: { message: string; hint?: string } | null
}) {
  if (error && !summary) {
    return <InlineError message={error.message} hint={error.hint} />
  }

  if (!summary) {
    return (
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        {SKELETON.map((label) => (
          <div key={label} className="panel px-4 py-3">
            <div className="label">{label}</div>
            <div className="mt-2 h-7 w-16 animate-pulse rounded bg-slate-800" />
          </div>
        ))}
      </div>
    )
  }

  const cards: Array<{ label: string; value: number; accent?: boolean }> = [
    { label: 'Nodes', value: summary.nodes },
    { label: 'Countries', value: summary.countries },
    { label: 'Commodities', value: summary.commodities },
    { label: 'Sectors', value: summary.sectors },
    { label: 'Dependencies', value: summary.dependencies, accent: true },
  ]

  return (
    <div className={`grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5 ${loading ? 'opacity-70' : ''}`}>
      {cards.map((c) => (
        <Stat key={c.label} label={c.label} value={compactInt(c.value)} accent={c.accent} />
      ))}
    </div>
  )
}
