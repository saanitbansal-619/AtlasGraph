package cli

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDBHealthDisabledDoesNotBreakServer(t *testing.T) {
	h := newAPIServer(serverConfig{GraphData: ""})

	rec := do(h, http.MethodGet, "/api/db/health", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("db health status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	var health struct {
		Enabled bool   `json:"enabled"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("decode db health: %v", err)
	}
	if health.Enabled || health.Status != "disabled" {
		t.Fatalf("db health = %+v, want disabled", health)
	}

	// Existing file-backed endpoints remain available with no database.
	rec = do(h, http.MethodGet, "/health", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("main health status = %d, want 200", rec.Code)
	}
}

func TestDBSummaryDisabledReturnsServiceUnavailable(t *testing.T) {
	rec := do(newAPIServer(serverConfig{}), http.MethodGet, "/api/db/summary", "", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503\n%s", rec.Code, rec.Body.String())
	}
}

func TestScenarioReportWorksWithoutPostgres(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodPost, "/api/reports/scenario",
		`{"source":"Taiwan","commodity":"semiconductors","shock_type":"export_collapse","drop_percent":30,"depth":3}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	var report scenarioReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.Title == "" {
		t.Fatal("scenario report title is empty")
	}
}
