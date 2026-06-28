package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
)

// seedGDELTFile writes a small GDELT event panel to a temp dir so the events
// risk command has data to score, without any network access.
func seedGDELTFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ev := func(code, name string, tone float64, terms ...string) gdelt.GDELTEventRecord {
		return gdelt.GDELTEventRecord{
			CountryCode: code, CountryName: name, Tone: tone,
			Themes: []string{}, RiskTermsMatched: terms,
			Source: gdelt.SourceName, PublishedAt: time.Now().UTC(),
		}
	}
	file := gdelt.EventFile{
		Source:    gdelt.SourceName,
		FetchedAt: time.Now().UTC(),
		Days:      7,
		Countries: []string{"TWN", "JPN"},
		Records: []gdelt.GDELTEventRecord{
			ev("TWN", "Taiwan", -8, "sanctions", "conflict", "semiconductor"),
			ev("TWN", "Taiwan", -7, "sanctions", "military"),
			ev("JPN", "Japan", 2, "energy"),
		},
	}
	if _, err := gdelt.Save(dir, file); err != nil {
		t.Fatalf("seeding gdelt file: %v", err)
	}
	return dir
}

func TestEventsRiskText(t *testing.T) {
	dir := seedGDELTFile(t)
	out, _, code := run("events", "risk", "--data", dir)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"EVENT RISK SCORES", "COUNTRY", "EVENTS", "AVG TONE", "SCORE", "RISK", "TOP TERMS", "Taiwan", "Japan", "Risk bands"} {
		if !strings.Contains(out, want) {
			t.Errorf("events risk output missing %q\n---\n%s", want, out)
		}
	}
	// Taiwan (negative tone + dense risk terms) should outrank Japan.
	if strings.Index(out, "Taiwan") > strings.Index(out, "Japan") {
		t.Errorf("expected Taiwan ranked above Japan\n---\n%s", out)
	}
}

func TestEventsRiskJSON(t *testing.T) {
	dir := seedGDELTFile(t)
	out, _, code := run("events", "risk", "--data", dir, "--output", "json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, key := range []string{"weights", "risk_bands", "scores"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("event JSON missing %q", key)
		}
	}
	var scores []map[string]json.RawMessage
	if err := json.Unmarshal(parsed["scores"], &scores); err != nil {
		t.Fatalf("scores is not an array: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("expected 2 country scores, got %d", len(scores))
	}
	for _, key := range []string{"country_code", "events", "avg_tone", "event_risk_score", "risk_level", "components", "top_drivers", "top_terms"} {
		if _, ok := scores[0][key]; !ok {
			t.Errorf("country score missing %q", key)
		}
	}
}

func TestEventsRiskSave(t *testing.T) {
	dir := seedGDELTFile(t)
	path := filepath.Join(t.TempDir(), "nested", "event_scores.json")
	out, _, code := run("events", "risk", "--data", dir, "--save", path)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Saved event risk scores") {
		t.Errorf("expected save confirmation, got %q", out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("saved file not readable: %v", err)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if _, ok := parsed["scores"]; !ok {
		t.Errorf("saved JSON missing scores")
	}
}

func TestEventsRiskMissingData(t *testing.T) {
	_, errOut, code := run("events", "risk", "--data", t.TempDir())
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "reading") {
		t.Errorf("expected a read error, got %q", errOut)
	}
}

func TestEventsRiskInvalidOutput(t *testing.T) {
	dir := seedGDELTFile(t)
	_, errOut, code := run("events", "risk", "--data", dir, "--output", "yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "invalid --output") {
		t.Errorf("expected invalid output error, got %q", errOut)
	}
}

func TestEventsUnknownSubcommand(t *testing.T) {
	_, errOut, code := run("events", "trends")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "unknown events subcommand") {
		t.Errorf("expected unknown subcommand error, got %q", errOut)
	}
}

// TestIngestGDELTViaFixtureServer exercises the full ingest path against a local
// httptest server standing in for GDELT — never the live API.
func TestIngestGDELTViaFixtureServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"articles":[
			{"url":"https://x/1","title":"sanctions and conflict over semiconductor exports","seendate":"20240115T103000Z","domain":"x.com","language":"English","sourcecountry":"United States"},
			{"url":"https://x/2","title":"calm trade talks continue","seendate":"20240114T090000Z","domain":"y.com","language":"English","sourcecountry":"Japan"}
		]}`))
	}))
	defer srv.Close()

	outDir := t.TempDir()
	out, _, code := run("ingest", "gdelt", "--countries", "TWN,CHN", "--days", "7", "--out", outDir, "--base-url", srv.URL)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"GDELT EVENT INGESTION", "Countries", "Days", "Records fetched", "Records with risk terms", "Top countries by event count", "Top matched risk terms"} {
		if !strings.Contains(out, want) {
			t.Errorf("ingest report missing %q\n---\n%s", want, out)
		}
	}

	// The saved file must load and feed the events risk command.
	file, err := gdelt.Load(outDir)
	if err != nil {
		t.Fatalf("loading ingested file: %v", err)
	}
	if len(file.Records) != 4 { // 2 countries × 2 articles
		t.Fatalf("expected 4 records, got %d", len(file.Records))
	}

	riskOut, _, riskCode := run("events", "risk", "--data", outDir)
	if riskCode != 0 {
		t.Fatalf("events risk exit code = %d, want 0", riskCode)
	}
	if !strings.Contains(riskOut, "EVENT RISK SCORES") {
		t.Errorf("events risk did not render from ingested data\n---\n%s", riskOut)
	}
}

func TestIngestGDELTRequiresCountries(t *testing.T) {
	_, errOut, code := run("ingest", "gdelt", "--out", t.TempDir())
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "required") {
		t.Errorf("expected required error, got %q", errOut)
	}
}
