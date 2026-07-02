package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
)

func seedProcessedEventRisk(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "data", "examples", "gdelt_events_sample.csv")
	file, _, err := eventrisk.IngestFromFile(path, eventrisk.SourceName)
	if err != nil {
		t.Fatalf("IngestFromFile: %v", err)
	}
	dir := t.TempDir()
	if _, err := eventrisk.Save(dir, file); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return dir
}

func TestResolveEventRiskProcessed(t *testing.T) {
	processed := seedProcessedEventRisk(t)
	resolved, err := resolveEventRisk(processed, "")
	if err != nil {
		t.Fatalf("resolveEventRisk: %v", err)
	}
	if !resolved.RealEventData {
		t.Fatal("expected real_event_data=true for processed GDELT ingest")
	}
	if resolved.Source != eventrisk.SourceName {
		t.Fatalf("source = %q, want %q", resolved.Source, eventrisk.SourceName)
	}
	if len(resolved.Scores) == 0 {
		t.Fatal("expected country scores")
	}
}

func TestResolveEventRiskDemoFallback(t *testing.T) {
	legacy := seedGDELTFile(t)
	resolved, err := resolveEventRisk("", legacy)
	if err != nil {
		t.Fatalf("resolveEventRisk: %v", err)
	}
	if resolved.RealEventData {
		t.Fatal("expected demo fallback to set real_event_data=false")
	}
	if resolved.Source != "demo" && resolved.Source != "sample" {
		t.Fatalf("source = %q, want demo or sample", resolved.Source)
	}
}

func TestResolveEventRiskProcessedPreferredOverLegacy(t *testing.T) {
	processed := seedProcessedEventRisk(t)
	legacy := seedGDELTFile(t)
	resolved, err := resolveEventRisk(processed, legacy)
	if err != nil {
		t.Fatalf("resolveEventRisk: %v", err)
	}
	if !resolved.RealEventData {
		t.Fatal("expected processed data to take precedence")
	}
}

func TestResolveEventRiskCountryFilter(t *testing.T) {
	processed := seedProcessedEventRisk(t)
	resolved, err := resolveEventRisk(processed, "")
	if err != nil {
		t.Fatalf("resolveEventRisk: %v", err)
	}
	filtered := resolved.withCountryFilter("Ukraine")
	if filtered.CountryFilter != "Ukraine" {
		t.Fatalf("country filter = %q", filtered.CountryFilter)
	}
	if len(filtered.Scores) != 1 {
		t.Fatalf("scores = %d, want 1", len(filtered.Scores))
	}
	if len(filtered.RecentEvents) == 0 {
		t.Fatal("expected recent events for Ukraine")
	}
}

func TestIngestEventsCLI(t *testing.T) {
	csv := filepath.Join("..", "..", "data", "examples", "gdelt_events_sample.csv")
	outDir := t.TempDir()
	out, errOut, code := run("ingest", "events", "--file", csv, "--out", outDir, "--source", "gdelt")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "Saved event risk panel") || !strings.Contains(out, "Ukraine") {
		t.Fatalf("unexpected output: %s", out)
	}
	loaded, err := eventrisk.Load(outDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Countries) == 0 {
		t.Fatal("expected countries in output file")
	}
}
