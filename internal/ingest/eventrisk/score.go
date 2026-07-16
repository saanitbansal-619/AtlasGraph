package eventrisk

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/atlasgraph/atlas/internal/scoring/events"
)

const recentWindowDays = 30

// Component weights for processed GDELT event risk. Sum to 1.0.
const (
	weightEventVolume         = 0.35
	weightNegativeTone        = 0.30
	weightSeverity            = 0.25
	weightStrategicRelevance  = 0.10
)

// Absolute log-volume reference used when the panel has too few countries for a
// stable percentile (avoids every singleton scoring 100).
const volumeReferenceCount = 16.0

// ScoreEvents computes country-level event risk from normalized events.
func ScoreEvents(records []NormalizedEvent, now time.Time) []CountryRisk {
	type group struct {
		events []NormalizedEvent
	}
	byCountry := map[string]*group{}
	var order []string
	for _, r := range records {
		g, ok := byCountry[r.Country]
		if !ok {
			g = &group{}
			byCountry[r.Country] = g
			order = append(order, r.Country)
		}
		g.events = append(g.events, r)
	}

	volumes := make([]float64, 0, len(order))
	stats := make(map[string]countryStats, len(order))
	for _, country := range order {
		st := computeCountryStats(byCountry[country].events, now)
		stats[country] = st
		volumes = append(volumes, st.volume)
	}
	p90 := percentile(volumes, 0.90)

	out := make([]CountryRisk, 0, len(order))
	for _, country := range order {
		out = append(out, scoreCountry(country, stats[country], p90, len(order)))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].EventRiskScore != out[j].EventRiskScore {
			return out[i].EventRiskScore > out[j].EventRiskScore
		}
		return out[i].Country < out[j].Country
	})
	return out
}

type countryStats struct {
	events       []NormalizedEvent
	count        int
	recent       int
	mentions     int
	volume       float64
	avgTone      float64
	typeCounts   map[string]int
}

func computeCountryStats(evs []NormalizedEvent, now time.Time) countryStats {
	st := countryStats{events: evs, typeCounts: map[string]int{}}
	var toneSum float64
	for _, e := range evs {
		days := daysSince(e.Date, now)
		if days >= 0 && days <= recentWindowDays {
			st.recent++
		}
		toneSum += e.Tone
		st.mentions += e.MentionCount
		st.typeCounts[e.EventType]++
	}
	st.count = len(evs)
	if st.count > 0 {
		st.avgTone = toneSum / float64(st.count)
	}
	// event_volume = event_count + log(1 + total_mentions)
	st.volume = float64(st.count) + math.Log1p(float64(st.mentions))
	return st
}

func scoreCountry(country string, st countryStats, volumeP90 float64, panelSize int) CountryRisk {
	volumeScore := eventVolumeScore(st.volume, volumeP90, panelSize)
	toneScore := negativeToneScore(st.avgTone)
	severityScore := eventSeverityScore(st.events)
	strategicScore := strategicRelevanceScore(st.events)

	comps := []events.Component{
		makeComponent("event_volume", "event volume", weightEventVolume, volumeScore),
		makeComponent("negative_tone", "negative tone", weightNegativeTone, toneScore),
		makeComponent("event_severity", "event severity", weightSeverity, severityScore),
		makeComponent("strategic_relevance", "strategic relevance", weightStrategicRelevance, strategicScore),
	}

	final := 0.0
	for _, c := range comps {
		final += c.Contribution
	}
	final = math.Min(100, final)

	return CountryRisk{
		Country:          country,
		CountryCode:      ISO3ForCountry(country),
		EventRiskScore:   round1(final),
		RiskLevel:        events.RiskLevel(final),
		EventCount:       st.count,
		RecentEventCount: st.recent,
		AverageTone:      round2(st.avgTone),
		TopEventTypes:    topEventTypes(st.typeCounts, 3),
		Source:           SourceName,
		Components:       comps,
	}
}

func makeComponent(key, name string, weight, score float64) events.Component {
	return events.Component{
		Key:          key,
		Name:         name,
		Score:        round1(score),
		Weight:       weight,
		Contribution: round2(weight * score),
	}
}

// eventVolumeScore maps log-scaled event volume to 0..100 using the panel's
// 90th-percentile volume (with an absolute reference fallback).
func eventVolumeScore(volume, p90 float64, panelSize int) float64 {
	scaled := math.Log1p(math.Max(0, volume))
	ref := math.Log1p(volumeReferenceCount)
	denom := math.Log1p(math.Max(0, p90))
	if panelSize < 2 || denom < 1e-9 {
		denom = ref
	} else if denom < ref*0.5 {
		// Tiny panels: blend percentile with absolute reference so a lone
		// moderate country does not automatically score 100.
		denom = (denom + ref) / 2
	}
	// Map p90 volume to ~90 so only clear outliers approach 100.
	return math.Min(100, 90*scaled/denom)
}

