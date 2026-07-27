import type { ReactNode } from 'react'

export function SectionCard({
  title,
  right,
  children,
  dense = false,
  className = '',
}: {
  title?: string
  right?: ReactNode
  children: ReactNode
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
      <div className={dense ? 'flex flex-1 flex-col p-3' : 'flex flex-1 flex-col p-4'}>
        {children}
      </div>
    </section>
  )
}
