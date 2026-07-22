import type {
  CustomConcentrationResult,
  CustomDataAnalysisResponse,
  CustomNormalizedRow,
} from '../types/api'

export interface ClientExposureRow {
  importer: string
  commodity: string
  shocked_supplier: string
  supplier_value_usd: number
  total_import_value_usd: number
  supplier_share: number
  hhi?: number
  concentration_risk?: string
}

export interface ClientExposureOverlay {
  exposures: ClientExposureRow[]
  matchedCount: number
  topImporter: string | null
  topShare: number
  topRisk: string | null
  commodity: string
}

function norm(value: string): string {
  return value.trim().toLowerCase()
}

function concentrationLookup(
  results: CustomConcentrationResult[],
): Map<string, CustomConcentrationResult> {
  const map = new Map<string, CustomConcentrationResult>()
  for (const row of results) {
    map.set(`${norm(row.importer)}\0${norm(row.commodity)}`, row)
  }
  return map
}

/**
 * Match client supplier rows to a shock source + commodity and aggregate
 * exposure by importer + commodity.
 */
export function computeClientExposureOverlay(
  analysis: CustomDataAnalysisResponse | null | undefined,
  source: string,
  commodity: string,
): ClientExposureOverlay | null {
  if (!analysis) return null

  const shockSource = norm(source)
  const shockCommodity = norm(commodity)
  if (!shockSource || !shockCommodity) {
    return {
      exposures: [],
      matchedCount: 0,
      topImporter: null,
      topShare: 0,
      topRisk: null,
      commodity: commodity.trim(),
    }
  }

  const rows: CustomNormalizedRow[] = analysis.normalized_rows ?? []
  const concentrations = concentrationLookup(analysis.concentration_results ?? [])

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
    if (norm(row.commodity) !== shockCommodity) continue

    const key = `${norm(row.importer)}\0${norm(row.commodity)}`
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
    exposures.push({
      importer: group.importer,
      commodity: group.commodity,
      shocked_supplier: group.shocked_supplier,
      supplier_value_usd: group.supplier_value_usd,
      total_import_value_usd: total,
      supplier_share: share,
      hhi: concentration?.hhi,
      concentration_risk: concentration?.concentration_risk,
    })
  }

  exposures.sort(
    (a, b) =>
      b.supplier_share - a.supplier_share ||
      b.supplier_value_usd - a.supplier_value_usd ||
      a.importer.localeCompare(b.importer),
  )

  const top = exposures[0]
  return {
    exposures,
    matchedCount: exposures.length,
    topImporter: top?.importer ?? null,
    topShare: top?.supplier_share ?? 0,
    topRisk: top?.concentration_risk ?? null,
    commodity: top?.commodity || commodity.trim(),
  }
}

export function clientOverlayBriefSentence(overlay: ClientExposureOverlay | null): string | null {
  if (!overlay || overlay.matchedCount === 0 || !overlay.topImporter) return null
  const sharePct = (overlay.topShare * 100).toFixed(0)
  return `Client overlay data shows direct supplier exposure to the shocked source for ${overlay.commodity}, led by ${overlay.topImporter} with ${sharePct}% supplier dependence.`
}
