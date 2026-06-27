package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

func run(args ...string) (string, string, int) {
	var out, errOut bytes.Buffer
	code := Run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestShockCommandRendersReport(t *testing.T) {
	out, _, code := run("shock", "--source", "Taiwan", "--commodity", "semiconductors", "--drop", "30", "--depth", "3")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"SCENARIO", "DIRECT EXPOSURE", "AFFECTED DEPENDENCY PATHS", "United States", "GRAPH IMPACT SUMMARY"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestShockRequiresSourceAndCommodity(t *testing.T) {
	_, errOut, code := run("shock", "--drop", "30")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "required") {
		t.Errorf("expected a 'required' error, got %q", errOut)
	}
}

func TestShockUnknownSourceErrors(t *testing.T) {
	_, errOut, code := run("shock", "--source", "Atlantis", "--commodity", "semiconductors")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "unknown source") {
		t.Errorf("expected unknown source error, got %q", errOut)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, _, code := run("frobnicate")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestNoArgsShowsUsage(t *testing.T) {
	out, _, code := run()
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "AtlasGraph") {
		t.Errorf("expected usage banner, got %q", out)
	}
}

func TestScenarioList(t *testing.T) {
	out, _, code := run("scenario", "list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, id := range []string{"taiwan_semiconductor_shock", "suez_route_disruption", "lithium_price_spike", "crude_oil_supply_shock"} {
		if !strings.Contains(out, id) {
			t.Errorf("scenario list missing %q", id)
		}
	}
}

func TestScenarioRun(t *testing.T) {
	out, _, code := run("scenario", "run", "taiwan_semiconductor_shock")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "SCENARIO PRESET") || !strings.Contains(out, "Taiwan Semiconductor") {
		t.Errorf("scenario run output missing preset banner: %q", out)
	}
}

func TestScenarioRunUnknown(t *testing.T) {
	_, errOut, code := run("scenario", "run", "nope")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "unknown scenario") {
		t.Errorf("expected unknown scenario error, got %q", errOut)
	}
}

func TestGraphSummary(t *testing.T) {
	out, _, code := run("graph", "summary")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"GRAPH SUMMARY", "Total entities", "Commodities", "HIGHEST-DEGREE NODES"} {
		if !strings.Contains(out, want) {
			t.Errorf("graph summary missing %q", want)
		}
	}
}

func TestGraphPaths(t *testing.T) {
	out, _, code := run("graph", "paths", "--from", "Taiwan", "--to", "cloud infrastructure")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Taiwan -> semiconductors") {
		t.Errorf("expected a dependency path, got %q", out)
	}
}

func TestGraphPathsUnknownEntity(t *testing.T) {
	_, errOut, code := run("graph", "paths", "--from", "Nowhere", "--to", "Taiwan")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "unknown entity") {
		t.Errorf("expected unknown entity error, got %q", errOut)
	}
}

func TestRiskLeaderboard(t *testing.T) {
	out, _, code := run("risk", "leaderboard")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"RISK LEADERBOARD", "Countries", "Commodities", "Sectors"} {
		if !strings.Contains(out, want) {
			t.Errorf("risk leaderboard missing %q", want)
		}
	}
}

func TestShockJSONOutputShape(t *testing.T) {
	out, _, code := run("shock", "--source", "Taiwan", "--commodity", "semiconductors", "--drop", "30", "--depth", "3", "--output", "json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	required := []string{
		"scenario", "direct_exposure", "second_order_exposure", "affected_paths",
		"changed_fragility_scores", "highest_risk_entities", "graph_impact_summary",
	}
	for _, key := range required {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON output missing required key %q", key)
		}
	}
}

func TestScenarioRunExplain(t *testing.T) {
	out, _, code := run("scenario", "run", "taiwan_semiconductor_shock", "--explain")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"PROPAGATION LOGIC", "Shock type", "Allowed relationships", "export_collapse", "Blocked unrelated branches"} {
		if !strings.Contains(out, want) {
			t.Errorf("explain output missing %q", want)
		}
	}
	// The blocked unrelated commodities should be named.
	for _, c := range []string{"crude oil", "lithium", "cobalt"} {
		if !strings.Contains(out, c) {
			t.Errorf("explain output should mention blocked branch %q", c)
		}
	}
}

func TestAffectedPathsAreLabeled(t *testing.T) {
	out, _, code := run("shock", "--source", "Taiwan", "--commodity", "semiconductors", "--type", "export_collapse")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "--exports/semiconductors-->") {
		t.Errorf("expected labeled relationship hops in paths, got %q", out)
	}
}

