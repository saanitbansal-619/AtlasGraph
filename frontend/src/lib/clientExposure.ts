import type {
  CustomConcentrationResult,
  CustomDataAnalysisResponse,
  CustomNormalizedRow,
} from '../types/api'
import { normalizeCommodityLabel } from './commodityNormalize'

export interface ClientExposureRow {
  importer: string
  commodity: string
  shocked_supplier: string
  supplier_value_usd: number
  total_import_value_usd: number
  supplier_share: number
  hhi?: number
  concentration_risk?: string
  estimated_exposed_trade: number
  estimated_remaining_trade: number
}

export interface ClientExposureOverlay {
  exposures: ClientExposureRow[]
  matchedCount: number
  topImporter: string | null
  topShare: number
  topRisk: string | null
  commodity: string
  source: string
  shockDropPercent: number
  totalEstimatedExposedTrade: number
  highestHHI: number | null
  averageConcentrationRisk: string | null
  assessment: string | null
}

function norm(value: string): string {
  return value.trim().toLowerCase()
}

function commoditiesMatch(a: string, b: string): boolean {
  return normalizeCommodityLabel(a) === normalizeCommodityLabel(b)
}

function concentrationLookup(
  results: CustomConcentrationResult[],
): Map<string, CustomConcentrationResult> {
  const map = new Map<string, CustomConcentrationResult>()
  for (const row of results) {
    map.set(`${norm(row.importer)}\0${normalizeCommodityLabel(row.commodity)}`, row)
  }
  return map
}

function riskToScore(risk?: string): number {
  switch (risk) {
    case 'High':
      return 3
    case 'Medium':
      return 2
    case 'Low':
      return 1
    default:
      return 0
  }
}

function averageRiskLabel(scores: number[]): string | null {
  if (scores.length === 0) return null
  const avg = scores.reduce((sum, v) => sum + v, 0) / scores.length
  if (avg >= 2.5) return 'High'
  if (avg >= 1.5) return 'Medium'
  return 'Low'
}

export function formatCompactUSD(value: number): string {
  const abs = Math.abs(value)
  if (abs >= 1e12) return `$${(value / 1e12).toFixed(1)}T`
  if (abs >= 1e9) return `$${(value / 1e9).toFixed(1)}B`
  if (abs >= 1e6) return `$${(value / 1e6).toFixed(1)}M`
  if (abs >= 1e3) return `$${(value / 1e3).toFixed(1)}K`
  return `$${value.toFixed(0)}`
}

export function buildClientExposureAssessment(overlay: ClientExposureOverlay): string | null {
  if (overlay.matchedCount === 0 || !overlay.topImporter) return null
  const topRow = overlay.exposures[0]
  const sharePct = ((topRow?.supplier_share ?? overlay.topShare) * 100).toFixed(0)
  const exposed = formatCompactUSD(topRow?.estimated_exposed_trade ?? 0)
  return `The uploaded client portfolio indicates direct dependence on ${overlay.source} for ${overlay.commodity} imports. ${overlay.topImporter} has approximately ${sharePct}% supplier dependence on ${overlay.source}, resulting in an estimated ${exposed} of directly exposed trade under the simulated disruption.`
}

/**
 * Match client supplier rows to a shock source + commodity and aggregate
 * exposure by importer + commodity.
 */
export function computeClientExposureOverlay(
  analysis: CustomDataAnalysisResponse | null | undefined,
  source: string,
  commodity: string,
  dropPercent = 0,
): ClientExposureOverlay | null {
  if (!analysis) return null

  const shockSource = norm(source)
  const shockCommodity = normalizeCommodityLabel(commodity)
  if (!shockSource || !shockCommodity) {
    return {
      exposures: [],
      matchedCount: 0,
      topImporter: null,
      topShare: 0,
      topRisk: null,
      commodity: commodity.trim(),
      source: source.trim(),
      shockDropPercent: dropPercent,
      totalEstimatedExposedTrade: 0,
      highestHHI: null,
      averageConcentrationRisk: null,
      assessment: null,
    }
  }

  const rows: CustomNormalizedRow[] = analysis.normalized_rows ?? []
  const concentrations = concentrationLookup(analysis.concentration_results ?? [])
  const dropFactor = Math.max(0, dropPercent) / 100

  type Acc = {
    importer: string
    commodity: string
    shocked_supplier: string
    supplier_value_usd: number
    total_import_value_usd: number
  }
  const groups = new Map<string, Acc>()

  for (const row of rows) {
    if (norm(row.supplier) !== shockSource) continue
    if (!commoditiesMatch(row.commodity, commodity)) continue

    const key = `${norm(row.importer)}\0${normalizeCommodityLabel(row.commodity)}`
    const existing = groups.get(key)
    if (existing) {
      existing.supplier_value_usd += row.value_usd
      continue
    }
    groups.set(key, {
      importer: row.importer,
      commodity: row.commodity,
      shocked_supplier: row.supplier,
      supplier_value_usd: row.value_usd,
      total_import_value_usd: 0,
    })
  }

  const exposures: ClientExposureRow[] = []
  for (const [key, group] of groups) {
    const concentration = concentrations.get(key)
    const total =
      concentration?.total_value_usd && concentration.total_value_usd > 0
        ? concentration.total_value_usd
        : group.supplier_value_usd
    const share = total > 0 ? group.supplier_value_usd / total : 0
    const exposed = group.supplier_value_usd * dropFactor
    const remaining = Math.max(0, group.supplier_value_usd - exposed)
    exposures.push({
      importer: group.importer,
      commodity: group.commodity,
      shocked_supplier: group.shocked_supplier,
      supplier_value_usd: group.supplier_value_usd,
      total_import_value_usd: total,
      supplier_share: share,
      hhi: concentration?.hhi,
      concentration_risk: concentration?.concentration_risk,
      estimated_exposed_trade: exposed,
      estimated_remaining_trade: remaining,
    })
  }

  exposures.sort(
    (a, b) =>
      b.estimated_exposed_trade - a.estimated_exposed_trade ||
      b.supplier_share - a.supplier_share ||
      a.importer.localeCompare(b.importer),
  )

  const top = exposures[0]
  const riskScores = exposures.map((row) => riskToScore(row.concentration_risk)).filter((v) => v > 0)
  const highestHHI =
    exposures.reduce<number | null>((max, row) => {
      if (row.hhi == null) return max
      return max == null || row.hhi > max ? row.hhi : max
    }, null)
  const totalEstimatedExposedTrade = exposures.reduce(
    (sum, row) => sum + row.estimated_exposed_trade,
    0,
  )
  const highestShare = exposures.reduce((max, row) => Math.max(max, row.supplier_share), 0)

  const overlay: ClientExposureOverlay = {
    exposures,
    matchedCount: exposures.length,
    topImporter: top?.importer ?? null,
    topShare: top?.supplier_share ?? highestShare,
    topRisk: top?.concentration_risk ?? null,
    commodity: top?.commodity || commodity.trim(),
    source: source.trim(),
    shockDropPercent: dropPercent,
    totalEstimatedExposedTrade,
    highestHHI,
    averageConcentrationRisk: averageRiskLabel(riskScores),
    assessment: null,
  }
  overlay.assessment = buildClientExposureAssessment(overlay)
  return overlay
}

export function clientOverlayBriefSentence(overlay: ClientExposureOverlay | null): string | null {
  return overlay?.assessment ?? null
}
