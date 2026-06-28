package cli

import (
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/scoring/events"
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
	Weights   events.Weights     `json:"weights"`
	RiskBands map[string]string  `json:"risk_bands"`
	Scores    []jsonEventCountry `json:"scores"`
}

type jsonEventCountry struct {
	CountryCode    string               `json:"country_code"`
	CountryName    string               `json:"country_name"`
	Events         int                  `json:"events"`
	AvgTone        float64              `json:"avg_tone"`
	EventRiskScore float64              `json:"event_risk_score"`
	RiskLevel      string               `json:"risk_level"`
	TopDrivers     []string             `json:"top_drivers"`
	TopTerms       []string             `json:"top_terms"`
	Components     []jsonEventComponent `json:"components"`
}

type jsonEventComponent struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Score        float64 `json:"score"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
}

func buildEventRiskJSON(scores []events.CountryScore) jsonEventFile {
	out := jsonEventFile{
		Weights: events.DefaultWeights(),
		RiskBands: map[string]string{
			"low": "0-30", "medium": "30-60", "high": "60-80", "critical": "80-100",
		},
		Scores: make([]jsonEventCountry, 0, len(scores)),
	}
	for _, s := range scores {
		jc := jsonEventCountry{
			CountryCode:    s.CountryCode,
			CountryName:    s.CountryName,
			Events:         s.Events,
			AvgTone:        round(s.AvgTone, 2),
			EventRiskScore: round(s.Score, 1),
			RiskLevel:      s.RiskLevel,
			TopDrivers:     s.TopDrivers,
			TopTerms:       s.TopTerms,
			Components:     make([]jsonEventComponent, 0, len(s.Components)),
		}
		if jc.TopDrivers == nil {
			jc.TopDrivers = []string{}
		}
		if jc.TopTerms == nil {
			jc.TopTerms = []string{}
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
		out.Scores = append(out.Scores, jc)
	}
	return out
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

type jsonTradeSupplier struct {
	ExporterCode string  `json:"exporter_code"`
	ExporterName string  `json:"exporter_name"`
	ValueUSD     float64 `json:"value_usd"`
	Share        float64 `json:"share"`
	SharePct     float64 `json:"share_pct"`
	Dependency   string  `json:"dependency"`
}

type jsonTradeDependency struct {
	Importer        string              `json:"importer"`
	ImporterCode    string              `json:"importer_code"`
	Commodity       string              `json:"commodity"`
	TotalImportsUSD float64             `json:"total_imports_usd"`
	Suppliers       []jsonTradeSupplier `json:"suppliers"`
}

type jsonTradeConcentration struct {
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

func buildTradeDependencyJSON(d trade.Dependency) jsonTradeDependency {
	out := jsonTradeDependency{
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

func buildTradeConcentrationJSON(c trade.Concentration) jsonTradeConcentration {
	return jsonTradeConcentration{
		Importer:          c.ImporterName,
		ImporterCode:      c.ImporterCode,
		Commodity:         c.Commodity,
		HHI:               round(c.HHI, 4),
		ConcentrationRisk: c.RiskLevel,
		TopSupplier:       tradeSupplierToJSON(c.TopSupplier),
	}
}
