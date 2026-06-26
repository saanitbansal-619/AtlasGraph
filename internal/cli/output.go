package cli

import (
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/models"
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

func round(v float64, places int) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	scale := math.Pow(10, float64(places))
	return math.Round(v*scale) / scale
}
