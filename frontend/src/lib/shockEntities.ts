import type { ExposureItem, ShockResponse } from '../types/api'

const SOURCE_ALIASES: Record<string, string[]> = {
  'democratic republic of the congo': ['drc', 'congo', 'dr congo'],
  taiwan: ['taiwan, china', 'chinese taipei'],
  'united states': ['usa', 'u.s.', 'u.s.a.', 'us', 'america'],
  'united kingdom': ['uk', 'u.k.', 'britain', 'great britain'],
  'south korea': ['korea, republic of', 'republic of korea'],
  'saudi arabia': ['ksa'],
}

const COMMODITY_SINGULARS: Record<string, string> = {
  semiconductors: 'semiconductor',
  batteries: 'battery',
  chips: 'chip',
  minerals: 'mineral',
  metals: 'metal',
  fertilizers: 'fertilizer',
  chemicals: 'chemical',
  pharmaceuticals: 'pharmaceutical',
  vehicles: 'vehicle',
  autos: 'auto',
  automobiles: 'automobile',
}

export function normalizeEntityKey(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[_\-/]+/g, ' ')
    .replace(/[^a-z0-9\s]/g, '')
    .replace(/\s+/g, ' ')
}

export function stemToken(token: string): string {
  if (token.endsWith('ies') && token.length > 4) return `${token.slice(0, -3)}y`
  if (token.endsWith('ses') || token.endsWith('xes') || token.endsWith('zes')) return token.slice(0, -2)
  if (token.endsWith('s') && !token.endsWith('ss') && token.length > 3) return token.slice(0, -1)
  return token
}

function conceptTokens(name: string): Set<string> {
  const stop = new Set(['and', 'the', 'of', 'for', 'in', 'to', 'a', 'an'])
  return new Set(
    normalizeEntityKey(name)
      .split(' ')
      .filter((t) => t.length > 1 && !stop.has(t))
      .map(stemToken),
  )
}

/** Singularize commodity labels for brief phrasing and alias matching. */
export function singularizeCommodity(raw: string): string {
  const name = raw.trim()
  if (!name) return name
  const lower = name.toLowerCase()
  const mapped = COMMODITY_SINGULARS[lower]
  if (mapped) return mapped
  return stemToken(lower)
}

export function sameEntityConcept(a: string, b: string): boolean {
  const ka = normalizeEntityKey(a)
  const kb = normalizeEntityKey(b)
  if (!ka || !kb) return false
  if (ka === kb) return true
  if (stemToken(ka) === stemToken(kb)) return true

  const ta = conceptTokens(a)
  const tb = conceptTokens(b)
  if (ta.size === 0 || tb.size === 0) return false
  if (ta.size === tb.size && [...ta].every((t) => tb.has(t))) return true

  const aSubset = [...ta].every((t) => tb.has(t))
  const bSubset = [...tb].every((t) => ta.has(t))
  if ((aSubset && ta.size === 1) || (bSubset && tb.size === 1)) return true
  return false
}

export function isShockSourceEntity(name: string, source: string): boolean {
  const n = normalizeEntityKey(name)
  const s = normalizeEntityKey(source)
  if (!n || !s) return false
  if (n === s || n.includes(s) || s.includes(n)) return true

  const aliases = SOURCE_ALIASES[s] ?? []
  for (const alias of aliases) {
    if (n === alias || n.includes(alias)) return true
  }
  for (const [canonical, list] of Object.entries(SOURCE_ALIASES)) {
    if (list.includes(s) && (n === canonical || n.includes(canonical))) return true
  }
  return false
}

export function isShockedCommodityEntity(name: string, commodity: string): boolean {
  if (!commodity.trim()) return false
  return (
    sameEntityConcept(name, commodity) || sameEntityConcept(name, singularizeCommodity(commodity))
  )
}

export function isShockOriginEntity(name: string, source: string, commodity: string): boolean {
  return isShockSourceEntity(name, source) || isShockedCommodityEntity(name, commodity)
}

function itemType(it: ExposureItem): string {
  return (it.type || '').trim().toLowerCase()
}

