// Package macro turns ingested World Bank macroeconomic indicators into an
// explainable baseline fragility score for each country.
//
// This is exposure/risk scoring, not forecasting: it measures how structurally
// exposed an economy is to trade, supply-chain, price and technology shocks
// today, given its latest macro fundamentals. It is deliberately transparent —
// every score decomposes into weighted, individually-named components — so it
// can later seed the graph engine's baseline country fragility.
package macro

import (
	"sort"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

// Weights controls how component scores combine into the final score. They sum
// to 1.0, so the weighted result is already on a 0..100 scale.
type Weights struct {
	Trade         float64 `json:"trade_exposure"`
	Manufacturing float64 `json:"manufacturing_dependency"`
	Inflation     float64 `json:"inflation_stress"`
	HighTech      float64 `json:"high_tech_concentration"`
	BufferRisk    float64 `json:"economic_buffer_risk"`
}

// DefaultWeights is the calibrated weighting used by the CLI.
func DefaultWeights() Weights {
	return Weights{
		Trade:         0.30,
		Manufacturing: 0.25,
		Inflation:     0.20,
		HighTech:      0.15,
		BufferRisk:    0.10,
	}
}

// Component is one contributor to a country's macro fragility score.
type Component struct {
	Key          string  // stable identifier, e.g. "trade_exposure"
	Name         string  // human-friendly label, e.g. "trade exposure"
	Score        float64 // 0..100
	Weight       float64 // weight applied in the blend
	Contribution float64 // Weight * Score (0 when unavailable)
	YearUsed     int     // year of the underlying indicator (0 if missing)
	Available    bool    // whether the inputs were present
}

// CountryScore is the full, explainable result for one country.
type CountryScore struct {
	CountryCode string
	CountryName string
	Year        int     // overall year lens (requested, or latest available)
	Score       float64 // 0..100 macro fragility
	RiskLevel   string
	Components  []Component
	TopDrivers  []string // friendly names of the largest contributors
}

// ScoreCountries computes a macro fragility score for every country present in
// the ingested file. targetYear <= 0 means "use the latest available year".
// Results are sorted by score, highest first.
func ScoreCountries(file worldbank.IndicatorFile, targetYear int, w Weights) []CountryScore {
	type group struct {
		name string
		recs []worldbank.CountryIndicatorRecord
	}
	byCountry := map[string]*group{}
	var order []string
	for _, r := range file.Records {
		g, ok := byCountry[r.CountryCode]
		if !ok {
			g = &group{}
			byCountry[r.CountryCode] = g
			order = append(order, r.CountryCode)
		}
		if g.name == "" && r.CountryName != "" {
			g.name = r.CountryName
		}
		g.recs = append(g.recs, r)
	}

	out := make([]CountryScore, 0, len(order))
	for _, code := range order {
		g := byCountry[code]
		out = append(out, scoreCountry(code, g.name, g.recs, targetYear, w))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].CountryName < out[j].CountryName
	})
	return out
}

func scoreCountry(code, name string, recs []worldbank.CountryIndicatorRecord, target int, w Weights) CountryScore {
	imp, impY, impOK := pickValue(recs, codeImports, target)
	exp, expY, expOK := pickValue(recs, codeExports, target)
	manuf, manY, manOK := pickValue(recs, codeManuf, target)
	infl, inflY, inflOK := pickValue(recs, codeInflation, target)
	gdp, gdpY, gdpOK := pickValue(recs, codeGDP, target)
	ht, htY, htOK := pickValue(recs, codeHighTech, target)

	var comps []Component

	// Trade exposure: usable if either side of the trade balance is present.
	tradeAvail := impOK || expOK
	tradeVal, tradeYear := 0.0, 0
	if impOK {
		tradeVal += imp
		tradeYear = maxInt(tradeYear, impY)
	}
	if expOK {
		tradeVal += exp
		tradeYear = maxInt(tradeYear, expY)
	}
	comps = append(comps, makeComponent("trade_exposure", "trade exposure",
		w.Trade, tradeExposureScore(tradeVal), tradeYear, tradeAvail))

	comps = append(comps, makeComponent("manufacturing_dependency", "manufacturing dependency",
		w.Manufacturing, manufacturingDependencyScore(manuf), manY, manOK))

	comps = append(comps, makeComponent("inflation_stress", "inflation stress",
		w.Inflation, inflationStressScore(infl), inflY, inflOK))

	// High-tech concentration is a ratio, so it needs both high-tech and GDP.
	htAvail := htOK && gdpOK
	htScore, htYear := 0.0, 0
	if htAvail {
		htScore = highTechConcentrationScore(ht, gdp)
		htYear = maxInt(htY, gdpY)
	}
	comps = append(comps, makeComponent("high_tech_concentration", "high-tech concentration",
		w.HighTech, htScore, htYear, htAvail))

	bufScore, bufYear := 0.0, 0
	if gdpOK {
		bufScore = economicBufferRiskScore(gdp)
		bufYear = gdpY
	}
	comps = append(comps, makeComponent("economic_buffer_risk", "low economic buffer",
		w.BufferRisk, bufScore, bufYear, gdpOK))

	// Blend available components, renormalising the weights so missing data does
	// not artificially deflate the score.
	num, den := 0.0, 0.0
	for _, c := range comps {
		if c.Available {
			num += c.Weight * c.Score
			den += c.Weight
		}
	}
	final := 0.0
	if den > 0 {
		final = num / den
	}

	year := target
	if year <= 0 {
		for _, c := range comps {
			if c.Available && c.YearUsed > year {
				year = c.YearUsed
			}
		}
	}

	return CountryScore{
		CountryCode: code,
		CountryName: name,
		Year:        year,
		Score:       final,
		RiskLevel:   RiskLevel(final),
		Components:  comps,
		TopDrivers:  topDrivers(comps, 2),
	}
}

func makeComponent(key, name string, weight, score float64, year int, available bool) Component {
	c := Component{Key: key, Name: name, Weight: weight, YearUsed: year, Available: available}
	if available {
		c.Score = score
		c.Contribution = weight * score
	}
	return c
}

// topDrivers returns the names of the n available components contributing the
// most to the score (largest weight × score first).
func topDrivers(comps []Component, n int) []string {
	avail := make([]Component, 0, len(comps))
	for _, c := range comps {
		if c.Available && c.Contribution > 0 {
			avail = append(avail, c)
		}
	}
	sort.SliceStable(avail, func(i, j int) bool {
		if avail[i].Contribution != avail[j].Contribution {
			return avail[i].Contribution > avail[j].Contribution
		}
		return avail[i].Name < avail[j].Name
	})
	if len(avail) > n {
		avail = avail[:n]
	}
	out := make([]string, 0, len(avail))
	for _, c := range avail {
		out = append(out, c.Name)
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
