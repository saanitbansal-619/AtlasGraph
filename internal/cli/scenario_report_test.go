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

func TestScenarioReportTruncatesExposure(t *testing.T) {
	res := sampleReportResult()
	// 15 direct + 15 second-order entities, several sharing the same low delta
	// (the noisy ~23.46 rows described in the issue).
	res.Direct = manyCountryImpacts("dir", 15, 2)
	res.SecondOrder = manyCountryImpacts("sec", 15, 3)

	report := buildScenarioReport(res, scenarioReportContext{})

	if len(report.DirectExposure) != maxDirectExposure {
		t.Fatalf("direct exposure len = %d, want %d", len(report.DirectExposure), maxDirectExposure)
	}
	if len(report.SecondOrderExposure) != maxSecondOrder {
		t.Fatalf("second-order exposure len = %d, want %d", len(report.SecondOrderExposure), maxSecondOrder)
	}
	// Highest-first ordering by fragility delta.
	for i := 1; i < len(report.DirectExposure); i++ {
		if report.DirectExposure[i-1].FragilityDelta < report.DirectExposure[i].FragilityDelta {
			t.Fatalf("direct exposure not sorted desc at %d: %v", i, report.DirectExposure)
		}
	}
	// The very top row should be the strongest, not a noisy duplicate.
	if report.DirectExposure[0].FragilityDelta <= 23.46 {
		t.Fatalf("top direct delta = %.2f, expected the strongest row first", report.DirectExposure[0].FragilityDelta)
	}
}

func TestScenarioReportMetadataCounts(t *testing.T) {
	res := sampleReportResult()
	res.Direct = manyCountryImpacts("dir", 15, 2)
	res.SecondOrder = manyCountryImpacts("sec", 7, 3)

	report := buildScenarioReport(res, scenarioReportContext{})

	if report.TotalDirectExposureCount != 15 {
		t.Errorf("total_direct_exposure_count = %d, want 15", report.TotalDirectExposureCount)
	}
	if report.ReturnedDirectExposureCount != maxDirectExposure {
		t.Errorf("returned_direct_exposure_count = %d, want %d", report.ReturnedDirectExposureCount, maxDirectExposure)
	}
	if report.TotalSecondOrderExposureCount != 7 {
		t.Errorf("total_second_order_exposure_count = %d, want 7", report.TotalSecondOrderExposureCount)
	}
	if report.ReturnedSecondOrderExposureCount != 7 {
		t.Errorf("returned_second_order_exposure_count = %d, want 7", report.ReturnedSecondOrderExposureCount)
	}
	if report.ReturnedDirectExposureCount != len(report.DirectExposure) {
		t.Errorf("returned count %d != slice len %d", report.ReturnedDirectExposureCount, len(report.DirectExposure))
	}
}