function collectImpactCandidates(result: ShockResponse): ExposureItem[] {
  const buckets: ExposureItem[][] = [
    result.changed_fragility_scores ?? [],
    result.direct_exposure ?? [],
    result.second_order_exposure ?? [],
    result.highest_risk_entities?.sectors ?? [],
    result.highest_risk_entities?.countries ?? [],
    result.highest_risk_entities?.commodities ?? [],
  ]

  const seen = new Set<string>()
  const out: ExposureItem[] = []
  for (const bucket of buckets) {
    for (const it of bucket) {
      const key = normalizeEntityKey(it.entity)
      if (!key || seen.has(key)) continue
      seen.add(key)
      out.push(it)
    }
  }
  return out
}

/** Nodes appearing after the shocked commodity (or source) on affected paths. */
function downstreamKeys(result: ShockResponse): Set<string> {
  const source = result.scenario.source
  const commodity = result.scenario.commodity
  const keys = new Set<string>()

  for (const path of result.affected_paths ?? []) {
    const nodes = path.path ?? []
    let originIdx = -1
    for (let i = 0; i < nodes.length; i++) {
      if (isShockedCommodityEntity(nodes[i], commodity) || isShockSourceEntity(nodes[i], source)) {
        originIdx = i
        break
      }
    }
    const start = originIdx >= 0 ? originIdx + 1 : 1
    for (let i = start; i < nodes.length; i++) {
      const n = nodes[i]
      if (isShockOriginEntity(n, source, commodity)) continue
      keys.add(normalizeEntityKey(n))
    }
  }
  return keys
}

export type TopImpactedSelection = {
  label: string
  /** True when falling back to the shocked commodity itself. */
  isDirectCommodityFallback: boolean
}

function bestByDelta(items: ExposureItem[]): ExposureItem | null {
  if (items.length === 0) return null
  return [...items].sort(
    (a, b) =>
      b.delta - a.delta ||
      b.impact - a.impact ||
      a.entity.localeCompare(b.entity),
  )[0]
}

/**
 * Highest-impact affected node for the Top Impacted KPI.
 * Priority: sector → country → other downstream entity.
 * Excludes the shock source and shocked commodity.
 */
export function selectTopImpactedEntity(result: ShockResponse): TopImpactedSelection {
  const source = result.scenario.source
  const commodity = result.scenario.commodity
  const downstream = downstreamKeys(result)

  const candidates = collectImpactCandidates(result).filter(
    (it) => !isShockOriginEntity(it.entity, source, commodity),
  )

  const ofType = (type: string) => candidates.filter((it) => itemType(it) === type)

  // 1) Highest-impact sector (matches sector chart when available).
  const topSector = bestByDelta(ofType('sector'))
  if (topSector) {
    return { label: topSector.entity, isDirectCommodityFallback: false }
  }

  // 2) Highest-impact country.
  const topCountry = bestByDelta(ofType('country'))
  if (topCountry) {
    return { label: topCountry.entity, isDirectCommodityFallback: false }
  }

  // 3) Highest-impact downstream commodity / route / other entity.
  const others = candidates.filter((it) => {
    const t = itemType(it)
    return t !== 'sector' && t !== 'country'
  })
  const downstreamOthers =
    downstream.size > 0
      ? others.filter((it) => downstream.has(normalizeEntityKey(it.entity)))
      : others
  const topOther = bestByDelta(downstreamOthers.length > 0 ? downstreamOthers : others)
  if (topOther) {
    return { label: topOther.entity, isDirectCommodityFallback: false }
  }

  // Path-only fallback: last non-origin node on highest-impact path.
  const paths = [...(result.affected_paths ?? [])].sort(
    (a, b) => b.end_impact - a.end_impact || b.path_weight - a.path_weight,
  )
  for (const path of paths) {
    const nodes = [...(path.path ?? [])].reverse()
    for (const n of nodes) {
      if (!isShockOriginEntity(n, source, commodity)) {
        return { label: n, isDirectCommodityFallback: false }
      }
    }
  }

  if (commodity.trim()) {
    return {
      label: `${commodity} (direct commodity shock)`,
      isDirectCommodityFallback: true,
    }
  }
  return { label: 'No downstream node', isDirectCommodityFallback: false }
}

