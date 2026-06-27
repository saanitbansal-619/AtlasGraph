package macro

import (
	"math"
	"testing"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

func f64(v float64) *float64 { return &v }

func approx(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestComponentNormalization(t *testing.T) {
	// Midpoints of each band should land at 50.
	approx(t, "trade mid", tradeExposureScore(100), 50)          // (100-20)/160
	approx(t, "manuf mid", manufacturingDependencyScore(19), 50) // (19-8)/22
	approx(t, "inflation mid", inflationStressScore(7), 50)      // (7-2)/10
	approx(t, "hightech 0.06", highTechConcentrationScore(0.06, 1), 50)

	// Clamping below/above the band.
	approx(t, "trade clamp lo", tradeExposureScore(0), 0)
	approx(t, "trade clamp hi", tradeExposureScore(500), 100)
	approx(t, "inflation deflation", inflationStressScore(-3), 0)

	// Buffer risk is inverse: a large economy has low risk; tiny has high risk.
	bigRisk := economicBufferRiskScore(math.Pow(10, 13.5)) // top of band
	approx(t, "big buffer risk", bigRisk, 0)
	smallRisk := economicBufferRiskScore(math.Pow(10, 9)) // bottom of band
	approx(t, "small buffer risk", smallRisk, 100)
	if economicBufferRiskScore(0) != 100 {
		t.Errorf("zero GDP should be maximum buffer risk")
	}
}

func TestRiskLevel(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0, "Low"}, {29.9, "Low"}, {30, "Medium"}, {59.9, "Medium"},
		{60, "High"}, {79.9, "High"}, {80, "Critical"}, {100, "Critical"},
	}
	for _, c := range cases {
		if got := RiskLevel(c.score); got != c.want {
			t.Errorf("RiskLevel(%.1f) = %q, want %q", c.score, got, c.want)
		}
	}
}

// buildFile is a small helper to assemble a single-country dataset.
func buildFile(code, name string, year int, vals map[string]*float64) worldbank.IndicatorFile {
	var recs []worldbank.CountryIndicatorRecord
	for indCode, v := range vals {
		recs = append(recs, worldbank.CountryIndicatorRecord{
			CountryCode:   code,
			CountryName:   name,
			IndicatorCode: indCode,
			Year:          year,
			Value:         v,
		})
	}
	return worldbank.IndicatorFile{Records: recs}
}

func TestWeightedScore(t *testing.T) {
	// Construct a country whose components land on clean values:
	//   trade 50, manufacturing 50, inflation 50, high-tech 83.33, buffer risk 33.33
	file := buildFile("TST", "Testland", 2023, map[string]*float64{
		codeImports:   f64(50),
		codeExports:   f64(50),
		codeManuf:     f64(19),
		codeInflation: f64(7),
		codeGDP:       f64(math.Pow(10, 12)), // log10 = 12 -> buffer 66.67, risk 33.33
		codeHighTech:  f64(math.Pow(10, 11)), // ratio 0.1 -> 83.33
	})

	scores := ScoreCountries(file, 2023, DefaultWeights())
	if len(scores) != 1 {
		t.Fatalf("expected 1 country, got %d", len(scores))
	}
	s := scores[0]

	// Final = 0.30*trade + 0.25*manuf + 0.20*inflation + 0.15*hightech + 0.10*bufferRisk.
	expect := 0.30*tradeExposureScore(100) +
		0.25*manufacturingDependencyScore(19) +
		0.20*inflationStressScore(7) +
		0.15*highTechConcentrationScore(math.Pow(10, 11), math.Pow(10, 12)) +
		0.10*economicBufferRiskScore(math.Pow(10, 12))
	approx(t, "final score", s.Score, expect)
	if s.RiskLevel != "Medium" {
		t.Errorf("risk level = %q, want Medium", s.RiskLevel)
	}
	if s.Year != 2023 {
		t.Errorf("year = %d, want 2023", s.Year)
	}
	if len(s.TopDrivers) == 0 || s.TopDrivers[0] != "trade exposure" {
		t.Errorf("expected trade exposure to be the top driver, got %v", s.TopDrivers)
	}
	if len(s.Components) != 5 {
		t.Errorf("expected 5 components, got %d", len(s.Components))
	}
}

