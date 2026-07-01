package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestScoreFragilityExplainFormula(t *testing.T) {
	out, _, code := run("score", "fragility", "--explain-formula")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"UNIFIED FRAGILITY SCORE — FORMULA",
		"macro_exposure_score",
		"commodity_stress_score",
		"explainable composite risk score, not a prediction",
		"Risk bands",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain output missing %q", want)
		}
	}
}

func TestScoreFragilityText(t *testing.T) {
	out, _, code := run("score", "fragility",
		"--graph-data", "",
		"--trade-data", seedProcessedTrade(t, tradeSampleCSV),
		"--macro-data", seedMacroFile(t),
		"--event-data", seedGDELTFile(t),
		"--commodity-data", seedCommodityPrices(t),
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, out)
	}
	for _, want := range []string{
		"UNIFIED FRAGILITY SCORES",
		"COUNTRIES",
		"COMMODITIES",
		"COUNTRY",
		"SCORE",
		"TOP DRIVERS",
		"Risk bands",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
}

func TestScoreFragilityJSON(t *testing.T) {
	out, _, code := run("score", "fragility",
		"--graph-data", "",
		"--trade-data", seedProcessedTrade(t, tradeSampleCSV),
		"--macro-data", seedMacroFile(t),
		"--event-data", seedGDELTFile(t),
		"--commodity-data", seedCommodityPrices(t),
		"--output", "json",
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, key := range []string{"country_weights", "commodity_weights", "risk_bands", "countries", "commodities"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("fragility JSON missing %q", key)
		}
	}
}

func TestAPIFragilityCountries(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/fragility/countries", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	var countries []map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &countries); err != nil {
		t.Fatalf("countries response is not an array: %v\n%s", err, rec.Body.String())
	}
	if len(countries) == 0 {
		t.Fatal("expected at least one country fragility score")
	}
	for _, key := range []string{"country_name", "score", "risk_level", "top_drivers", "components", "missing_components"} {
		if _, ok := countries[0][key]; !ok {
			t.Errorf("country JSON missing %q", key)
		}
	}
}

func TestAPIFragilityCommodities(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/fragility/commodities", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	var commodities []map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &commodities); err != nil {
		t.Fatalf("commodities response is not an array: %v", err)
	}
	if len(commodities) == 0 {
		t.Fatal("expected at least one commodity fragility score")
	}
}

func TestAPIFragilitySummary(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/fragility/summary", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	for _, key := range []string{"countries", "commodities"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("summary JSON missing %q", key)
		}
	}
}

func TestAPIFragilityPartialDataDoesNotPanic(t *testing.T) {
	// Server with only embedded graph — no trade/macro/event/commodity dirs.
	h := newAPIServer(serverConfig{GraphData: ""})
	for _, path := range []string{"/api/fragility/countries", "/api/fragility/commodities", "/api/fragility/summary"} {
		rec := do(h, http.MethodGet, path, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200\n%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestAPIFragilityRejectsPOST(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodPost, "/api/fragility/summary", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
