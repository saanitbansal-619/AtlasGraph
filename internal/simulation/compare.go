package simulation

import (
	"sort"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
)

// CompareScenario is one shock to run as part of a side-by-side comparison.
type CompareScenario struct {
	Label   string
	Request ShockRequest
}

// CompareEntity is a compact affected-entity row for comparison output.
type CompareEntity struct {
	Entity string
	Type   string
	Delta  float64
}

// ScenarioComparison holds the systemic-impact metrics for one compared shock.
type ScenarioComparison struct {
	Label                  string
	Source                 string
	Commodity              string
	ShockType              string
	Drop                   float64
	Depth                  int
	AffectedNodesCount     int
	AffectedPathsCount     int
	AffectedCountriesCount int
	AffectedSectorsCount   int
	AvgFragilityDelta      float64
	MaxFragilityDelta      float64
	TopAffectedEntities    []CompareEntity
	TopAffectedCountries   []CompareEntity
	TopAffectedSectors     []CompareEntity
	RunError               string // set when simulation.Run fails; metrics stay zero
	Profile                ShockProfile `json:"-"` // retained for graph-aware warnings in the API layer
}

// CompareSummary highlights which scenario wins each impact dimension.
type CompareSummary struct {
	WorstOverallScenario         string
	MostCountriesAffected        string
	MostSectorsAffected          string
	HighestAverageFragilityDelta string
	HighestMaxFragilityDelta     string
}

// ComparisonResult is the full output of a multi-scenario comparison run.
type ComparisonResult struct {
	Summary CompareSummary
	Results []ScenarioComparison
}

const compareTopN = 5

// CompareScenarios runs each scenario independently and ranks them by systemic
// impact. A scenario that fails to run is included with RunError set; the
// comparison as a whole still succeeds.
func CompareScenarios(g *graph.Graph, cfg config.Config, scenarios []CompareScenario) ComparisonResult {
	return CompareScenariosWithContext(g, cfg, scenarios, nil)
}

// CompareScenariosWithContext runs comparisons with optional real-data context.
func CompareScenariosWithContext(g *graph.Graph, cfg config.Config, scenarios []CompareScenario, ctx *Context) ComparisonResult {
	results := make([]ScenarioComparison, 0, len(scenarios))
	for _, sc := range scenarios {
		results = append(results, runOneComparisonWithContext(g, cfg, sc, ctx))
	}
	sort.SliceStable(results, func(i, j int) bool {
		return compareRank(results[i]) > compareRank(results[j])
	})
	return ComparisonResult{
		Summary: buildCompareSummary(results),
		Results: results,
	}
}

func runOneComparison(g *graph.Graph, cfg config.Config, sc CompareScenario) ScenarioComparison {
	return runOneComparisonWithContext(g, cfg, sc, nil)
}

func runOneComparisonWithContext(g *graph.Graph, cfg config.Config, sc CompareScenario, ctx *Context) ScenarioComparison {
	req := sc.Request
	label := sc.Label
	if label == "" {
		label = req.Source + " · " + req.Commodity
	}

	out := ScenarioComparison{
		Label:     label,
		Source:    req.Source,
		Commodity: req.Commodity,
		ShockType: req.ShockType,
		Drop:      req.DropPct,
		Depth:     req.Depth,
	}

	var res Result
	var err error
	if ctx != nil {
		res, err = RunWithContext(g, cfg, req, ctx)
	} else {
		res, err = Run(g, cfg, req)
	}
	if err != nil {
		out.RunError = err.Error()
		return out
	}

	metrics := metricsFromResult(res, cfg.TopN)
	out.AffectedNodesCount = metrics.affectedNodes
	out.AffectedPathsCount = metrics.affectedPaths
	out.AffectedCountriesCount = metrics.affectedCountries
	out.AffectedSectorsCount = metrics.affectedSectors
	out.AvgFragilityDelta = metrics.avgDelta
	out.MaxFragilityDelta = metrics.maxDelta
	out.TopAffectedEntities = metrics.topEntities
	out.TopAffectedCountries = metrics.topCountries
	out.TopAffectedSectors = metrics.topSectors
	out.Profile = res.Profile
	return out
}

