package events

import (
	"math"
	"testing"

	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
)

func rec(code, name string, tone float64, terms ...string) gdelt.GDELTEventRecord {
	return gdelt.GDELTEventRecord{
		CountryCode:      code,
		CountryName:      name,
		Tone:             tone,
		RiskTermsMatched: terms,
	}
}

func TestDefaultWeightsSumToOne(t *testing.T) {
	w := DefaultWeights()
	sum := w.EventCount + w.NegativeTone + w.RiskTermDensity
	if math.Abs(sum-1.0) > 1e-9 {
		t.Errorf("weights should sum to 1.0, got %v", sum)
	}
}

func TestComponentBands(t *testing.T) {
	// Event count: 0 -> 0, 20 (midpoint of 0..40) -> 50, 40+ -> 100.
	if got := eventCountScore(0); got != 0 {
		t.Errorf("eventCountScore(0) = %v, want 0", got)
	}
	if got := eventCountScore(20); math.Abs(got-50) > 1e-9 {
		t.Errorf("eventCountScore(20) = %v, want 50", got)
	}
	if got := eventCountScore(100); got != 100 {
		t.Errorf("eventCountScore(100) = %v, want 100 (clamped)", got)
	}
	// Negative tone: neutral/positive -> 0, -5 (midpoint of 0..10) -> 50, -10 -> 100.
	if got := negativeToneScore(2); got != 0 {
		t.Errorf("negativeToneScore(+2) = %v, want 0", got)
	}
	if got := negativeToneScore(-5); math.Abs(got-50) > 1e-9 {
		t.Errorf("negativeToneScore(-5) = %v, want 50", got)
	}
	if got := negativeToneScore(-10); got != 100 {
		t.Errorf("negativeToneScore(-10) = %v, want 100", got)
	}
	// Risk-term density: 1.5 distinct terms/event (midpoint of 0..3) -> 50.
	if got := riskTermDensityScore(1.5); math.Abs(got-50) > 1e-9 {
		t.Errorf("riskTermDensityScore(1.5) = %v, want 50", got)
	}
}

func TestRiskLevelBands(t *testing.T) {
	cases := map[float64]string{
		0: "Low", 29.9: "Low", 30: "Medium", 59.9: "Medium",
		60: "High", 79.9: "High", 80: "Critical", 100: "Critical",
	}
	for score, want := range cases {
		if got := RiskLevel(score); got != want {
			t.Errorf("RiskLevel(%v) = %q, want %q", score, got, want)
		}
	}
}

func TestScoreCountriesFormula(t *testing.T) {
	// One country, 20 events, avg tone -5, 1.5 distinct terms/event.
	// Each component should land at 50, so the blended score is 50.
	var recs []gdelt.GDELTEventRecord
	for i := 0; i < 20; i++ {
		// alternate 1 and 2 matched terms => average 1.5 terms/event.
		if i%2 == 0 {
			recs = append(recs, rec("TWN", "Taiwan", -5, "sanctions"))
		} else {
			recs = append(recs, rec("TWN", "Taiwan", -5, "sanctions", "semiconductor"))
		}
	}
	file := gdelt.EventFile{Records: recs}
	scores := ScoreCountries(file, DefaultWeights())
	if len(scores) != 1 {
		t.Fatalf("expected 1 country, got %d", len(scores))
	}
	s := scores[0]
	if s.Events != 20 {
		t.Errorf("Events = %d, want 20", s.Events)
	}
	if math.Abs(s.AvgTone-(-5)) > 1e-9 {
		t.Errorf("AvgTone = %v, want -5", s.AvgTone)
	}
	if math.Abs(s.Score-50) > 1e-6 {
		t.Errorf("Score = %v, want 50", s.Score)
	}
	if s.RiskLevel != "Medium" {
		t.Errorf("RiskLevel = %q, want Medium", s.RiskLevel)
	}
	if len(s.TopTerms) == 0 || s.TopTerms[0] != "sanctions" {
		t.Errorf("expected sanctions as top term, got %v", s.TopTerms)
	}
}

func TestScoreCountriesSortedDescending(t *testing.T) {
	file := gdelt.EventFile{Records: []gdelt.GDELTEventRecord{
		// Low-risk country: few events, positive tone, no terms.
		rec("JPN", "Japan", 3),
		// High-risk country: negative tone + dense risk terms.
		rec("TWN", "Taiwan", -8, "sanctions", "conflict", "semiconductor"),
		rec("TWN", "Taiwan", -8, "sanctions", "conflict", "military"),
	}}
	scores := ScoreCountries(file, DefaultWeights())
	if len(scores) != 2 {
		t.Fatalf("expected 2 countries, got %d", len(scores))
	}
	if scores[0].CountryName != "Taiwan" {
		t.Errorf("expected Taiwan ranked first, got %q", scores[0].CountryName)
	}
	if scores[0].Score <= scores[1].Score {
		t.Errorf("scores not sorted descending: %v", scores)
	}
}

func TestScoreCountriesEmpty(t *testing.T) {
	scores := ScoreCountries(gdelt.EventFile{}, DefaultWeights())
	if len(scores) != 0 {
		t.Errorf("expected no scores for empty file, got %d", len(scores))
	}
}
