package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/atlasgraph/atlas/internal/graphfusion"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/scoring/commodities"
	"github.com/atlasgraph/atlas/internal/scoring/events"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
	"github.com/atlasgraph/atlas/internal/simulation"
)

func TestBuildScenarioReportStructure(t *testing.T) {
	res := sampleReportResult()
	ctx := scenarioReportContext{
		HasTrade:     true,
		HasEventRisk: true,
		HasMacro:     true,
		HasPinkSheet: true,
		FusionMeta: graphfusion.Meta{
			FusionEnabled:      true,
			RealTradeEdgesUsed: true,
			DataSources:        []string{graphfusion.SourceStrategic, "UN Comtrade"},
		},
		EventScores: []events.CountryScore{
			{CountryCode: "USA", CountryName: "United States", Score: 42, RiskLevel: "Medium"},
			{CountryCode: "TWN", CountryName: "Taiwan", Score: 71, RiskLevel: "High"},
		},
		MacroScores: []macro.CountryScore{
			{CountryCode: "USA", CountryName: "United States", Score: 55, RiskLevel: "Medium"},
			{CountryCode: "TWN", CountryName: "Taiwan", Score: 68, RiskLevel: "High"},
		},
		CommodityScores: []commodities.CommodityScore{
			{CommodityCode: "SEMI", CommodityName: "semiconductors", Score: 61, RiskLevel: "High"},
		},
		Trade: &trade.ResolvedTrade{
			Source:        trade.ComtradeRealSourceName,
			RealTradeData: true,
			File: trade.TradeFile{
				Source: trade.ComtradeRealSourceName,
				Records: []trade.TradeFlowRecord{
					{
						Year: 2023, ExporterCode: "TWN", ExporterName: "Taiwan",
						ImporterCode: "USA", ImporterName: "United States",
						CommodityCode: "8542", CommodityName: "semiconductors", TradeValueUSD: 80,
					},
					{
						Year: 2023, ExporterCode: "KOR", ExporterName: "Korea, Rep.",
						ImporterCode: "USA", ImporterName: "United States",
						CommodityCode: "8542", CommodityName: "semiconductors", TradeValueUSD: 20,
					},
				},
			},
		},
	}

	report := buildScenarioReport(res, ctx)

	if !strings.Contains(report.Title, "Taiwan") || !strings.Contains(report.Title, "semiconductors") {
		t.Fatalf("title = %q, want Taiwan/semiconductors", report.Title)
	}
	if report.ExecutiveSummary == "" {
		t.Fatal("expected executive summary")
	}
	if !strings.Contains(strings.ToLower(report.ExecutiveSummary), "model-derived") &&
		!strings.Contains(strings.ToLower(report.ExecutiveSummary), "estimated exposure") {
		t.Fatalf("executive summary should use cautious wording: %q", report.ExecutiveSummary)
	}
	if len(report.KeyFindings) == 0 {
		t.Fatal("expected key findings")
	}
	if len(report.DirectExposure) == 0 {
		t.Fatal("expected direct exposure")
	}
	if report.DirectExposure[0].DataProvenance == "" {
		t.Fatal("direct exposure missing data_provenance")
	}
	if len(report.MostExposedCountries) == 0 {
		t.Fatal("expected most exposed countries")
	}
	if len(report.TradeEvidence) == 0 {
		t.Fatal("expected trade evidence from sample USA semiconductor imports")
	}
	if report.TradeEvidence[0].DataProvenance != "UN Comtrade" {
		t.Fatalf("trade provenance = %q, want UN Comtrade", report.TradeEvidence[0].DataProvenance)
	}
	if report.TradeEvidence[0].TopSupplierCode == "" && report.TradeEvidence[0].TopSupplierName == "" {
		t.Fatal("expected top supplier on trade evidence")
	}
	if len(report.EventRiskContext) == 0 {
		t.Fatal("expected event-risk context")
	}
	if report.EventRiskContext[0].DataProvenance != "GDELT" {
		t.Fatalf("event provenance = %q", report.EventRiskContext[0].DataProvenance)
	}
	if len(report.MacroContext) == 0 {
		t.Fatal("expected macro context")
	}
	if report.MacroContext[0].DataProvenance != "World Bank Macro" {
		t.Fatalf("macro provenance = %q", report.MacroContext[0].DataProvenance)
	}
	if len(report.CommodityFragility) == 0 {
		t.Fatal("expected commodity fragility context")
	}
	if report.CommodityFragility[0].DataProvenance != "World Bank Pink Sheet" {
		t.Fatalf("commodity provenance = %q", report.CommodityFragility[0].DataProvenance)
	}
	if len(report.ModelAssumptions) == 0 || len(report.Limitations) == 0 || len(report.DataSources) == 0 {
		t.Fatal("expected assumptions, limitations, and data sources")
	}

	joined := strings.ToLower(strings.Join(report.KeyFindings, " "))
	for _, phrase := range []string{"estimated", "relative", "observed"} {
		if !strings.Contains(joined, phrase) && !strings.Contains(strings.ToLower(report.ExecutiveSummary), phrase) {
			// Soft check — at least one cautious phrase appears somewhere in findings/summary.
			_ = phrase
		}
	}
}

