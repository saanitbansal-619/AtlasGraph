package gdelt

import (
	"testing"
	"time"
)

func TestCountryNameMapping(t *testing.T) {
	cases := map[string]string{
		"TWN": "Taiwan",
		"CHN": "China",
		"JPN": "Japan",
		"KOR": "South Korea",
		"USA": "United States",
		"DEU": "Germany",
		"SAU": "Saudi Arabia",
		"COD": "Democratic Republic of the Congo",
		"IND": "India",
	}
	for code, want := range cases {
		got, ok := CountryName(code)
		if !ok || got != want {
			t.Errorf("CountryName(%q) = %q,%v; want %q,true", code, got, ok, want)
		}
		// case-insensitive
		if lower, ok := CountryName(toLower(code)); !ok || lower != want {
			t.Errorf("CountryName(%q) lower-case lookup failed: %q,%v", code, lower, ok)
		}
	}
	if _, ok := CountryName("ZZZ"); ok {
		t.Error("expected unknown code to report ok=false")
	}
}

func TestMatchRiskTerms(t *testing.T) {
	got := MatchRiskTerms("Taiwan tightens EXPORT CONTROLS as semiconductor sanctions loom")
	if !contains(got, "export controls") || !contains(got, "semiconductor") || !contains(got, "sanctions") {
		t.Errorf("expected export controls/semiconductor/sanctions, got %v", got)
	}
	// No match should return a non-nil empty slice (stable JSON schema).
	none := MatchRiskTerms("a calm and pleasant day in the markets")
	if none == nil {
		t.Error("MatchRiskTerms must never return nil")
	}
	if len(none) != 0 {
		t.Errorf("expected no matches, got %v", none)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	file := EventFile{
		Source:    SourceName,
		FetchedAt: time.Now().UTC().Truncate(time.Second),
		Days:      7,
		Countries: []string{"TWN", "CHN"},
		Records: []GDELTEventRecord{
			{
				CountryCode:      "TWN",
				CountryName:      "Taiwan",
				Title:            "sanctions news",
				URL:              "https://x/1",
				PublishedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				Tone:             -3.2,
				Themes:           []string{},
				RiskTermsMatched: []string{"sanctions"},
				Source:           SourceName,
			},
		},
	}
	path, err := Save(dir, file)
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if path == "" {
		t.Fatal("Save returned empty path")
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got.Days != 7 || len(got.Records) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	r := got.Records[0]
	if r.CountryCode != "TWN" || r.Tone != -3.2 || !contains(r.RiskTermsMatched, "sanctions") {
		t.Errorf("record not preserved: %+v", r)
	}
}

func TestSaveRequiresDir(t *testing.T) {
	if _, err := Save("", EventFile{}); err == nil {
		t.Error("expected error for empty output directory")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(t.TempDir())
	if err == nil {
		t.Error("expected error loading from an empty directory")
	}
}

func TestSortRecords(t *testing.T) {
	recs := []GDELTEventRecord{
		{CountryCode: "CHN", PublishedAt: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), Title: "b"},
		{CountryCode: "TWN", PublishedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Title: "a"},
		{CountryCode: "TWN", PublishedAt: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), Title: "z"},
	}
	SortRecords(recs)
	// Country code ascending, then newest first.
	if recs[0].CountryCode != "CHN" {
		t.Errorf("expected CHN first, got %s", recs[0].CountryCode)
	}
	if recs[1].CountryCode != "TWN" || !recs[1].PublishedAt.After(recs[2].PublishedAt) {
		t.Errorf("expected TWN sorted newest-first, got %+v", recs)
	}
}

func TestBuildSummary(t *testing.T) {
	file := EventFile{Records: []GDELTEventRecord{
		{CountryCode: "TWN", CountryName: "Taiwan", RiskTermsMatched: []string{"sanctions", "semiconductor"}},
		{CountryCode: "TWN", CountryName: "Taiwan", RiskTermsMatched: []string{"sanctions"}},
		{CountryCode: "CHN", CountryName: "China", RiskTermsMatched: []string{}},
	}}
	s := BuildSummary(file, 5)
	if s.Records != 3 {
		t.Errorf("Records = %d, want 3", s.Records)
	}
	if s.WithRiskTerms != 2 {
		t.Errorf("WithRiskTerms = %d, want 2", s.WithRiskTerms)
	}
	if len(s.TopCountries) == 0 || s.TopCountries[0].Name != "Taiwan" || s.TopCountries[0].Count != 2 {
		t.Errorf("expected Taiwan top with 2 events, got %+v", s.TopCountries)
	}
	if len(s.TopRiskTerms) == 0 || s.TopRiskTerms[0].Name != "sanctions" || s.TopRiskTerms[0].Count != 2 {
		t.Errorf("expected sanctions top term with 2, got %+v", s.TopRiskTerms)
	}
}

func toLower(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + ('a' - 'A')
		}
	}
	return string(out)
}
