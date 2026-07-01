package simulation

import (
	"testing"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
)

func TestCompareScenariosRunsMultiple(t *testing.T) {
	ds, err := data.Default()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	cfg := config.Default()

	scenarios := []CompareScenario{
		{Label: "Taiwan semiconductors", Request: ShockRequest{
			Source: "Taiwan", Commodity: "semiconductors",
			ShockType: "export_collapse", DropPct: 30, Depth: 3,
		}},
		{Label: "Saudi crude oil", Request: ShockRequest{
			Source: "Saudi Arabia", Commodity: "crude oil",
			ShockType: "supply_cut", DropPct: 25, Depth: 3,
		}},
	}

	cmp := CompareScenarios(ds.Graph, cfg, scenarios)
	if len(cmp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(cmp.Results))
	}
	for _, r := range cmp.Results {
		if r.RunError != "" {
			t.Errorf("scenario %q failed: %s", r.Label, r.RunError)
		}
		if r.AffectedNodesCount == 0 {
			t.Errorf("scenario %q expected affected nodes", r.Label)
		}
	}
}

func TestCompareSummaryPicksWorstScenario(t *testing.T) {
	ds, err := data.Default()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	cfg := config.Default()

	// Taiwan semiconductor shock should generally outrank a weak invalid combo.
	scenarios := []CompareScenario{
		{Label: "Weak invalid", Request: ShockRequest{
			Source: "Atlantis", Commodity: "unicorns",
			ShockType: "export_collapse", DropPct: 10, Depth: 1,
		}},
		{Label: "Taiwan semiconductors", Request: ShockRequest{
			Source: "Taiwan", Commodity: "semiconductors",
			ShockType: "export_collapse", DropPct: 30, Depth: 3,
		}},
	}

	cmp := CompareScenarios(ds.Graph, cfg, scenarios)
	if cmp.Results[0].Label != "Taiwan semiconductors" {
		t.Errorf("expected Taiwan ranked first, got %q", cmp.Results[0].Label)
	}
	if cmp.Summary.WorstOverallScenario != "Taiwan semiconductors" {
		t.Errorf("worst overall = %q, want Taiwan semiconductors", cmp.Summary.WorstOverallScenario)
	}
	if cmp.Summary.HighestAverageFragilityDelta == "" {
		t.Error("expected highest_average_fragility_delta in summary")
	}
}

func TestCompareInvalidScenarioDoesNotCrash(t *testing.T) {
	ds, err := data.Default()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}

	cmp := CompareScenarios(ds.Graph, config.Default(), []CompareScenario{{
		Label: "Bad combo",
		Request: ShockRequest{
			Source: "Nowhere", Commodity: "nothing",
			ShockType: "export_collapse", DropPct: 30, Depth: 3,
		},
	}})
	if len(cmp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(cmp.Results))
	}
	if cmp.Results[0].RunError == "" {
		t.Fatal("expected run error for invalid scenario")
	}
}

func TestCompareRankOrdersByImpact(t *testing.T) {
	a := ScenarioComparison{Label: "high", AvgFragilityDelta: 20, MaxFragilityDelta: 15, AffectedNodesCount: 10}
	b := ScenarioComparison{Label: "low", AvgFragilityDelta: 2, MaxFragilityDelta: 1, AffectedNodesCount: 2}
	if compareRank(a) <= compareRank(b) {
		t.Error("expected higher impact scenario to rank higher")
	}
}
