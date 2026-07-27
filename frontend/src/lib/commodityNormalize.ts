import { normalizeEntityKey, sameEntityConcept } from './shockEntities'

/** Canonical strategic commodities from the GFIP dependency graph datasets. */
export const STRATEGIC_COMMODITIES = [
  'semiconductors',
  'crude oil',
  'natural gas',
  'lng',
  'lithium',
  'cobalt',
  'nickel',
  'copper',
  'rare earths',
  'wheat',
  'corn',
  'rice',
  'fertilizer',
  'uranium',
  'steel',
  'aluminum',
  'batteries',
  'solar panels',
  'pharmaceuticals',
  'shipping containers',
] as const

const CANONICAL_ALIASES: Record<string, string> = {
  lng: 'natural gas',
  'liquefied natural gas': 'natural gas',
  'natural gas': 'natural gas',
  'rare earth': 'rare earths',
  'rare earth metals': 'rare earths',
  'rare earth compounds': 'rare earths',
  oil: 'crude oil',
  petroleum: 'crude oil',
  'lithium carbonate': 'lithium',
  'lithium carbonates': 'lithium',
}

/** Collapse aliases (e.g. LNG → natural gas) for display and aggregation. */
export function normalizeCommodityLabel(name: string): string {
  const key = normalizeEntityKey(name)
  if (!key) return name.trim()
  if (CANONICAL_ALIASES[key]) return CANONICAL_ALIASES[key]
  for (const [alias, canonical] of Object.entries(CANONICAL_ALIASES)) {
    if (sameEntityConcept(key, alias)) return canonical
  }
  return name.trim().toLowerCase()
}

export function isStrategicCommodity(name: string): boolean {
  const normalized = normalizeCommodityLabel(name)
  const key = normalizeEntityKey(normalized)
  return STRATEGIC_COMMODITIES.some(
    (c) => normalizeEntityKey(normalizeCommodityLabel(c)) === key || sameEntityConcept(c, name),
  )
}

export type RankedCommodity = {
  label: string
  value: number
}

/**
 * Normalize commodity impact rows: merge LNG/natural gas, drop unsupported names,
 * and sort by descending score.
 */
export function normalizeCommodityRankings(
  rows: Array<{ label: string; value: number }>,
): RankedCommodity[] {
  const merged = new Map<string, RankedCommodity>()
  for (const row of rows) {
    if (!isStrategicCommodity(row.label)) continue
    const label = normalizeCommodityLabel(row.label)
    const key = normalizeEntityKey(label)
    const existing = merged.get(key)
    if (!existing || row.value > existing.value) {
      merged.set(key, { label, value: row.value })
    }
  }
  return [...merged.values()].sort(
    (a, b) => b.value - a.value || a.label.localeCompare(b.label),
  )
}

/** Sort country impact rows by descending score. */
export function sortCountriesByScore(
  rows: Array<{ label: string; value: number }>,
): Array<{ label: string; value: number }> {
  return [...rows].sort((a, b) => b.value - a.value || a.label.localeCompare(b.label))
}
