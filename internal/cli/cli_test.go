package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
