import type {
  CustomDataAnalysisResponse,
  ExecutiveActionPlan,
  MitigationRecommendation,
  PriorityDistribution,
  RecommendationPriority,
  ScenarioReportResponse,
  ShockResponse,
} from '../types/api'
import { computeClientExposureOverlay, type ClientExposureOverlay } from './clientExposure'
import { normalizeCommodityLabel } from './commodityNormalize'

export type MitigationDifficulty = 'Low' | 'Medium' | 'High'
export type MitigationConfidence = 'Low' | 'Medium' | 'High'

export interface MitigationInputs {
  source: string
  commodity: string
  shockType: string
  dropPercent: number
  depth: number
  hhi: number | null
  supplierShare: number | null
  tradeConcentration: string
  eventRisk: number
  macroExposure: number
  fragilityScore: number
  inventoryBufferDays: number | null
  affectedPaths: number
  affectedNodes: number
}

type RuleCandidate = Omit<MitigationRecommendation, 'id'> & { id?: string }

const PRIORITY_RANK: Record<RecommendationPriority, number> = {
  Critical: 4,
  High: 3,
  Medium: 2,
  Low: 1,
}

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
): { hhi: number | null; supplierShare: number | null; concentrationRisk: string } {
  if (!report?.trade_evidence?.length) {
    return { hhi: null, supplierShare: null, concentrationRisk: '' }
  }
  const commodityKey = norm(normalizeCommodityLabel(commodity))
  const match =
    report.trade_evidence.find(
      (row) => norm(normalizeCommodityLabel(row.commodity)) === commodityKey,
    ) ?? report.trade_evidence[0]
  return {
    hhi: match.hhi ?? null,
    supplierShare: match.top_supplier_share ?? null,
    concentrationRisk: match.concentration_risk ?? '',
  }
}

export function buildMitigationInputs(
  result: ShockResponse,
  clientData?: CustomDataAnalysisResponse | null,
  report?: ScenarioReportResponse | null,
): MitigationInputs {
  const scenario = result.scenario
  const overlay = computeClientExposureOverlay(
    clientData,
    scenario.source,
    scenario.commodity,
    scenario.shock_percent,
  )
  const reportConcentration = concentrationFromReport(report, scenario.commodity)

  const overlayHhi = overlay?.exposures.find((row) => row.hhi != null)?.hhi ?? null
  const hhi = overlayHhi ?? reportConcentration.hhi
  const supplierShare =
    overlay && overlay.matchedCount > 0 ? overlay.topShare : reportConcentration.supplierShare

  const fusion = result.data_fusion
  const eventFallback = fusion?.real_event_risk_used ? 52 : 28
  const macroFallback = fusion?.data_sources?.some((source) => /macro/i.test(source)) ? 48 : 26

  const s = result.graph_impact_summary
  const topCountry = result.highest_risk_entities?.countries?.[0]

  return {
    source: scenario.source,
    commodity: normalizeCommodityLabel(scenario.commodity),
    shockType: scenario.shock_type || result.shock_profile.type,
    dropPercent: scenario.shock_percent,
    depth: scenario.depth,
    hhi,
    supplierShare,
    tradeConcentration: reportConcentration.concentrationRisk,
    eventRisk: contextScore(report?.event_risk_context ?? [], scenario.source, eventFallback),
    macroExposure: contextScore(report?.macro_context ?? [], scenario.source, macroFallback),
    fragilityScore: topCountry?.delta ?? result.direct_exposure?.[0]?.delta ?? 0,
    inventoryBufferDays: result.operational_assumptions?.inventory_buffer_days ?? null,
    affectedPaths: s?.affected_paths ?? 0,
    affectedNodes: s?.affected_nodes ?? 0,
  }
}

function boostPriority(p: RecommendationPriority): RecommendationPriority {
  switch (p) {
    case 'Low':
      return 'Medium'
    case 'Medium':
      return 'High'
    case 'High':
      return 'Critical'
    default:
      return 'Critical'
  }
}

function applyDropBoost(p: RecommendationPriority, dropPercent: number): RecommendationPriority {
  return dropPercent > 40 ? boostPriority(p) : p
}

function shareValue(input: MitigationInputs): number {
  return input.supplierShare ?? 0
}

function sharePercent(input: MitigationInputs): number {
  return shareValue(input) * 100
}

function hhiValue(input: MitigationInputs): number {
  return input.hhi ?? 0
}

function shockLabel(shockType: string): string {
  return shockType.replace(/_/g, ' ')
}

