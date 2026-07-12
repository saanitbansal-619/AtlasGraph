package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fusionBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("decode body: %v\n%s", err, rec.Body.String())
	}
	return parsed
}

func TestAPIGraphSummaryFusionFields(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/graph/summary", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := fusionBody(t, rec)
	for _, key := range []string{
		"fusion_enabled", "base_entities", "base_dependencies",
		"fused_entities", "fused_dependencies", "real_trade_edges",
		"real_trade_edges_used", "real_event_risk_used", "data_sources",
	} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("graph summary JSON missing %q", key)
		}
	}
	if parsed["real_event_risk_used"] != true {
		t.Errorf("real_event_risk_used = %v, want true when processed events seeded", parsed["real_event_risk_used"])
	}
	sources, ok := parsed["data_sources"].([]any)
	if !ok {
		t.Fatalf("data_sources = %T", parsed["data_sources"])
	}
	if !sliceContainsString(sources, "GDELT") {
		t.Errorf("data_sources = %v, want GDELT", sources)
	}
}

func TestAPIFragilitySummaryFusionMetadata(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/fragility/summary", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := fusionBody(t, rec)
	for _, key := range []string{"data_sources", "fusion_enabled", "real_trade_edges_used", "real_event_risk_used"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("fragility summary JSON missing %q", key)
		}
	}
	if parsed["real_event_risk_used"] != true {
		t.Errorf("real_event_risk_used = %v, want true", parsed["real_event_risk_used"])
	}
}

func TestAPIShockDataFusionNote(t *testing.T) {
	body := `{"source":"Taiwan","commodity":"semiconductors","drop":30,"depth":3,"shock_type":"export_collapse"}`
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := fusionBody(t, rec)
	fusion, ok := parsed["data_fusion"].(map[string]any)
	if !ok {
		t.Fatal("shock JSON missing data_fusion object")
	}
	if fusion["real_event_risk_used"] != true {
		t.Errorf("real_event_risk_used = %v, want true", fusion["real_event_risk_used"])
	}
	note, _ := fusion["propagation_note"].(string)
	if note == "" || !strings.Contains(note, "event risk") {
		t.Errorf("propagation_note = %q, want event risk mentioned", note)
	}
}

func TestGraphSummaryEventRiskFalseWithoutProcessedFile(t *testing.T) {
	h := newAPIServer(serverConfig{
		GraphData:          "",
		TradeData:          "",
		ProcessedEventData: t.TempDir(),
	})
	rec := do(h, http.MethodGet, "/api/graph/summary", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	parsed := fusionBody(t, rec)
	if parsed["real_event_risk_used"] == true {
		t.Fatal("expected real_event_risk_used=false without event_risk.json")
	}
}

func TestLoadFusedEventRiskUsed(t *testing.T) {
	processed := seedProcessedEventRisk(t)
	out, err := loadFusedDataset(fusionConfig{
		GraphData:          "",
		TradeData:          seedProcessedTrade(t, tradeSampleCSV),
		ProcessedEventData: processed,
		CommodityData:      seedCommodityPrices(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.SimCtx.RealEventRiskUsed {
		t.Fatal("expected SimCtx.RealEventRiskUsed=true")
	}
	if !out.Meta.RealEventRiskUsed {
		t.Fatal("expected Meta.RealEventRiskUsed=true after FinalizeMeta")
	}
}

func sliceContainsString(items []any, want string) bool {
	for _, it := range items {
		if s, ok := it.(string); ok && s == want {
			return true
		}
	}
	return false
}

func TestFuseWithoutTradeDataUnchanged(t *testing.T) {
	base, err := loadDataset("")
	if err != nil {
		t.Fatal(err)
	}
	out, err := loadFusedDataset(fusionConfig{GraphData: ""})
	if err != nil {
		t.Fatal(err)
	}
	if out.Meta.FusionEnabled {
		t.Fatal("expected fusion disabled")
	}
	if out.Dataset.Graph.NodeCount() != base.Graph.NodeCount() {
		t.Fatalf("nodes changed without trade data")
	}
}
