package eventrisk

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/atlasgraph/atlas/internal/scoring/events"
)

const recentWindowDays = 30

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
	var impactSum float64
	var toneSum float64
	recent := 0
	typeCounts := map[string]int{}

	for _, e := range evs {
		days := daysSince(e.Date, now)
		if days >= 0 && days <= recentWindowDays {
			recent++
		}
		impactSum += eventImpact(e, days)
		toneSum += e.Tone
		typeCounts[e.EventType]++
	}

	count := len(evs)
	avgTone := 0.0
	if count > 0 {
		avgTone = toneSum / float64(count)
	}

	score := math.Min(100, impactSum*14+float64(recent)*1.8+float64(count)*0.35)
	return CountryRisk{
		Country:          country,
		CountryCode:      ISO3ForCountry(country),
		EventRiskScore:   round1(score),
		RiskLevel:        events.RiskLevel(score),
		EventCount:       count,
		RecentEventCount: recent,
		AverageTone:      round2(avgTone),
		TopEventTypes:    topEventTypes(typeCounts, 3),
		Source:           SourceName,
	}
}

func eventImpact(e NormalizedEvent, daysAgo int) float64 {
	if daysAgo < 0 {
		daysAgo = 0
	}
	recency := math.Exp(-float64(daysAgo) / 30.0)

	tone := 0.0
	if e.Tone < 0 {
		tone = math.Min(-e.Tone/10.0, 1.0)
	}
	sev := clamp01(e.Severity)
	typeMult := eventTypeWeight(e.EventType)
	base := 0.35*sev + 0.35*tone + 0.30*typeMult
	return recency * base * typeMult
}

func eventTypeWeight(eventType string) float64 {
	switch normalizeEventType(eventType) {
	case "conflict", "military", "security":
		return 1.0
	case "sanctions":
		return 0.95
	case "infrastructure disruption", "disruption", "shipping disruption":
		return 0.9
	case "protest", "strike":
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
	case strings.Contains(s, "protest"), strings.Contains(s, "demonstrat"):
		return "protest"
	case strings.Contains(s, "strike"):
		return "strike"
	case strings.Contains(s, "military"):
		return "military"
	case strings.Contains(s, "security"):
		return "security"
	case strings.Contains(s, "infrastructure"), strings.Contains(s, "disruption"), strings.Contains(s, "shipping"):
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
	if v > 1 && v <= 10 {
		return clamp01(v / 10.0)
	}
	if v > 10 && v <= 100 {
		return clamp01(v / 100.0)
	}
	return clamp01(v)
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
