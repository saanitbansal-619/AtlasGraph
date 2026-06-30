package commodities

import "math"

// Calibrated reference bands. As with the macro scorer we use absolute bounds
// rather than min-max over the loaded panel, so a commodity's score is stable
// regardless of which other commodities happen to be in the dataset. Each band
// maps a raw measure onto a 0..100 component score.
const (
	// |% change over 3 months|: a 40% three-month move is treated as max stress.
	recentChangeHiPct = 40.0
	// Stddev of monthly returns (%): a 15% monthly-return stddev is extreme.
	volatilityHiPct = 15.0
	// |% change over 12 months|: an 80% annual move is treated as max stress.
	momentumHiPct = 80.0
)

// RiskLevel maps a 0..100 stress score to a qualitative band.
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

// pctChangeOverMonths returns the signed percentage change between the latest
// price and the price n months earlier. ok is false when there are too few
// observations or the baseline price is non-positive.
func pctChangeOverMonths(prices []float64, n int) (float64, bool) {
	if n <= 0 || len(prices) <= n {
		return 0, false
	}
	base := prices[len(prices)-1-n]
	if base <= 0 {
		return 0, false
	}
	latest := prices[len(prices)-1]
	return (latest - base) / base * 100, true
}

// volatilityPct returns the standard deviation (population) of the monthly
// returns over the last maxMonths months, expressed as a percentage. It returns
// 0 when fewer than two observations are available.
func volatilityPct(prices []float64, maxMonths int) float64 {
	returns := monthlyReturns(prices, maxMonths)
	if len(returns) < 2 {
		return 0
	}
	return stdDev(returns) * 100
}

// monthlyReturns computes period-over-period fractional returns for the tail of
// the series. With maxMonths returns requested it uses up to maxMonths+1 prices.
func monthlyReturns(prices []float64, maxMonths int) []float64 {
	if len(prices) < 2 {
		return nil
	}
	// Keep the last maxMonths+1 prices so we produce up to maxMonths returns.
	start := 0
	if maxMonths > 0 && len(prices) > maxMonths+1 {
		start = len(prices) - (maxMonths + 1)
	}
	window := prices[start:]
	returns := make([]float64, 0, len(window)-1)
	for i := 1; i < len(window); i++ {
		prev := window[i-1]
		if prev <= 0 {
			continue
		}
		returns = append(returns, (window[i]-prev)/prev)
	}
	return returns
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func stdDev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	var ss float64
	for _, x := range xs {
		d := x - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(xs)))
}

// scaleLinear maps v from [lo,hi] onto [0,100], clamping outside the band.
func scaleLinear(v, lo, hi float64) float64 {
	if hi <= lo {
		return 0
	}
	return clamp01((v-lo)/(hi-lo)) * 100
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

func absVal(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
