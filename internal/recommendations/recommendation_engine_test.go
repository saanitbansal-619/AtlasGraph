package recommendations

import (
	"strings"
	"testing"
)

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

func baseInput() Input {
	return Input{
		Source:         "Taiwan",
		Commodity:      "semiconductors",
		ShockType:      "export_collapse",
		DropPercent:    30,
		Depth:          3,
		HHI:            ptrF(0.50),
		SupplierShare:  ptrF(0.63),
		EventRisk:      55,
		MacroExposure:  45,
		FragilityScore: 12,
		AffectedPaths:  8,
		AffectedNodes:  14,
	}
}

func hasCategory(recs []Recommendation, cat Category) bool {
	for _, r := range recs {
		if r.Category == cat {
			return true
		}
	}
	return false
}

func findByCategory(recs []Recommendation, cat Category) (Recommendation, bool) {
	for _, r := range recs {
		if r.Category == cat {
			return r, true
		}
	}
	return Recommendation{}, false
}

func TestSupplierDiversificationRule(t *testing.T) {
	in := baseInput()
	in.SupplierShare = ptrF(0.65)
	in.HHI = ptrF(0.20)
	recs := GenerateExecutiveActionPlan(in).Recommendations
	r, ok := findByCategory(recs, CategorySupplierDiversification)
	if !ok {
		t.Fatal("expected supplier diversification when share > 60%")
	}
	if r.Priority != PriorityHigh {
		t.Fatalf("priority = %q, want High", r.Priority)
	}
	if r.Confidence != ConfidenceHigh {
		t.Fatalf("confidence = %q, want High", r.Confidence)
	}
	if r.SupportingMetrics["supplier_share"] != 65 {
		t.Fatalf("supplier_share metric = %v, want 65", r.SupportingMetrics["supplier_share"])
	}
}

func TestAlternativeSourcingRule(t *testing.T) {
	in := baseInput()
	in.HHI = ptrF(0.48)
	in.SupplierShare = ptrF(0.30)
	recs := GenerateExecutiveActionPlan(in).Recommendations
	r, ok := findByCategory(recs, CategoryAlternativeSourcing)
	if !ok {
		t.Fatal("expected alternative sourcing when HHI > 0.45")
	}
	if r.Priority != PriorityHigh {
		t.Fatalf("priority = %q, want High", r.Priority)
	}
}

func TestSupplierMonitoringRule(t *testing.T) {
	in := baseInput()
	in.EventRisk = 55
	in.SupplierShare = ptrF(0.20)
	in.HHI = ptrF(0.10)
	recs := GenerateExecutiveActionPlan(in).Recommendations
	r, ok := findByCategory(recs, CategorySupplierMonitoring)
	if !ok {
		t.Fatal("expected supplier monitoring when event risk > 50")
	}
	if r.Priority != PriorityMedium {
		t.Fatalf("priority = %q, want Medium", r.Priority)
	}
}

func TestCountryRiskMonitoringRule(t *testing.T) {
	in := baseInput()
	in.MacroExposure = 42
	in.EventRisk = 30
	in.SupplierShare = ptrF(0.20)
	in.HHI = ptrF(0.10)
	recs := GenerateExecutiveActionPlan(in).Recommendations
	r, ok := findByCategory(recs, CategoryCountryRiskMonitoring)
	if !ok {
		t.Fatal("expected country risk monitoring when macro > 40")
	}
	if r.Priority != PriorityMedium {
		t.Fatalf("priority = %q, want Medium", r.Priority)
	}
}

func TestInventoryIncreaseExportCollapse(t *testing.T) {
	in := baseInput()
	in.ShockType = "export_collapse"
	in.SupplierShare = ptrF(0.20)
	in.HHI = ptrF(0.10)
	recs := GenerateExecutiveActionPlan(in).Recommendations
	if !hasCategory(recs, CategoryInventoryIncrease) {
		t.Fatal("expected inventory increase for export collapse")
	}
}

func TestLogisticsReroutingRule(t *testing.T) {
	in := baseInput()
	in.ShockType = "route_disruption"
	in.SupplierShare = ptrF(0.20)
	in.HHI = ptrF(0.10)
	recs := GenerateExecutiveActionPlan(in).Recommendations
	r, ok := findByCategory(recs, CategoryLogisticsRerouting)
	if !ok {
		t.Fatal("expected logistics rerouting for route disruption")
	}
	if r.Priority != PriorityHigh {
		t.Fatalf("priority = %q, want High", r.Priority)
	}
}

