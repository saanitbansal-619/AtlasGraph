package fragility_test

import (
	"testing"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/scoring/fragility"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

func TestFragilityUsesProcessedMacroFirst(t *testing.T) {
	processed := macro.ProcessedScoreFile{
		Source: macro.ProcessedSourceName,
		Scores: []macro.ProcessedCountryScore{
			{
				CountryCode:        "USA",
				CountryName:        "United States",
				Year:               2023,
				MacroExposureScore: 77.7,
				Source:             macro.ProcessedSourceName,
				Components: []macro.ProcessedComponent{
					{Key: "inflation_stress", Name: "inflation stress", Score: 77.7, Available: true, YearUsed: 2023},
				},
			},
		},
	}

	rawGDP := 1.0
	raw := worldbank.IndicatorFile{
		Records: []worldbank.CountryIndicatorRecord{
			{CountryCode: "USA", CountryName: "United States", IndicatorCode: "NY.GDP.MKTP.CD", Year: 2023, Value: &rawGDP},
		},
	}

	src := fragility.Sources{
		ProcessedMacro: &processed,
		Macro:          &raw,
	}
	res := fragility.Score(src)
	if len(res.Countries) == 0 {
		t.Fatal("no country scores")
	}
	found := false
	for _, c := range res.Countries {
		if c.CountryCode == "USA" {
			found = true
			for _, comp := range c.Components {
				if comp.Key == "macro_exposure_score" && comp.Available {
					if comp.Score < 70 || comp.Score > 85 {
						t.Errorf("macro component score = %.1f, want processed 77.7", comp.Score)
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("USA not in fragility results")
	}
}
