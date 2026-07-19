// TypeScript shapes for the AtlasGraph Go API. These mirror the JSON the server
// emits (internal/cli/server.go and internal/cli/output.go) closely enough to
// be useful while keeping the surface readable.

export interface HealthResponse {
  status: string
  service: string
  version: string
}

export interface DBHealthResponse {
  enabled: boolean
  status: 'ok' | 'error' | 'disabled' | string
  error?: string
}

export interface DBSummaryResponse {
  trade_flows: number
  event_risk_signals: number
  macro_scores: number
  commodity_prices: number
  dependency_edges: number
  scenario_runs: number
  data_quality_checks: number
}

export interface CustomDataValidationError {
  row: number
  field: string
  message: string
}

export interface CustomDataSummary {
  rows_processed: number
  valid_rows: number
  invalid_rows: number
  importers: number
  commodities: number
  suppliers: number
  total_value_usd: number
}

export interface CustomConcentrationResult {
  importer: string
  commodity: string
  total_value_usd: number
  supplier_count: number
  top_supplier: string
  top_supplier_share: number
  hhi: number
  concentration_risk: 'Low' | 'Medium' | 'High'
}

export interface CustomDataAnalysisResponse {
  dataset_summary: CustomDataSummary
  concentration_results: CustomConcentrationResult[]
  validation_errors: CustomDataValidationError[]
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
  fusion_enabled?: boolean
  base_entities?: number
  base_dependencies?: number
  fused_entities?: number
  fused_dependencies?: number
  real_trade_edges?: number
  real_trade_edges_used?: boolean
  real_event_risk_used?: boolean
  real_price_stress_used?: boolean
  data_sources?: string[]
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
  duration_days: number
  recovery_speed: string
  substitute_availability: string
  inventory_buffer_days: number
}

export interface ExposureItem {
  entity: string
  type: string
  distance: number
  impact: number
  base_fragility: number
  shock_fragility: number
  delta: number
  operational_multiplier: number
  resilience_note: string
}

export interface OperationalAssumptions {
  duration_days: number
  recovery_speed: string
  substitute_availability: string
  inventory_buffer_days: number
  duration_factor: number
  recovery_factor: number
  substitute_factor: number
  inventory_factor: number
  explanation: string
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
  data_fusion?: DataFusionInfo
  operational_assumptions?: OperationalAssumptions
}

export interface DataFusionInfo {
  fusion_enabled: boolean
  real_trade_edges_used: boolean
  real_event_risk_used: boolean
  real_price_stress_used: boolean
  event_risk_multiplier_applied?: boolean
  data_sources: string[]
  propagation_note?: string
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
  fusion_enabled?: boolean
  real_trade_edges_used?: boolean
  real_event_risk_used?: boolean
  real_price_stress_used?: boolean
  data_sources?: string[]
  trade_concentration_source?: string
  trade_concentration_note?: string
}

export interface FragilityComponent {
  key: string
  name: string
  score: number
  weight: number
  contribution: number
  available: boolean
  source?: string
  note?: string
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
  latest_event_date?: string
  rows_processed?: number
  countries_covered?: number
  event_type_breakdown?: Record<string, number>
  scoring_note?: string
  scores: EventRiskScore[]
}

export interface TradeSummaryResponse {
  source: string
  real_trade_data: boolean
  records: number
  years: number[]
  countries: number
  commodities: number
  total_value_usd: number
  top_exporters: Array<{ code: string; name: string; value_usd: number }>
  top_importers: Array<{ code: string; name: string; value_usd: number }>
  top_commodities: Array<{ code: string; name: string; value_usd: number }>
  available_commodities?: string[]
  available_importers?: string[]
}

export interface TradeImportOption {
  name: string
  code: string
  commodities: string[]
}

export interface TradeOptionsResponse {
  source: string
  real_trade_data: boolean
  importers: TradeImportOption[]
}

export interface TradeSupplier {
  exporter_code: string
  exporter_name: string
  value_usd: number
  share: number
  share_pct?: number
  dependency: string
}

export interface TradeDependencyResponse {
  source: string
  real_trade_data: boolean
  importer: string
  importer_code: string
  commodity: string
  total_imports_usd: number
  suppliers: TradeSupplier[]
}

export interface TradeConcentrationResponse {
  source: string
  real_trade_data: boolean
  importer: string
  importer_code: string
  commodity: string
  hhi: number
  concentration_risk: string
  top_supplier: TradeSupplier
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

// POST /api/reports/scenario
export interface ScenarioReportRequest {
  source: string
  commodity: string
  shock_type: string
  drop_percent: number
  depth: number
  duration_days: number
  recovery_speed: string
  substitute_availability: string
  inventory_buffer_days: number
}

export interface ReportExposureItem {
  entity: string
  type: string
  distance: number
  estimated_impact: number
  fragility_delta: number
  base_fragility: number
  shock_fragility: number
  note?: string
  data_provenance: string
  operational_multiplier: number
  resilience_note: string
}

export interface ReportTradeEvidence {
  importer: string
  commodity: string
  hhi: number
  concentration_risk: string
  top_supplier_name: string
  top_supplier_code: string
  top_supplier_share: number
  summary: string
  data_provenance: string
}

export interface ReportContextItem {
  entity: string
  available: boolean
  score?: number
  risk_level?: string
  summary: string
  data_provenance: string
}

export interface ScenarioReportResponse {
  title: string
  executive_summary: string
  key_findings: string[]
  direct_exposure: ReportExposureItem[]
  second_order_exposure: ReportExposureItem[]
  total_direct_exposure_count: number
  total_second_order_exposure_count: number
  returned_direct_exposure_count: number
  returned_second_order_exposure_count: number
  most_exposed_countries: ReportExposureItem[]
  most_exposed_commodities: ReportExposureItem[]
  most_exposed_sectors: ReportExposureItem[]
  trade_evidence: ReportTradeEvidence[]
  event_risk_context: ReportContextItem[]
  macro_context: ReportContextItem[]
  commodity_fragility_context: ReportContextItem[]
  model_assumptions: string[]
  data_sources: string[]
  limitations: string[]
  operational_assumptions?: OperationalAssumptions
}