func TestStrategicStockpilingRule(t *testing.T) {
	in := baseInput()
	in.Commodity = "crude oil"
	in.DropPercent = 35
	recs := GenerateExecutiveActionPlan(in).Recommendations
	if !hasCategory(recs, CategoryStrategicStockpiling) {
		t.Fatal("expected strategic stockpiling for strategic commodity with severe drop")
	}
}

func TestContractHedgingRule(t *testing.T) {
	in := baseInput()
	in.ShockType = "price_spike"
	recs := GenerateExecutiveActionPlan(in).Recommendations
	if !hasCategory(recs, CategoryContractHedging) {
		t.Fatal("expected contract hedging for price spike")
	}
}

func TestDemandReductionRule(t *testing.T) {
	in := baseInput()
	in.DropPercent = 40
	in.FragilityScore = 15
	recs := GenerateExecutiveActionPlan(in).Recommendations
	if !hasCategory(recs, CategoryDemandReduction) {
		t.Fatal("expected demand reduction for severe drop and high fragility")
	}
}

func TestDualSourcingRule(t *testing.T) {
	in := baseInput()
	in.Commodity = "semiconductors"
	in.SupplierShare = ptrF(0.55)
	in.HHI = ptrF(0.35)
	recs := GenerateExecutiveActionPlan(in).Recommendations
	if !hasCategory(recs, CategoryDualSourcing) {
		t.Fatal("expected dual sourcing for semiconductors with elevated share and HHI")
	}
}

func TestDropPercentBoostsPriority(t *testing.T) {
	lowDrop := baseInput()
	lowDrop.DropPercent = 30
	lowDrop.EventRisk = 55
	lowDrop.SupplierShare = ptrF(0.20)
	lowDrop.HHI = ptrF(0.10)

	highDrop := lowDrop
	highDrop.DropPercent = 45

	lowRec, ok := findByCategory(GenerateExecutiveActionPlan(lowDrop).Recommendations, CategorySupplierMonitoring)
	if !ok {
		t.Fatal("expected supplier monitoring")
	}
	highRec, ok := findByCategory(GenerateExecutiveActionPlan(highDrop).Recommendations, CategorySupplierMonitoring)
	if !ok {
		t.Fatal("expected supplier monitoring")
	}
	if priorityRank(highRec.Priority) <= priorityRank(lowRec.Priority) {
		t.Fatalf("drop boost failed: low=%q high=%q", lowRec.Priority, highRec.Priority)
	}
}

func TestPrioritySortingDescending(t *testing.T) {
	plan := GenerateExecutiveActionPlan(baseInput())
	recs := plan.Recommendations
	if len(recs) < 2 {
		t.Fatal("expected multiple recommendations")
	}
	for i := 1; i < len(recs); i++ {
		if priorityRank(recs[i].Priority) > priorityRank(recs[i-1].Priority) {
			t.Fatalf("sort order wrong at %d: %q before %q", i, recs[i-1].Priority, recs[i].Priority)
		}
	}
}

func TestPriorityDistributionCounts(t *testing.T) {
	plan := GenerateExecutiveActionPlan(baseInput())
	total := plan.PriorityDistribution.Critical +
		plan.PriorityDistribution.High +
		plan.PriorityDistribution.Medium +
		plan.PriorityDistribution.Low
	if total != len(plan.Recommendations) {
		t.Fatalf("distribution total %d != recommendations %d", total, len(plan.Recommendations))
	}
}

func TestExecutiveSummaryMentionsSource(t *testing.T) {
	plan := GenerateExecutiveActionPlan(baseInput())
	if !strings.Contains(plan.Summary, "Taiwan") {
		t.Fatalf("summary should mention source: %q", plan.Summary)
	}
	if plan.EstimatedBusinessImpact == "" {
		t.Fatal("expected estimated business impact")
	}
	if len(plan.SupportingEvidence) == 0 {
		t.Fatal("expected supporting evidence")
	}
}

func TestNoRecommendationsForBenignScenario(t *testing.T) {
	in := Input{
		Source:      "Canada",
		Commodity:   "aluminum",
		ShockType:   "price_spike",
		DropPercent: 10,
		Depth:       2,
		HHI:         ptrF(0.08),
		SupplierShare: ptrF(0.20),
		EventRisk:     25,
		MacroExposure: 20,
		FragilityScore: 3,
	}
	plan := GenerateExecutiveActionPlan(in)
	// price_spike may still trigger contract hedging; ensure no critical/high concentration rules
	for _, r := range plan.Recommendations {
		if r.Category == CategorySupplierDiversification || r.Category == CategoryAlternativeSourcing {
			t.Fatalf("unexpected concentration recommendation: %+v", r)
		}
	}
}
