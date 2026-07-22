package cli

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAPIPipelineSummaryReturnsETLMetrics(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/pipeline/summary", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}

	var summary struct {
		RunID                  string `json:"run_id"`
		Status                 string `json:"status"`
		TotalRowsProcessed     int    `json:"total_rows_processed"`
		ValidationChecksPassed int    `json:"validation_checks_passed"`
		ValidationChecks       []struct {
			CheckName   string  `json:"check_name"`
			Status      string  `json:"status"`
			MetricValue float64 `json:"metric_value"`
			Source      string  `json:"source"`
		} `json:"validation_checks"`
		SourcesProcessed []struct {
			Name          string `json:"name"`
			RowsProcessed int    `json:"rows_processed"`
		} `json:"sources_processed"`
		OutputTables []string `json:"output_tables"`
		Notes        []string `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode pipeline summary: %v", err)
	}
	if summary.RunID == "" {
		t.Fatal("run_id is empty")
	}
	if summary.Status != "completed" && summary.Status != "warning" {
		t.Fatalf("status = %q, want completed or warning\n%s", summary.Status, rec.Body.String())
	}
	if len(summary.ValidationChecks) == 0 {
		t.Fatal("validation_checks is empty")
	}
	for _, check := range summary.ValidationChecks {
		if check.CheckName == "" || check.Source == "" || check.Status == "" {
			t.Fatalf("invalid validation check: %+v", check)
		}
	}
	if summary.TotalRowsProcessed <= 0 {
		t.Fatalf("total_rows_processed = %d, want > 0", summary.TotalRowsProcessed)
	}
	if len(summary.SourcesProcessed) == 0 {
		t.Fatal("sources_processed is empty")
	}

	foundTrade := false
	for _, src := range summary.SourcesProcessed {
		if src.Name == "UN Comtrade trade rows" && src.RowsProcessed > 0 {
			foundTrade = true
		}
	}
	if !foundTrade {
		t.Fatalf("expected UN Comtrade source rows, got %+v", summary.SourcesProcessed)
	}
	if len(summary.OutputTables) == 0 {
		t.Fatal("output_tables is empty")
	}
}

func TestAPIPipelineSummaryWorksWithoutPostgres(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/pipeline/summary", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	var summary struct {
		TotalRowsLoaded int      `json:"total_rows_loaded"`
		Notes           []string `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode pipeline summary: %v", err)
	}
	if summary.TotalRowsLoaded != 0 {
		t.Fatalf("total_rows_loaded = %d, want 0 without postgres", summary.TotalRowsLoaded)
	}
	foundNote := false
	for _, note := range summary.Notes {
		if note != "" {
			foundNote = true
		}
	}
	if !foundNote {
		t.Fatal("expected postgres-disabled note in pipeline summary")
	}
}

func TestAPIPipelineSummaryRejectsNonGet(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodPost, "/api/pipeline/summary", "{}", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