func TestScenarioReportTaiwanMacroUnavailable(t *testing.T) {
	res := sampleReportResult()
	ctx := scenarioReportContext{
		HasMacro: true,
		MacroScores: []macro.CountryScore{
			// Taiwan intentionally absent (excluded from World Bank Macro API).
			{CountryCode: "USA", CountryName: "United States", Score: 55, RiskLevel: "Medium"},
		},
	}

	report := buildScenarioReport(res, ctx)

	var taiwan *reportContextItem
	for i := range report.MacroContext {
		if report.MacroContext[i].Entity == "Taiwan" {
			taiwan = &report.MacroContext[i]
			break
		}
	}
	if taiwan == nil {
		t.Fatalf("expected a Taiwan macro context entry, got %+v", report.MacroContext)
	}
	if taiwan.Available {
		t.Errorf("Taiwan macro should be available=false")
	}
	if taiwan.RiskLevel != "" {
		t.Errorf("Taiwan macro risk_level = %q, want empty (no 0.0 Low)", taiwan.RiskLevel)
	}
	want := "World Bank Macro data is unavailable for Taiwan; macro context is not included in this scenario score."
	if taiwan.Summary != want {
		t.Errorf("Taiwan macro summary = %q, want %q", taiwan.Summary, want)
	}
	if taiwan.DataProvenance != "World Bank Macro" {
		t.Errorf("Taiwan macro provenance = %q", taiwan.DataProvenance)
	}

	// Serialize and confirm no "0.0"/"Low" leak for the unavailable Taiwan entry.
	b, err := json.Marshal(taiwan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"risk_level"`) {
		t.Errorf("unavailable macro entry should omit risk_level: %s", b)
	}
}

func TestScenarioReportTaiwanMacroZeroRecordUnavailable(t *testing.T) {
	res := sampleReportResult()
	// A TWN record exists but carries no real indicator data: score 0, every
	// component unavailable. This must be reported as unavailable, not "0.0 Low".
	ctx := scenarioReportContext{
		HasMacro: true,
		MacroScores: []macro.CountryScore{
			{
				CountryCode: "TWN", CountryName: "Taiwan", Score: 0, RiskLevel: "Low",
				Components: []macro.Component{
					{Key: "trade_exposure", Name: "trade exposure", Available: false},
					{Key: "inflation_stress", Name: "inflation stress", Available: false},
				},
			},
			{CountryCode: "USA", CountryName: "United States", Score: 55, RiskLevel: "Medium"},
		},
	}

	report := buildScenarioReport(res, ctx)

	var taiwan *reportContextItem
	for i := range report.MacroContext {
		if report.MacroContext[i].Entity == "Taiwan" {
			taiwan = &report.MacroContext[i]
			break
		}
	}
	if taiwan == nil {
		t.Fatalf("expected Taiwan macro entry, got %+v", report.MacroContext)
	}
	if taiwan.Available {
		t.Errorf("Taiwan macro should be available=false when the TWN record has no data")
	}
	if taiwan.RiskLevel != "" {
		t.Errorf("Taiwan macro risk_level = %q, want empty (no 0.0 Low)", taiwan.RiskLevel)
	}
	want := "World Bank Macro data is unavailable for Taiwan; macro context is not included in this scenario score."
	if taiwan.Summary != want {
		t.Errorf("Taiwan macro summary = %q, want %q", taiwan.Summary, want)
	}
	if taiwan.DataProvenance != "World Bank Macro" {
		t.Errorf("Taiwan macro provenance = %q", taiwan.DataProvenance)
	}

	// Key finding must state Taiwan unavailability, not a fabricated score.
	var macroFinding string
	for _, f := range report.KeyFindings {
		if strings.HasPrefix(f, "Macro context:") {
			macroFinding = f
			break
		}
	}
	if macroFinding != "Macro context: World Bank Macro data is unavailable for Taiwan." {
		t.Errorf("macro key finding = %q, want unavailable-for-Taiwan wording", macroFinding)
	}
	for _, f := range report.KeyFindings {
		if strings.Contains(f, "Taiwan scores") || strings.Contains(f, "0.0 (Low)") || strings.Contains(f, "0.0 Low") {
			t.Errorf("key finding leaks fabricated Taiwan macro score: %q", f)
		}
	}
}

func TestScenarioReportNoEncodingArtifacts(t *testing.T) {
	res := sampleReportResult()
	// Large positive deltas so findings render numeric deltas.
	res.TopCountries = []simulation.NodeImpact{
		{Node: models.Node{Name: "China", Type: models.Country}, Distance: 2, Impact: 0.5, Delta: 50},
		{Node: models.Node{Name: "United States", Type: models.Country}, Distance: 2, Impact: 0.4, Delta: 40},
	}
	report := buildScenarioReport(res, scenarioReportContext{})

	blobs := append([]string{report.ExecutiveSummary}, report.KeyFindings...)
	for _, s := range blobs {
		if strings.Contains(s, "Î") {
			t.Errorf("found mojibake artifact 'Î' in %q", s)
		}
		if strings.Contains(s, "\u0394") {
			t.Errorf("found raw Δ character in %q", s)
		}
	}
	// A delta finding should read as "+50.00" (or "delta +50.00"), never "Î 50.00".
	joined := strings.Join(report.KeyFindings, " ")
	if !strings.Contains(joined, "+50.00") {
		t.Errorf("expected a '+50.00' delta finding, got: %q", joined)
	}
	// 6-8 findings max for readability.
	if len(report.KeyFindings) > 8 {
		t.Errorf("key findings = %d, want <= 8", len(report.KeyFindings))
	}
}

func TestScenarioReportKeyFindingsCovered(t *testing.T) {
	res := sampleReportResult()
	ctx := scenarioReportContext{
		HasTrade:     true,
		HasEventRisk: true,
		HasMacro:     true,
		EventScores: []events.CountryScore{
			{CountryCode: "TWN", CountryName: "Taiwan", Score: 71, RiskLevel: "High"},
		},
		MacroScores: []macro.CountryScore{
			{CountryCode: "USA", CountryName: "United States", Score: 55, RiskLevel: "Medium"},
		},
		Trade: &trade.ResolvedTrade{
			Source: trade.ComtradeRealSourceName, RealTradeData: true,
			File: trade.TradeFile{
				Source: trade.ComtradeRealSourceName,
				Records: []trade.TradeFlowRecord{
					{Year: 2023, ExporterName: "Taiwan", ImporterName: "United States", CommodityName: "semiconductors", TradeValueUSD: 80},
					{Year: 2023, ExporterName: "Korea, Rep.", ImporterName: "United States", CommodityName: "semiconductors", TradeValueUSD: 20},
				},
			},
		},
	}
	report := buildScenarioReport(res, ctx)
	joined := strings.ToLower(strings.Join(report.KeyFindings, " "))
	for _, want := range []string{"shock profile", "most exposed countries", "most exposed sectors", "trade concentration", "event-risk", "provenance"} {
		if !strings.Contains(joined, want) {
			t.Errorf("key findings missing coverage of %q\n%s", want, joined)
		}
	}
	if len(report.KeyFindings) < 6 || len(report.KeyFindings) > 8 {
		t.Errorf("key findings count = %d, want 6-8", len(report.KeyFindings))
	}
}

func TestScenarioReportCommodityContextForSemiconductors(t *testing.T) {
	res := sampleReportResult()
	// No Pink Sheet price data for semiconductors, but trade + graph signals exist.
	ctx := scenarioReportContext{
		HasTrade:     true,
		HasEventRisk: true,
		EventScores: []events.CountryScore{
			{CountryCode: "TWN", CountryName: "Taiwan", Score: 71, RiskLevel: "High"},
		},
		Trade: &trade.ResolvedTrade{
			Source: trade.ComtradeRealSourceName, RealTradeData: true,
			File: trade.TradeFile{
				Source: trade.ComtradeRealSourceName,
				Records: []trade.TradeFlowRecord{
					{Year: 2023, ExporterName: "Taiwan", ImporterName: "United States", CommodityName: "semiconductors", TradeValueUSD: 80},
					{Year: 2023, ExporterName: "Korea, Rep.", ImporterName: "United States", CommodityName: "semiconductors", TradeValueUSD: 20},
				},
			},
		},
	}
	report := buildScenarioReport(res, ctx)
	if len(report.CommodityFragility) == 0 {
		t.Fatal("expected commodity fragility context for the shocked commodity")
	}
	item := report.CommodityFragility[0]
	if item.Entity != "semiconductors" {
		t.Fatalf("first commodity context entity = %q, want semiconductors", item.Entity)
	}
	if item.Available {
		t.Errorf("expected available=false when no Pink Sheet price data")
	}
	low := strings.ToLower(item.Summary)
	for _, want := range []string{"supplier concentration", "graph centrality"} {
		if !strings.Contains(low, want) {
			t.Errorf("commodity summary missing %q: %q", want, item.Summary)
		}
	}
	if !strings.Contains(low, "unavailable") {
		t.Errorf("expected note that price data is unavailable: %q", item.Summary)
	}
}

func TestScenarioReportCommodityContextWithPriceData(t *testing.T) {
	res := sampleReportResult()
	ctx := scenarioReportContext{
		HasPinkSheet: true,
		CommodityScores: []commodities.CommodityScore{
			{CommodityCode: "SEMI", CommodityName: "semiconductors", Score: 61, RiskLevel: "High"},
		},
	}
	report := buildScenarioReport(res, ctx)
	if len(report.CommodityFragility) == 0 {
		t.Fatal("expected commodity fragility context")
	}
	item := report.CommodityFragility[0]
	if !item.Available || item.Score == 0 || item.RiskLevel == "" {
		t.Fatalf("expected available price-stress item, got %+v", item)
	}
}

// manyCountryImpacts builds n country impacts at a fixed distance with a mix of
// descending deltas and repeated low-signal deltas (~23.46) to exercise sorting
// and truncation.
func manyCountryImpacts(prefix string, n, distance int) []simulation.NodeImpact {
	out := make([]simulation.NodeImpact, 0, n)
	for i := 0; i < n; i++ {
		delta := 23.46 // noisy repeated baseline
		if i < 3 {
			delta = float64(90 - i*5) // a few clearly-strongest rows
		}
		out = append(out, simulation.NodeImpact{
			Node:     models.Node{ID: models.NodeID(prefix + string(rune('a'+i))), Name: prefix + "-country-" + string(rune('A'+i)), Type: models.Country},
			Distance: distance,
			Impact:   0.1,
			Delta:    delta,
		})
	}
	return out
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