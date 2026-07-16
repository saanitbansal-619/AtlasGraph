package cli

import (
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/graphfusion"
	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/scoring/commodities"
	"github.com/atlasgraph/atlas/internal/scoring/events"
	"github.com/atlasgraph/atlas/internal/scoring/fragility"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
	"github.com/atlasgraph/atlas/internal/simulation"
)

// jsonResult is the structured JSON representation of a shock simulation. It is
// a presentation concern, so it lives in the cli package rather than leaking
// formatting decisions into the engine.
type jsonResult struct {
	Scenario                jsonScenario      `json:"scenario"`
	ShockProfile            jsonShockProfile  `json:"shock_profile"`
	PropagationRulesApplied jsonRulesApplied  `json:"propagation_rules_applied"`
	DirectExposure          []jsonNodeImpact  `json:"direct_exposure"`
	SecondOrderExposure     []jsonNodeImpact  `json:"second_order_exposure"`
	AffectedPaths           []jsonPath        `json:"affected_paths"`
	ChangedFragilityScores  []jsonNodeImpact  `json:"changed_fragility_scores"`
	HighestRiskEntities     jsonTopEntities   `json:"highest_risk_entities"`
	GraphImpactSummary      jsonSummary       `json:"graph_impact_summary"`
	BlockedEdges            []jsonBlockedEdge `json:"blocked_edges,omitempty"`
	// Warnings are non-fatal, graph-aware advisories for suboptimal but still
	// valid shock combinations. Omitted (and never set) for the CLI path.
	Warnings []string `json:"warnings,omitempty"`

	DataFusion *jsonDataFusion `json:"data_fusion,omitempty"`
}

type jsonDataFusion struct {
	FusionEnabled              bool     `json:"fusion_enabled"`
	RealTradeEdgesUsed         bool     `json:"real_trade_edges_used"`
	RealEventRiskUsed          bool     `json:"real_event_risk_used"`
	RealPriceStressUsed        bool     `json:"real_price_stress_used"`
	EventRiskMultiplierApplied bool     `json:"event_risk_multiplier_applied,omitempty"`
	DataSources                []string `json:"data_sources"`
	PropagationNote            string   `json:"propagation_note,omitempty"`
}

func attachDataFusion(jr *jsonResult, meta graphfusion.Meta, simCtx simulation.Context) {
	note := graphfusion.PropagationNote(meta, simCtx)
	if !meta.FusionEnabled && !simCtx.RealEventRiskUsed && !simCtx.RealPriceStressUsed && note == "" {
		return
	}
	jr.DataFusion = &jsonDataFusion{
		FusionEnabled:              meta.FusionEnabled,
		RealTradeEdgesUsed:         meta.RealTradeEdgesUsed,
		RealEventRiskUsed:          simCtx.RealEventRiskUsed || meta.RealEventRiskUsed,
		RealPriceStressUsed:        simCtx.RealPriceStressUsed || meta.RealPriceStressUsed,
		EventRiskMultiplierApplied: simCtx.EventRiskMultiplierApplied,
		DataSources:                meta.DataSources,
		PropagationNote:            note,
	}
	if jr.DataFusion.DataSources == nil {
		jr.DataFusion.DataSources = []string{graphfusion.SourceStrategic}
	}
}

type jsonShockProfile struct {
	Type                 string   `json:"type"`
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	AllowedRelationships []string `json:"allowed_relationships"`
	Attenuation          float64  `json:"attenuation"`
	RecommendedDepth     int      `json:"recommended_depth"`
	CrossCommodity       bool     `json:"cross_commodity"`
}

type jsonRulesApplied struct {
	ShockType             string   `json:"shock_type"`
	AllowedRelationships  []string `json:"allowed_relationships"`
	CrossCommodityEnabled bool     `json:"cross_commodity_enabled"`
	BlockedCommodities    []string `json:"blocked_commodities"`
}

type jsonBlockedEdge struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Relationship string `json:"relationship_type"`
	Commodity    string `json:"commodity,omitempty"`
	Reason       string `json:"reason"`
}

type jsonScenario struct {
	ID            string  `json:"id,omitempty"`
	Name          string  `json:"name,omitempty"`
	Source        string  `json:"source"`
	Commodity     string  `json:"commodity"`
	ShockType     string  `json:"shock_type"`
	ShockPercent  float64 `json:"shock_percent"`
	Depth         int     `json:"depth"`
	Description   string  `json:"description,omitempty"`
	InitialImpact float64 `json:"initial_impact"`
}

type jsonNodeImpact struct {
	Entity         string  `json:"entity"`
	Type           string  `json:"type"`
	Distance       int     `json:"distance"`
	Impact         float64 `json:"impact"`
	BaseFragility  float64 `json:"base_fragility"`
	ShockFragility float64 `json:"shock_fragility"`
	Delta          float64 `json:"delta"`
}

