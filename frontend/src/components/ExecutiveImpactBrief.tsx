import type { ExposureItem, ShockResponse } from '../types/api'
import {
  isShockOriginEntity,
  normalizeEntityKey,
  sameEntityConcept,
  singularizeCommodity,
} from '../lib/shockEntities'
import { Panel } from './ui'

/** Multi-word / mass nouns that already read naturally as attributive modifiers. */
const NATURAL_COMMODITY_FORMS = new Set([
  'crude oil',
  'natural gas',
  'wheat',
  'copper',
  'lithium',
  'cobalt',
  'rare earths',
  'rare earth',
  'lng',
  'oil',
  'gas',
])

function formatShockType(type: string): string {
  return type.trim().replace(/_/g, ' ')
}

/** Attributive commodity form: “semiconductor export collapse”, not “semiconductors export…”. */
function commodityForBrief(raw: string): string {
  const name = raw.trim()
  if (!name) return name
  const lower = name.toLowerCase()
  if (NATURAL_COMMODITY_FORMS.has(lower)) return lower
  return singularizeCommodity(lower)
}

function joinList(names: string[]): string {
  if (names.length === 0) return ''
  if (names.length === 1) return names[0]
  if (names.length === 2) return `${names[0]} and ${names[1]}`
  return `${names.slice(0, -1).join(', ')}, and ${names[names.length - 1]}`
}

function fusionDrivers(result: ShockResponse): string[] {
  const fusion = result.data_fusion
  if (!fusion) return []

  const drivers: string[] = []
  if (fusion.real_trade_edges_used) {
    drivers.push('real UN Comtrade dependencies')
  }
  if (fusion.real_event_risk_used) {
    drivers.push('GDELT event-risk context')
  }
  if (fusion.real_price_stress_used) {
    drivers.push('World Bank price signals')
  }
  if (fusion.fusion_enabled || drivers.length > 0) {
    drivers.push('baseline graph centrality')
  }
  return drivers
}

function itemType(it: ExposureItem): string {
  return (it.type || '').trim().toLowerCase()
}

function collectByType(result: ShockResponse, type: string): ExposureItem[] {
  const buckets: ExposureItem[][] = [
    result.highest_risk_entities?.sectors ?? [],
    result.highest_risk_entities?.countries ?? [],
    result.highest_risk_entities?.commodities ?? [],
    result.direct_exposure ?? [],
    result.second_order_exposure ?? [],
    result.changed_fragility_scores ?? [],
  ]

  const seen = new Set<string>()
  const out: ExposureItem[] = []
  for (const bucket of buckets) {
    for (const it of bucket) {
      if (itemType(it) !== type) continue
      const key = normalizeEntityKey(it.entity)
      if (!key || seen.has(key)) continue
      seen.add(key)
      out.push(it)
    }
  }
  out.sort((a, b) => b.delta - a.delta || b.impact - a.impact)
  return out
}

/**
 * Downstream-focused exposure list for the brief:
 * exclude source + shocked commodity; prefer sectors → countries → routes → other commodities.
 */
function selectImpactedEntities(result: ShockResponse, max = 4): string[] {
  const source = result.scenario.source
  const commodity = result.scenario.commodity

  const pushUnique = (into: string[], name: string) => {
    if (!name || isShockOriginEntity(name, source, commodity)) return
    if (into.some((existing) => sameEntityConcept(existing, name))) return
    into.push(name)
  }

  const shockLooksLikeCommodity = Boolean(commodity?.trim())
  const typeOrder = shockLooksLikeCommodity
    ? (['sector', 'country', 'route', 'commodity'] as const)
    : (['country', 'sector', 'route', 'commodity'] as const)

  const selected: string[] = []
  for (const type of typeOrder) {
    for (const it of collectByType(result, type)) {
      pushUnique(selected, it.entity)
      if (selected.length >= max) return selected
    }
  }

  if (selected.length < max) {
    for (const path of result.affected_paths ?? []) {
      const nodes = path.path ?? []
      for (let i = nodes.length - 1; i >= 0; i--) {
        pushUnique(selected, nodes[i])
        if (selected.length >= max) return selected
      }
    }
  }

  return selected
}

/** Build a concise executive narrative from existing shock result fields (2 sentences max). */
export function buildExecutiveImpactBrief(result: ShockResponse): string {
  const sc = result.scenario
  const drop = Math.round(sc.shock_percent)
  const shockLabel = formatShockType(sc.shock_type || result.shock_profile.type)
  const commodity = commodityForBrief(sc.commodity)
  const source = sc.source

  const exposed = selectImpactedEntities(result, 4)

  const exposureClause =
    exposed.length > 0
      ? `creates elevated model-estimated exposure across ${joinList(exposed)}`
      : 'produces elevated model-estimated exposure across connected downstream industries'

  const sentence1 = `A ${drop}% ${commodity} ${shockLabel} from ${source} ${exposureClause}.`

  const drivers = fusionDrivers(result)
  const sentence2 =
    drivers.length > 0
      ? `Propagation is driven by ${joinList(drivers)}.`
      : 'Propagation follows the baseline dependency graph under the selected shock profile.'

  return `${sentence1} ${sentence2}`
}

export function ExecutiveImpactBrief({ result }: { result: ShockResponse }) {
  const brief = buildExecutiveImpactBrief(result)

  return (
    <Panel title="Executive Impact Brief" dense>
      <p className="text-sm leading-relaxed text-slate-300">{brief}</p>
    </Panel>
  )
}