function titleCase(s: string): string {
  return s.replace(/\b\w/g, (c) => c.toUpperCase())
}

function baseMetrics(input: MitigationInputs): Record<string, number> {
  const m: Record<string, number> = {
    drop_percent: input.dropPercent,
    event_risk: input.eventRisk,
    macro_exposure: input.macroExposure,
    fragility_score: input.fragilityScore,
    depth: input.depth,
  }
  if (input.hhi != null) m.hhi = input.hhi
  if (input.supplierShare != null) m.supplier_share = sharePercent(input)
  return m
}

function isStrategicCommodity(commodity: string): boolean {
  return [
    'semiconductors', 'crude oil', 'natural gas', 'lng', 'lithium', 'cobalt', 'nickel',
    'copper', 'rare earths', 'wheat', 'corn', 'rice', 'fertilizer', 'uranium', 'steel',
    'aluminum', 'batteries', 'solar panels', 'pharmaceuticals',
  ].includes(norm(commodity))
}

function isEnergyCommodity(commodity: string): boolean {
  return ['crude oil', 'natural gas', 'lng', 'uranium'].includes(norm(commodity))
}

function isSemiconductorOrMineral(commodity: string): boolean {
  return ['semiconductors', 'lithium', 'cobalt', 'nickel', 'copper', 'rare earths'].includes(
    norm(commodity),
  )
}