type jsonPath struct {
	Path          []string `json:"path"`
	Relationships []string `json:"relationships"`
	LabeledPath   string   `json:"labeled_path"`
	PathWeight    float64  `json:"path_weight"`
	EndImpact     float64  `json:"end_impact"`
}

type jsonTopEntities struct {
	Countries   []jsonNodeImpact `json:"countries"`
	Commodities []jsonNodeImpact `json:"commodities"`
	Sectors     []jsonNodeImpact `json:"sectors"`
}

type jsonSummary struct {
	NodesInGraph              int     `json:"nodes_in_graph"`
	AffectedNodes             int     `json:"affected_nodes"`
	AffectedCountries         int     `json:"affected_countries"`
	AffectedCommodities       int     `json:"affected_commodities"`
	AffectedSectors           int     `json:"affected_sectors"`
	AffectedPaths             int     `json:"affected_paths"`
	AvgFragilityDelta         float64 `json:"avg_fragility_delta"`
	LargestSingleImpactEntity string  `json:"largest_single_impact_entity,omitempty"`
	LargestSingleImpactDelta  float64 `json:"largest_single_impact_delta"`
}

// buildJSONResult maps a simulation result (and optional preset metadata) into
// the JSON wire shape. When explain is set, blocked edges are included.
func buildJSONResult(res simulation.Result, scen *data.Scenario, explain bool) jsonResult {
	profile := res.Profile
	jr := jsonResult{
		Scenario: jsonScenario{
			Source:        res.SourceNode.Name,
			Commodity:     res.CommodityNode.Name,
			ShockType:     profile.Type,
			ShockPercent:  round(res.Request.DropPct, 2),
			Depth:         res.Request.Depth,
			InitialImpact: round(res.InitialImpact, 4),
		},
		ShockProfile: jsonShockProfile{
			Type:                 profile.Type,
			Name:                 profile.Name,
			Description:          profile.Description,
			AllowedRelationships: profile.AllowedRelationshipStrings(),
			Attenuation:          profile.Attenuation,
			RecommendedDepth:     profile.RecommendedDepth,
			CrossCommodity:       profile.CrossCommodity,
		},
		PropagationRulesApplied: jsonRulesApplied{
			ShockType:             profile.Type,
			AllowedRelationships:  profile.AllowedRelationshipStrings(),
			CrossCommodityEnabled: profile.CrossCommodity,
			BlockedCommodities:    res.BlockedCommodities(),
		},
		DirectExposure:         impactsToJSON(res.Direct),
		SecondOrderExposure:    impactsToJSON(res.SecondOrder),
		ChangedFragilityScores: impactsToJSON(res.AllAffected),
		AffectedPaths:          pathsToJSON(res.Paths),
		HighestRiskEntities: jsonTopEntities{
			Countries:   impactsToJSON(res.TopCountries),
			Commodities: impactsToJSON(res.TopCommodities),
			Sectors:     impactsToJSON(res.TopSectors),
		},
		GraphImpactSummary: summaryToJSON(res),
	}
	if explain {
		jr.BlockedEdges = blockedEdgesToJSON(res.BlockedEdges)
	}
	if scen != nil {
		jr.Scenario.ID = scen.ID
		jr.Scenario.Name = scen.Name
		jr.Scenario.Description = scen.Description
	}
	return jr
}

func blockedEdgesToJSON(edges []simulation.BlockedEdge) []jsonBlockedEdge {
	out := make([]jsonBlockedEdge, 0, len(edges))
	for _, b := range edges {
		out = append(out, jsonBlockedEdge{
			From:         b.From.Name,
			To:           b.To.Name,
			Relationship: b.Relationship,
			Commodity:    b.Commodity,
			Reason:       b.Reason,
		})
	}
	return out
}

func impactsToJSON(items []simulation.NodeImpact) []jsonNodeImpact {
	out := make([]jsonNodeImpact, 0, len(items))
	for _, ni := range items {
		out = append(out, jsonNodeImpact{
			Entity:         ni.Node.Name,
			Type:           string(ni.Node.Type),
			Distance:       ni.Distance,
			Impact:         round(ni.Impact, 4),
			BaseFragility:  round(ni.BaseFragility, 2),
			ShockFragility: round(ni.ShockFragility, 2),
			Delta:          round(ni.Delta, 2),
		})
	}
	return out
}

func pathsToJSON(paths []simulation.Path) []jsonPath {
	out := make([]jsonPath, 0, len(paths))
	for _, p := range paths {
		names := make([]string, len(p.Nodes))
		for i, n := range p.Nodes {
			names[i] = n.Name
		}
		rels := make([]string, len(p.Edges))
		for i, e := range p.Edges {
			rels[i] = e.Relationship
		}
		out = append(out, jsonPath{
			Path:          names,
			Relationships: rels,
			LabeledPath:   joinLabeledPath(p),
			PathWeight:    round(p.PathWeight, 4),
			EndImpact:     round(p.EndImpact, 4),
		})
	}
	return out
}

