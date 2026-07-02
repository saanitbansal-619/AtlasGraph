package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// fullTestServer wires the API to seeded trade/macro/event data and the
// embedded sample graph, so every endpoint has something to serve.
func fullTestServer(t *testing.T) http.Handler {
	t.Helper()
	return newAPIServer(serverConfig{
		GraphData:     "", // embedded sample dataset
		TradeData:     seedProcessedTrade(t, tradeSampleCSV),
		MacroData:     seedMacroFile(t),
		EventData:     seedGDELTFile(t),
		CommodityData: seedCommodityPrices(t),
	})
}

// do issues a request against the handler and returns the recorder.
func do(h http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response is not a JSON object: %v\n---\n%s", err, rec.Body.String())
	}
	return parsed
}

func TestAPIHealth(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/health", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	parsed := decodeBody(t, rec)
	if string(parsed["status"]) != `"ok"` {
		t.Errorf("status field = %s, want \"ok\"", parsed["status"])
	}
}

func TestAPIGraphSummary(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/graph/summary", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	for _, key := range []string{"nodes", "countries", "commodities", "dependencies", "top_nodes"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("graph summary JSON missing %q", key)
		}
	}
}

func TestAPIScenarios(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/scenarios", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	var scenarios []map[string]json.RawMessage
	if err := json.Unmarshal(parsed["scenarios"], &scenarios); err != nil {
		t.Fatalf("scenarios is not an array: %v", err)
	}
	if len(scenarios) == 0 {
		t.Fatal("expected at least one scenario from the embedded sample")
	}
}

func TestAPIShockValid(t *testing.T) {
	body := `{"source":"Taiwan","commodity":"semiconductors","drop":30,"depth":3,"shock_type":"export_collapse"}`
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	for _, key := range []string{"scenario", "direct_exposure", "highest_risk_entities", "graph_impact_summary"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("shock JSON missing %q", key)
		}
	}
}

func TestAPIShockDefaultsWhenOmitted(t *testing.T) {
	// Only source and commodity provided: drop/depth/shock_type should default.
	body := `{"source":"Taiwan","commodity":"semiconductors"}`
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
}

func TestAPIShockInvalidMissingFields(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock", `{"drop":30}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	if _, ok := parsed["error"]; !ok {
		t.Errorf("expected error field in 400 response, got %s", rec.Body.String())
	}
}

func TestAPIShockInvalidBadJSON(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock", `{not json`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAPIShockUnknownEntity(t *testing.T) {
	body := `{"source":"Atlantis","commodity":"semiconductors"}`
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock", body, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	if _, ok := parsed["error"]; !ok {
		t.Errorf("expected error field for unknown entity")
	}
}

func TestAPIShockRejectsGET(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/shock", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestAPITradeSummary(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/trade/summary", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	for _, key := range []string{"records", "countries", "commodities", "total_value_usd"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("trade summary JSON missing %q", key)
		}
	}
}

func TestAPITradeDependency(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/trade/dependency?importer=USA&commodity=semiconductors", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	for _, key := range []string{"importer", "commodity", "suppliers"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("trade dependency JSON missing %q", key)
		}
	}
}

func TestAPITradeDependencyMissingParams(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/trade/dependency", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	if _, ok := parsed["error"]; !ok {
		t.Errorf("expected error field for missing params")
	}
	if _, ok := parsed["hint"]; !ok {
		t.Errorf("expected hint field for missing params")
	}
}

func TestAPITradeConcentration(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/trade/concentration?importer=USA&commodity=semiconductors", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	for _, key := range []string{"importer", "commodity", "hhi", "concentration_risk"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("trade concentration JSON missing %q", key)
		}
	}
}

func TestAPIMacroScores(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/macro/scores", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	for _, key := range []string{"weights", "risk_bands", "scores"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("macro scores JSON missing %q", key)
		}
	}
}

func TestAPIEventsRisk(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/events/risk", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	for _, key := range []string{"weights", "risk_bands", "scores"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("events risk JSON missing %q", key)
		}
	}
}

// TestAPIErrorShape confirms failures carry the documented {error, hint} shape.
func TestAPIErrorShape(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/trade/dependency?importer=USA", "", nil)
	if rec.Code == http.StatusOK {
		t.Fatalf("expected a non-200 status, got 200")
	}
	var e apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("error body is not valid JSON: %v", err)
	}
	if e.Error == "" {
		t.Errorf("error message should not be empty")
	}
	if e.Hint == "" {
		t.Errorf("expected a hint in the error response")
	}
}