func TestMissingIndicatorFallbackAndRenormalization(t *testing.T) {
	// Only GDP is present, and only for 2020; request the 2023 lens.
	file := worldbank.IndicatorFile{Records: []worldbank.CountryIndicatorRecord{
		{CountryCode: "TST", CountryName: "Testland", IndicatorCode: codeGDP, Year: 2020, Value: f64(math.Pow(10, 12))},
		{CountryCode: "TST", CountryName: "Testland", IndicatorCode: codeGDP, Year: 2025, Value: f64(math.Pow(10, 13))}, // after the lens, must be ignored
	}}

	scores := ScoreCountries(file, 2023, DefaultWeights())
	s := scores[0]

	// Only the buffer-risk component is available, so the score equals it.
	approx(t, "fallback score", s.Score, economicBufferRiskScore(math.Pow(10, 12)))
	if s.Year != 2023 {
		t.Errorf("overall year should be the requested lens 2023, got %d", s.Year)
	}

	var buffer Component
	available := 0
	for _, c := range s.Components {
		if c.Available {
			available++
		}
		if c.Key == "economic_buffer_risk" {
			buffer = c
		}
	}
	if available != 1 {
		t.Errorf("expected exactly 1 available component, got %d", available)
	}
	if !buffer.Available || buffer.YearUsed != 2020 {
		t.Errorf("buffer should use the 2020 fallback, got %+v", buffer)
	}
}

func TestLatestYearWhenNoLensRequested(t *testing.T) {
	file := buildFile("TST", "Testland", 2019, map[string]*float64{codeGDP: f64(math.Pow(10, 12))})
	file.Records = append(file.Records, worldbank.CountryIndicatorRecord{
		CountryCode: "TST", CountryName: "Testland", IndicatorCode: codeGDP, Year: 2022, Value: f64(math.Pow(10, 12)),
	})
	scores := ScoreCountries(file, 0, DefaultWeights())
	if scores[0].Year != 2022 {
		t.Errorf("latest year should be 2022, got %d", scores[0].Year)
	}
}

func TestScoresSortedByScoreDescending(t *testing.T) {
	file := worldbank.IndicatorFile{Records: []worldbank.CountryIndicatorRecord{
		// Low-risk: huge GDP, tiny trade.
		{CountryCode: "BIG", CountryName: "Bigland", IndicatorCode: codeGDP, Year: 2023, Value: f64(math.Pow(10, 13.4))},
		{CountryCode: "BIG", CountryName: "Bigland", IndicatorCode: codeImports, Year: 2023, Value: f64(5)},
		{CountryCode: "BIG", CountryName: "Bigland", IndicatorCode: codeExports, Year: 2023, Value: f64(5)},
		// Higher-risk: small GDP, very open trade, high manufacturing.
		{CountryCode: "OPN", CountryName: "Openland", IndicatorCode: codeGDP, Year: 2023, Value: f64(math.Pow(10, 10))},
		{CountryCode: "OPN", CountryName: "Openland", IndicatorCode: codeImports, Year: 2023, Value: f64(90)},
		{CountryCode: "OPN", CountryName: "Openland", IndicatorCode: codeExports, Year: 2023, Value: f64(90)},
		{CountryCode: "OPN", CountryName: "Openland", IndicatorCode: codeManuf, Year: 2023, Value: f64(28)},
	}}
	scores := ScoreCountries(file, 2023, DefaultWeights())
	if len(scores) != 2 {
		t.Fatalf("expected 2 countries, got %d", len(scores))
	}
	if scores[0].CountryCode != "OPN" {
		t.Errorf("expected Openland first (higher fragility), got %s", scores[0].CountryCode)
	}
	if scores[0].Score < scores[1].Score {
		t.Errorf("scores not sorted descending: %v < %v", scores[0].Score, scores[1].Score)
	}
}
