package macro_test

import (
	"testing"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

func TestMissingIndicatorsTracked(t *testing.T) {
	val := 3.0
	file := worldbank.IndicatorFile{
		Records: []worldbank.CountryIndicatorRecord{
			{CountryCode: "USA", CountryName: "United States", IndicatorCode: "FP.CPI.TOTL.ZG", Year: 2023, Value: &val},
		},
	}
	out := macro.ScorePipelineCountries(file, 0, []string{"USA"})
	if len(out.Scores) != 1 {
		t.Fatalf("scores = %d", len(out.Scores))
	}
	missing := out.Scores[0].MissingIndicators
	if len(missing) == 0 {
		t.Fatal("expected missing indicators")
	}
	foundGDP := false
	for _, code := range missing {
		if code == "NY.GDP.MKTP.CD" {
			foundGDP = true
		}
	}
	if !foundGDP {
		t.Fatalf("missing indicators = %v, want GDP among others", missing)
	}
}

func TestRequestedCountryWithoutDataIncluded(t *testing.T) {
	file := worldbank.IndicatorFile{
		Records: []worldbank.CountryIndicatorRecord{
			{CountryCode: "TWN", CountryName: "Taiwan"},
		},
	}
	out := macro.ScorePipelineCountries(file, 0, []string{"TWN", "USA"})
	if len(out.Scores) != 2 {
		t.Fatalf("scores = %d, want 2", len(out.Scores))
	}
	for _, s := range out.Scores {
		if s.CountryCode == "TWN" && len(s.MissingIndicators) != 6 {
			t.Fatalf("TWN missing = %v, want all 6 indicators", s.MissingIndicators)
		}
	}
}