func TestAPICORSHeaders(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/health", "", map[string]string{
		"Origin": "http://localhost:5173",
	})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want http://localhost:5173", got)
	}

	// 127.0.0.1 origin is also allowed.
	rec = do(fullTestServer(t), http.MethodGet, "/health", "", map[string]string{
		"Origin": "http://127.0.0.1:5173",
	})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want http://127.0.0.1:5173", got)
	}

	// A disallowed origin gets no CORS allow header.
	rec = do(fullTestServer(t), http.MethodGet, "/health", "", map[string]string{
		"Origin": "http://evil.example.com",
	})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin should not be echoed, got %q", got)
	}
}

func TestAPICORSPreflight(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodOptions, "/api/shock", "", map[string]string{
		"Origin":                        "http://localhost:5173",
		"Access-Control-Request-Method": "POST",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("Allow-Methods = %q, want it to include POST", got)
	}
}

// TestAPIMissingDataPathDoesNotPanic confirms that a configured-but-missing data
// path yields a helpful JSON error (not a panic / crash) and that unrelated
// endpoints still work.
func TestAPIMissingDataPathDoesNotPanic(t *testing.T) {
	srv := newAPIServer(serverConfig{
		GraphData:     "", // embedded still works
		TradeData:     filepath.Join(t.TempDir(), "missing-trade"),
		MacroData:     filepath.Join(t.TempDir(), "missing-macro"),
		EventData:     filepath.Join(t.TempDir(), "missing-events"),
		CommodityData: filepath.Join(t.TempDir(), "missing-commodities"),
	})

	for _, path := range []string{"/api/trade/summary", "/api/macro/scores", "/api/events/risk", "/api/commodities/stress"} {
		rec := do(srv, http.MethodGet, path, "", nil)
		if rec.Code == http.StatusOK {
			t.Errorf("%s with missing data should not return 200", path)
		}
		var e apiError
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
			t.Errorf("%s error body is not valid JSON: %v", path, err)
			continue
		}
		if e.Error == "" {
			t.Errorf("%s should return a helpful error message", path)
		}
	}

	// The embedded-graph endpoints remain healthy.
	if rec := do(srv, http.MethodGet, "/health", "", nil); rec.Code != http.StatusOK {
		t.Errorf("/health should still work, got %d", rec.Code)
	}
	if rec := do(srv, http.MethodGet, "/api/graph/summary", "", nil); rec.Code != http.StatusOK {
		t.Errorf("/api/graph/summary should still work with embedded data, got %d", rec.Code)
	}
}

func TestAPICommodityStress(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/commodities/stress", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	for _, key := range []string{"weights", "risk_bands", "scores", "data_source", "real_price_data"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("commodity stress JSON missing %q", key)
		}
	}
	var scores []map[string]json.RawMessage
	if err := json.Unmarshal(parsed["scores"], &scores); err != nil {
		t.Fatalf("scores is not an array: %v", err)
	}
	if len(scores) == 0 {
		t.Fatal("expected at least one commodity score")
	}
}

func TestAPICommodityHistoryIndex(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/commodities/history", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Source      string   `json:"source"`
		Commodities []string `json:"commodities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Commodities) == 0 {
		t.Fatal("expected at least one commodity with history")
	}
}

func TestAPICommodityHistoryByCommodity(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/commodities/history?commodity=crude%20oil", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Commodity string `json:"commodity"`
		Source    string `json:"source"`
		Points    []struct {
			Month string  `json:"month"`
			Price float64 `json:"price"`
		} `json:"points"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Commodity == "" || len(body.Points) == 0 {
		t.Fatalf("unexpected history body: %+v", body)
	}
}

func TestAPICommodityHistoryMissingCommodity(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/commodities/history?commodity=semiconductors", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404\n%s", rec.Code, rec.Body.String())
	}
}

func TestAPICommodityStressRejectsPOST(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodPost, "/api/commodities/stress", "{}", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestAPINotFound(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/nope", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var e apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("404 body is not valid JSON: %v", err)
	}
	if e.Error == "" {
		t.Errorf("404 should return an error message")
	}
}
