package gdelt

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleFixture = `[
  {
    "country_code": "twn",
    "country_name": "Taiwan",
    "title": "Taiwan tightens semiconductor export controls",
    "url": "https://example.com/twn/1",
    "domain": "example.com",
    "published_at": "2026-06-24T08:30:00Z",
    "tone": -6.8,
    "language": "English",
    "themes": ["TECH"],
    "risk_terms_matched": ["semiconductor", "export controls"],
    "source": "GDELT fixture (synthetic demo data)"
  },
  {
    "country_code": "JPN",
    "title": "Japan steadies energy supply as commodity prices ease",
    "url": "https://example.com/jpn/1",
    "domain": "example.com",
    "published_at": "2026-06-23T06:00:00Z",
    "tone": 1.8,
    "language": "English"
  }
]`

func TestLoadFixtureNormalizes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(path, []byte(sampleFixture), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	recs, err := LoadFixture(path)
	if err != nil {
		t.Fatalf("LoadFixture error: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}

	twn := recs[0]
	if twn.CountryCode != "TWN" { // lower-cased input must upcase
		t.Errorf("country code = %q, want TWN", twn.CountryCode)
	}
	if twn.CountryName != "Taiwan" {
		t.Errorf("country name = %q, want Taiwan", twn.CountryName)
	}
	if !contains(twn.RiskTermsMatched, "semiconductor") || !contains(twn.RiskTermsMatched, "export controls") {
		t.Errorf("risk terms not preserved: %v", twn.RiskTermsMatched)
	}
	if twn.Source != FixtureSourceName {
		t.Errorf("source = %q, want %q", twn.Source, FixtureSourceName)
	}
	if twn.FetchedAt.IsZero() {
		t.Errorf("fetched_at should be stamped")
	}
	if twn.PublishedAt.IsZero() {
		t.Errorf("published_at should be parsed")
	}

	jpn := recs[1]
	// Name not provided: resolved from the code.
	if jpn.CountryName != "Japan" {
		t.Errorf("country name should resolve from code, got %q", jpn.CountryName)
	}
	// Themes omitted: must be non-nil for a stable schema.
	if jpn.Themes == nil {
		t.Errorf("themes should be non-nil")
	}
	// risk_terms_matched omitted: derived from the title (energy, commodity).
	if !contains(jpn.RiskTermsMatched, "energy") || !contains(jpn.RiskTermsMatched, "commodity") {
		t.Errorf("risk terms should be derived from title, got %v", jpn.RiskTermsMatched)
	}
}

func TestLoadFixtureMissingFile(t *testing.T) {
	if _, err := LoadFixture(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected error for missing fixture file")
	}
}

func TestLoadFixtureRequiresPath(t *testing.T) {
	if _, err := LoadFixture(""); err == nil {
		t.Error("expected error for empty fixture path")
	}
}

// TestLoadRepoFixture loads the committed demo fixture to guard its schema and
// confirm the documented offline demo file stays valid.
func TestLoadRepoFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "examples", "gdelt_events_sample.json")
	recs, err := LoadFixture(path)
	if err != nil {
		t.Fatalf("loading repo fixture: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("repo fixture should contain events")
	}
	codes := map[string]bool{}
	for _, r := range recs {
		codes[r.CountryCode] = true
	}
	for _, want := range []string{"TWN", "CHN", "USA", "DEU", "KOR", "JPN", "SAU", "COD"} {
		if !codes[want] {
			t.Errorf("repo fixture missing country %q", want)
		}
	}
}