function evaluateRules(input: MitigationInputs): RuleCandidate[] {
  const out: RuleCandidate[] = []
  const share = shareValue(input)
  const hhi = hhiValue(input)
  const commodity = input.commodity

  if (share > 0.6) {
    out.push({
      id: 'supplier-diversification',
      title: `Diversify ${titleCase(commodity)} Suppliers`,
      description: 'Reduce single-source dependency by qualifying additional suppliers across regions.',
      category: 'Supplier Diversification',
      reason: `${input.source} accounts for ${sharePercent(input).toFixed(0)}% of analyzed ${commodity} imports.`,
      expected_impact: 'Reduce supplier concentration and improve resilience against follow-on disruptions.',
      priority: 'High',
      implementation_difficulty: 'Medium',
      confidence: 'High',
      supporting_metrics: { ...baseMetrics(input), supplier_share: sharePercent(input) },
    })
  }

  if (hhi > 0.45) {
    out.push({
      id: 'alternative-sourcing',
      title: `Activate Alternative ${titleCase(commodity)} Sources`,
      description: 'Shift volume to pre-qualified alternate suppliers to lower concentration risk.',
      category: 'Alternative Sourcing',
      reason: `Import concentration HHI of ${hhi.toFixed(3)} exceeds the high-risk threshold for ${commodity}.`,
      expected_impact: 'Can restore 20–50% of disrupted volume within one procurement cycle when alternates exist.',
      priority: 'High',
      implementation_difficulty: 'High',
      confidence: 'High',
      supporting_metrics: { ...baseMetrics(input), hhi },
    })
  }

  if (input.eventRisk > 50) {
    out.push({
      id: 'supplier-monitoring',
      title: `Increase Monitoring of ${input.source} Supply Chain`,
      description: 'Expand supplier and transit monitoring to detect escalation early.',
      category: 'Supplier Monitoring',
      reason: `Event-risk score of ${input.eventRisk.toFixed(0)} for ${input.source} exceeds the monitoring threshold during this scenario.`,
      expected_impact: 'Improves early detection of escalation paths and shortens response lead time.',
      priority: 'Medium',
      implementation_difficulty: 'Low',
      confidence: 'High',
      supporting_metrics: { ...baseMetrics(input), event_risk: input.eventRisk },
    })
  }

  if (input.macroExposure > 40) {
    out.push({
      id: 'country-risk-monitoring',
      title: `Monitor Country Risk for ${input.source}`,
      description: 'Track macroeconomic and structural indicators affecting supplier-country stability.',
      category: 'Country Risk Monitoring',
      reason: `Macro exposure score of ${input.macroExposure.toFixed(0)} for ${input.source} indicates elevated structural country risk.`,
      expected_impact: 'Supports proactive contingency planning before macro stress translates into supply delays.',
      priority: 'Medium',
      implementation_difficulty: 'Low',
      confidence: 'High',
      supporting_metrics: { ...baseMetrics(input), macro_exposure: input.macroExposure },
    })
  }

  if (input.shockType === 'export_collapse' || input.shockType === 'supply_cut') {
    const priority: RecommendationPriority = input.shockType === 'export_collapse' ? 'High' : 'Medium'
    const metrics = { ...baseMetrics(input) }
    if (input.inventoryBufferDays != null) metrics.inventory_buffer_days = input.inventoryBufferDays
    out.push({
      id: 'inventory-increase',
      title: `Increase ${titleCase(commodity)} Inventory Buffer`,
      description: 'Build safety stock above routine operating inventory to absorb supply interruption.',
      category: 'Inventory Increase',
      reason:
        input.inventoryBufferDays != null && input.inventoryBufferDays < 21
          ? `Scenario assumes only ${input.inventoryBufferDays} days of inventory buffer during a ${shockLabel(input.shockType)} shock with ${input.dropPercent.toFixed(0)}% supply reduction.`
          : `A ${input.dropPercent.toFixed(0)}% ${shockLabel(input.shockType)} on ${commodity} warrants additional buffer stock while supply normalizes.`,
      expected_impact: 'Extends operational runway by 2–6 weeks and dampens immediate production disruptions.',
      priority,
      implementation_difficulty: 'Medium',
      confidence: input.shockType === 'export_collapse' ? 'High' : 'Medium',
      supporting_metrics: metrics,
    })
  }

  if (input.shockType === 'route_disruption') {
    out.push({
      id: 'logistics-rerouting',
      title: `Reroute ${titleCase(commodity)} Logistics Corridors`,
      description: 'Activate alternate shipping lanes and inland transport to bypass disrupted routes.',
      category: 'Logistics Rerouting',
      reason: `Route disruption on ${input.source} blocks established ${commodity} corridors; alternate lanes should be evaluated immediately.`,
      expected_impact: 'Maintains delivery continuity on secondary corridors and reduces dwell-time risk.',
      priority: 'High',
      implementation_difficulty: 'Medium',
      confidence: 'High',
      supporting_metrics: baseMetrics(input),
    })
  }

  if (
    isStrategicCommodity(commodity) &&
    (input.dropPercent >= 30 || input.macroExposure >= 55 || input.shockType === 'price_spike')
  ) {
    out.push({
      id: 'strategic-stockpiling',
      title: `Build Strategic ${titleCase(commodity)} Reserve`,
      description: 'Establish a dedicated strategic reserve beyond routine operating inventory.',
      category: 'Strategic Stockpiling',
      reason: `A ${input.dropPercent.toFixed(0)}% shock on strategic ${commodity} with macro exposure ${input.macroExposure.toFixed(0)} supports dedicated stockpiling.`,
      expected_impact: 'Buffers against prolonged supply tightness and reduces spot-market exposure during recovery.',
      priority: 'Medium',
      implementation_difficulty: 'High',
      confidence: 'Medium',
      supporting_metrics: baseMetrics(input),
    })
  }

  if (input.shockType === 'price_spike' || (isEnergyCommodity(commodity) && input.macroExposure >= 45)) {
    out.push({
      id: 'contract-hedging',
      title: `Hedge ${titleCase(commodity)} Price Exposure`,
      description: 'Use forward contracts or indexed pricing to limit spot-market volatility during disruption.',
      category: 'Contract Hedging',
      reason: `Price-spike or macro pressure (${input.macroExposure.toFixed(0)}) on ${commodity} increases cost volatility under the simulated shock.`,
      expected_impact: 'Stabilizes procurement cost and protects margin during price volatility.',
      priority: 'Medium',
      implementation_difficulty: 'Medium',
      confidence: 'Medium',
      supporting_metrics: baseMetrics(input),
    })
  }

  if (input.dropPercent >= 35 && input.fragilityScore >= 10) {
    out.push({
      id: 'demand-reduction',
      title: 'Temporarily Reduce Non-Critical Demand',
      description: 'Prioritize essential consumption and defer discretionary demand until supply stabilizes.',
      category: 'Demand Reduction',
      reason: `A ${input.dropPercent.toFixed(0)}% shock with fragility impact ${input.fragilityScore.toFixed(1)} indicates supply cannot fully meet baseline demand.`,
      expected_impact: 'Preserves limited supply for critical operations and reduces systemic amplification.',
      priority: 'Medium',
      implementation_difficulty: 'Low',
      confidence: 'Medium',
      supporting_metrics: baseMetrics(input),
    })
  }

  if (share > 0.5 && hhi > 0.3 && isSemiconductorOrMineral(commodity)) {
    out.push({
      id: 'dual-sourcing',
      title: `Establish Dual Sourcing for ${titleCase(commodity)}`,
      description: 'Split procurement across two qualified suppliers to limit single-point failure.',
      category: 'Dual Sourcing',
      reason: `Supplier share ${sharePercent(input).toFixed(0)}% and HHI ${hhi.toFixed(3)} on ${commodity} indicate dual sourcing is required for resilience.`,
      expected_impact: 'Provides operational continuity if one supplier is disrupted.',
      priority: 'High',
      implementation_difficulty: 'High',
      confidence: 'High',
      supporting_metrics: { ...baseMetrics(input), supplier_share: sharePercent(input), hhi },
    })
  }

  if (hhi >= 0.15 && hhi <= 0.45 && share > 0.35 && share <= 0.6) {
    out.push({
      id: 'supplier-diversification-moderate',
      title: `Broaden ${titleCase(commodity)} Supplier Base`,
      description: 'Qualify additional suppliers to reduce moderate concentration risk.',
      category: 'Supplier Diversification',
      reason: `Moderate concentration (HHI ${hhi.toFixed(3)}, share ${sharePercent(input).toFixed(0)}%) on ${commodity} from ${input.source} warrants diversification.`,
      expected_impact: 'Lowers concentration risk before it reaches critical thresholds.',
      priority: 'Medium',
      implementation_difficulty: 'Medium',
      confidence: 'Medium',
      supporting_metrics: { ...baseMetrics(input), hhi, supplier_share: sharePercent(input) },
    })
  }

  return out.map((item) => ({
    ...item,
    priority: applyDropBoost(item.priority, input.dropPercent),
  }))
}

