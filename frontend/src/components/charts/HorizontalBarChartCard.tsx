import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  LabelList,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Panel } from '../ui'
import { fixed, signed } from '../../lib/format'
import { chartHeightForRows, clampItems } from './chartUtils'

export type HorizontalBarDatum = {
  label: string
  value: number
  meta?: Record<string, string>
  color?: string
}

function defaultColor(i: number): string {
  const palette = [
    'rgba(34,211,238,0.55)',
    'rgba(56,189,248,0.50)',
    'rgba(129,140,248,0.48)',
    'rgba(167,139,250,0.45)',
    'rgba(251,191,36,0.42)',
    'rgba(248,113,113,0.40)',
  ]
  return palette[i % palette.length]
}

function formatBarValue(value: number, digits: number, suffix?: string): string {
  const s = suffix?.trim() ?? ''
  if (s.includes('Δ') || s === 'Δ') return `${signed(value, digits)} Δ`
  if (s.includes('score')) return `${fixed(value, digits)} score`
  if (s) return `${fixed(value, digits)}${s.startsWith(' ') ? s : ` ${s}`}`
  return fixed(value, digits)
}

export function HorizontalBarChartCard({
  title,
  subtitle,
  data,
  valueLabel,
  emptyLabel = 'No data available.',
  height,
  topN = 8,
  valueDigits = 1,
  showValueLabels = true,
  valueSuffix,
  barSize = 10,
}: {
  title: string
  subtitle?: string
  data: HorizontalBarDatum[]
  valueLabel: string
  emptyLabel?: string
  height?: number
  topN?: number
  valueDigits?: number
  showValueLabels?: boolean
  valueSuffix?: string
  barSize?: number
}) {
  const trimmed = clampItems(data, topN)
  const chartHeight = height ?? chartHeightForRows(trimmed.length)

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
      {trimmed.length === 0 ? (
        <div className="px-4 py-5 text-sm text-slate-500">{emptyLabel}</div>
      ) : (
        <div style={{ height: chartHeight }} className="px-2 py-2">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={trimmed}
              layout="vertical"
              margin={{ top: 4, right: showValueLabels ? 44 : 12, bottom: 4, left: 4 }}
              barCategoryGap="28%"
              barGap={4}
            >
              <CartesianGrid stroke="rgba(148,163,184,0.06)" horizontal={false} />
              <XAxis
                type="number"
                tick={{ fill: 'rgba(148,163,184,0.55)', fontSize: 10 }}
                axisLine={{ stroke: 'rgba(148,163,184,0.12)' }}
                tickLine={false}
              />
              <YAxis
                type="category"
                dataKey="label"
                width={132}
                tick={{ fill: 'rgba(203,213,225,0.82)', fontSize: 10 }}
                axisLine={false}
                tickLine={false}
              />
              <Tooltip
                cursor={{ fill: 'rgba(148,163,184,0.04)' }}
                content={
                  <BarTooltip
                    valueLabel={valueLabel}
                    digits={valueDigits}
                    valueSuffix={valueSuffix}
                  />
                }
              />
              <Bar
                dataKey="value"
                barSize={barSize}
                radius={[2, 4, 4, 2]}
                isAnimationActive={false}
              >
                {trimmed.map((d, i) => (
                  <Cell key={`${d.label}-${i}`} fill={d.color ?? defaultColor(i)} />
                ))}
                {showValueLabels && (
                  <LabelList
                    dataKey="value"
                    position="right"
                    formatter={(v) =>
                      formatBarValue(Number(v), valueDigits, valueSuffix)
                    }
                    className="fill-slate-400 font-mono text-[10px]"
                  />
                )}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </Panel>
  )
}

function BarTooltip({
  active,
  payload,
  label,
  valueLabel,
  digits,
  valueSuffix,
}: {
  active?: boolean
  payload?: Array<{ payload: HorizontalBarDatum; value: number }>
  label?: string
  valueLabel: string
  digits: number
  valueSuffix?: string
}) {
  if (!active || !payload || payload.length === 0) return null
  const row = payload[0].payload
  const meta = row.meta ?? {}
  const metaEntries = Object.entries(meta).filter(([, v]) => v)
  return (
    <div className="rounded border border-slate-700/70 bg-slate-950/95 px-3 py-2 text-xs shadow-panel">
      <div className="font-semibold text-slate-100">{label}</div>
      <div className="mt-1 font-mono text-slate-200">
        {valueLabel}:{' '}
        <span className="text-cyan-300/90">
          {formatBarValue(row.value, digits, valueSuffix)}
        </span>
      </div>
      {metaEntries.length > 0 && (
        <div className="mt-1.5 space-y-0.5 text-[11px] text-slate-400">
          {metaEntries.map(([k, v]) => (
            <div key={k}>
              <span className="text-slate-500">{k}:</span> {v}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
