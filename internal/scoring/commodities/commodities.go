// Package commodities turns ingested commodity price time series into an
// explainable per-commodity price-stress score.
//
// This is a price-stress signal, not a forecast: it measures how much recent
// price movement, volatility and 12-month momentum a commodity is exhibiting,
// so a downstream view can flag commodities under acute price pressure. Like the
// macro and event scorers it is deliberately transparent — every score
// decomposes into weighted, individually-named components — so it can later
// combine with trade dependency and graph fragility into a unified view.
package commodities

import (
	"sort"

	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
)

// Weights controls how component scores combine into the final score. They sum
// to 1.0, so the weighted result is already on a 0..100 scale.
type Weights struct {
	RecentChange float64 `json:"recent_change"`
	Volatility   float64 `json:"volatility"`
	Momentum     float64 `json:"momentum"`
}

// DefaultWeights is the calibrated weighting used by the CLI.
func DefaultWeights() Weights {
	return Weights{
		RecentChange: 0.40,
		Volatility:   0.40,
		Momentum:     0.20,
	}
}

// Component is one contributor to a commodity's stress score.
type Component struct {
	Key          string  // stable identifier, e.g. "recent_change"
	Name         string  // human-friendly label, e.g. "recent price change"
	Score        float64 // 0..100
	Weight       float64 // weight applied in the blend
	Contribution float64 // Weight * Score
}

// CommodityScore is the full, explainable price-stress result for one commodity.
type CommodityScore struct {
	CommodityCode string
	CommodityName string
	Unit          string
	Months        int    // number of monthly observations available
	LatestDate    string // month of the latest observation ("YYYY-MM")
	LatestPrice   float64

	Change3M           float64 // signed % change over the last 3 months
	Change3MAvailable  bool
	Change12M          float64 // signed % change over the last 12 months
	Change12MAvailable bool
	Volatility         float64 // stddev of monthly returns over last 12 months, in %

	Score      float64 // 0..100 price stress
	RiskLevel  string
	Components []Component
}

// ScoreCommodities computes a price-stress score for every commodity present in
// the ingested file. Results are sorted by score, highest first (tie-break on
// name).
func ScoreCommodities(file commodityprices.PriceFile, w Weights) []CommodityScore {
	type group struct {
		name string
		unit string
		recs []commodityprices.PriceRecord
	}
	byCode := map[string]*group{}
	var order []string
	for _, r := range file.Records {
		g, ok := byCode[r.CommodityCode]
		if !ok {
			g = &group{}
			byCode[r.CommodityCode] = g
			order = append(order, r.CommodityCode)
		}
		if g.name == "" && r.CommodityName != "" {
			g.name = r.CommodityName
		}
		if g.unit == "" && r.Unit != "" {
			g.unit = r.Unit
		}
		g.recs = append(g.recs, r)
	}

	out := make([]CommodityScore, 0, len(order))
	for _, code := range order {
		g := byCode[code]
		out = append(out, scoreCommodity(code, g.name, g.unit, g.recs, w))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].CommodityName < out[j].CommodityName
	})
	return out
}

func scoreCommodity(code, name, unit string, recs []commodityprices.PriceRecord, w Weights) CommodityScore {
	// Order observations chronologically so the series maths is well-defined.
	ordered := make([]commodityprices.PriceRecord, len(recs))
	copy(ordered, recs)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Date < ordered[j].Date })

	prices := make([]float64, len(ordered))
	for i, r := range ordered {
		prices[i] = r.PriceUSD
	}

	cs := CommodityScore{
		CommodityCode: code,
		CommodityName: name,
		Unit:          unit,
		Months:        len(ordered),
	}
	if len(ordered) > 0 {
		last := ordered[len(ordered)-1]
		cs.LatestDate = last.Date
		cs.LatestPrice = last.PriceUSD
	}

	cs.Change3M, cs.Change3MAvailable = pctChangeOverMonths(prices, 3)
	cs.Change12M, cs.Change12MAvailable = pctChangeOverMonths(prices, 12)
	cs.Volatility = volatilityPct(prices, 12)

	// Component scores. Large moves in either direction are treated as stress,
	// so the recent-change and momentum components use the magnitude of the move.
	recentScore := scaleLinear(absVal(cs.Change3M), 0, recentChangeHiPct)
	volScore := scaleLinear(cs.Volatility, 0, volatilityHiPct)
	momentumScore := scaleLinear(absVal(cs.Change12M), 0, momentumHiPct)

	cs.Components = []Component{
		makeComponent("recent_change", "recent price change", w.RecentChange, recentScore),
		makeComponent("volatility", "price volatility", w.Volatility, volScore),
		makeComponent("momentum", "12-month momentum", w.Momentum, momentumScore),
	}

	final := 0.0
	for _, c := range cs.Components {
		final += c.Contribution
	}
	cs.Score = final
	cs.RiskLevel = RiskLevel(final)
	return cs
}

func makeComponent(key, name string, weight, score float64) Component {
	return Component{
		Key:          key,
		Name:         name,
		Score:        score,
		Weight:       weight,
		Contribution: weight * score,
	}
}