func TestShockUnknownTypeErrors(t *testing.T) {
	_, errOut, code := run("shock", "--source", "Taiwan", "--commodity", "semiconductors", "--type", "meteor")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "unknown shock type") {
		t.Errorf("expected unknown shock type error, got %q", errOut)
	}
}

func TestShockJSONIncludesProfileAndRules(t *testing.T) {
	out, _, code := run("shock", "--source", "Taiwan", "--commodity", "semiconductors", "--output", "json", "--explain")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, key := range []string{"shock_profile", "propagation_rules_applied", "blocked_edges"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON output missing %q", key)
		}
	}
}

func TestIngestRequiresCountries(t *testing.T) {
	_, errOut, code := run("ingest", "worldbank")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "required") {
		t.Errorf("expected a 'required' error, got %q", errOut)
	}
}

func TestIngestUnknownSource(t *testing.T) {
	_, errOut, code := run("ingest", "imf")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "unknown ingest source") {
		t.Errorf("expected unknown source error, got %q", errOut)
	}
}

// seedIndicatorFile writes a small World Bank dataset to a temp dir for the
// indicators command tests.
func seedIndicatorFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gdp := 27360935000000.0
	cpi := 4.1
	file := worldbank.IndicatorFile{
		Source:    worldbank.SourceName,
		FetchedAt: time.Now().UTC(),
		StartYear: 2018,
		EndYear:   2023,
		Countries: []string{"USA"},
		Records: []worldbank.CountryIndicatorRecord{
			{CountryCode: "USA", CountryName: "United States", IndicatorCode: "NY.GDP.MKTP.CD", IndicatorName: "GDP (current US$)", Year: 2023, Value: &gdp, Source: worldbank.SourceName},
			{CountryCode: "USA", CountryName: "United States", IndicatorCode: "FP.CPI.TOTL.ZG", IndicatorName: "Inflation, consumer prices (annual %)", Year: 2023, Value: &cpi, Source: worldbank.SourceName},
		},
	}
	if _, err := worldbank.Save(dir, file); err != nil {
		t.Fatalf("seeding indicator file: %v", err)
	}
	return dir
}

func TestIndicatorsCountry(t *testing.T) {
	dir := seedIndicatorFile(t)
	out, _, code := run("indicators", "country", "USA", "--data", dir)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"COUNTRY INDICATORS", "United States (USA)", "2023", "GDP (current US$)", "27,360,935,000,000", "4.10%"} {
		if !strings.Contains(out, want) {
			t.Errorf("indicators output missing %q\n---\n%s", want, out)
		}
	}
}

func TestIndicatorsCountryUnknown(t *testing.T) {
	dir := seedIndicatorFile(t)
	_, errOut, code := run("indicators", "country", "BRA", "--data", dir)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "no data for country") {
		t.Errorf("expected no-data error, got %q", errOut)
	}
}

func TestIndicatorsCountryMissingFile(t *testing.T) {
	_, errOut, code := run("indicators", "country", "USA", "--data", t.TempDir())
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "reading") {
		t.Errorf("expected a read error, got %q", errOut)
	}
}

func TestIndicatorsCountryRequiresCode(t *testing.T) {
	_, errOut, code := run("indicators", "country")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "required") {
		t.Errorf("expected required error, got %q", errOut)
	}
}

// seedMacroFile writes a richer multi-country, multi-indicator dataset so the
// macro scorer has all components to work with.
func seedMacroFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	rec := func(code, name, ind string, year int, v float64) worldbank.CountryIndicatorRecord {
		val := v
		return worldbank.CountryIndicatorRecord{
			CountryCode: code, CountryName: name, IndicatorCode: ind,
			Year: year, Value: &val, Source: worldbank.SourceName,
		}
	}
	file := worldbank.IndicatorFile{
		Source: worldbank.SourceName, StartYear: 2018, EndYear: 2023,
		Countries: []string{"USA", "DEU"},
		Records: []worldbank.CountryIndicatorRecord{
			rec("USA", "United States", "NY.GDP.MKTP.CD", 2023, 27e12),
			rec("USA", "United States", "NE.IMP.GNFS.ZS", 2023, 14.1),
			rec("USA", "United States", "NE.EXP.GNFS.ZS", 2023, 11.2),
			rec("USA", "United States", "NV.IND.MANF.ZS", 2021, 10.7),
			rec("USA", "United States", "FP.CPI.TOTL.ZG", 2023, 4.1),
			rec("USA", "United States", "TX.VAL.TECH.CD", 2023, 208e9),
			rec("DEU", "Germany", "NY.GDP.MKTP.CD", 2023, 4.5e12),
			rec("DEU", "Germany", "NE.IMP.GNFS.ZS", 2023, 39.0),
			rec("DEU", "Germany", "NE.EXP.GNFS.ZS", 2023, 43.0),
			rec("DEU", "Germany", "NV.IND.MANF.ZS", 2023, 18.9),
			rec("DEU", "Germany", "FP.CPI.TOTL.ZG", 2023, 5.9),
			rec("DEU", "Germany", "TX.VAL.TECH.CD", 2023, 260e9),
		},
	}
	if _, err := worldbank.Save(dir, file); err != nil {
		t.Fatalf("seeding macro file: %v", err)
	}
	return dir
}

