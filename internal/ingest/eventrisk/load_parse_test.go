package eventrisk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseRiskFileJSONStandard(t *testing.T) {
	b := []byte(`{
		"source": "GDELT",
		"date_from": "2026-01-01",
		"date_to": "2026-05-01",
		"countries": [{
			"country": "Ukraine",
			"event_risk_score": 78.4,
			"risk_level": "HIGH",
			"event_count": 42,
			"recent_event_count": 9,
			"average_tone": -5.7,
			"top_event_types": ["conflict", "sanctions"],
			"source": "GDELT"
		}]
	}`)
	file, err := ParseRiskFileJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Countries) != 1 || file.Countries[0].Country != "Ukraine" {
		t.Fatalf("countries = %+v", file.Countries)
	}
	if !IsRealProcessedEventRisk(file) {
		t.Fatal("expected real processed event risk")
	}
}

func TestParseRiskFileJSONRisksWrapper(t *testing.T) {
	b := []byte(`{
		"source": "GDELT",
		"risks": [{
			"country": "USA",
			"event_risk_score": 40,
			"risk_level": "Medium"
		}]
	}`)
	file, err := ParseRiskFileJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Countries) != 1 || file.Countries[0].Country != "United States" {
		t.Fatalf("countries = %+v", file.Countries)
	}
}

func TestParseRiskFileJSONTopLevelArray(t *testing.T) {
	b := []byte(`[{"country":"Russia","event_risk_score":65,"risk_level":"High"}]`)
	file, err := ParseRiskFileJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Countries) != 1 || file.Countries[0].Country != "Russia" {
		t.Fatalf("countries = %+v", file.Countries)
	}
}

func TestIndexCountryScoresAliases(t *testing.T) {
	file := RiskFile{
		Source: SourceName,
		Countries: []CountryRisk{{
			Country: "United States", EventRiskScore: 55,
		}},
	}
	idx := IndexCountryScores(file)
	for _, alias := range []string{"usa", "united states", "united states of america"} {
		if idx[alias] != 55 {
			t.Fatalf("alias %q score = %v, want 55", alias, idx[alias])
		}
	}
}

func TestLookupScoreNormalized(t *testing.T) {
	idx := IndexCountryScores(RiskFile{
		Source: SourceName,
		Countries: []CountryRisk{{
			Country: "Korea, Rep.", EventRiskScore: 70,
		}},
	})
	score, ok := LookupScore(idx, "South Korea")
	if !ok || score != 70 {
		t.Fatalf("LookupScore(South Korea) = %v, %v want 70, true", score, ok)
	}
}

func TestTryLoadProcessedFromDir(t *testing.T) {
	dir := t.TempDir()
	b, _ := json.Marshal(RiskFile{
		Source: SourceName,
		Countries: []CountryRisk{{
			Country: "Ukraine", EventRiskScore: 80, RiskLevel: "Critical",
		}},
	})
	if err := os.WriteFile(filepath.Join(dir, OutputFileName), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	file, ok := TryLoadProcessed(dir)
	if !ok || len(file.Countries) != 1 {
		t.Fatalf("TryLoadProcessed = %+v, %v", file, ok)
	}
}

func TestNormalizeCountryTurkey(t *testing.T) {
	for _, in := range []string{"Turkey", "Türkiye", "turkiye"} {
		got, _, ok := NormalizeCountry(in)
		if !ok || got != "Turkey" {
			t.Fatalf("NormalizeCountry(%q) = %q, %v", in, got, ok)
		}
	}
}
