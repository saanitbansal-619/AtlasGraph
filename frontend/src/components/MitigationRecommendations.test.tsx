import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { RecommendedActionsPanel } from '../components/MitigationRecommendations'
import type { ShockResponse } from '../types/api'

function sampleResult(): ShockResponse {
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
    direct_exposure: [],
    second_order_exposure: [],
    affected_paths: [],
    changed_fragility_scores: [],
    highest_risk_entities: { countries: [], commodities: [], sectors: [] },
    graph_impact_summary: {
      nodes_in_graph: 100,
      affected_nodes: 10,
      affected_countries: 2,
      affected_commodities: 1,
      affected_sectors: 1,
      affected_paths: 5,
      avg_fragility_delta: 6,
      largest_single_impact_delta: 10,
    },
  }
}

describe('RecommendedActionsPanel', () => {
  it('renders executive summary and recommendation cards', () => {
    render(
      <RecommendedActionsPanel
        result={sampleResult()}
        clientData={null}
        report={{
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
          macro_context: [
            {
              entity: 'Taiwan',
              available: true,
              score: 45,
              risk_level: 'Medium',
              summary: 'Macro exposure',
              data_provenance: 'World Bank Macro',
            },
          ],
          commodity_fragility_context: [],
          model_assumptions: [],
          data_sources: [],
          limitations: [],
        }}
      />,
    )

    expect(screen.getByText('Recommended Actions')).toBeInTheDocument()
    expect(screen.getByText('Executive Action Summary')).toBeInTheDocument()
    expect(screen.getByText('Executive Recommendations')).toBeInTheDocument()
    expect(screen.getByText('Recommendation Priority Distribution')).toBeInTheDocument()
    expect(screen.getByText('Estimated Business Impact')).toBeInTheDocument()
    expect(screen.getByText('Supporting Evidence')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: /Activate Alternative Semiconductors Sources/i })).toBeInTheDocument()
    expect(screen.getAllByText(/Diversify Semiconductors Suppliers/i).length).toBeGreaterThan(0)
  })
})
