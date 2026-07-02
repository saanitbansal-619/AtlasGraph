import { deltaClass, fixed, signed } from '../../lib/format'
import { Panel } from '../ui'
import { clampItems, sortByValueDesc } from './chartUtils'

export type RankingItem = {
  label: string
  value: number
  chip?: string
}

export function ImpactRankingList({
  title,
  subtitle,
  items,
  valueFormatter,
  valueSuffix = '',
  maxValue,
  limit = 5,
  emptyMessage = 'No data available.',
}: {
  title: string
  subtitle?: string
  items: RankingItem[]
  valueFormatter?: (value: number) => string
  valueSuffix?: string
  maxValue?: number
  limit?: number
  emptyMessage?: string
}) {
  const sorted = clampItems(sortByValueDesc(items), limit)
  const peak = maxValue ?? Math.max(...sorted.map((i) => i.value), 1)

  const displayValue = (v: number) => {
    if (valueFormatter) return valueFormatter(v)
    if (valueSuffix.includes('Δ')) return `${signed(v, 1)} Δ`
    if (valueSuffix.includes('score')) return `${fixed(v, 1)} score`
    if (valueSuffix.trim()) return `${fixed(v, 1)}${valueSuffix}`
    return fixed(v, 1)
  }

  const valueClass = (v: number) =>
    valueSuffix.includes('Δ') ? deltaClass(v) : 'text-slate-200'

  return (
    <Panel
      title={title}
      right={
        subtitle ? (
          <span className="text-[10px] font-mono uppercase tracking-wider text-slate-500">
            {subtitle}
          </span>
        ) : null
      }
      noPad
    >
      {sorted.length === 0 ? (
        <div className="px-4 py-5 text-sm text-slate-500">{emptyMessage}</div>
      ) : (
        <ul className="divide-y divide-slate-800/60 px-4 py-2">
          {sorted.map((item) => {
            const pct = peak > 0 ? Math.min(100, (item.value / peak) * 100) : 0
            return (
              <li key={item.label} className="py-2.5 first:pt-1 last:pb-1">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-sm font-medium text-slate-100">
                        {item.label}
                      </span>
                      {item.chip && (
                        <span className="shrink-0 rounded border border-slate-700/60 bg-slate-800/40 px-1.5 py-0.5 text-[10px] text-slate-400">
                          {item.chip}
                        </span>
                      )}
                    </div>
                    <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-slate-800/80">
                      <div
                        className="h-full rounded-full bg-cyan-500/45 transition-all"
                        style={{ width: `${Math.max(pct, item.value > 0 ? 4 : 0)}%` }}
                      />
                    </div>
                  </div>
                  <span
                    className={`shrink-0 font-mono text-xs font-semibold tabular-nums ${valueClass(item.value)}`}
                  >
                    {displayValue(item.value)}
                  </span>
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </Panel>
  )
}
