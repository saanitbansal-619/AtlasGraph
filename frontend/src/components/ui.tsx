import type { ReactNode } from 'react'
import { typeClass } from '../lib/format'

export function Panel({
  title,
  right,
  children,
  noPad = false,
  dense = false,
  className = '',
}: {
  title?: string
  right?: ReactNode
  children: ReactNode
  noPad?: boolean
  dense?: boolean
  className?: string
}) {
  return (
    <section className={`panel flex flex-col ${className}`}>
      {title && (
        <div className={`panel-header ${dense ? 'px-3 py-1.5' : ''}`}>
          <span className="panel-title">{title}</span>
          {right}
        </div>
      )}
      <div className={`${noPad ? 'flex-1' : dense ? 'flex flex-1 flex-col p-3' : 'flex flex-1 flex-col p-4'}`}>
        {children}
      </div>
    </section>
  )
}

export function Stat({
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
      <div className={`stat-value ${compact ? 'mt-0.5 text-xl' : 'mt-1'} ${accent ? 'text-cyan-300' : ''}`}>
        {value}
      </div>
    </div>
  )
}

export function TypeBadge({ type }: { type: string }) {
  return <span className={`badge ${typeClass(type)}`}>{type}</span>
}

export function Dot({ className = '' }: { className?: string }) {
  return <span className={`inline-block h-2 w-2 rounded-full ${className}`} />
}

export function Spinner({ className = '' }: { className?: string }) {
  return (
    <span
      className={`inline-block h-4 w-4 animate-spin rounded-full border-2 border-slate-600 border-t-cyan-400 ${className}`}
      aria-label="loading"
    />
  )
}

export function EmptyHint({ children }: { children: ReactNode }) {
  return (
    <div className="rounded border border-dashed border-slate-800 px-4 py-8 text-center text-sm text-slate-500">
      {children}
    </div>
  )
}
