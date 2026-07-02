// Small presentation helpers for the control-room number formatting.

export const pct = (v: number, digits = 1): string =>
  `${(v * 100).toFixed(digits)}%`

export const fixed = (v: number, digits = 1): string => v.toFixed(digits)

export const signed = (v: number, digits = 1): string =>
  `${v >= 0 ? '+' : ''}${v.toFixed(digits)}`

export const compactInt = (v: number): string =>
  new Intl.NumberFormat('en-US').format(v)

// deltaClass colours a fragility delta by severity for at-a-glance scanning.
export function deltaClass(v: number): string {
  if (v >= 15) return 'text-rose-400'
  if (v >= 7) return 'text-amber-300'
  if (v > 0) return 'text-cyan-300'
  return 'text-slate-400'
}

// Risk band derived from a 0..100 fragility-style value, matching the engine's
// qualitative bands (Low/Medium/High/Critical).
export type RiskBand = 'Low' | 'Medium' | 'High' | 'Critical'

export function riskBand(score: number): RiskBand {
  if (score >= 80) return 'Critical'
  if (score >= 60) return 'High'
  if (score >= 30) return 'Medium'
  return 'Low'
}

// Format relationship_type for display (exports, used_by → used by).
export function formatRelationship(rel: string): string {
  return rel.replace(/_/g, ' ')
}

// Group blocked-edge reasons into analyst-facing categories.
export function blockedEdgeCategory(reason: string): string {
  if (reason.includes('cross-commodity branch blocked')) {
    return 'Cross-commodity branch blocked'
  }
  if (reason.includes('relationship not propagated')) {
    return 'Relationship not propagated by shock type'
  }
  if (reason.includes('propagation disabled on edge')) {
    return 'Propagation disabled on edge'
  }
  if (reason.includes('edge restricted') || reason.includes('edge allows')) {
    return 'Edge restricted to other shock types'
  }
  return 'Other branch blocked'
}

const TYPE_STYLES: Record<string, string> = {
  country: 'border-cyan-500/40 text-cyan-300 bg-cyan-500/10',
  commodity: 'border-amber-500/40 text-amber-300 bg-amber-500/10',
  sector: 'border-violet-500/40 text-violet-300 bg-violet-500/10',
  route: 'border-emerald-500/40 text-emerald-300 bg-emerald-500/10',
  company: 'border-slate-500/40 text-slate-300 bg-slate-500/10',
}

export function typeClass(type: string): string {
  return TYPE_STYLES[type] ?? 'border-slate-600/40 text-slate-300 bg-slate-600/10'
}

// riskBadgeClass colours a Low/Medium/High/Critical band badge.
export function riskBadgeClass(level: string): string {
  switch (level) {
    case 'Critical':
      return 'border-rose-500/50 bg-rose-500/15 text-rose-300'
    case 'High':
      return 'border-amber-500/50 bg-amber-500/15 text-amber-300'
    case 'Medium':
      return 'border-yellow-500/40 bg-yellow-500/10 text-yellow-200'
    default:
      return 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300'
  }
}
