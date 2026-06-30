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

const cliFixtureJSON = `[
  {"country_code":"TWN","country_name":"Taiwan","title":"semiconductor export controls amid military tension","url":"https://example.com/twn/1","domain":"example.com","published_at":"2026-06-24T08:30:00Z","tone":-6.8,"language":"English","themes":["TECH"],"risk_terms_matched":["semiconductor","export controls","military"],"source":"GDELT fixture (synthetic demo data)"},
  {"country_code":"JPN","country_name":"Japan","title":"calm trade talks continue","url":"https://example.com/jpn/1","domain":"example.com","published_at":"2026-06-19T09:30:00Z","tone":2.5,"language":"English","themes":[],"risk_terms_matched":[],"source":"GDELT fixture (synthetic demo data)"}
]`

func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.json")
	if err := os.WriteFile(path, []byte(cliFixtureJSON), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

// TestIngestGDELTFixtureMode exercises offline fixture ingestion end-to-end: it
// must save the normalised file, print a FIXTURE MODE report, and feed the
// events risk command.
func TestIngestGDELTFixtureMode(t *testing.T) {
	fixture := writeFixture(t)
	outDir := t.TempDir()

	out, _, code := run("ingest", "gdelt", "--fixture", fixture, "--out", outDir)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"GDELT EVENT INGESTION — FIXTURE MODE", "Source fixture", "Output",
		"Records loaded", "Countries", "Records with risk terms",
		"Top countries by event count", "Top matched risk terms", "synthetic",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fixture report missing %q\n---\n%s", want, out)
		}
	}
	// Must NOT look like a live pull.
	if strings.Contains(out, "Records fetched") {
		t.Errorf("fixture report should not claim live ingestion\n---\n%s", out)
	}

	// The saved file must load with the same schema and feed events risk.
	file, err := gdelt.Load(outDir)
	if err != nil {
		t.Fatalf("loading fixture output: %v", err)
	}
	if len(file.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(file.Records))
	}

	riskOut, _, riskCode := run("events", "risk", "--data", outDir)
	if riskCode != 0 {
		t.Fatalf("events risk exit = %d, want 0", riskCode)
	}
	if !strings.Contains(riskOut, "EVENT RISK SCORES") || !strings.Contains(riskOut, "Taiwan") {
		t.Errorf("events risk did not render from fixture data\n---\n%s", riskOut)
	}

	// JSON form must also work on fixture output.
	jsonOut, _, jsonCode := run("events", "risk", "--data", outDir, "--output", "json")
	if jsonCode != 0 {
		t.Fatalf("events risk json exit = %d, want 0", jsonCode)
	}
	if !strings.Contains(jsonOut, "\"scores\"") {
		t.Errorf("events risk json missing scores\n---\n%s", jsonOut)
	}
}

// TestIngestGDELTAllLiveFail confirms a permanently rate-limited host yields the
// helpful all-countries-failed message and a non-zero exit.
func TestIngestGDELTAllLiveFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Please limit requests to one every 5 seconds", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, errOut, code := run("ingest", "gdelt", "--countries", "TWN,CHN", "--out", t.TempDir(), "--base-url", srv.URL)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "Live GDELT ingestion failed for all countries") {
		t.Errorf("expected all-fail helpful message, got %q", errOut)
	}
	if !strings.Contains(errOut, "--fixture data/examples/gdelt_events_sample.json") {
		t.Errorf("expected fixture hint in failure message, got %q", errOut)
	}
}

// TestIngestGDELTPartialSuccess confirms that when some countries succeed and
// others fail, records are saved and the command still exits 0.
func TestIngestGDELTPartialSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("query"), "Taiwan") {
			w.Write([]byte(`{"articles":[{"url":"https://example.com/a","title":"semiconductor sanctions","seendate":"20240115T103000Z","domain":"example.com","language":"English","sourcecountry":"United States"}]}`))
			return
		}
		http.Error(w, "Please limit requests to one every 5 seconds", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	outDir := t.TempDir()
	out, _, code := run("ingest", "gdelt", "--countries", "TWN,CHN", "--out", outDir, "--base-url", srv.URL)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (partial success)", code)
	}
	if !strings.Contains(out, "Countries succeeded    : TWN") {
		t.Errorf("expected TWN reported as succeeded\n---\n%s", out)
	}
	if !strings.Contains(out, "Countries failed       : CHN") {
		t.Errorf("expected CHN reported as failed\n---\n%s", out)
	}
	file, err := gdelt.Load(outDir)
	if err != nil {
		t.Fatalf("loading partial output: %v", err)
	}
	if len(file.Records) == 0 {
		t.Errorf("expected partial records to be saved")
	}
}

// TestIngestGDELTLimitFlag confirms --limit is forwarded as the GDELT
// maxrecords parameter.
func TestIngestGDELTLimitFlag(t *testing.T) {
	var gotMax string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMax = r.URL.Query().Get("maxrecords")
		w.Write([]byte(`{"articles":[]}`))
	}))
	defer srv.Close()

	_, _, code := run("ingest", "gdelt", "--countries", "TWN", "--limit", "25", "--out", t.TempDir(), "--base-url", srv.URL)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if gotMax != "25" {
		t.Errorf("expected maxrecords=25, got %q", gotMax)
	}
}

// TestIngestGDELTLiveReportFields confirms the live report prints all the
// documented fields.
func TestIngestGDELTLiveReportFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"articles":[{"url":"https://example.com/a","title":"semiconductor sanctions","seendate":"20240115T103000Z","domain":"example.com","language":"English","sourcecountry":"United States"}]}`))
	}))
	defer srv.Close()

	out, _, code := run("ingest", "gdelt", "--countries", "TWN", "--days", "7", "--limit", "25", "--delay-seconds", "6", "--out", t.TempDir(), "--base-url", srv.URL)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"Countries requested", "Days", "Limit per country", "Delay seconds",
		"Countries succeeded", "Countries failed", "Records fetched",
		"Records with risk terms", "Output",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("live report missing %q\n---\n%s", want, out)
		}
	}
}

// TestIngestGDELTDelayClamped confirms a sub-5 --delay-seconds is clamped up to
// the 5s minimum in the live report.
func TestIngestGDELTDelayClamped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"articles":[]}`))
	}))
	defer srv.Close()

	out, _, code := run("ingest", "gdelt", "--countries", "TWN", "--delay-seconds", "1", "--out", t.TempDir(), "--base-url", srv.URL)
	// A single country with zero records still succeeds (the country responded).
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Delay seconds          : 5") {
		t.Errorf("expected delay clamped to 5, got\n---\n%s", out)
	}
}