function countPriorities(recs: MitigationRecommendation[]): PriorityDistribution {
  const dist: PriorityDistribution = { critical: 0, high: 0, medium: 0, low: 0 }
  for (const r of recs) {
    switch (r.priority) {
      case 'Critical':
        dist.critical++
        break
      case 'High':
        dist.high++
        break
      case 'Medium':
        dist.medium++
        break
      default:
        dist.low++
    }
  }
  return dist
}

function buildExecutiveSummary(input: MitigationInputs, recs: MitigationRecommendation[]): string {
  if (recs.length === 0) {
    return `The simulated ${input.dropPercent.toFixed(0)}% ${shockLabel(input.shockType)} disruption on ${input.commodity} did not trigger high-priority mitigation rules at current concentration and risk thresholds.`
  }

  let lead = `The simulated disruption indicates elevated dependency on ${input.source} for ${input.commodity} imports`
  if (shareValue(input) > 0) lead += ` (${sharePercent(input).toFixed(0)}% supplier share)`
  if (hhiValue(input) > 0) lead += ` with HHI ${hhiValue(input).toFixed(3)}`
  lead += '.'

  const top = recs[0]
  const second = recs[1]
  const tail = second
    ? `${top.title} and ${second.title} are expected to significantly improve resilience.`
    : `${top.title} is expected to significantly improve resilience.`

  return `${lead} ${tail}`
}

function buildEstimatedBusinessImpact(recs: MitigationRecommendation[]): string {
  if (recs.length === 0) {
    return 'Limited immediate mitigation need; continue monitoring scenario signals.'
  }
  const dist = countPriorities(recs)
  if (dist.critical > 0) {
    return `${dist.critical} critical and ${dist.high} high-priority actions could reduce estimated fragility exposure by 20–40% if implemented within 30 days.`
  }
  if (dist.high > 0) {
    return `${dist.high} high-priority actions could reduce supply disruption impact by 15–30% over the next procurement cycle.`
  }
  return 'Targeted monitoring and buffer actions could improve response readiness with moderate operational effort.'
}

function buildSupportingEvidence(input: MitigationInputs): string[] {
  const evidence = [
    `Shock: ${input.dropPercent.toFixed(0)}% ${shockLabel(input.shockType)} on ${input.source} at depth ${input.depth}`,
    `Event-risk score ${input.eventRisk.toFixed(0)}; macro exposure ${input.macroExposure.toFixed(0)}`,
  ]
  if (input.hhi != null) {
    evidence.push(`Trade concentration HHI ${input.hhi.toFixed(3)}${input.tradeConcentration ? ` (${input.tradeConcentration})` : ''}`)
  }
  if (input.supplierShare != null) {
    evidence.push(`Top supplier share ${sharePercent(input).toFixed(0)}%`)
  }
  if (input.fragilityScore > 0) {
    evidence.push(`Modeled fragility impact ${input.fragilityScore.toFixed(1)}`)
  }
  if (input.affectedPaths > 0) {
    evidence.push(`${input.affectedPaths} affected dependency paths in propagation graph`)
  }
  return evidence
}

