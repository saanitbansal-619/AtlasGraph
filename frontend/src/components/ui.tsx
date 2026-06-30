import type { ReactNode } from 'react'
import { typeClass } from '../lib/format'

export function Panel({
  title,
  right,
  children,
  noPad = false,
  className = '',
}: {
  title?: string
  right?: ReactNode
  children: ReactNode
  noPad?: boolean
  className?: string
}) {
  return (
    <section className={`panel ${className}`}>
      {title && (
        <div className="panel-header">
          <span className="panel-title">{title}</span>
          {right}
        </div>
      )}
      <div className={noPad ? '' : 'p-4'}>{children}</div>
    </section>
  )
}

export function Stat({
  label,
  value,
  accent = false,
}: {
  label: string
  value: ReactNode
  accent?: boolean
}) {
  return (
    <div className="panel px-4 py-3">
      <div className="label">{label}</div>
      <div className={`stat-value mt-1 ${accent ? 'text-cyan-300' : ''}`}>{value}</div>
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
