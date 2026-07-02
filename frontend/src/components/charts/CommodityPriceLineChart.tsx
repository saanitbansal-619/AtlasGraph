import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { CommodityHistoryPoint } from '../../types/api'
import { fixed } from '../../lib/format'

export function CommodityPriceLineChart({
  points,
  height = 220,
}: {
  points: CommodityHistoryPoint[]
  height?: number
}) {
  const chartData = points.map((p) => ({
    month: p.month,
    price: p.price,
    label: p.month.length >= 7 ? p.month.slice(0, 7) : p.month,
  }))

  if (chartData.length === 0) {
    return null
  }

  return (
    <div style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={chartData} margin={{ top: 8, right: 8, bottom: 4, left: 0 }}>
          <CartesianGrid stroke="rgba(148,163,184,0.06)" vertical={false} />
          <XAxis
            dataKey="label"
            tick={{ fill: 'rgba(148,163,184,0.55)', fontSize: 10 }}
            axisLine={{ stroke: 'rgba(148,163,184,0.12)' }}
            tickLine={false}
            minTickGap={24}
          />
          <YAxis
            tick={{ fill: 'rgba(148,163,184,0.55)', fontSize: 10 }}
            axisLine={false}
            tickLine={false}
            width={48}
            tickFormatter={(v) => fixed(Number(v), 0)}
          />
          <Tooltip
            cursor={{ stroke: 'rgba(148,163,184,0.15)' }}
            content={({ active, payload, label }) => {
              if (!active || !payload?.length) return null
              return (
                <div className="rounded border border-slate-700/70 bg-slate-950/95 px-3 py-2 text-xs shadow-panel">
                  <div className="font-mono text-slate-400">{label}</div>
                  <div className="mt-1 font-mono text-slate-100">
                    {fixed(Number(payload[0].value), 2)} USD
                  </div>
                </div>
              )
            }}
          />
          <Line
            type="monotone"
            dataKey="price"
            stroke="rgba(34,211,238,0.75)"
            strokeWidth={1.5}
            dot={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
