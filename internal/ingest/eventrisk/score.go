package eventrisk

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/atlasgraph/atlas/internal/scoring/events"
)

const recentWindowDays = 30

// Component weights for processed GDELT event risk. Sum to 1.0 so the blended
// score stays on a 0..100 scale.
const (
	weightRecentEvents = 0.40
	weightNegativeTone = 0.35
	weightSeverity     = 0.25
)

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

	out := make([]CountryRisk, 0, len(order))
	for _, country := range order {
		out = append(out, scoreCountry(country, byCountry[country].events, now))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].EventRiskScore != out[j].EventRiskScore {
			return out[i].EventRiskScore > out[j].EventRiskScore
		}
		return out[i].Country < out[j].Country
	})
	return out
}

func scoreCountry(country string, evs []NormalizedEvent, now time.Time) CountryRisk {
	recent := 0
	var toneSum float64
	typeCounts := map[string]int{}

	for _, e := range evs {
		days := daysSince(e.Date, now)
		if days >= 0 && days <= recentWindowDays {
			recent++
		}
		toneSum += e.Tone
		typeCounts[e.EventType]++
	}

	count := len(evs)
	avgTone := 0.0
	if count > 0 {
		avgTone = toneSum / float64(count)
	}

	recentScore := recentEventsScore(recent, count)
	toneScore := negativeToneScore(avgTone)
	severityScore := eventSeverityScore(evs)

	comps := []events.Component{
		makeComponent("recent_events", "recent events", weightRecentEvents, recentScore),
		makeComponent("negative_tone", "negative tone", weightNegativeTone, toneScore),
		makeComponent("event_severity", "event severity", weightSeverity, severityScore),
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
		EventCount:       count,
		RecentEventCount: recent,
		AverageTone:      round2(avgTone),
		TopEventTypes:    topEventTypes(typeCounts, 3),
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

// recentEventsScore maps recent-window activity and overall event volume to 0..100.
func recentEventsScore(recent, total int) float64 {
	recentPart := scaleLinear100(float64(recent), 0, 3)
	volumePart := scaleLinear100(float64(total), 0, 5)
	if recentPart > volumePart {
		return recentPart
	}
	return volumePart
}

// negativeToneScore maps average GDELT tone to 0..100. Neutral/positive tone is 0;
// increasingly negative tone approaches 100 (tone -10 saturates).
func negativeToneScore(avgTone float64) float64 {
	if avgTone >= 0 {
		return 0
	}
	return math.Min(100, scaleLinear100(-avgTone, 0, 10))
}

// eventSeverityScore maps CSV severity (0..1) and event type to 0..100 using the
// highest-risk event in the country group.
func eventSeverityScore(evs []NormalizedEvent) float64 {
	max := 0.0
	for _, e := range evs {
		sev := severityTo100(e.Severity) * eventTypeWeight(e.EventType)
		if sev > max {
			max = sev
		}
	}
	return math.Min(100, max)
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
	case "protest", "strike", "political risk":
		return 0.85
	case "economic", "trade":
		return 0.5
	default:
		return 0.6
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
	case strings.Contains(s, "export control"), strings.Contains(s, "export_control"):
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
	case strings.Contains(s, "supply chain"), strings.Contains(s, "supply_chain"):
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
	// Map conflictual (negative) values to higher severity.
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
