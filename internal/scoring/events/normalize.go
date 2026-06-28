package events

// Calibrated reference bands. As with the macro scorer we use *absolute* bounds
// rather than min-max over the loaded panel, so a country's score is stable
// regardless of which other countries are in the dataset. Each band maps a raw
// signal onto a 0..100 component score.
const (
	// Event volume: number of risk-relevant documents in the window. A country
	// with ~40+ matching articles saturates the volume signal.
	eventCountLo, eventCountHi = 0.0, 40.0
	// Negative tone: GDELT tone is roughly [-10, +10] with negative = adverse.
	// We score on the negated scale, so neutral/positive tone reads as no risk
	// and increasingly negative coverage scales up to the maximum.
	negToneLo, negToneHi = 0.0, 10.0
	// Risk-term density: average distinct risk terms matched per article. Three
	// or more risk themes per headline saturates the density signal.
	densityLo, densityHi = 0.0, 3.0
)

// eventCountScore rises with the number of risk-relevant documents.
func eventCountScore(count int) float64 {
	return scaleLinear(float64(count), eventCountLo, eventCountHi)
}

// negativeToneScore rises as average coverage tone becomes more negative.
// Neutral or positive tone contributes no risk.
func negativeToneScore(avgTone float64) float64 {
	return scaleLinear(-avgTone, negToneLo, negToneHi)
}

// riskTermDensityScore rises with how densely articles hit risk themes.
func riskTermDensityScore(avgTermsPerEvent float64) float64 {
	return scaleLinear(avgTermsPerEvent, densityLo, densityHi)
}

// RiskLevel maps a 0..100 event-risk score to a qualitative band.
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
