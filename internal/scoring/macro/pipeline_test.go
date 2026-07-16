package macro_test

import (
	"math"
	"testing"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

func pipelineFile(code, name string, year int, vals map[string]float64) worldbank.IndicatorFile {
	var recs []worldbank.CountryIndicatorRecord
	for ind, v := range vals {
		val := v
		recs = append(recs, worldbank.CountryIndicatorRecord{
			CountryCode: code, CountryName: name, IndicatorCode: ind, Year: year, Value: &val,
		})
	}
	return worldbank.IndicatorFile{Records: recs, Countries: []string{code}}
}

func TestScorePipelineCountries(t *testing.T) {
	// High inflation, high trade, high fuel, low reserves, small GDP => high exposure.
	file := pipelineFile("TST", "Testland", 2023, map[string]float64{
		"NY.GDP.MKTP.CD":    5e10,
		"FP.CPI.TOTL.ZG":    10.0,
		"NE.TRD.GNFS.ZS":    120.0,
		"TM.VAL.FUEL.ZS.UN": 35.0,
		"FI.RES.TOTL.CD":    1e9,
	})
	out := macro.ScorePipelineCountries(file, 2023, []string{"TST"})
	if len(out.Scores) != 1 {
		t.Fatalf("scores = %d, want 1", len(out.Scores))
	}
	s := out.Scores[0]
	if s.MacroExposureScore < 50 {
		t.Errorf("macro exposure = %.1f, want elevated risk", s.MacroExposureScore)
	}
	if s.EconomicResilienceScore > 50 {
		t.Errorf("resilience = %.1f, want low resilience", s.EconomicResilienceScore)
	}
	if s.ImportDependencyScore < 50 {
		t.Errorf("import dependency = %.1f, want elevated", s.ImportDependencyScore)
	}
	if s.Source != macro.ProcessedSourceName {
		t.Errorf("source = %q", s.Source)
	}
}

func TestScorePipelineMissingValuesRenormalizes(t *testing.T) {
	file := pipelineFile("TST", "Testland", 2023, map[string]float64{
		"FP.CPI.TOTL.ZG": 4.0,
		"NE.TRD.GNFS.ZS": 40.0,
	})
	out := macro.ScorePipelineCountries(file, 2023, []string{"TST"})
	if len(out.Scores) != 1 {
		t.Fatalf("scores = %d, want 1", len(out.Scores))
	}
	if math.IsNaN(out.Scores[0].MacroExposureScore) {
		t.Fatal("macro exposure is NaN")
	}
}

func TestProcessedRoundTrip(t *testing.T) {
	file := pipelineFile("USA", "United States", 2023, map[string]float64{
		"NY.GDP.MKTP.CD":    2.7e13,
		"FP.CPI.TOTL.ZG":    3.0,
		"NE.TRD.GNFS.ZS":    25.0,
		"TM.VAL.FUEL.ZS.UN": 10.0,
		"FI.RES.TOTL.CD":    7e11,
	})
	scores := macro.ScorePipelineCountries(file, 2023, []string{"USA"})
	dir := t.TempDir()
	path, err := macro.SaveProcessed(dir, scores)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("empty path")
	}
	loaded, err := macro.LoadProcessed(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Scores) != 1 {
		t.Fatalf("loaded scores = %d", len(loaded.Scores))
	}
	legacy := loaded.ToCountryScores()
	if len(legacy) != 1 || legacy[0].Score != loaded.Scores[0].MacroExposureScore {
		t.Fatalf("legacy score mismatch: %+v", legacy)
	}
}