func summaryToJSON(res simulation.Result) jsonSummary {
	var countries, commodities, sectors int
	var sumDelta, maxDelta float64
	var maxNode string
	for _, ni := range res.AllAffected {
		switch ni.Node.Type {
		case models.Country:
			countries++
		case models.Commodity:
			commodities++
		case models.Sector:
			sectors++
		}
		sumDelta += ni.Delta
		if ni.Delta > maxDelta {
			maxDelta = ni.Delta
			maxNode = ni.Node.Name
		}
	}
	avg := 0.0
	if len(res.AllAffected) > 0 {
		avg = sumDelta / float64(len(res.AllAffected))
	}
	return jsonSummary{
		NodesInGraph:              res.GraphNodeCount,
		AffectedNodes:             len(res.AllAffected),
		AffectedCountries:         countries,
		AffectedCommodities:       commodities,
		AffectedSectors:           sectors,
		AffectedPaths:             len(res.Paths),
		AvgFragilityDelta:         round(avg, 2),
		LargestSingleImpactEntity: maxNode,
		LargestSingleImpactDelta:  round(maxDelta, 2),
	}
}

func writeResultJSON(w io.Writer, res simulation.Result, scen *data.Scenario, explain bool) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(buildJSONResult(res, scen, explain))
}

func saveResultJSON(path string, res simulation.Result, scen *data.Scenario, explain bool) error {
	b, err := json.MarshalIndent(buildJSONResult(res, scen, explain), "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// --- scenario comparison ---------------------------------------------------

type jsonCompareEntity struct {
	Entity string  `json:"entity"`
	Type   string  `json:"type"`
	Delta  float64 `json:"delta"`
}

type jsonScenarioCompareItem struct {
	Label                  string              `json:"label"`
	Source                 string              `json:"source"`
	Commodity              string              `json:"commodity"`
	ShockType              string              `json:"shock_type"`
	Drop                   float64             `json:"drop"`
	Depth                  int                 `json:"depth"`
	AffectedNodesCount     int                 `json:"affected_nodes_count"`
	AffectedPathsCount     int                 `json:"affected_paths_count"`
	AverageFragilityDelta  float64             `json:"average_fragility_delta"`
	MaxFragilityDelta      float64             `json:"max_fragility_delta"`
	TopAffectedEntities    []jsonCompareEntity `json:"top_affected_entities"`
	TopAffectedCountries   []jsonCompareEntity `json:"top_affected_countries"`
	TopAffectedSectors     []jsonCompareEntity `json:"top_affected_sectors"`
	Warnings               []string            `json:"warnings,omitempty"`
}

type jsonCompareSummary struct {
	WorstOverallScenario         string `json:"worst_overall_scenario"`
	MostCountriesAffected        string `json:"most_countries_affected"`
	MostSectorsAffected          string `json:"most_sectors_affected"`
	HighestAverageFragilityDelta string `json:"highest_average_fragility_delta"`
	HighestMaxFragilityDelta     string `json:"highest_max_fragility_delta"`
}

type jsonCompareResponse struct {
	Summary jsonCompareSummary        `json:"summary"`
	Results []jsonScenarioCompareItem `json:"results"`
}

func compareEntities(items []simulation.CompareEntity) []jsonCompareEntity {
	out := make([]jsonCompareEntity, len(items))
	for i, e := range items {
		out[i] = jsonCompareEntity{
			Entity: e.Entity,
			Type:   e.Type,
			Delta:  round(e.Delta, 2),
		}
	}
	return out
}

func buildCompareJSON(cmp simulation.ComparisonResult, warn func(simulation.ShockProfile, string, string) []string) jsonCompareResponse {
	results := make([]jsonScenarioCompareItem, 0, len(cmp.Results))
	for _, sc := range cmp.Results {
		item := jsonScenarioCompareItem{
			Label:                 sc.Label,
			Source:                sc.Source,
			Commodity:             sc.Commodity,
			ShockType:             sc.ShockType,
			Drop:                  sc.Drop,
			Depth:                 sc.Depth,
			AffectedNodesCount:    sc.AffectedNodesCount,
			AffectedPathsCount:    sc.AffectedPathsCount,
			AverageFragilityDelta: round(sc.AvgFragilityDelta, 2),
			MaxFragilityDelta:     round(sc.MaxFragilityDelta, 2),
			TopAffectedEntities:   compareEntities(sc.TopAffectedEntities),
			TopAffectedCountries:  compareEntities(sc.TopAffectedCountries),
			TopAffectedSectors:    compareEntities(sc.TopAffectedSectors),
		}
		if sc.RunError != "" {
			item.Warnings = []string{sc.RunError}
		} else if warn != nil {
			item.Warnings = warn(sc.Profile, sc.Source, sc.Commodity)
		}
		results = append(results, item)
	}
	return jsonCompareResponse{
		Summary: jsonCompareSummary{
			WorstOverallScenario:         cmp.Summary.WorstOverallScenario,
			MostCountriesAffected:        cmp.Summary.MostCountriesAffected,
			MostSectorsAffected:          cmp.Summary.MostSectorsAffected,
			HighestAverageFragilityDelta: cmp.Summary.HighestAverageFragilityDelta,
			HighestMaxFragilityDelta:     cmp.Summary.HighestMaxFragilityDelta,
		},
		Results: results,
	}
}

func writeCompareJSON(w io.Writer, g *graph.Graph, cmp simulation.ComparisonResult) error {
	out := buildCompareJSON(cmp, func(p simulation.ShockProfile, source, commodity string) []string {
		return shockWarnings(g, p, source, commodity)
	})
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// --- macro exposure scores -------------------------------------------------

type jsonMacroFile struct {
	YearLens  any                `json:"year_lens"` // int year, or "latest"
	Weights   macro.Weights      `json:"weights"`
	RiskBands map[string]string  `json:"risk_bands"`
	Scores    []jsonMacroCountry `json:"scores"`
}

type jsonMacroCountry struct {
	CountryCode        string               `json:"country_code"`
	CountryName        string               `json:"country_name"`
	Year               int                  `json:"year"`
	MacroExposureScore float64              `json:"macro_exposure_score"`
	RiskLevel          string               `json:"risk_level"`
	TopDrivers         []string             `json:"top_drivers"`
	Components         []jsonMacroComponent `json:"components"`
}

type jsonMacroComponent struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Score        float64 `json:"score"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
	YearUsed     int     `json:"year_used"`
	Available    bool    `json:"available"`
}

func buildMacroJSON(scores []macro.CountryScore, yearLens int) jsonMacroFile {
	out := jsonMacroFile{
		Weights: macro.DefaultWeights(),
		RiskBands: map[string]string{
			"low": "0-30", "medium": "30-60", "high": "60-80", "critical": "80-100",
		},
		Scores: make([]jsonMacroCountry, 0, len(scores)),
	}
	if yearLens > 0 {
		out.YearLens = yearLens
	} else {
		out.YearLens = "latest"
	}
	for _, s := range scores {
		jc := jsonMacroCountry{
			CountryCode:        s.CountryCode,
			CountryName:        s.CountryName,
			Year:               s.Year,
			MacroExposureScore: round(s.Score, 1),
			RiskLevel:          s.RiskLevel,
			TopDrivers:         s.TopDrivers,
			Components:         make([]jsonMacroComponent, 0, len(s.Components)),
		}
		if jc.TopDrivers == nil {
			jc.TopDrivers = []string{}
		}
		for _, c := range s.Components {
			jc.Components = append(jc.Components, jsonMacroComponent{
				Key:          c.Key,
				Name:         c.Name,
				Score:        round(c.Score, 1),
				Weight:       c.Weight,
				Contribution: round(c.Contribution, 2),
				YearUsed:     c.YearUsed,
				Available:    c.Available,
			})
		}
		out.Scores = append(out.Scores, jc)
	}
	return out
}

func writeMacroJSON(w io.Writer, scores []macro.CountryScore, yearLens int) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(buildMacroJSON(scores, yearLens))
}

func saveMacroJSON(path string, scores []macro.CountryScore, yearLens int) error {
	b, err := json.MarshalIndent(buildMacroJSON(scores, yearLens), "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// --- event risk scores -----------------------------------------------------

type jsonEventFile struct {
	Source             string             `json:"source"`
	RealEventData      bool               `json:"real_event_data"`
	DateFrom           string             `json:"date_from,omitempty"`
	DateTo             string             `json:"date_to,omitempty"`
	LatestEventDate    string             `json:"latest_event_date,omitempty"`
	RowsProcessed      int                `json:"rows_processed,omitempty"`
	CountriesCovered   int                `json:"countries_covered,omitempty"`
	EventTypeBreakdown map[string]int     `json:"event_type_breakdown,omitempty"`
	ScoringNote        string             `json:"scoring_note,omitempty"`
	Weights            events.Weights     `json:"weights"`
	RiskBands          map[string]string  `json:"risk_bands"`
	Scores             []jsonEventCountry `json:"scores"`
	Country            *jsonEventCountry  `json:"country,omitempty"`
	RecentEvents       []jsonEventRecord  `json:"recent_events,omitempty"`
}

type jsonEventRecord struct {
	Country   string  `json:"country"`
	Date      string  `json:"date"`
	EventType string  `json:"event_type"`
	Severity  float64 `json:"severity"`
	Tone      float64 `json:"tone"`
	Source    string  `json:"source"`
	Summary   string  `json:"summary,omitempty"`
}

type jsonEventCountry struct {
	Country          string               `json:"country,omitempty"`
	CountryCode      string               `json:"country_code"`
	CountryName      string               `json:"country_name"`
	Events           int                  `json:"events"`
	EventCount       int                  `json:"event_count,omitempty"`
	RecentEventCount int                  `json:"recent_event_count,omitempty"`
	AvgTone          float64              `json:"avg_tone"`
	AverageTone      float64              `json:"average_tone,omitempty"`
	EventRiskScore   float64              `json:"event_risk_score"`
	RiskLevel        string               `json:"risk_level"`
	TopDrivers       []string             `json:"top_drivers"`
	TopTerms         []string             `json:"top_terms"`
	TopEventTypes    []string             `json:"top_event_types,omitempty"`
	Components       []jsonEventComponent `json:"components"`
}

type jsonEventComponent struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Score        float64 `json:"score"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
}

func buildEventRiskJSON(scores []events.CountryScore) jsonEventFile {
	return buildResolvedEventRiskJSON(resolvedEventRisk{Scores: scores, Source: "sample", RealEventData: false})
}

func buildResolvedEventRiskJSON(r resolvedEventRisk) jsonEventFile {
	out := jsonEventFile{
		Source:             r.Source,
		RealEventData:      r.RealEventData,
		DateFrom:           r.DateFrom,
		DateTo:             r.DateTo,
		LatestEventDate:    r.LatestEventDate,
		RowsProcessed:      r.RowsProcessed,
		CountriesCovered:   r.CountriesCovered,
		EventTypeBreakdown: r.EventTypeBreakdown,
		ScoringNote:        r.ScoringNote,
		Weights:            events.DefaultWeights(),
		RiskBands: map[string]string{
			"low": "0-30", "medium": "30-60", "high": "60-80", "critical": "80-100",
		},
		Scores: make([]jsonEventCountry, 0, len(r.Scores)),
	}
	for _, s := range r.Scores {
		out.Scores = append(out.Scores, jsonCountryFromScore(s, r.Processed))
	}
	if r.CountryFilter != "" && len(out.Scores) > 0 {
		country := out.Scores[0]
		out.Country = &country
	}
	if len(r.RecentEvents) > 0 {
		out.RecentEvents = make([]jsonEventRecord, 0, len(r.RecentEvents))
		for _, e := range r.RecentEvents {
			out.RecentEvents = append(out.RecentEvents, jsonEventRecord{
				Country: e.Country, Date: e.Date, EventType: e.EventType,
				Severity: round(e.Severity, 2), Tone: round(e.Tone, 2),
				Source: e.Source, Summary: e.Summary,
			})
		}
	}
	return out
}

func jsonCountryFromScore(s events.CountryScore, processed *eventrisk.RiskFile) jsonEventCountry {
	jc := jsonEventCountry{
		Country:        s.CountryName,
		CountryCode:    s.CountryCode,
		CountryName:    s.CountryName,
		Events:         s.Events,
		EventCount:     s.Events,
		AvgTone:        round(s.AvgTone, 2),
		AverageTone:    round(s.AvgTone, 2),
		EventRiskScore: round(s.Score, 1),
		RiskLevel:      s.RiskLevel,
		TopDrivers:     s.TopDrivers,
		TopTerms:       s.TopTerms,
		TopEventTypes:  s.TopTerms,
		Components:     make([]jsonEventComponent, 0, len(s.Components)),
	}
	if processed != nil {
		if row, ok := eventrisk.CountryRiskFor(*processed, s.CountryName); ok {
			jc.RecentEventCount = row.RecentEventCount
			jc.TopEventTypes = row.TopEventTypes
			jc.EventCount = row.EventCount
			jc.AverageTone = round(row.AverageTone, 2)
		}
	}
	if jc.TopDrivers == nil {
		jc.TopDrivers = []string{}
	}
	if jc.TopTerms == nil {
		jc.TopTerms = []string{}
	}
	if jc.TopEventTypes == nil {
		jc.TopEventTypes = []string{}
	}
	for _, c := range s.Components {
		jc.Components = append(jc.Components, jsonEventComponent{
			Key:          c.Key,
			Name:         c.Name,
			Score:        round(c.Score, 1),
			Weight:       c.Weight,
			Contribution: round(c.Contribution, 2),
		})
	}
	return jc
}

func writeEventRiskJSON(w io.Writer, scores []events.CountryScore) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(buildEventRiskJSON(scores))
}

func saveEventRiskJSON(path string, scores []events.CountryScore) error {
	b, err := json.MarshalIndent(buildEventRiskJSON(scores), "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// --- commodity stress scores -----------------------------------------------

type jsonCommodityFile struct {
	DataSource    string               `json:"data_source"`
	RealPriceData bool                 `json:"real_price_data"`
	Weights       commodities.Weights  `json:"weights"`
	RiskBands     map[string]string    `json:"risk_bands"`
	Scores        []jsonCommodityScore `json:"scores"`
}

type jsonCommodityScore struct {
	CommodityCode string                   `json:"commodity_code"`
	CommodityName string                   `json:"commodity_name"`
	Unit          string                   `json:"unit"`
	Months        int                      `json:"months"`
	LatestDate    string                   `json:"latest_date"`
	LatestPrice   float64                  `json:"latest_price_usd"`
	Change3MPct   *float64                 `json:"change_3m_pct"`
	Change12MPct  *float64                 `json:"change_12m_pct"`
	VolatilityPct float64                  `json:"volatility_pct"`
	StressScore   float64                  `json:"commodity_stress_score"`
	RiskLevel     string                   `json:"risk_level"`
	Components    []jsonCommodityComponent `json:"components"`
}

type jsonCommodityComponent struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Score        float64 `json:"score"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
}

func buildCommodityStressJSON(scores []commodities.CommodityScore, dataSource string) jsonCommodityFile {
	out := jsonCommodityFile{
		DataSource:    dataSource,
		RealPriceData: commodityprices.IsRealPriceSource(dataSource),
		Weights:       commodities.DefaultWeights(),
		RiskBands: map[string]string{
			"low": "0-30", "medium": "30-60", "high": "60-80", "critical": "80-100",
		},
		Scores: make([]jsonCommodityScore, 0, len(scores)),
	}
	for _, s := range scores {
		jc := jsonCommodityScore{
			CommodityCode: s.CommodityCode,
			CommodityName: s.CommodityName,
			Unit:          s.Unit,
			Months:        s.Months,
			LatestDate:    s.LatestDate,
			LatestPrice:   round(s.LatestPrice, 2),
			VolatilityPct: round(s.Volatility, 2),
			StressScore:   round(s.Score, 1),
			RiskLevel:     s.RiskLevel,
			Components:    make([]jsonCommodityComponent, 0, len(s.Components)),
		}
		if s.Change3MAvailable {
			v := round(s.Change3M, 2)
			jc.Change3MPct = &v
		}
		if s.Change12MAvailable {
			v := round(s.Change12M, 2)
			jc.Change12MPct = &v
		}
		for _, c := range s.Components {
			jc.Components = append(jc.Components, jsonCommodityComponent{
				Key:          c.Key,
				Name:         c.Name,
				Score:        round(c.Score, 1),
				Weight:       c.Weight,
				Contribution: round(c.Contribution, 2),
			})
		}
		out.Scores = append(out.Scores, jc)
	}
	return out
}

func writeCommodityStressJSON(w io.Writer, scores []commodities.CommodityScore, dataSource string) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(buildCommodityStressJSON(scores, dataSource))
}

func saveCommodityStressJSON(path string, scores []commodities.CommodityScore, dataSource string) error {
	b, err := json.MarshalIndent(buildCommodityStressJSON(scores, dataSource), "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func round(v float64, places int) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	scale := math.Pow(10, float64(places))
	return math.Round(v*scale) / scale
}

// writeJSON encodes any value as pretty-printed JSON to w.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// --- trade dependency & concentration -------------------------------------

type jsonTradeSummary struct {
	Source        string        `json:"source"`
	RealTradeData bool          `json:"real_trade_data"`
	trade.Summary
}

type jsonTradeSupplier struct {
	ExporterCode string  `json:"exporter_code"`
	ExporterName string  `json:"exporter_name"`
	ValueUSD     float64 `json:"value_usd"`
	Share        float64 `json:"share"`
	SharePct     float64 `json:"share_pct"`
	Dependency   string  `json:"dependency"`
}

type jsonTradeDependency struct {
	Source          string              `json:"source"`
	RealTradeData   bool                `json:"real_trade_data"`
	Importer        string              `json:"importer"`
	ImporterCode    string              `json:"importer_code"`
	Commodity       string              `json:"commodity"`
	TotalImportsUSD float64             `json:"total_imports_usd"`
	Suppliers       []jsonTradeSupplier `json:"suppliers"`
}

type jsonTradeConcentration struct {
	Source            string            `json:"source"`
	RealTradeData     bool              `json:"real_trade_data"`
	Importer          string            `json:"importer"`
	ImporterCode      string            `json:"importer_code"`
	Commodity         string            `json:"commodity"`
	HHI               float64           `json:"hhi"`
	ConcentrationRisk string            `json:"concentration_risk"`
	TopSupplier       jsonTradeSupplier `json:"top_supplier"`
}

func tradeSupplierToJSON(s trade.Supplier) jsonTradeSupplier {
	return jsonTradeSupplier{
		ExporterCode: s.ExporterCode,
		ExporterName: s.ExporterName,
		ValueUSD:     s.ValueUSD,
		Share:        round(s.Share, 4),
		SharePct:     round(s.Share*100, 1),
		Dependency:   s.Dependency,
	}
}

func buildTradeSummaryJSON(resolved trade.ResolvedTrade, summary trade.Summary) jsonTradeSummary {
	if resolved.DependencyFile != nil {
		if reporters := trade.AvailableImportReporters(*resolved.DependencyFile); len(reporters) > 0 {
			summary.AvailableImporters = reporters
		}
	}
	return jsonTradeSummary{
		Source:        resolved.Source,
		RealTradeData: resolved.RealTradeData,
		Summary:       summary,
	}
}

type jsonTradeOptions struct {
	Source        string               `json:"source"`
	RealTradeData bool                 `json:"real_trade_data"`
	Importers     []trade.ImportOption `json:"importers"`
}

func buildTradeOptionsJSON(resolved trade.ResolvedTrade, opts trade.TradeOptions) jsonTradeOptions {
	importers := opts.Importers
	if importers == nil {
		importers = []trade.ImportOption{}
	}
	return jsonTradeOptions{
		Source:        resolved.Source,
		RealTradeData: resolved.RealTradeData,
		Importers:     importers,
	}
}

func buildTradeDependencyJSON(resolved trade.ResolvedTrade, d trade.Dependency) jsonTradeDependency {
	out := jsonTradeDependency{
		Source:          resolved.Source,
		RealTradeData:   resolved.RealTradeData,
		Importer:        d.ImporterName,
		ImporterCode:    d.ImporterCode,
		Commodity:       d.Commodity,
		TotalImportsUSD: d.TotalImportsUSD,
		Suppliers:       make([]jsonTradeSupplier, 0, len(d.Suppliers)),
	}
	for _, s := range d.Suppliers {
		out.Suppliers = append(out.Suppliers, tradeSupplierToJSON(s))
	}
	return out
}

func buildTradeConcentrationJSON(resolved trade.ResolvedTrade, c trade.Concentration) jsonTradeConcentration {
	return jsonTradeConcentration{
		Source:            resolved.Source,
		RealTradeData:     resolved.RealTradeData,
		Importer:          c.ImporterName,
		ImporterCode:      c.ImporterCode,
		Commodity:         c.Commodity,
		HHI:               round(c.HHI, 4),
		ConcentrationRisk: c.RiskLevel,
		TopSupplier:       tradeSupplierToJSON(c.TopSupplier),
	}
}

// --- unified fragility -----------------------------------------------------

type jsonFragilityFile struct {
	CountryWeights    fragility.CountryWeights    `json:"country_weights"`
	CommodityWeights  fragility.CommodityWeights  `json:"commodity_weights"`
	RiskBands         map[string]string           `json:"risk_bands"`
	Countries         []jsonFragilityCountry      `json:"countries"`
	Commodities       []jsonFragilityCommodity    `json:"commodities"`
}

type jsonFragilityCountry struct {
	CountryCode       string                 `json:"country_code"`
	CountryName       string                 `json:"country_name"`
	Score             float64                `json:"score"`
	RiskLevel         string                 `json:"risk_level"`
	TopDrivers        []string               `json:"top_drivers"`
	MissingComponents []string               `json:"missing_components"`
	Components        []jsonFragilityComponent `json:"components"`
}

type jsonFragilityCommodity struct {
	CommodityCode     string                 `json:"commodity_code"`
	CommodityName     string                 `json:"commodity_name"`
	Score             float64                `json:"score"`
	RiskLevel         string                 `json:"risk_level"`
	TopDrivers        []string               `json:"top_drivers"`
	MissingComponents []string               `json:"missing_components"`
	Components        []jsonFragilityComponent `json:"components"`
}

type jsonFragilityComponent struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Score        float64 `json:"score"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
	Available    bool    `json:"available"`
	Source       string  `json:"source,omitempty"`
	Note         string  `json:"note,omitempty"`
}

type jsonFragilitySummary struct {
	Countries   []jsonFragilityCountry   `json:"countries"`
	Commodities []jsonFragilityCommodity `json:"commodities"`

	FusionEnabled       bool     `json:"fusion_enabled"`
	RealTradeEdgesUsed  bool     `json:"real_trade_edges_used"`
	RealEventRiskUsed   bool     `json:"real_event_risk_used"`
	RealPriceStressUsed bool     `json:"real_price_stress_used"`
	DataSources         []string `json:"data_sources"`

	TradeConcentrationSource string `json:"trade_concentration_source,omitempty"`
	TradeConcentrationNote   string `json:"trade_concentration_note,omitempty"`
}

func buildFragilityJSON(res fragility.Result) jsonFragilityFile {
	out := jsonFragilityFile{
		CountryWeights:   fragility.DefaultCountryWeights(),
		CommodityWeights: fragility.DefaultCommodityWeights(),
		RiskBands: map[string]string{
			"low": "0-30", "medium": "30-60", "high": "60-80", "critical": "80-100",
		},
		Countries:   make([]jsonFragilityCountry, 0, len(res.Countries)),
		Commodities: make([]jsonFragilityCommodity, 0, len(res.Commodities)),
	}
	for _, s := range res.Countries {
		out.Countries = append(out.Countries, countryToJSON(s))
	}
	for _, s := range res.Commodities {
		out.Commodities = append(out.Commodities, commodityToJSON(s))
	}
	return out
}

func buildFragilitySummaryJSON(res fragility.Result, topN int, meta graphfusion.Meta) jsonFragilitySummary {
	out := jsonFragilitySummary{
		Countries:   make([]jsonFragilityCountry, 0, topN),
		Commodities: make([]jsonFragilityCommodity, 0, topN),
	}
	for i, s := range res.Countries {
		if i >= topN {
			break
		}
		out.Countries = append(out.Countries, countryToJSON(s))
	}
	for i, s := range res.Commodities {
		if i >= topN {
			break
		}
		out.Commodities = append(out.Commodities, commodityToJSON(s))
	}
	out.FusionEnabled = meta.FusionEnabled
	out.RealTradeEdgesUsed = meta.RealTradeEdgesUsed
	out.RealEventRiskUsed = meta.RealEventRiskUsed
	out.RealPriceStressUsed = meta.RealPriceStressUsed
	out.DataSources = meta.DataSources
	if out.DataSources == nil {
		out.DataSources = []string{graphfusion.SourceStrategic}
	}
	out.TradeConcentrationSource = res.TradeConcentrationSource
	out.TradeConcentrationNote = res.TradeConcentrationNote
	return out
}

func appendMacroDataSource(sources []string) []string {
	for _, s := range sources {
		if s == graphfusion.SourceWorldBankMacro {
			return sources
		}
	}
	return append(sources, graphfusion.SourceWorldBankMacro)
}

func countryToJSON(s fragility.CountryScore) jsonFragilityCountry {
	jc := jsonFragilityCountry{
		CountryCode:       s.CountryCode,
		CountryName:       s.CountryName,
		Score:             round(s.Score, 1),
		RiskLevel:         s.RiskLevel,
		TopDrivers:        s.TopDrivers,
		MissingComponents: s.MissingComponents,
		Components:        make([]jsonFragilityComponent, 0, len(s.Components)),
	}
	if jc.TopDrivers == nil {
		jc.TopDrivers = []string{}
	}
	if jc.MissingComponents == nil {
		jc.MissingComponents = []string{}
	}
	for _, c := range s.Components {
		jc.Components = append(jc.Components, fragilityComponentToJSON(c))
	}
	return jc
}

func commodityToJSON(s fragility.CommodityScore) jsonFragilityCommodity {
	jc := jsonFragilityCommodity{
		CommodityCode:     s.CommodityCode,
		CommodityName:     s.CommodityName,
		Score:             round(s.Score, 1),
		RiskLevel:         s.RiskLevel,
		TopDrivers:        s.TopDrivers,
		MissingComponents: s.MissingComponents,
		Components:        make([]jsonFragilityComponent, 0, len(s.Components)),
	}
	if jc.TopDrivers == nil {
		jc.TopDrivers = []string{}
	}
	if jc.MissingComponents == nil {
		jc.MissingComponents = []string{}
	}
	for _, c := range s.Components {
		jc.Components = append(jc.Components, fragilityComponentToJSON(c))
	}
	return jc
}

func fragilityComponentToJSON(c fragility.Component) jsonFragilityComponent {
	return jsonFragilityComponent{
		Key:          c.Key,
		Name:         c.Name,
		Score:        round(c.Score, 1),
		Weight:       c.Weight,
		Contribution: round(c.Contribution, 2),
		Available:    c.Available,
		Source:       c.Source,
		Note:         c.Note,
	}
}

func writeFragilityJSON(w io.Writer, res fragility.Result) error {
	return writeJSON(w, buildFragilityJSON(res))
}

func saveFragilityJSON(path string, res fragility.Result) error {
	raw, err := json.MarshalIndent(buildFragilityJSON(res), "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
