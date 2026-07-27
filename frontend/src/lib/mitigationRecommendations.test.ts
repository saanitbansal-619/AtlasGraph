import { describe, expect, it } from 'vitest'
import {
  buildMitigationInputs,
  generateExecutiveActionPlan,
  type MitigationInputs,
} from '../lib/mitigationRecommendations'
import type { ShockResponse } from '../types/api'

function sampleShockResult(): ShockResponse {
  return {
    scenario: {
      source: 'Taiwan',
      commodity: 'semiconductors',
      shock_type: 'export_collapse',
      shock_percent: 45,
      depth: 3,
      initial_impact: 0.45,
    },
    shock_profile: {
      type: 'export_collapse',
      name: 'Export Collapse',
      description: '',
      allowed_relationships: [],
      attenuation: 0.7,
      recommended_depth: 3,
      cross_commodity: false,
    },
    propagation_rules_applied: {
      shock_type: 'export_collapse',
      allowed_relationships: [],
      cross_commodity_enabled: false,
      blocked_commodities: [],
    },
    direct_exposure: [{ entity: 'United States', type: 'country', distance: 1, impact: 0.4, base_fragility: 20, shock_fragility: 32, delta: 12, operational_multiplier: 1, resilience_note: '' }],
    second_order_exposure: [],
    affected_paths: [],
    changed_fragility_scores: [],
    highest_risk_entities: {
      countries: [{ entity: 'United States', type: 'country', distance: 1, impact: 0.4, base_fragility: 20, shock_fragility: 32, delta: 12, operational_multiplier: 1, resilience_note: '' }],
      commodities: [],
      sectors: [],
    },
    graph_impact_summary: {
      nodes_in_graph: 100,
      affected_nodes: 12,
      affected_countries: 3,
      affected_commodities: 2,
      affected_sectors: 1,
      affected_paths: 8,
      avg_fragility_delta: 8,
      largest_single_impact_delta: 12,
    },
    operational_assumptions: {
      duration_days: 30,
      recovery_speed: 'medium',
      substitute_availability: 'medium',
      inventory_buffer_days: 14,
      duration_factor: 1,
      recovery_factor: 1,
      substitute_factor: 1,
      inventory_factor: 1,
      explanation: 'Operational assumptions applied.',
    },
  }
}

describe('mitigation recommendation engine', () => {
  it('triggers supplier diversification when supplier share exceeds 60%', () => {
    const input: MitigationInputs = {
      source: 'Taiwan',
      commodity: 'semiconductors',
      shockType: 'export_collapse',
      dropPercent: 30,
      depth: 3,
      hhi: 0.5,
      supplierShare: 0.63,
      tradeConcentration: 'High',
      eventRisk: 55,
      macroExposure: 45,
      fragilityScore: 12,
      inventoryBufferDays: 14,
      affectedPaths: 8,
      affectedNodes: 12,
    }
    const plan = generateExecutiveActionPlan(input)
    expect(plan.recommendations.some((r) => r.category === 'Supplier Diversification')).toBe(true)
    const diversify = plan.recommendations.find((r) => r.category === 'Supplier Diversification')
    expect(diversify?.priority).toBe('High')
    expect(diversify?.confidence).toBe('High')
  })

  it('boosts priority when drop exceeds 40%', () => {
    const base: MitigationInputs = {
      source: 'Taiwan',
      commodity: 'semiconductors',
      shockType: 'export_collapse',
      dropPercent: 30,
      depth: 3,
      hhi: 0.1,
      supplierShare: 0.2,
      tradeConcentration: '',
      eventRisk: 55,
      macroExposure: 30,
      fragilityScore: 5,
      inventoryBufferDays: null,
      affectedPaths: 0,
      affectedNodes: 0,
    }
    const lowDrop = generateExecutiveActionPlan(base)
    const highDrop = generateExecutiveActionPlan({ ...base, dropPercent: 45 })
    const low = lowDrop.recommendations.find((r) => r.category === 'Supplier Monitoring')
    const high = highDrop.recommendations.find((r) => r.category === 'Supplier Monitoring')
    expect(low?.priority).toBe('Medium')
    expect(high?.priority).toBe('High')
  })

  it('builds inputs from shock result and generates executive summary', () => {
    const result = sampleShockResult()
    const input = buildMitigationInputs(result, null, {
      title: 'Report',
      executive_summary: '',
      key_findings: [],
      direct_exposure: [],
      second_order_exposure: [],
      total_direct_exposure_count: 0,
      total_second_order_exposure_count: 0,
      returned_direct_exposure_count: 0,
      returned_second_order_exposure_count: 0,
      most_exposed_countries: [],
      most_exposed_commodities: [],
      most_exposed_sectors: [],
      trade_evidence: [
        {
          importer: 'United States',
          commodity: 'semiconductors',
          hhi: 0.501,
          concentration_risk: 'High',
          top_supplier_name: 'Taiwan',
          top_supplier_code: 'TWN',
          top_supplier_share: 0.63,
          summary: 'High concentration',
          data_provenance: 'UN Comtrade',
        },
      ],
      event_risk_context: [
        {
          entity: 'Taiwan',
          available: true,
          score: 71,
          risk_level: 'High',
          summary: 'Elevated event risk',
          data_provenance: 'GDELT',
        },
      ],
      macro_context: [],
      commodity_fragility_context: [],
      model_assumptions: [],
      data_sources: [],
      limitations: [],
    })
    expect(input.hhi).toBe(0.501)
    expect(input.supplierShare).toBe(0.63)

    const plan = generateExecutiveActionPlan(input)
    expect(plan.summary).toContain('Taiwan')
    expect(plan.priority_distribution.critical + plan.priority_distribution.high).toBeGreaterThan(0)
    expect(plan.supporting_evidence.length).toBeGreaterThan(0)
  })
})
