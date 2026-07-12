package simulation

import "testing"

func TestEventRiskMultiplierCapped(t *testing.T) {
	if got := EventRiskMultiplier(0); got != 1 {
		t.Fatalf("score 0 = %v, want 1", got)
	}
	if got := EventRiskMultiplier(80); got != 1.2 {
		t.Fatalf("score 80 = %v, want 1.2", got)
	}
	if got := EventRiskMultiplier(200); got != 1.25 {
		t.Fatalf("score 200 = %v, want capped 1.25", got)
	}
}

func TestCommodityStressMultiplierCapped(t *testing.T) {
	if got := CommodityStressMultiplier(0); got != 1 {
		t.Fatalf("score 0 = %v, want 1", got)
	}
	if got := CommodityStressMultiplier(100); got != 1.2 {
		t.Fatalf("score 100 = %v, want 1.2", got)
	}
	if got := CommodityStressMultiplier(200); got != 1.2 {
		t.Fatalf("score 200 = %v, want capped 1.2", got)
	}
}
