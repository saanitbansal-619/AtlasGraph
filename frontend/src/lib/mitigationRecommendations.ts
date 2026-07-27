import type {
  CustomDataAnalysisResponse,
  ScenarioReportResponse,
  ShockResponse,
} from '../types/api'
import { computeClientExposureOverlay } from './clientExposure'
import { normalizeCommodityLabel } from './commodityNormalize'

export type MitigationCategory =
  | 'diversify_suppliers'
  | 'increase_inventory'
  | 'increase_monitoring'
  | 'alternative_sourcing'
  | 'logistics_rerouting'
  | 'strategic_stockpiling'

export type MitigationDifficulty = 'Low' | 'Medium' | 'High'
export type MitigationConfidence = 'Low' | 'Medium' | 'High'

export interface MitigationInputs {
  hhi: number | null
  supplierShare: number | null
  eventRisk: number
  macroRisk: number
  commodity: string
  shockType: string
  dropPercent: number
  inventoryBufferDays: number | null
  source: string
}

export interface MitigationRecommendation {
  category: MitigationCategory
  title: string
  priority: number
  reason: string
  expectedImpact: string
  difficulty: MitigationDifficulty
  confidence: MitigationConfidence
}

const CATEGORY_TITLES: Record<MitigationCategory, string> = {
  diversify_suppliers: 'Diversify suppliers',
  increase_inventory: 'Increase inventory',
  increase_monitoring: 'Increase monitoring',
  alternative_sourcing: 'Alternative sourcing',
  logistics_rerouting: 'Logistics rerouting',
  strategic_stockpiling: 'Strategic stockpiling',
}

const ENERGY_COMMODITIES = new Set(['crude oil', 'natural gas', 'lng', 'uranium'])
const FOOD_COMMODITIES = new Set(['wheat', 'corn', 'rice'])
const CRITICAL_MINERALS = new Set([
  'lithium',
  'cobalt',
  'nickel',
  'copper',
  'rare earths',
  'uranium',
])

function norm(value: string): string {
  return value.trim().toLowerCase()
}

function riskLevelToScore(level?: string): number | null {
  switch (level) {
    case 'Critical':
      return 90
    case 'High':
      return 75
    case 'Medium':
      return 50
    case 'Low':
      return 25
    default:
      return null
  }
}

function contextScore(
  items: { entity: string; available: boolean; score?: number; risk_level?: string }[],
  source: string,
  fallback: number,
): number {
  const sourceKey = norm(source)
  const direct = items.find((item) => norm(item.entity) === sourceKey && item.available)
  if (direct?.score != null) return direct.score
  if (direct?.risk_level) return riskLevelToScore(direct.risk_level) ?? fallback

  const available = items.filter((item) => item.available)
  if (available.length === 0) return fallback

  const scored = available
    .map((item) => item.score ?? riskLevelToScore(item.risk_level) ?? 0)
    .filter((score) => score > 0)
  if (scored.length === 0) return fallback
  return Math.max(...scored)
}

function concentrationFromReport(
  report: ScenarioReportResponse | null | undefined,
  commodity: string,
): { hhi: number | null; supplierShare: number | null } {
  if (!report?.trade_evidence?.length) return { hhi: null, supplierShare: null }
  const commodityKey = norm(normalizeCommodityLabel(commodity))
  const match =
    report.trade_evidence.find((row) => norm(normalizeCommodityLabel(row.commodity)) === commodityKey) ??
    report.trade_evidence[0]
  return {
    hhi: match.hhi ?? null,
    supplierShare: match.top_supplier_share ?? null,
  }
}

export function buildMitigationInputs(
  result: ShockResponse,
  clientData?: CustomDataAnalysisResponse | null,
  report?: ScenarioReportResponse | null,
): MitigationInputs {
  const scenario = result.scenario
  const overlay = computeClientExposureOverlay(clientData, scenario.source, scenario.commodity)
  const reportConcentration = concentrationFromReport(report, scenario.commodity)

  const overlayHhi = overlay?.exposures.find((row) => row.hhi != null)?.hhi ?? null
  const hhi = overlayHhi ?? reportConcentration.hhi
  const supplierShare =
    overlay && overlay.matchedCount > 0 ? overlay.topShare : reportConcentration.supplierShare

  const fusion = result.data_fusion
  const eventFallback = fusion?.real_event_risk_used ? 52 : 28
  const macroFallback = fusion?.data_sources?.some((source) => /macro/i.test(source)) ? 48 : 26

  const eventRisk = contextScore(report?.event_risk_context ?? [], scenario.source, eventFallback)
  const macroRisk = contextScore(report?.macro_context ?? [], scenario.source, macroFallback)

  return {
    hhi,
    supplierShare,
    eventRisk,
    macroRisk,
    commodity: normalizeCommodityLabel(scenario.commodity),
    shockType: scenario.shock_type || result.shock_profile.type,
    dropPercent: scenario.shock_percent,
    inventoryBufferDays: result.operational_assumptions?.inventory_buffer_days ?? null,
    source: scenario.source,
  }
}

function clampPriority(value: number): number {
  return Math.max(1, Math.min(100, Math.round(value)))
}