// negativeToneScore maps average GDELT tone to 0..100. Neutral/positive tone is 0;
// increasingly negative tone approaches 100 (tone -10 saturates).
func negativeToneScore(avgTone float64) float64 {
	if avgTone >= 0 {
		return 0
	}
	return math.Min(100, scaleLinear100(-avgTone, 0, 10))
}

// eventSeverityScore averages type-weighted severities across all events so a
// single extreme observation does not saturate the country score.
func eventSeverityScore(evs []NormalizedEvent) float64 {
	if len(evs) == 0 {
		return 0
	}
	sum := 0.0
	for _, e := range evs {
		sum += severityTo100(e.Severity) * eventTypeWeight(e.EventType)
	}
	return math.Min(100, sum/float64(len(evs)))
}

// strategicRelevanceScore rises with the share of strategically relevant event types.
func strategicRelevanceScore(evs []NormalizedEvent) float64 {
	if len(evs) == 0 {
		return 0
	}
	high := 0
	for _, e := range evs {
		if isStrategicEventType(e.EventType) {
			high++
		}
	}
	share := float64(high) / float64(len(evs))
	return math.Min(100, share*100)
}

func isStrategicEventType(eventType string) bool {
	switch normalizeEventType(eventType) {
	case "conflict", "military", "security", "sanctions", "export control",
		"shipping disruption", "port disruption", "energy disruption",
		"supply chain disruption", "infrastructure disruption":
		return true
	default:
		return false
	}
}

func severityTo100(v float64) float64 {
	return clamp01(normalizeSeverity(v)) * 100
}

func eventTypeWeight(eventType string) float64 {
	switch normalizeEventType(eventType) {
	case "conflict", "military", "security":
		return 1.0
	case "sanctions", "export control":
		return 0.95
	case "shipping disruption", "port disruption", "energy disruption", "supply chain disruption", "infrastructure disruption", "disruption":
		return 0.9
	case "protest", "strike":
		return 0.7
	case "political risk":
		return 0.55
	case "economic", "trade":
		return 0.45
	default:
		return 0.5
	}
}

func normalizeEventType(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	switch {
	case strings.Contains(s, "conflict"), strings.Contains(s, "war"), strings.Contains(s, "battle"):
		return "conflict"
	case strings.Contains(s, "sanction"):
		return "sanctions"
	case strings.Contains(s, "export control"):
		return "export control"
	case strings.Contains(s, "protest"), strings.Contains(s, "demonstrat"):
		return "protest"
	case strings.Contains(s, "strike"):
		return "strike"
	case strings.Contains(s, "military"):
		return "military"
	case strings.Contains(s, "security"):
		return "security"
	case strings.Contains(s, "shipping"):
		return "shipping disruption"
	case strings.Contains(s, "port"):
		return "port disruption"
	case strings.Contains(s, "energy"):
		return "energy disruption"
	case strings.Contains(s, "supply chain"):
		return "supply chain disruption"
	case strings.Contains(s, "political"):
		return "political risk"
	case strings.Contains(s, "infrastructure"), strings.Contains(s, "disruption"):
		return "infrastructure disruption"
	case strings.Contains(s, "economic"), strings.Contains(s, "trade"):
		return "economic"
	default:
		if s == "" {
			return "other"
		}
		return s
	}
}

func normalizeSeverity(v float64) float64 {
	// GDELT Goldstein scale is typically -10 (conflictual) to +10 (cooperative).
	if v < 0 && v >= -10 {
		return clamp01(-v / 10.0)
	}
	if v > 1 && v <= 10 {
		return clamp01(v / 10.0)
	}
	if v > 10 && v <= 100 {
		return clamp01(v / 100.0)
	}
	return clamp01(v)
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64{}, values...)
	sort.Float64s(sorted)
	if len(sorted) == 1 {
		return sorted[0]
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	w := idx - float64(lo)
	return sorted[lo]*(1-w) + sorted[hi]*w
}

func scaleLinear100(v, lo, hi float64) float64 {
	if hi <= lo {
		return 0
	}
	x := (v - lo) / (hi - lo)
	return clamp01(x) * 100
}

func topEventTypes(counts map[string]int, n int) []string {
	type row struct {
		t string
		c int
	}
	rows := make([]row, 0, len(counts))
	for t, c := range counts {
		rows = append(rows, row{t, c})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].c != rows[j].c {
			return rows[i].c > rows[j].c
		}
		return rows[i].t < rows[j].t
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.t)
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func daysSince(date string, now time.Time) int {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0
	}
	now = now.UTC()
	t = t.UTC()
	d := now.Sub(t)
	return int(d.Hours() / 24)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
