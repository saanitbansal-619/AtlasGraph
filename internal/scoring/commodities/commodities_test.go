package commodities

import (
	"math"
	"testing"

	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
)

func approx(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestPctChangeOverMonths(t *testing.T) {
	prices := []float64{100, 110, 120, 121}
	v, ok := pctChangeOverMonths(prices, 3)
	if !ok {
		t.Fatal("3-month change should be available with 4 prices")
	}
	if !approx(v, 21, 1e-9) {
		t.Errorf("3-month change = %v, want 21", v)
	}

	if _, ok := pctChangeOverMonths(prices, 12); ok {
		t.Error("12-month change should be unavailable with only 4 prices")
	}
}

func TestPctChange12M(t *testing.T) {
	// 13 prices => a 12-month change from the first to the last is defined.
	prices := make([]float64, 13)
	for i := range prices {
		prices[i] = 100
	}
	prices[12] = 150
	v, ok := pctChangeOverMonths(prices, 12)
	if !ok {
		t.Fatal("12-month change should be available with 13 prices")
	}
	if !approx(v, 50, 1e-9) {
		t.Errorf("12-month change = %v, want 50", v)
	}
}

func TestVolatilityPct(t *testing.T) {
	// Returns alternate +0.1 / -0.1, so the population stddev of returns is 0.1
	// => 10%.
	prices := []float64{100, 110, 99, 108.9, 98.01}
	got := volatilityPct(prices, 12)
	if !approx(got, 10, 1e-6) {
		t.Errorf("volatility = %v%%, want ~10%%", got)
	}

	// A perfectly flat series has zero volatility.
	if got := volatilityPct([]float64{50, 50, 50, 50}, 12); got != 0 {
		t.Errorf("flat volatility = %v, want 0", got)
	}
}

func TestScaleLinearAndRiskBands(t *testing.T) {
	if s := scaleLinear(0, 0, 40); s != 0 {
		t.Errorf("scaleLinear lo = %v, want 0", s)
	}
	if s := scaleLinear(40, 0, 40); s != 100 {
		t.Errorf("scaleLinear hi = %v, want 100", s)
	}
	if s := scaleLinear(100, 0, 40); s != 100 {
		t.Errorf("scaleLinear above hi should clamp to 100, got %v", s)
	}

	cases := map[float64]string{10: "Low", 45: "Medium", 70: "High", 95: "Critical"}
	for score, want := range cases {
		if got := RiskLevel(score); got != want {
			t.Errorf("RiskLevel(%v) = %q, want %q", score, got, want)
		}
	}
}

// buildSeries makes a flat series of length n at price p, then overrides the
// tail with overrides (applied to the final len(overrides) months).
func buildSeries(code, name string, base []float64) []commodityprices.PriceRecord {
	recs := make([]commodityprices.PriceRecord, len(base))
	for i, p := range base {
		recs[i] = commodityprices.PriceRecord{
			Date:          monthLabel(i),
			CommodityCode: code,
			CommodityName: name,
			Unit:          "USD/unit",
			PriceUSD:      p,
		}
	}
	return recs
}

func monthLabel(i int) string {
	// 2023-01 .. produces lexically sortable YYYY-MM labels for up to 24 months.
	year := 2023 + i/12
	month := i%12 + 1
	return string(rune('0'+year/1000)) + string(rune('0'+(year/100)%10)) +
		string(rune('0'+(year/10)%10)) + string(rune('0'+year%10)) + "-" +
		string(rune('0'+month/10)) + string(rune('0'+month%10))
}

func TestScoreCommoditiesStressOrderingAndScore(t *testing.T) {
	// A flat commodity (no stress) vs a volatile, crashing one.
	flat := make([]float64, 14)
	for i := range flat {
		flat[i] = 100
	}
	// Big, volatile decline: high recent change, high volatility, high momentum.
	volatile := []float64{100, 140, 90, 150, 80, 160, 70, 150, 60, 140, 50, 120, 40, 30}

	recs := append(buildSeries("flat", "flat", flat), buildSeries("volatile", "volatile", volatile)...)
	file := commodityprices.PriceFile{Records: recs}

	scores := ScoreCommodities(file, DefaultWeights())
	if len(scores) != 2 {
		t.Fatalf("got %d scores, want 2", len(scores))
	}
	// Sorted highest-first: the volatile commodity must lead.
	if scores[0].CommodityCode != "volatile" {
		t.Errorf("expected volatile commodity first, got %q", scores[0].CommodityCode)
	}

	flatScore := scores[1]
	if flatScore.CommodityCode != "flat" {
		t.Fatalf("expected flat second, got %q", flatScore.CommodityCode)
	}
	if flatScore.Score != 0 {
		t.Errorf("flat commodity score = %v, want 0", flatScore.Score)
	}
	if !approx(flatScore.LatestPrice, 100, 1e-9) {
		t.Errorf("flat latest price = %v, want 100", flatScore.LatestPrice)
	}

	vol := scores[0]
	if vol.Score <= flatScore.Score {
		t.Errorf("volatile score %v should exceed flat score %v", vol.Score, flatScore.Score)
	}
	// Weighted blend of three components on a 0..100 scale.
	if vol.Score < 0 || vol.Score > 100 {
		t.Errorf("score out of range: %v", vol.Score)
	}
	if !vol.Change3MAvailable || !vol.Change12MAvailable {
		t.Error("volatile commodity should have 3M and 12M changes available")
	}
	// Components carry the configured weights.
	want := map[string]float64{"recent_change": 0.40, "volatility": 0.40, "momentum": 0.20}
	for _, c := range vol.Components {
		if w, ok := want[c.Key]; ok && c.Weight != w {
			t.Errorf("component %q weight = %v, want %v", c.Key, c.Weight, w)
		}
		if !approx(c.Contribution, c.Weight*c.Score, 1e-9) {
			t.Errorf("component %q contribution mismatch: %v vs %v*%v", c.Key, c.Contribution, c.Weight, c.Score)
		}
	}
}

func TestScoreCommoditiesEmpty(t *testing.T) {
	if got := ScoreCommodities(commodityprices.PriceFile{}, DefaultWeights()); len(got) != 0 {
		t.Errorf("empty file should produce no scores, got %d", len(got))
	}
}
