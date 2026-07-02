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
  // Non-fatal, graph-aware advisories for suboptimal but still-valid combos.
  warnings?: string[]
}

// GET /api/graph/entities — graph nodes grouped by type.
export interface GraphEntitiesResponse {
  countries: string[]
  commodities: string[]
  sectors: string[]
  routes: string[]
  companies: string[]
}

// One shock type as described by GET /api/shock/options.
export interface ShockTypeOption {
  type: string
  name: string
  description: string
  recommended_for: string[]
  requires: string[]
}

// A graph-validated recommended scenario from GET /api/shock/options.
export interface RecommendedScenario {
  label: string
  source: string
  commodity: string
  shock_type: string
  drop: number
  depth: number
}

export interface ShockOptionsResponse {
  sources: string[]
  commodities: string[]
  shock_types: ShockTypeOption[]
  recommended_scenarios: RecommendedScenario[]
}

// GET /api/shock/valid-options — graph-valid source → commodity → shock_type combos.
export interface ValidCommodityOption {
  commodity: string
  shock_types: string[]
  relationships: string[]
}

export interface ValidSourceOption {
  source: string
  type: string
  commodities: ValidCommodityOption[]
}

export interface ShockValidOptionsResponse {
  sources: ValidSourceOption[]
}

export interface FragilitySummaryResponse {
  countries: CountryFragilityScore[]
  commodities: CommodityFragilityScore[]
}

export interface FragilityComponent {
  key: string
  name: string
  score: number
  weight: number
  contribution: number
  available: boolean
}

export interface CountryFragilityScore {
  country_code: string
  country_name: string
  score: number
  risk_level: string
  top_drivers: string[]
  missing_components: string[]
  components: FragilityComponent[]
}

export interface CommodityFragilityScore {
  commodity_code: string
  commodity_name: string
  score: number
  risk_level: string
  top_drivers: string[]
  missing_components: string[]
  components: FragilityComponent[]
}

export interface CommodityStressScore {
  commodity_code: string
  commodity_name: string
  unit: string
  months: number
  latest_date: string
  latest_price_usd: number
  change_3m_pct?: number | null
  change_12m_pct?: number | null
  volatility_pct: number
  commodity_stress_score: number
  risk_level: string
}

export interface CommodityStressResponse {
  data_source: string
  real_price_data: boolean
  scores: CommodityStressScore[]
}

export interface CommodityHistoryPoint {
  month: string
  price: number
}

export interface CommodityHistoryResponse {
  commodity: string
  source: string
  points: CommodityHistoryPoint[]
}

export interface CommodityHistoryIndexResponse {
  source: string
  commodities: string[]
}

export interface EventRiskScore {
  country?: string
  country_code: string
  country_name: string
  events: number
  event_count?: number
  recent_event_count?: number
  avg_tone: number
  average_tone?: number
  event_risk_score: number
  risk_level: string
  top_drivers: string[]
  top_terms: string[]
  top_event_types?: string[]
}

export interface EventRiskResponse {
  source: string
  real_event_data: boolean
  date_from?: string
  date_to?: string
  scores: EventRiskScore[]
}

// JSON error envelope returned by the API on failure.
export interface ApiError {
  error: string
  hint?: string
}

// POST /api/scenarios/compare — one scenario in the request body.
export interface CompareScenarioInput {
  label: string
  source: string
  commodity: string
  shock_type: string
  drop: number
  depth: number
  explain?: boolean
}

export interface CompareEntityImpact {
  entity: string
  type: string
  delta: number
}

export interface ScenarioCompareResult {
  label: string
  source: string
  commodity: string
  shock_type: string
  drop: number
  depth: number
  affected_nodes_count: number
  affected_paths_count: number
  average_fragility_delta: number
  max_fragility_delta: number
  top_affected_entities: CompareEntityImpact[]
  top_affected_countries: CompareEntityImpact[]
  top_affected_sectors: CompareEntityImpact[]
  warnings?: string[]
}

export interface ScenarioCompareSummary {
  worst_overall_scenario: string
  most_countries_affected: string
  most_sectors_affected: string
  highest_average_fragility_delta: string
  highest_max_fragility_delta: string
}

export interface ScenarioCompareResponse {
  summary: ScenarioCompareSummary
  results: ScenarioCompareResult[]
}

export interface CompareScenariosRequest {
  scenarios: CompareScenarioInput[]
}
