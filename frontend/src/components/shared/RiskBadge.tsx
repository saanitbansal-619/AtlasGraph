import { riskBadgeClass } from '../../lib/format'

export function RiskBadge({ level }: { level: string }) {
  return <span className={`badge ${riskBadgeClass(level)}`}>{level}</span>
}
