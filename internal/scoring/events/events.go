// Package events turns ingested GDELT news/event records into an explainable
// country-level event-risk score.
//
// This is a public-signal risk layer, not ground truth: it measures how much
// recent global news/event activity around a country is volume-heavy,
// negative-toned and concentrated on geopolitical / supply-chain risk themes.
// Like the macro scorer it is deliberately transparent — every score decomposes
// into weighted, individually-named components — so it can later combine with
// macro exposure and trade concentration into a unified fragility score.
package events

import (
	"sort"

	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
)

// Weights controls how component scores combine into the final score. They sum
// to 1.0, so the weighted result is already on a 0..100 scale.
type Weights struct {
	EventCount      float64 `json:"event_count"`
	NegativeTone    float64 `json:"negative_tone"`
	RiskTermDensity float64 `json:"risk_term_density"`
}

// DefaultWeights is the calibrated weighting used by the CLI.
func DefaultWeights() Weights {
	return Weights{
		EventCount:      0.45,
		NegativeTone:    0.35,
		RiskTermDensity: 0.20,
	}
}

// Component is one contributor to a country's event-risk score.
type Component struct {
	Key          string  // stable identifier, e.g. "event_count"
	Name         string  // human-friendly label, e.g. "event volume"
	Score        float64 // 0..100
	Weight       float64 // weight applied in the blend
	Contribution float64 // Weight * Score
}

// CountryScore is the full, explainable event-risk result for one country.
type CountryScore struct {
	CountryCode string
	CountryName string
	Events      int
	AvgTone     float64
	Score       float64 // 0..100 event risk
	RiskLevel   string
	Components  []Component
	TopDrivers  []string // friendly names of the largest contributors
	TopTerms    []string // most frequent matched risk terms
}

// ScoreCountries computes an event-risk score for every country present in the
// ingested file. Results are sorted by score, highest first (tie-break on name).
func ScoreCountries(file gdelt.EventFile, w Weights) []CountryScore {
	type group struct {
		name string
		recs []gdelt.GDELTEventRecord
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
		out = append(out, scoreCountry(code, g.name, g.recs, w))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].CountryName < out[j].CountryName
	})
	return out
}

func scoreCountry(code, name string, recs []gdelt.GDELTEventRecord, w Weights) CountryScore {
	count := len(recs)

	var toneSum float64
	var matchedTermTotal int
	termCounts := map[string]int{}
	for _, r := range recs {
		toneSum += r.Tone
		matchedTermTotal += len(r.RiskTermsMatched)
		for _, t := range r.RiskTermsMatched {
			termCounts[t]++
		}
	}
	avgTone := 0.0
	avgTermsPerEvent := 0.0
	if count > 0 {
		avgTone = toneSum / float64(count)
		avgTermsPerEvent = float64(matchedTermTotal) / float64(count)
	}

	comps := []Component{
		makeComponent("event_count", "event volume", w.EventCount, eventCountScore(count)),
		makeComponent("negative_tone", "negative news tone", w.NegativeTone, negativeToneScore(avgTone)),
		makeComponent("risk_term_density", "risk-term density", w.RiskTermDensity, riskTermDensityScore(avgTermsPerEvent)),
	}

	final := 0.0
	for _, c := range comps {
		final += c.Contribution
	}

	return CountryScore{
		CountryCode: code,
		CountryName: name,
		Events:      count,
		AvgTone:     avgTone,
		Score:       final,
		RiskLevel:   RiskLevel(final),
		Components:  comps,
		TopDrivers:  topDrivers(comps, 2),
		TopTerms:    topTerms(termCounts, 3),
	}
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

// topDrivers returns the names of the n components contributing the most to the
// score (largest weight × score first).
func topDrivers(comps []Component, n int) []string {
	avail := make([]Component, 0, len(comps))
	for _, c := range comps {
		if c.Contribution > 0 {
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

// topTerms returns the n most frequent matched risk terms (ties broken
// alphabetically).
func topTerms(counts map[string]int, n int) []string {
	type tc struct {
		term  string
		count int
	}
	items := make([]tc, 0, len(counts))
	for term, count := range counts {
		items = append(items, tc{term, count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].term < items[j].term
	})
	if len(items) > n {
		items = items[:n]
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.term)
	}
	return out
}