function tailorRecommendationsWithClientOverlay(
  recommendations: MitigationRecommendation[],
  overlay: ClientExposureOverlay | null,
  input: MitigationInputs,
): MitigationRecommendation[] {
  if (!overlay || overlay.matchedCount === 0 || !overlay.topImporter) {
    return recommendations
  }
  const sharePct = (overlay.topShare * 100).toFixed(0)
  const commodity = titleCase(input.commodity)

  return recommendations.map((rec) => {
    const tailored = { ...rec }
    switch (rec.category) {
      case 'Supplier Diversification':
        tailored.title = `Diversify ${input.source} ${commodity} Sourcing for ${overlay.topImporter} Operations`
        tailored.reason = `Client overlay shows ${sharePct}% supplier dependence on ${input.source} for ${commodity} imports to ${overlay.topImporter}.`
        break
      case 'Alternative Sourcing':
        tailored.title = `Activate Alternative ${commodity} Sources for ${overlay.topImporter}`
        tailored.reason = `${overlay.topImporter} relies on ${input.source} for ${sharePct}% of ${commodity} supply; alternate suppliers should be qualified.`
        break
      case 'Dual Sourcing':
        tailored.title = `Establish Dual Sourcing for ${overlay.topImporter} ${commodity}`
        tailored.reason = `Client concentration (share ${sharePct}%, HHI ${(overlay.highestHHI ?? 0).toFixed(3)}) requires dual sourcing for ${overlay.topImporter}.`
        break
      case 'Inventory Increase':
        tailored.title = `Increase ${commodity} Inventory for ${overlay.topImporter}`
        tailored.reason = `Estimated ${formatCompactUSD(overlay.totalEstimatedExposedTrade)} exposed trade under the ${input.dropPercent.toFixed(0)}% shock warrants additional buffer for ${overlay.topImporter}.`
        break
      case 'Supplier Monitoring':
        tailored.reason = `Monitor ${input.source} supply chain signals affecting ${overlay.topImporter}'s ${commodity} imports (${sharePct}% dependence).`
        break
      default:
        break
    }
    return tailored
  })
}

function formatCompactUSD(value: number): string {
  const abs = Math.abs(value)
  if (abs >= 1e9) return `$${(value / 1e9).toFixed(1)}B`
  if (abs >= 1e6) return `$${(value / 1e6).toFixed(1)}M`
  if (abs >= 1e3) return `$${(value / 1e3).toFixed(1)}K`
  return `$${value.toFixed(0)}`
}

export function generateExecutiveActionPlan(
  input: MitigationInputs,
  overlay?: ClientExposureOverlay | null,
): ExecutiveActionPlan {
  const candidates = evaluateRules(input)
  let recommendations = candidates
    .map((c) => ({ ...c, id: c.id ?? c.category }))
    .sort((a, b) => {
      const diff = PRIORITY_RANK[b.priority] - PRIORITY_RANK[a.priority]
      return diff !== 0 ? diff : a.title.localeCompare(b.title)
    })

  recommendations = tailorRecommendationsWithClientOverlay(recommendations, overlay ?? null, input)

  return {
    summary: buildExecutiveSummary(input, recommendations),
    recommendations,
    priority_distribution: countPriorities(recommendations),
    estimated_business_impact: buildEstimatedBusinessImpact(recommendations),
    supporting_evidence: buildSupportingEvidence(input),
  }
}

export function executiveActionPlanForScenario(
  result: ShockResponse,
  clientData?: CustomDataAnalysisResponse | null,
  report?: ScenarioReportResponse | null,
): ExecutiveActionPlan {
  const input = buildMitigationInputs(result, clientData, report)
  const overlay = computeClientExposureOverlay(
    clientData,
    result.scenario.source,
    result.scenario.commodity,
    result.scenario.shock_percent,
  )

  if (report?.executive_action_plan?.recommendations?.length) {
    const plan = { ...report.executive_action_plan }
    plan.recommendations = tailorRecommendationsWithClientOverlay(
      plan.recommendations,
      overlay,
      input,
    )
    if (overlay?.assessment) {
      plan.summary = `${overlay.assessment} ${plan.summary}`
    }
    return plan
  }
  return generateExecutiveActionPlan(input, overlay)
}

// Backward-compatible helpers
export function mitigationRecommendationsForScenario(
  result: ShockResponse,
  clientData?: CustomDataAnalysisResponse | null,
  report?: ScenarioReportResponse | null,
): MitigationRecommendation[] {
  return executiveActionPlanForScenario(result, clientData, report).recommendations
}
