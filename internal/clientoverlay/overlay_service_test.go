package clientoverlay

import (
	"strings"
	"testing"

	"github.com/atlasgraph/atlas/internal/customdata"
)

func sampleAnalysis() customdata.Analysis {
	return customdata.Analysis{
		ValidRows: []customdata.Row{
			{Importer: "United States", Commodity: "semiconductors", Supplier: "Taiwan", ValueUSD: 20_000_000_000},
			{Importer: "Germany", Commodity: "semiconductors", Supplier: "Taiwan", ValueUSD: 5_000_000_000},
			{Importer: "United States", Commodity: "LNG", Supplier: "Qatar", ValueUSD: 3_000_000_000},
		},
		ConcentrationResults: []customdata.ConcentrationResult{
			{
				Importer: "United States", Commodity: "semiconductors",
				TotalValueUSD: 32_000_000_000, SupplierCount: 3,
				TopSupplier: "Taiwan", TopSupplierShare: 0.625, HHI: 0.501, ConcentrationRisk: "High",
			},
			{
				Importer: "Germany", Commodity: "semiconductors",
				TotalValueUSD: 8_000_000_000, SupplierCount: 2,
				TopSupplier: "Taiwan", TopSupplierShare: 0.625, HHI: 0.52, ConcentrationRisk: "High",
			},
		},
	}
}

func TestComputeMatchesSupplierAndCommodity(t *testing.T) {
	result := Compute(sampleAnalysis(), Request{
		Source: "Taiwan", Commodity: "semiconductors", DropPercent: 30,
	})
	if result.MatchedImporters != 2 {
		t.Fatalf("matched importers = %d, want 2", result.MatchedImporters)
	}
	if result.Exposures[0].Importer != "United States" {
		t.Fatalf("top importer = %q, want United States", result.Exposures[0].Importer)
	}
}

func TestCommodityAliasMatching(t *testing.T) {
	analysis := customdata.Analysis{
		ValidRows: []customdata.Row{
			{Importer: "Japan", Commodity: "LNG", Supplier: "Qatar", ValueUSD: 10_000_000_000},
		},
		ConcentrationResults: []customdata.ConcentrationResult{
			{
				Importer: "Japan", Commodity: "LNG",
				TotalValueUSD: 10_000_000_000, TopSupplierShare: 1, HHI: 1, ConcentrationRisk: "High",
			},
		},
	}
	result := Compute(analysis, Request{Source: "Qatar", Commodity: "Natural Gas", DropPercent: 20})
	if result.MatchedImporters != 1 {
		t.Fatalf("matched importers = %d, want 1 for LNG/natural gas alias", result.MatchedImporters)
	}
}

func TestEstimatedExposedTrade(t *testing.T) {
	result := Compute(sampleAnalysis(), Request{
		Source: "Taiwan", Commodity: "semiconductors", DropPercent: 30,
	})
	us := result.Exposures[0]
	want := 20_000_000_000.0 * 0.30
	if us.EstimatedExposedTrade != want {
		t.Fatalf("exposed trade = %v, want %v", us.EstimatedExposedTrade, want)
	}
	if us.EstimatedRemainingTrade != 20_000_000_000.0-want {
		t.Fatalf("remaining trade = %v, want %v", us.EstimatedRemainingTrade, 20_000_000_000.0-want)
	}
	if result.TotalEstimatedExposedUSD != want+5_000_000_000.0*0.30 {
		t.Fatalf("total exposed = %v", result.TotalEstimatedExposedUSD)
	}
}

func TestSortByEstimatedExposedTradeDescending(t *testing.T) {
	result := Compute(sampleAnalysis(), Request{
		Source: "Taiwan", Commodity: "semiconductors", DropPercent: 30,
	})
	if len(result.Exposures) < 2 {
		t.Fatal("expected at least 2 exposures")
	}
	if result.Exposures[0].EstimatedExposedTrade < result.Exposures[1].EstimatedExposedTrade {
		t.Fatal("expected sort by estimated exposed trade descending")
	}
}

func TestBuildAssessment(t *testing.T) {
	result := Compute(sampleAnalysis(), Request{
		Source: "Taiwan", Commodity: "semiconductors", DropPercent: 30,
	})
	if !strings.Contains(result.Assessment, "Taiwan") {
		t.Fatalf("assessment missing source: %q", result.Assessment)
	}
	if !strings.Contains(result.Assessment, "United States") {
		t.Fatalf("assessment missing importer: %q", result.Assessment)
	}
	if !strings.Contains(result.Assessment, "$6.0B") {
		t.Fatalf("assessment missing exposed trade: %q", result.Assessment)
	}
}

func TestNoMatchReturnsEmpty(t *testing.T) {
	result := Compute(sampleAnalysis(), Request{
		Source: "Brazil", Commodity: "coffee", DropPercent: 30,
	})
	if result.MatchedImporters != 0 {
		t.Fatalf("matched = %d, want 0", result.MatchedImporters)
	}
	if result.Assessment != "" {
		t.Fatalf("assessment should be empty, got %q", result.Assessment)
	}
}

func TestSupplierShareAndHHI(t *testing.T) {
	result := Compute(sampleAnalysis(), Request{
		Source: "Taiwan", Commodity: "semiconductors", DropPercent: 10,
	})
	us := result.Exposures[0]
	if us.SupplierShare < 0.62 || us.SupplierShare > 0.63 {
		t.Fatalf("supplier share = %v, want ~0.625", us.SupplierShare)
	}
	if us.HHI != 0.501 {
		t.Fatalf("hhi = %v, want 0.501", us.HHI)
	}
	if us.ConcentrationRisk != "High" {
		t.Fatalf("risk = %q, want High", us.ConcentrationRisk)
	}
}
