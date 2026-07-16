package eventrisk

import (
	"testing"
	"time"
)

func TestSevereNegativeCountryScoresHigher(t *testing.T) {
	now := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)
	scores := ScoreEvents([]NormalizedEvent{
		{Country: "Ukraine", Date: "2024-11-01", EventType: "conflict", Severity: 0.95, Tone: -8.5, MentionCount: 80},
		{Country: "Ukraine", Date: "2024-10-15", EventType: "sanctions", Severity: 0.85, Tone: -7.0, MentionCount: 40},
		{Country: "Ukraine", Date: "2024-09-20", EventType: "energy disruption", Severity: 0.8, Tone: -6.5, MentionCount: 30},
		{Country: "Chile", Date: "2024-10-01", EventType: "political risk", Severity: 0.4, Tone: -1.5, MentionCount: 5},
		{Country: "Chile", Date: "2024-08-01", EventType: "protest", Severity: 0.35, Tone: -1.0, MentionCount: 4},
	}, now)
	if len(scores) != 2 {
		t.Fatalf("scores = %d", len(scores))
	}
	if scores[0].Country != "Ukraine" {
		t.Fatalf("top = %q, want Ukraine", scores[0].Country)
	}
	if scores[0].EventRiskScore <= scores[1].EventRiskScore {
		t.Fatalf("Ukraine %.1f should exceed Chile %.1f", scores[0].EventRiskScore, scores[1].EventRiskScore)
	}
	if scores[0].RiskLevel == "Low" {
		t.Fatalf("Ukraine risk = %q, want elevated", scores[0].RiskLevel)
	}
}

func TestModerateCountryScoresMedium(t *testing.T) {
	now := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)
	// Mix of countries so percentile normalization has room.
	var events []NormalizedEvent
	for i := 0; i < 18; i++ {
		events = append(events, NormalizedEvent{
			Country: "Ukraine", Date: "2024-06-01", EventType: "conflict",
			Severity: 0.9, Tone: -8.0, MentionCount: 50,
		})
	}
	for i := 0; i < 4; i++ {
		events = append(events, NormalizedEvent{
			Country: "Germany", Date: "2024-06-01", EventType: "political risk",
			Severity: 0.35, Tone: -2.0, MentionCount: 8,
		})
	}
	events = append(events, NormalizedEvent{
		Country: "Australia", Date: "2024-06-01", EventType: "economic",
		Severity: 0.2, Tone: 1.0, MentionCount: 2,
	})

	scores := ScoreEvents(events, now)
	var deu *CountryRisk
	for i := range scores {
		if scores[i].Country == "Germany" {
			deu = &scores[i]
			break
		}
	}
	if deu == nil {
		t.Fatal("Germany missing")
	}
	if deu.RiskLevel != "Medium" && deu.RiskLevel != "Low" {
		t.Fatalf("Germany risk = %q score=%.1f, want Medium or Low", deu.RiskLevel, deu.EventRiskScore)
	}
	if deu.EventRiskScore >= 60 {
		t.Fatalf("Germany score = %.1f, want < 60 for moderate panel", deu.EventRiskScore)
	}
}

func TestPositiveToneDoesNotInflateRisk(t *testing.T) {
	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	scores := ScoreEvents([]NormalizedEvent{
		{Country: "United States", Date: "2024-05-01", EventType: "economic", Severity: 0.2, Tone: 2.5, MentionCount: 10},
		{Country: "United States", Date: "2024-04-01", EventType: "political risk", Severity: 0.25, Tone: 1.0, MentionCount: 5},
	}, now)
	if len(scores) != 1 {
		t.Fatalf("scores = %d", len(scores))
	}
	for _, c := range scores[0].Components {
		if c.Key == "negative_tone" && c.Score != 0 {
			t.Fatalf("negative_tone = %.1f, want 0 for positive/neutral tone", c.Score)
		}
	}
	if scores[0].RiskLevel == "Critical" || scores[0].RiskLevel == "High" {
		t.Fatalf("positive-tone country risk = %q score=%.1f, should not be High/Critical", scores[0].RiskLevel, scores[0].EventRiskScore)
	}
}

func TestSingleCountryNoDivideByZero(t *testing.T) {
	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	scores := ScoreEvents([]NormalizedEvent{{
		Country: "Japan", Date: "2024-05-01", EventType: "export control",
		Severity: 0.6, Tone: -3.0, MentionCount: 12,
	}}, now)
	if len(scores) != 1 {
		t.Fatalf("scores = %d", len(scores))
	}
	s := scores[0]
	if s.EventRiskScore < 0 || s.EventRiskScore > 100 {
		t.Fatalf("score = %.1f out of range", s.EventRiskScore)
	}
	if len(s.Components) != 4 {
		t.Fatalf("components = %d, want 4", len(s.Components))
	}
}

func TestExpandedPanelNotAllHigh(t *testing.T) {
	path := "../../../data/raw/gdelt_events/gdelt_events_2024_expanded.csv"
	file, _, err := IngestFromFile(path, SourceName)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	highOrAbove := 0
	for _, c := range file.Countries {
		if c.EventRiskScore >= 60 {
			highOrAbove++
		}
	}
	if highOrAbove == len(file.Countries) {
		t.Fatalf("all %d countries scored High/Critical — scoring is not differentiated", len(file.Countries))
	}
	if len(file.Countries) < 2 {
		t.Fatal("expected multiple countries")
	}
	// Spread: max - min should be meaningful
	max, min := file.Countries[0].EventRiskScore, file.Countries[0].EventRiskScore
	for _, c := range file.Countries {
		if c.EventRiskScore > max {
			max = c.EventRiskScore
		}
		if c.EventRiskScore < min {
			min = c.EventRiskScore
		}
	}
	if max-min < 10 {
		t.Fatalf("score spread = %.1f (%.1f..%.1f), want more differentiation", max-min, min, max)
	}
}