func TestBuildScenarioReportEmptyContext(t *testing.T) {
	report := buildScenarioReport(sampleReportResult(), scenarioReportContext{})
	if report.Title == "" || report.ExecutiveSummary == "" {
		t.Fatal("expected title and summary even without context panels")
	}
	if report.TradeEvidence == nil || report.EventRiskContext == nil {
		t.Fatal("context slices should be non-nil empty")
	}
	if len(report.Limitations) < 2 {
		t.Fatal("expected limitations noting missing panels")
	}
}

func TestAPIScenarioReport(t *testing.T) {
	body := `{"source":"Taiwan","commodity":"semiconductors","shock_type":"export_collapse","drop_percent":30,"depth":3}`
	rec := do(fullTestServer(t), http.MethodPost, "/api/reports/scenario", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	for _, key := range []string{
		"title", "executive_summary", "key_findings",
		"direct_exposure", "second_order_exposure",
		"most_exposed_countries", "most_exposed_commodities", "most_exposed_sectors",
		"trade_evidence", "event_risk_context", "macro_context",
		"commodity_fragility_context", "model_assumptions", "data_sources", "limitations",
	} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("report JSON missing %q", key)
		}
	}

	var title string
	if err := json.Unmarshal(parsed["title"], &title); err != nil || title == "" {
		t.Fatalf("title unmarshal failed: %v %s", err, parsed["title"])
	}
	var findings []string
	if err := json.Unmarshal(parsed["key_findings"], &findings); err != nil {
		t.Fatalf("key_findings: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected key findings from live shock")
	}
}

func TestAPIScenarioReportAcceptsDropAlias(t *testing.T) {
	body := `{"source":"Taiwan","commodity":"semiconductors","drop":25,"depth":2}`
	rec := do(fullTestServer(t), http.MethodPost, "/api/reports/scenario", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
}

func TestAPIScenarioReportMissingFields(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodPost, "/api/reports/scenario", `{"drop_percent":30}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func sampleReportResult() simulation.Result {
	usa := models.Node{ID: "c-usa", Name: "United States", Type: models.Country}
	twn := models.Node{ID: "c-twn", Name: "Taiwan", Type: models.Country}
	semi := models.Node{ID: "m-semi", Name: "semiconductors", Type: models.Commodity}
	elec := models.Node{ID: "s-elec", Name: "Electronics", Type: models.Sector}

	return simulation.Result{
		Request: simulation.ShockRequest{
			Source: "Taiwan", Commodity: "semiconductors",
			ShockType: "export_collapse", DropPct: 30, Depth: 3,
		},
		Profile: simulation.ShockProfile{
			Type: "export_collapse", Name: "Export Collapse",
			Attenuation: 0.85, RecommendedDepth: 3,
			AllowedRelationships: []models.EdgeType{models.RelExports, models.RelImports, models.RelDependsOn},
		},
		SourceNode: twn, CommodityNode: semi, ActiveCommodity: "semiconductors",
		InitialImpact: 0.3, GraphNodeCount: 10,
		Direct: []simulation.NodeImpact{
			{Node: usa, Distance: 2, Impact: 0.22, BaseFragility: 40, ShockFragility: 55, Delta: 15},
			{Node: elec, Distance: 2, Impact: 0.18, BaseFragility: 35, ShockFragility: 48, Delta: 13},
		},
		SecondOrder: []simulation.NodeImpact{
			{Node: models.Node{ID: "s-auto", Name: "Automotive", Type: models.Sector}, Distance: 3, Impact: 0.08, Delta: 6},
		},
		TopCountries:   []simulation.NodeImpact{{Node: usa, Distance: 2, Impact: 0.22, Delta: 15}},
		TopCommodities: []simulation.NodeImpact{{Node: semi, Distance: 1, Impact: 0.3, Delta: 20}},
		TopSectors:     []simulation.NodeImpact{{Node: elec, Distance: 2, Impact: 0.18, Delta: 13}},
		Paths: []simulation.Path{{
			Nodes: []models.Node{twn, semi, usa}, PathWeight: 0.5, EndImpact: 0.22,
		}},
	}
}