function confidenceFromSignals(
  known: number,
  total: number,
  strongThreshold = 2,
): MitigationConfidence {
  if (known >= strongThreshold) return 'High'
  if (known >= 1) return 'Medium'
  return total > 0 ? 'Low' : 'Medium'
}

function commodityFamily(commodity: string): 'energy' | 'food' | 'minerals' | 'semiconductors' | 'other' {
  if (ENERGY_COMMODITIES.has(commodity)) return 'energy'
  if (FOOD_COMMODITIES.has(commodity)) return 'food'
  if (CRITICAL_MINERALS.has(commodity)) return 'minerals'
  if (commodity === 'semiconductors') return 'semiconductors'
  return 'other'
}

function shockLabel(shockType: string): string {
  return shockType.replace(/_/g, ' ')
}

type RuleResult = Omit<MitigationRecommendation, 'title'> & { title?: string }

function ruleDiversifySuppliers(input: MitigationInputs): RuleResult | null {
  const concentrated = (input.hhi ?? 0) >= 0.15 || (input.supplierShare ?? 0) >= 0.4
  if (!concentrated) return null

  let priority = 42
  if (input.hhi != null) priority += input.hhi * 120
  if (input.supplierShare != null) priority += input.supplierShare * 35
  priority += input.dropPercent * 0.2

  const difficulty: MitigationDifficulty =
    (input.supplierShare ?? 0) >= 0.65 || (input.hhi ?? 0) >= 0.25
      ? 'High'
      : (input.hhi ?? 0) >= 0.15
        ? 'Medium'
        : 'Low'

  const knownSignals = [input.hhi, input.supplierShare].filter((v) => v != null).length

  return {
    category: 'diversify_suppliers',
    priority: clampPriority(priority),
    reason:
      input.hhi != null && input.supplierShare != null
        ? `Supplier concentration is elevated (HHI ${input.hhi.toFixed(2)}, top share ${(input.supplierShare * 100).toFixed(0)}%) for ${input.commodity} from ${input.source}.`
        : input.hhi != null
          ? `Import concentration HHI of ${input.hhi.toFixed(2)} indicates dependency risk on ${input.source} for ${input.commodity}.`
          : `Top supplier share of ${((input.supplierShare ?? 0) * 100).toFixed(0)}% creates single-source exposure to ${input.source}.`,
    expectedImpact:
      'Reduces single-point failure risk and can lower fragility amplification on follow-on shocks by 15–30%.',
    difficulty,
    confidence: confidenceFromSignals(knownSignals, 2),
  }
}

function ruleIncreaseInventory(input: MitigationInputs): RuleResult | null {
  const thinBuffer = input.inventoryBufferDays != null && input.inventoryBufferDays < 21
  const supplyShock = ['supply_cut', 'export_collapse', 'price_spike'].includes(input.shockType)
  const severeDrop = input.dropPercent >= 20
  const macroPressure = input.macroRisk >= 55

  if (!thinBuffer && !supplyShock && !severeDrop && !macroPressure) return null

  let priority = 36
  if (thinBuffer) priority += 22
  if (supplyShock) priority += 14
  priority += input.dropPercent * 0.45
  if (input.macroRisk >= 55) priority += 8
  if (commodityFamily(input.commodity) === 'food') priority += 10

  const difficulty: MitigationDifficulty =
    commodityFamily(input.commodity) === 'semiconductors' ? 'High' : 'Medium'

  return {
    category: 'increase_inventory',
    priority: clampPriority(priority),
    reason: thinBuffer
      ? `Scenario assumes only ${input.inventoryBufferDays} days of inventory buffer during a ${shockLabel(input.shockType)} shock with ${input.dropPercent.toFixed(0)}% supply reduction.`
      : `A ${input.dropPercent.toFixed(0)}% ${shockLabel(input.shockType)} on ${input.commodity} warrants additional buffer stock while supply normalizes.`,
    expectedImpact:
      'Extends operational runway by 2–6 weeks and dampens immediate production or fulfillment disruptions.',
    difficulty,
    confidence: input.inventoryBufferDays != null ? 'High' : 'Medium',
  }
}

function ruleIncreaseMonitoring(input: MitigationInputs): RuleResult | null {
  const elevatedContext = input.eventRisk >= 45 || input.macroRisk >= 45
  const meaningfulShock = input.dropPercent >= 15
  if (!elevatedContext && !meaningfulShock) return null

  let priority = 34
  priority += Math.max(input.eventRisk, input.macroRisk) * 0.35
  priority += input.dropPercent * 0.15
  if ((input.hhi ?? 0) >= 0.15) priority += 6

  return {
    category: 'increase_monitoring',
    priority: clampPriority(priority),
    reason: `Event-risk score ${input.eventRisk.toFixed(0)} and macro-risk score ${input.macroRisk.toFixed(0)} around ${input.source} justify tighter surveillance during the ${shockLabel(input.shockType)} scenario.`,
    expectedImpact:
      'Improves early detection of escalation paths and shortens response lead time by several days.',
    difficulty: 'Low',
    confidence: input.eventRisk >= 45 && input.macroRisk >= 45 ? 'High' : 'Medium',
  }
}

