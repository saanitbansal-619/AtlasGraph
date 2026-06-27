package worldbank

import (
	"testing"
	"time"
)

func f64(v float64) *float64 { return &v }

func TestNormalizeRecords(t *testing.T) {
	now := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	points := []apiPoint{
		{Indicator: apiNamed{ID: "NY.GDP.MKTP.CD", Value: "GDP"}, Country: apiNamed{ID: "US", Value: "United States"}, CountryISO3: "USA", Date: "2023", Value: f64(27.0)},
		{Indicator: apiNamed{ID: "NY.GDP.MKTP.CD", Value: "GDP"}, Country: apiNamed{ID: "US", Value: "United States"}, CountryISO3: "USA", Date: "2022", Value: nil},
		{Indicator: apiNamed{ID: "NY.GDP.MKTP.CD", Value: "GDP"}, Country: apiNamed{ID: "US", Value: "United States"}, CountryISO3: "USA", Date: "not-a-year", Value: f64(1)},
	}
	recs := normalizeRecords(points, gdpIndicator(), now)
	if len(recs) != 2 {
		t.Fatalf("expected 2 records (bad year skipped), got %d", len(recs))
	}
	if recs[0].Value == nil || *recs[0].Value != 27.0 {
		t.Errorf("unexpected value: %+v", recs[0])
	}
	if recs[1].Value != nil {
		t.Errorf("expected preserved nil value, got %v", *recs[1].Value)
	}
	if recs[0].IndicatorName != "GDP (current US$)" {
		t.Errorf("indicator name should come from the catalog, got %q", recs[0].IndicatorName)
	}
	if !recs[0].FetchedAt.Equal(now) {
		t.Errorf("fetchedAt not propagated")
	}
}

func TestNormalizeFallsBackToCountryID(t *testing.T) {
	points := []apiPoint{
		{Country: apiNamed{ID: "JP", Value: "Japan"}, CountryISO3: "", Date: "2021", Value: f64(5)},
	}
	recs := normalizeRecords(points, gdpIndicator(), time.Now())
	if len(recs) != 1 || recs[0].CountryCode != "JP" {
		t.Fatalf("expected fallback to country id, got %+v", recs)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := IndicatorFile{
		Source:    SourceName,
		FetchedAt: time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		StartYear: 2018,
		EndYear:   2023,
		Countries: []string{"USA"},
		Records: []CountryIndicatorRecord{
			{CountryCode: "USA", CountryName: "United States", IndicatorCode: "NY.GDP.MKTP.CD", IndicatorName: "GDP (current US$)", Year: 2023, Value: f64(27.0), Source: SourceName},
			{CountryCode: "USA", CountryName: "United States", IndicatorCode: "FP.CPI.TOTL.ZG", IndicatorName: "Inflation", Year: 2023, Value: nil, Source: SourceName},
		},
	}
	path, err := Save(dir, in)
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if path == "" {
		t.Fatal("expected a non-empty path")
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(got.Records) != 2 || got.Records[0].CountryCode != "USA" {
		t.Fatalf("round trip mismatch: %+v", got.Records)
	}
	if got.Records[1].Value != nil {
		t.Errorf("nil value should survive round trip")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("expected error loading a missing file")
	}
}

func TestBuildSummary(t *testing.T) {
	file := IndicatorFile{Records: []CountryIndicatorRecord{
		{CountryCode: "USA", CountryName: "United States", IndicatorCode: "NY.GDP.MKTP.CD", Year: 2022, Value: f64(25.0)},
		{CountryCode: "USA", CountryName: "United States", IndicatorCode: "NY.GDP.MKTP.CD", Year: 2023, Value: f64(27.0)},
		{CountryCode: "USA", CountryName: "United States", IndicatorCode: "FP.CPI.TOTL.ZG", Year: 2023, Value: nil},
		{CountryCode: "USA", CountryName: "United States", IndicatorCode: "FP.CPI.TOTL.ZG", Year: 2022, Value: f64(8.0)},
		{CountryCode: "CHN", CountryName: "China", IndicatorCode: "NY.GDP.MKTP.CD", Year: 2023, Value: f64(18.0)},
	}}

	sum := BuildSummary(file, "usa")
	if !sum.HasData {
		t.Fatal("expected USA to have data")
	}
	if sum.CountryName != "United States" || sum.CountryCode != "USA" {
		t.Errorf("bad country identity: %+v", sum)
	}
	if sum.LatestYear != 2023 {
		t.Errorf("latest year = %d, want 2023", sum.LatestYear)
	}
	if len(sum.Lines) != len(DefaultIndicators) {
		t.Fatalf("expected one line per default indicator, got %d", len(sum.Lines))
	}
	// GDP: latest non-null is 2023 = 27.
	gdp := sum.Lines[0]
	if gdp.IndicatorCode != "NY.GDP.MKTP.CD" || gdp.Year != 2023 || gdp.Value == nil || *gdp.Value != 27.0 {
		t.Errorf("GDP line wrong: %+v", gdp)
	}
	// Inflation: 2023 is null, so latest non-null is 2022 = 8.
	var infl SummaryLine
	for _, l := range sum.Lines {
		if l.IndicatorCode == "FP.CPI.TOTL.ZG" {
			infl = l
		}
	}
	if infl.Year != 2022 || infl.Value == nil || *infl.Value != 8.0 {
		t.Errorf("inflation should fall back to 2022=8, got %+v", infl)
	}
}

func TestBuildSummaryUnknownCountry(t *testing.T) {
	file := IndicatorFile{Records: []CountryIndicatorRecord{
		{CountryCode: "USA", IndicatorCode: "NY.GDP.MKTP.CD", Year: 2023, Value: f64(1)},
	}}
	if sum := BuildSummary(file, "ZZZ"); sum.HasData {
		t.Fatal("expected no data for unknown country")
	}
}
