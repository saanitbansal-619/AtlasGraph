import { describe, expect, it } from 'vitest'
import {
  buildClientExposureAssessment,
  computeClientExposureOverlay,
} from '../lib/clientExposure'
import type { CustomDataAnalysisResponse } from '../types/api'

function sampleAnalysis(): CustomDataAnalysisResponse {
  return {
    dataset_summary: {
      rows_processed: 2,
      valid_rows: 2,
      invalid_rows: 0,
      importers: 1,
      commodities: 1,
      suppliers: 1,
      total_value_usd: 20_000_000_000,
    },
    concentration_results: [
      {
        importer: 'United States',
        commodity: 'semiconductors',
        total_value_usd: 32_000_000_000,
        supplier_count: 3,
        top_supplier: 'Taiwan',
        top_supplier_share: 0.63,
        hhi: 0.501,
        concentration_risk: 'High',
      },
    ],
    validation_errors: [],
    normalized_rows: [
      {
        importer: 'United States',
        commodity: 'semiconductors',
        supplier: 'Taiwan',
        value_usd: 20_000_000_000,
      },
    ],
  }
}

describe('client exposure overlay', () => {
  it('matches supplier and commodity with alias normalization', () => {
    const lngAnalysis: CustomDataAnalysisResponse = {
      ...sampleAnalysis(),
      normalized_rows: [
        { importer: 'Japan', commodity: 'LNG', supplier: 'Qatar', value_usd: 10_000_000_000 },
      ],
      concentration_results: [
        {
          importer: 'Japan',
          commodity: 'LNG',
          total_value_usd: 10_000_000_000,
          supplier_count: 1,
          top_supplier: 'Qatar',
          top_supplier_share: 1,
          hhi: 1,
          concentration_risk: 'High',
        },
      ],
    }
    const overlay = computeClientExposureOverlay(lngAnalysis, 'Qatar', 'Natural Gas', 20)
    expect(overlay?.matchedCount).toBe(1)
    expect(overlay?.exposures[0].estimated_exposed_trade).toBe(2_000_000_000)
  })

  it('computes estimated exposed and remaining trade', () => {
    const overlay = computeClientExposureOverlay(sampleAnalysis(), 'Taiwan', 'semiconductors', 30)
    expect(overlay?.exposures[0].estimated_exposed_trade).toBe(6_000_000_000)
    expect(overlay?.exposures[0].estimated_remaining_trade).toBe(14_000_000_000)
    expect(overlay?.totalEstimatedExposedTrade).toBe(6_000_000_000)
  })

  it('sorts rows by estimated exposed trade descending', () => {
    const analysis: CustomDataAnalysisResponse = {
      ...sampleAnalysis(),
      normalized_rows: [
        { importer: 'Germany', commodity: 'semiconductors', supplier: 'Taiwan', value_usd: 5_000_000_000 },
        { importer: 'United States', commodity: 'semiconductors', supplier: 'Taiwan', value_usd: 20_000_000_000 },
      ],
      concentration_results: [
        {
          importer: 'United States',
          commodity: 'semiconductors',
          total_value_usd: 32_000_000_000,
          supplier_count: 3,
          top_supplier: 'Taiwan',
          top_supplier_share: 0.63,
          hhi: 0.501,
          concentration_risk: 'High',
        },
        {
          importer: 'Germany',
          commodity: 'semiconductors',
          total_value_usd: 8_000_000_000,
          supplier_count: 2,
          top_supplier: 'Taiwan',
          top_supplier_share: 0.625,
          hhi: 0.52,
          concentration_risk: 'High',
        },
      ],
    }
    const overlay = computeClientExposureOverlay(analysis, 'Taiwan', 'semiconductors', 30)
    expect(overlay?.exposures[0].importer).toBe('United States')
  })

  it('builds client exposure assessment narrative', () => {
    const overlay = computeClientExposureOverlay(sampleAnalysis(), 'Taiwan', 'semiconductors', 30)
    const text = buildClientExposureAssessment(overlay!)
    expect(text).toContain('Taiwan')
    expect(text).toContain('United States')
    expect(text).toContain('$6.0B')
  })
})
