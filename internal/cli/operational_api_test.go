package cli

import (
	"encoding/json"
	"net/http"
	"testing"
)

const severeOperationalBody = `{
	"source":"Taiwan",
	"commodity":"semiconductors",
	"shock_type":"export_collapse",
	"drop":30,
	"depth":3,
	"duration_days":120,
	"recovery_speed":"Slow",
	"substitute_availability":"Low",
	"inventory_buffer_days":0
}`

func TestAPIShockOldBodyUsesOperationalDefaults(t *testing.T) {
	body := `{"source":"Taiwan","commodity":"semiconductors","shock_type":"export_collapse","drop":30,"depth":3}`
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("old request status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	var result jsonResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	got := result.OperationalAssumptions
	if got == nil {
		t.Fatal("missing default operational_assumptions")
	}
	if got.DurationDays != 30 || got.RecoverySpeed != "Moderate" ||
		got.SubstituteAvailability != "Medium" || got.InventoryBufferDays != 30 {
		t.Errorf("operational defaults = %+v", *got)
	}
	if got.DurationFactor != 1 || got.RecoveryFactor != 1 ||
		got.SubstituteFactor != 1 || got.InventoryFactor != 0.90 {
		t.Errorf("operational default factors = %+v", *got)
	}
}

func TestAPIShockReturnsOperationalAdjustments(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock", severeOperationalBody, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\n%s", rec.Code, rec.Body.String())
	}
	var result jsonResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.OperationalAssumptions == nil {
		t.Fatal("operational_assumptions is missing")
	}
	if result.OperationalAssumptions.RecoveryFactor != 1.30 {
		t.Errorf("recovery factor = %.2f, want 1.30", result.OperationalAssumptions.RecoveryFactor)
	}
	for _, impact := range result.ChangedFragilityScores {
		if impact.OperationalMultiplier < 0.40 || impact.OperationalMultiplier > 2.25 {
			t.Errorf("%s multiplier %.2f outside cap", impact.Entity, impact.OperationalMultiplier)
		}
		if impact.ResilienceNote == "" {
			t.Errorf("%s missing resilience note", impact.Entity)
		}
		if impact.ShockFragility > 100 {
			t.Errorf("%s shock fragility %.2f exceeds 100", impact.Entity, impact.ShockFragility)
		}
	}
}

func TestScenarioReportIncludesOperationalAssumptions(t *testing.T) {
	body := `{
		"source":"Taiwan",
		"commodity":"semiconductors",
		"shock_type":"export_collapse",
		"drop_percent":30,
		"depth":3,
		"duration_days":7,
		"recovery_speed":"Fast",
		"substitute_availability":"High",
		"inventory_buffer_days":60
	}`
	rec := do(fullTestServer(t), http.MethodPost, "/api/reports/scenario", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\n%s", rec.Code, rec.Body.String())
	}
	var report scenarioReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.OperationalAssumptions == nil {
		t.Fatal("scenario report operational_assumptions is missing")
	}
	if report.OperationalAssumptions.Explanation != "Operational assumptions reduce modeled exposure because recovery is fast, substitutes are available, and inventory buffers are high." {
		t.Errorf("unexpected explanation: %q", report.OperationalAssumptions.Explanation)
	}
}