func TestScoreMacroText(t *testing.T) {
	dir := seedMacroFile(t)
	out, _, code := run("score", "macro", "--data", dir, "--year", "2023")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"MACRO EXPOSURE SCORES", "COUNTRY", "Germany", "United States", "Risk bands"} {
		if !strings.Contains(out, want) {
			t.Errorf("macro output missing %q\n---\n%s", want, out)
		}
	}
}

func TestScoreMacroExplainFormula(t *testing.T) {
	out, _, code := run("score", "macro", "--explain-formula")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// Score name and the weighted formula terms.
	for _, want := range []string{
		"Macro Exposure Score",
		"0.30 * trade_exposure_score",
		"0.25 * manufacturing_dependency_score",
		"0.20 * inflation_stress_score",
		"0.15 * high_tech_concentration_score",
		"0.10 * economic_buffer_risk_score",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain-formula output missing weight %q\n---\n%s", want, out)
		}
	}
	// Component definitions and risk bands.
	for _, want := range []string{
		"imports % GDP + exports % GDP exposure",
		"inverse GDP-size buffer risk",
		"Low      : 0-30",
		"Critical : 80-100",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain-formula output missing %q", want)
		}
	}
	// Limitation / disclaimer.
	for _, want := range []string{"not a prediction of recession", "Full AtlasGraph fragility"} {
		if !strings.Contains(out, want) {
			t.Errorf("explain-formula output missing disclaimer %q", want)
		}
	}
}

func TestScoreMacroVerbose(t *testing.T) {
	dir := seedMacroFile(t)
	out, _, code := run("score", "macro", "--data", dir, "--verbose")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"COMPONENT", "trade exposure", "manufacturing dependency", "CONTRIBUTION"} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose output missing %q", want)
		}
	}
}

func TestScoreMacroJSON(t *testing.T) {
	dir := seedMacroFile(t)
	out, _, code := run("score", "macro", "--data", dir, "--output", "json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, key := range []string{"year_lens", "weights", "risk_bands", "scores"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("macro JSON missing %q", key)
		}
	}
	var scores []map[string]json.RawMessage
	if err := json.Unmarshal(parsed["scores"], &scores); err != nil {
		t.Fatalf("scores is not an array: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("expected 2 country scores, got %d", len(scores))
	}
	for _, key := range []string{"country_code", "macro_exposure_score", "risk_level", "components", "top_drivers"} {
		if _, ok := scores[0][key]; !ok {
			t.Errorf("country score missing %q", key)
		}
	}
}

func TestScoreMacroSave(t *testing.T) {
	dir := seedMacroFile(t)
	path := filepath.Join(t.TempDir(), "nested", "macro_scores.json")
	out, _, code := run("score", "macro", "--data", dir, "--save", path)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Saved macro exposure scores") {
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

func TestScoreMacroMissingData(t *testing.T) {
	_, errOut, code := run("score", "macro", "--data", t.TempDir())
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "reading") {
		t.Errorf("expected a read error, got %q", errOut)
	}
}

func TestScoreMacroInvalidOutput(t *testing.T) {
	dir := seedMacroFile(t)
	_, errOut, code := run("score", "macro", "--data", dir, "--output", "xml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "invalid --output") {
		t.Errorf("expected invalid output error, got %q", errOut)
	}
}

func TestShockInvalidOutput(t *testing.T) {
	_, errOut, code := run("shock", "--source", "Taiwan", "--commodity", "semiconductors", "--output", "yaml")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "invalid --output") {
		t.Errorf("expected invalid output error, got %q", errOut)
	}
}

func TestShockSaveWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "result.json")
	out, _, code := run("shock", "--source", "Taiwan", "--commodity", "semiconductors", "--save", path)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Saved JSON results") {
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
	if _, ok := parsed["graph_impact_summary"]; !ok {
		t.Errorf("saved JSON missing graph_impact_summary")
	}
}
