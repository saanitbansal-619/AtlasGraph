package cli

import (
	"strings"
	"testing"

	"github.com/atlasgraph/atlas/internal/customdata"
	"github.com/atlasgraph/atlas/internal/graphfusion"
)

func TestBuildScenarioReportWithClientOverlay(t *testing.T) {
	res := sampleReportResult()
	ctx := scenarioReportContext{
		HasTrade: true,
		FusionMeta: graphfusion.Meta{FusionEnabled: true},
	}
	payload := &clientAnalysisPayload{
		NormalizedRows: []customdata.Row{
			{Importer: "United States", Commodity: "semiconductors", Supplier: "Taiwan", ValueUSD: 20_000_000_000},
		},
		ConcentrationResults: []customdata.ConcentrationResult{
			{
				Importer: "United States", Commodity: "semiconductors",
				TotalValueUSD: 32_000_000_000, TopSupplierShare: 0.63, HHI: 0.501, ConcentrationRisk: "High",
			},
		},
	}
	overlay := buildClientOverlay(payload, res)
	report := buildScenarioReport(res, ctx, overlay)

	if report.ClientExposureOverlay == nil {
		t.Fatal("expected client exposure overlay on report")
	}
	if report.ClientExposureAssessment == "" {
		t.Fatal("expected client exposure assessment")
	}
	if !strings.Contains(report.ExecutiveSummary, "uploaded client portfolio") {
		t.Fatalf("executive summary should include client overlay: %q", report.ExecutiveSummary)
	}
	if report.ClientExposureOverlay.MatchedImporters != 1 {
		t.Fatalf("matched importers = %d, want 1", report.ClientExposureOverlay.MatchedImporters)
	}
}
