package eventrisk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atlasgraph/atlas/internal/scoring/events"
)

func TestLoadSampleCSV(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "examples", "gdelt_events_sample.csv")
	res, err := LoadCSV(path)
	if err != nil {
		t.Fatalf("LoadCSV: %v", err)
	}
	if len(res.Events) != 8 {
		t.Fatalf("events = %d, want 8 (unknown country skipped)", len(res.Events))
	}
	for _, w := range res.Warnings {
		if !strings.Contains(w, "Unknownistan") {
			continue
		}
		return
	}
	t.Fatal("expected warning for Unknownistan")
}

func TestIngestFromFileWritesRiskFile(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "examples", "gdelt_events_sample.csv")
	file, warnings, err := IngestFromFile(path, SourceName)
	if err != nil {
		t.Fatalf("IngestFromFile: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected at least one warning for unknown country")
	}
	if len(file.Countries) == 0 {
		t.Fatal("expected country risk rows")
	}

	dir := t.TempDir()
	out, err := Save(dir, file)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.EventCount != file.EventCount {
		t.Fatalf("event_count = %d, want %d", loaded.EventCount, file.EventCount)
	}
	if filepath.Base(out) != OutputFileName {
		t.Fatalf("output file = %q, want %q", filepath.Base(out), OutputFileName)
	}
}

func TestScoreEventsRecencyAndRiskTypes(t *testing.T) {
	now := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	events := []NormalizedEvent{
		{Country: "Ukraine", Date: "2026-05-01", EventType: "conflict", Severity: 0.9, Tone: -8},
		{Country: "Ukraine", Date: "2026-04-01", EventType: "economic", Severity: 0.2, Tone: 1},
		{Country: "United States", Date: "2026-05-02", EventType: "economic", Severity: 0.2, Tone: 1},
	}
	scores := ScoreEvents(events, now)
	if len(scores) != 2 {
		t.Fatalf("scores = %d, want 2", len(scores))
	}
	if scores[0].Country != "Ukraine" {
		t.Fatalf("top country = %q, want Ukraine", scores[0].Country)
	}
	if scores[0].RecentEventCount != 1 {
		t.Fatalf("recent_event_count = %d, want 1", scores[0].RecentEventCount)
	}
	if scores[0].EventRiskScore <= scores[1].EventRiskScore {
		t.Fatalf("expected Ukraine to outrank United States: %.1f vs %.1f", scores[0].EventRiskScore, scores[1].EventRiskScore)
	}
	assertScoreScale0To100(t, scores[0])
}

func TestUkraineConflictNotLowRisk(t *testing.T) {
	now := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	scores := ScoreEvents([]NormalizedEvent{{
		Country: "Ukraine", Date: "2024-03-05", EventType: "conflict",
		Severity: 0.9, Tone: -8.2,
	}}, now)
	if len(scores) != 1 {
		t.Fatalf("scores = %d, want 1", len(scores))
	}
	s := scores[0]
	if s.EventRiskScore <= 30 {
		t.Fatalf("Ukraine event_risk_score = %.1f, want > 30", s.EventRiskScore)
	}
	if s.RiskLevel == "Low" {
		t.Fatalf("RiskLevel = %q, want above Low for severity 0.9 and tone -8.2", s.RiskLevel)
	}
}

func TestSevereNegativeEventMeaningfulScore(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	scores := ScoreEvents([]NormalizedEvent{{
		Country: "Ukraine", Date: "2026-05-20", EventType: "conflict",
		Severity: 0.95, Tone: -9.0,
	}}, now)
	if scores[0].EventRiskScore <= 30 {
		t.Fatalf("score = %.1f, want > 30 for severe recent event", scores[0].EventRiskScore)
	}
}

func TestComponentScoresAre0To100(t *testing.T) {
	now := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	scores := ScoreEvents([]NormalizedEvent{{
		Country: "Russia", Date: "2024-02-18", EventType: "energy disruption",
		Severity: 0.81, Tone: -7.1,
	}}, now)
	assertScoreScale0To100(t, scores[0])
	for _, c := range scores[0].Components {
		if c.Score < 0 || c.Score > 100 {
			t.Fatalf("component %q score = %.1f, want 0..100", c.Key, c.Score)
		}
		if c.Contribution < 0 || c.Contribution > 100 {
			t.Fatalf("component %q contribution = %.2f, want 0..100", c.Key, c.Contribution)
		}
	}
}

func TestToLegacyCountryScoresUsesStoredComponents(t *testing.T) {
	file := RiskFile{
		Countries: []CountryRisk{{
			Country: "Ukraine", CountryCode: "UKR", EventRiskScore: 80, RiskLevel: "Critical",
			EventCount: 3, RecentEventCount: 2, AverageTone: -5, TopEventTypes: []string{"conflict"},
			Components: []events.Component{
				{Key: "event_volume", Name: "event volume", Score: 70, Weight: 0.35, Contribution: 24.5},
				{Key: "negative_tone", Name: "negative tone", Score: 82, Weight: 0.30, Contribution: 24.6},
				{Key: "event_severity", Name: "event severity", Score: 90, Weight: 0.25, Contribution: 22.5},
				{Key: "strategic_relevance", Name: "strategic relevance", Score: 80, Weight: 0.10, Contribution: 8.0},
			},
		}},
	}
	legacy := ToLegacyCountryScores(file)
	if len(legacy) != 1 || legacy[0].CountryCode != "UKR" {
		t.Fatalf("legacy scores = %+v", legacy)
	}
	if legacy[0].Components[1].Score != 82 {
		t.Fatalf("negative_tone component = %.1f, want 82", legacy[0].Components[1].Score)
	}
}

func assertScoreScale0To100(t *testing.T, s CountryRisk) {
	t.Helper()
	if s.EventRiskScore < 0 || s.EventRiskScore > 100 {
		t.Fatalf("event_risk_score = %.1f, want 0..100", s.EventRiskScore)
	}
}

func TestParseJSONWrapped(t *testing.T) {
	b := []byte(`{"events":[{"country":"Ukraine","date":"2026-05-01","event_type":"conflict","severity":0.8,"tone":-5}]}`)
	res, err := ParseJSON(b)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if len(res.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(res.Events))
	}
}

func TestToLegacyCountryScores(t *testing.T) {
	file := RiskFile{
		Countries: []CountryRisk{{
			Country: "Ukraine", CountryCode: "UKR", EventRiskScore: 80, RiskLevel: "Critical",
			EventCount: 3, RecentEventCount: 2, AverageTone: -5, TopEventTypes: []string{"conflict"},
		}},
	}
	legacy := ToLegacyCountryScores(file)
	if len(legacy) != 1 || legacy[0].CountryCode != "UKR" {
		t.Fatalf("legacy scores = %+v", legacy)
	}
}

func TestRecentEventsForCountry(t *testing.T) {
	file := RiskFile{
		Events: []NormalizedEvent{
			{Country: "Ukraine", Date: "2026-05-01"},
			{Country: "Ukraine", Date: "2026-04-01"},
			{Country: "Russia", Date: "2026-05-02"},
		},
	}
	recent := RecentEventsForCountry(file, "Ukraine", 1)
	if len(recent) != 1 || recent[0].Date != "2026-05-01" {
		t.Fatalf("recent = %+v", recent)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := LoadCSV("does-not-exist.csv"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestIngestFromFileUnsupportedExt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := IngestFromFile(path, SourceName); err == nil {
		t.Fatal("expected unsupported extension error")
	}
}
