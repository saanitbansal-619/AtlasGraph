import type { ReactNode } from 'react'

export function StatCard({
  label,
  value,
  accent = false,
  compact = false,
}: {
  label: string
  value: ReactNode
  accent?: boolean
  compact?: boolean
}) {
  return (
    <div className={`panel ${compact ? 'px-3 py-2' : 'px-4 py-3'}`}>
      <div className="label">{label}</div>
      <div
        className={`stat-value ${compact ? 'mt-0.5 text-xl' : 'mt-1'} ${
          accent ? 'text-cyan-300' : ''
        }`}
      >
        {value}
      </div>
    </div>
  )
}
