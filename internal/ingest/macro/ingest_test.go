package macroingest_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	macroingest "github.com/atlasgraph/atlas/internal/ingest/macro"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

func TestIngestMacroPartialCSVCreatesScoresWithMissingIndicators(t *testing.T) {
	rawDir := t.TempDir()
	csv := `country_code,country_name,indicator_code,year,value
USA,United States,NY.GDP.MKTP.CD,2023,27000000000000
TWN,Taiwan,NY.GDP.MKTP.CD,2023,
`
	if err := os.WriteFile(filepath.Join(rawDir, "macro.csv"), []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()

	res, err := macroingest.Ingest(context.Background(), macroingest.Options{
		Source:    "csv",
		RawDir:    rawDir,
		OutDir:    outDir,
		Countries: []string{"USA", "TWN"},
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if res.ScoreCount < 2 {
		t.Fatalf("score count = %d, want >= 2", res.ScoreCount)
	}

	loaded, err := macro.LoadProcessed(outDir)
	if err != nil {
		t.Fatal(err)
	}
	var twn *macro.ProcessedCountryScore
	for i := range loaded.Scores {
		if loaded.Scores[i].CountryCode == "TWN" {
			twn = &loaded.Scores[i]
			break
		}
	}
	if twn == nil {
		t.Fatal("TWN missing from scores")
	}
	if len(twn.MissingIndicators) == 0 {
		t.Fatal("expected missing indicators for TWN")
	}
}
