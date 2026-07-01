package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScenarioCompareDefaultText(t *testing.T) {
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	code := scenarioCompare([]string{}, out, errOut)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	text := out.String()
	if !strings.Contains(text, "SCENARIO COMPARISON") {
		t.Errorf("expected comparison header, got: %q", text)
	}
	if !strings.Contains(text, "Worst overall") {
		t.Errorf("expected summary section, got: %q", text)
	}
}

func TestScenarioCompareJSONOutput(t *testing.T) {
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	code := scenarioCompare([]string{"--output", "json"}, out, errOut)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	var resp jsonCompareResponse
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out.String())
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected at least one compared scenario")
	}
	if resp.Summary.WorstOverallScenario == "" {
		if resp.Summary.HighestAverageFragilityDelta == "" {
			t.Error("expected summary fields when scenarios run")
		}
	}
}