type resultMetrics struct {
	affectedNodes      int
	affectedPaths      int
	affectedCountries  int
	affectedSectors    int
	avgDelta           float64
	maxDelta           float64
	topEntities        []CompareEntity
	topCountries       []CompareEntity
	topSectors         []CompareEntity
}

func metricsFromResult(res Result, topN int) resultMetrics {
	if topN < 1 {
		topN = 3
	}
	var countries, commodities, sectors int
	var sumDelta, maxDelta float64
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
		}
	}
	_ = commodities
	avg := 0.0
	if n := len(res.AllAffected); n > 0 {
		avg = sumDelta / float64(n)
	}

	topEntities := entitiesFromImpacts(res.AllAffected, compareTopN)
	topCountries := entitiesFromImpacts(res.TopCountries, topN)
	topSectors := entitiesFromImpacts(res.TopSectors, topN)

	return resultMetrics{
		affectedNodes:     len(res.AllAffected),
		affectedPaths:     len(res.Paths),
		affectedCountries: countries,
		affectedSectors:   sectors,
		avgDelta:          avg,
		maxDelta:          maxDelta,
		topEntities:       topEntities,
		topCountries:      topCountries,
		topSectors:        topSectors,
	}
}

func entitiesFromImpacts(items []NodeImpact, n int) []CompareEntity {
	out := make([]CompareEntity, 0, n)
	for i, ni := range items {
		if i >= n {
			break
		}
		out = append(out, CompareEntity{
			Entity: ni.Node.Name,
			Type:   string(ni.Node.Type),
			Delta:  ni.Delta,
		})
	}
	return out
}

// compareRank scores a scenario for worst-overall ordering (higher = more impact).
func compareRank(sc ScenarioComparison) float64 {
	if sc.RunError != "" {
		return -1
	}
	return sc.AvgFragilityDelta*1000 + sc.MaxFragilityDelta*10 + float64(sc.AffectedNodesCount)
}

func buildCompareSummary(results []ScenarioComparison) CompareSummary {
	var summary CompareSummary
	if len(results) == 0 {
		return summary
	}

	ok := filterOK(results)
	if len(ok) == 0 {
		return summary
	}

	summary.WorstOverallScenario = ok[0].Label
	summary.MostCountriesAffected = labelWithMaxInt(ok, func(s ScenarioComparison) int { return s.AffectedCountriesCount })
	summary.MostSectorsAffected = labelWithMaxInt(ok, func(s ScenarioComparison) int { return s.AffectedSectorsCount })
	summary.HighestAverageFragilityDelta = labelWithMaxFloat(ok, func(s ScenarioComparison) float64 { return s.AvgFragilityDelta })
	summary.HighestMaxFragilityDelta = labelWithMaxFloat(ok, func(s ScenarioComparison) float64 { return s.MaxFragilityDelta })
	return summary
}

func filterOK(results []ScenarioComparison) []ScenarioComparison {
	out := make([]ScenarioComparison, 0, len(results))
	for _, r := range results {
		if r.RunError == "" {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return compareRank(out[i]) > compareRank(out[j])
	})
	return out
}

func labelWithMaxInt(items []ScenarioComparison, fn func(ScenarioComparison) int) string {
	best := items[0]
	max := fn(best)
	for _, s := range items[1:] {
		if v := fn(s); v > max {
			max, best = v, s
		}
	}
	return best.Label
}

func labelWithMaxFloat(items []ScenarioComparison, fn func(ScenarioComparison) float64) string {
	best := items[0]
	max := fn(best)
	for _, s := range items[1:] {
		if v := fn(s); v > max {
			max, best = v, s
		}
	}
	return best.Label
}
