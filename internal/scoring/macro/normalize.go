package macro

import (
	"math"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

// World Bank indicator codes this scorer consumes.
const (
	codeGDP       = "NY.GDP.MKTP.CD"
	codeImports   = "NE.IMP.GNFS.ZS"
	codeExports   = "NE.EXP.GNFS.ZS"
	codeManuf     = "NV.IND.MANF.ZS"
	codeInflation = "FP.CPI.TOTL.ZG"
	codeHighTech  = "TX.VAL.TECH.CD"
)

// Calibrated reference bands. We deliberately use *absolute* bounds rather than
// min-max over the loaded panel, so a country's score is stable regardless of
// which other countries happen to be in the dataset, and a single-country file
// still produces a meaningful number. Each band maps a raw indicator value onto
// a 0..100 component score.
const (
	// Trade openness = imports% + exports% of GDP.
	tradeLo, tradeHi = 20.0, 180.0
	// Manufacturing value added, % of GDP.
	manufLo, manufHi = 8.0, 30.0
	// Inflation, annual %. Below the lower band is treated as no stress.
	inflationLo, inflationHi = 2.0, 12.0
	// High-tech exports as a share of GDP (ratio, not %).
	highTechLo, highTechHi = 0.0, 0.12
	// GDP buffer on a log10(US$) scale: ~1e9 to ~3.2e13.
	gdpLogLo, gdpLogHi = 9.0, 13.5
)

// tradeExposureScore rises with overall trade openness: economies that import
// and export a large share of GDP are more exposed to trade disruption.
func tradeExposureScore(importsPlusExportsPctGDP float64) float64 {
	return scaleLinear(importsPlusExportsPctGDP, tradeLo, tradeHi)
}

// manufacturingDependencyScore rises with manufacturing's share of GDP: a more
// manufacturing-heavy economy is more exposed to supply-chain shocks.
func manufacturingDependencyScore(manufPctGDP float64) float64 {
	return scaleLinear(manufPctGDP, manufLo, manufHi)
}

// inflationStressScore rises with inflation above a healthy baseline.
func inflationStressScore(inflationPct float64) float64 {
	return scaleLinear(inflationPct, inflationLo, inflationHi)
}

// highTechConcentrationScore rises with high-tech exports as a share of GDP:
// the more an economy leans on technology exports, the more exposed it is to a
// technology-trade disruption.
func highTechConcentrationScore(highTechUSD, gdpUSD float64) float64 {
	if gdpUSD <= 0 {
		return 0
	}
	return scaleLinear(highTechUSD/gdpUSD, highTechLo, highTechHi)
}

// economicBufferRiskScore is the inverse-risk component: a larger economy is a
// bigger shock absorber, so its *risk* contribution is lower. We score the
// buffer on a log GDP scale and return the complementary risk.
func economicBufferRiskScore(gdpUSD float64) float64 {
	if gdpUSD <= 0 {
		return 100 // no measurable buffer => maximum buffer risk
	}
	buffer := scaleLinear(math.Log10(gdpUSD), gdpLogLo, gdpLogHi)
	return 100 - buffer
}

// RiskLevel maps a 0..100 fragility score to a qualitative band.
func RiskLevel(score float64) string {
	switch {
	case score < 30:
		return "Low"
	case score < 60:
		return "Medium"
	case score < 80:
		return "High"
	default:
		return "Critical"
	}
}

// scaleLinear maps v from [lo,hi] onto [0,100], clamping outside the band.
func scaleLinear(v, lo, hi float64) float64 {
	if hi <= lo {
		return 0
	}
	x := (v - lo) / (hi - lo)
	return clamp01(x) * 100
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// pickValue returns the most recent non-null observation for an indicator at or
// before the target year (target <= 0 means "latest available"). It returns the
// value, the year it came from, and whether anything was found.
func pickValue(records []worldbank.CountryIndicatorRecord, code string, target int) (float64, int, bool) {
	bestYear := -1
	var bestVal float64
	for _, r := range records {
		if r.IndicatorCode != code || r.Value == nil {
			continue
		}
		if target > 0 && r.Year > target {
			continue
		}
		if r.Year > bestYear {
			bestYear = r.Year
			bestVal = *r.Value
		}
	}
	if bestYear < 0 {
		return 0, 0, false
	}
	return bestVal, bestYear, true
}