function ruleAlternativeSourcing(input: MitigationInputs): RuleResult | null {
  const supplyShock = ['supply_cut', 'export_collapse'].includes(input.shockType)
  const shareRisk = (input.supplierShare ?? 0) >= 0.35
  const concentrationRisk = (input.hhi ?? 0) >= 0.18
  if (!supplyShock && !shareRisk && !concentrationRisk) return null

  let priority = 40
  if (supplyShock) priority += 18
  if (input.supplierShare != null) priority += input.supplierShare * 40
  if (input.hhi != null) priority += input.hhi * 80
  priority += input.dropPercent * 0.25
  if (commodityFamily(input.commodity) === 'minerals') priority += 8

  const difficulty: MitigationDifficulty =
    commodityFamily(input.commodity) === 'semiconductors' ||
    commodityFamily(input.commodity) === 'minerals'
      ? 'High'
      : 'Medium'

  return {
    category: 'alternative_sourcing',
    priority: clampPriority(priority),
    reason: supplyShock
      ? `${shockLabel(input.shockType)} on ${input.source} directly threatens ${input.commodity} inflows; qualified alternate suppliers should be activated.`
      : `Concentration metrics indicate over-reliance on ${input.source} for ${input.commodity}.`,
    expectedImpact:
      'Can restore 20–50% of disrupted volume within one procurement cycle when alternates are pre-qualified.',
    difficulty,
    confidence: confidenceFromSignals(
      [input.supplierShare, input.hhi].filter((v) => v != null).length,
      2,
    ),
  }
}

function ruleLogisticsRerouting(input: MitigationInputs): RuleResult | null {
  const routeShock = input.shockType === 'route_disruption'
  const transportSensitive =
    commodityFamily(input.commodity) === 'energy' || input.commodity === 'shipping containers'
  const eventElevated = input.eventRisk >= 58

  if (!routeShock && !(transportSensitive && eventElevated)) return null

  let priority = routeShock ? 72 : 48
  priority += input.eventRisk * 0.2
  priority += input.dropPercent * 0.2

  return {
    category: 'logistics_rerouting',
    priority: clampPriority(priority),
    reason: routeShock
      ? `Route disruption shock on ${input.source} blocks established ${input.commodity} corridors; alternate lanes should be evaluated immediately.`
      : `Elevated event-risk (${input.eventRisk.toFixed(0)}) for ${input.source} increases transit uncertainty for ${input.commodity}.`,
    expectedImpact:
      'Maintains delivery continuity on secondary corridors and reduces dwell-time risk at chokepoints.',
    difficulty: routeShock ? 'Medium' : 'Low',
    confidence: routeShock ? 'High' : 'Medium',
  }
}

function ruleStrategicStockpiling(input: MitigationInputs): RuleResult | null {
  const strategic =
    commodityFamily(input.commodity) !== 'other' || input.commodity === 'fertilizer'
  const severe = input.dropPercent >= 30
  const macroPressure = input.macroRisk >= 55
  const priceShock = input.shockType === 'price_spike'

  if (!strategic || (!severe && !macroPressure && !priceShock)) return null

  let priority = 38
  priority += input.dropPercent * 0.5
  if (macroPressure) priority += 12
  if (priceShock) priority += 10
  if (commodityFamily(input.commodity) === 'energy') priority += 8

  return {
    category: 'strategic_stockpiling',
    priority: clampPriority(priority),
    reason: `A ${input.dropPercent.toFixed(0)}% shock on strategic ${input.commodity} with macro-risk ${input.macroRisk.toFixed(0)} supports building a dedicated reserve beyond routine inventory.`,
    expectedImpact:
      'Buffers against prolonged supply tightness and reduces spot-market exposure during recovery.',
    difficulty: commodityFamily(input.commodity) === 'energy' ? 'High' : 'Medium',
    confidence: severe && macroPressure ? 'High' : 'Medium',
  }
}

const RULES: Array<(input: MitigationInputs) => RuleResult | null> = [
  ruleDiversifySuppliers,
  ruleIncreaseInventory,
  ruleIncreaseMonitoring,
  ruleAlternativeSourcing,
  ruleLogisticsRerouting,
  ruleStrategicStockpiling,
]

export function generateMitigationRecommendations(
  input: MitigationInputs,
): MitigationRecommendation[] {
  const recommendations: MitigationRecommendation[] = []

  for (const rule of RULES) {
    const result = rule(input)
    if (!result) continue
    recommendations.push({
      ...result,
      title: result.title ?? CATEGORY_TITLES[result.category],
    })
  }

  recommendations.sort((a, b) => b.priority - a.priority || a.title.localeCompare(b.title))
  return recommendations
}

export function mitigationRecommendationsForScenario(
  result: ShockResponse,
  clientData?: CustomDataAnalysisResponse | null,
  report?: ScenarioReportResponse | null,
): MitigationRecommendation[] {
  const inputs = buildMitigationInputs(result, clientData, report)
  return generateMitigationRecommendations(inputs)
}
