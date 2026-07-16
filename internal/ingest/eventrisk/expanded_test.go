package eventrisk_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
)

func TestExpandedGDELTCSVIngest(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "raw", "gdelt_events", "gdelt_events_2024_expanded.csv")
	file, warnings, err := eventrisk.IngestFromFile(path, "gdelt")
	if err != nil {
		t.Fatalf("IngestFromFile: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	if file.RowsProcessed < 100 || file.RowsProcessed > 300 {
		t.Fatalf("rows_processed = %d, want 100-300", file.RowsProcessed)
	}
	if file.CountriesCovered < 12 {
		t.Fatalf("countries_covered = %d, want >= 12", file.CountriesCovered)
	}
	if file.LatestEventDate == "" {
		t.Fatal("latest_event_date empty")
	}
	if len(file.EventTypeBreakdown) < 5 {
		t.Fatalf("event_type_breakdown too small: %v", file.EventTypeBreakdown)
	}
	if file.ScoringNote == "" {
		t.Fatal("scoring_note empty")
	}
	if file.Source != eventrisk.SourceName {
		t.Fatalf("source = %q", file.Source)
	}
	dir := t.TempDir()
	if _, err := eventrisk.Save(dir, file); err != nil {
		t.Fatal(err)
	}
	loaded, err := eventrisk.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RowsProcessed != file.RowsProcessed {
		t.Fatalf("loaded rows = %d", loaded.RowsProcessed)
	}
}

func TestGoldsteinNegativeMapsToSeverity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.csv")
	csv := "date,country_code,country_name,event_type,tone,goldstein_score,mention_count,source,notes\n" +
		"2024-06-01,USA,United States,conflict,-6.0,-8.0,20,GDELT,test\n"
	if err := os.WriteFile(path, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := eventrisk.LoadCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Events) != 1 {
		t.Fatalf("events = %d", len(res.Events))
	}
	if res.Events[0].Severity < 0.7 {
		t.Fatalf("severity = %.2f, want high from goldstein -8", res.Events[0].Severity)
	}
}
