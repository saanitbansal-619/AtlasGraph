// TypeScript shapes for the AtlasGraph Go API. These mirror the JSON the server
// emits (internal/cli/server.go and internal/cli/output.go) closely enough to
// be useful while keeping the surface readable.

export interface HealthResponse {
  status: string
  service: string
  version: string
}

export interface GraphNode {
  name: string
  type: string
  degree: number
  in_degree: number
  out_degree: number
}

export interface GraphSummaryResponse {
  nodes: number
  countries: number
  commodities: number
  sectors: number
  routes: number
  companies: number
  dependencies: number
  top_nodes: GraphNode[]
}

export interface Scenario {
  id: string
  name: string
  source: string
  commodity: string
  shock_type: string
  shock_percent: number
  depth: number
  description?: string
}

export interface ScenariosResponse {
  scenarios: Scenario[]
}

export type ShockType =
  | 'export_collapse'
  | 'supply_cut'
  | 'price_spike'
  | 'route_disruption'

export interface ShockRequest {
  source: string
  commodity: string
  drop: number
  depth: number
  shock_type: string
  explain?: boolean
}

export interface ExposureItem {
  entity: string
  type: string
  distance: number
  impact: number
  base_fragility: number
  shock_fragility: number
  delta: number
}

export interface AffectedPath {
  path: string[]
  relationships: string[]
  labeled_path: string
  path_weight: number
  end_impact: number
}

export interface BlockedEdge {
  from: string
  to: string
  relationship_type: string
  commodity?: string
  reason: string
}

export interface ShockScenarioInfo {
  id?: string
  name?: string
  source: string
  commodity: string
  shock_type: string
  shock_percent: number
  depth: number
  description?: string
  initial_impact: number
}

export interface ShockProfile {
  type: string
  name: string
  description: string
  allowed_relationships: string[]
  attenuation: number
  recommended_depth: number
  cross_commodity: boolean
}

export interface PropagationRulesApplied {
  shock_type: string
  allowed_relationships: string[]
  cross_commodity_enabled: boolean
  blocked_commodities: string[]
}

export interface HighestRiskEntities {
  countries: ExposureItem[]
  commodities: ExposureItem[]
  sectors: ExposureItem[]
}

export interface GraphImpactSummary {
  nodes_in_graph: number
  affected_nodes: number
  affected_countries: number
  affected_commodities: number
  affected_sectors: number
  affected_paths: number
  avg_fragility_delta: number
  largest_single_impact_entity?: string
  largest_single_impact_delta: number
}

export interface ShockResponse {
  scenario: ShockScenarioInfo
  shock_profile: ShockProfile
  propagation_rules_applied: PropagationRulesApplied
  direct_exposure: ExposureItem[]
  second_order_exposure: ExposureItem[]
  affected_paths: AffectedPath[]
  changed_fragility_scores: ExposureItem[]
  highest_risk_entities: HighestRiskEntities
  graph_impact_summary: GraphImpactSummary
  blocked_edges?: BlockedEdge[]
}

// JSON error envelope returned by the API on failure.
export interface ApiError {
  error: string
  hint?: string
}
