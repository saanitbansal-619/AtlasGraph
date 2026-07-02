package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
)

// cliCommodityCSV is a small, valid commodity price panel (synthetic) used to
// exercise the ingest + score commands end to end.
const cliCommodityCSV = `date,commodity_code,commodity_name,price_usd,unit,source
2023-01,crude_oil,crude oil,80,USD/barrel,synthetic
2023-02,crude_oil,crude oil,82,USD/barrel,synthetic
2023-03,crude_oil,crude oil,78,USD/barrel,synthetic
2023-04,crude_oil,crude oil,84,USD/barrel,synthetic
2023-01,lithium_carbonate,lithium carbonate,60000,USD/metric ton,synthetic
2023-02,lithium_carbonate,lithium carbonate,40000,USD/metric ton,synthetic
2023-03,lithium_carbonate,lithium carbonate,25000,USD/metric ton,synthetic
2023-04,lithium_carbonate,lithium carbonate,15000,USD/metric ton,synthetic
`

// writeCommodityCSV writes the sample CSV to a temp file and returns its path.
func writeCommodityCSV(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prices.csv")
	if err := os.WriteFile(path, []byte(cliCommodityCSV), 0o644); err != nil {
		t.Fatalf("writing commodity CSV: %v", err)
	}
	return path
}

// seedCommodityPrices ingests the sample CSV into a temp output dir and returns
// that dir, so the score command and API server have data to read. It is also
// used by the HTTP server tests.
func seedCommodityPrices(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	res, err := commodityprices.LoadFile(writeCommodityCSV(t))
	if err != nil {
		t.Fatalf("loading commodity CSV: %v", err)
	}
	commodityprices.SortRecords(res.Records)
	if _, err := commodityprices.Save(dir, commodityprices.PriceFile{
		Source:  commodityprices.SourceName,
		Records: res.Records,
	}); err != nil {
		t.Fatalf("seeding commodity prices: %v", err)
	}
	return dir
}

func TestIngestCommodityPrices(t *testing.T) {
	csv := writeCommodityCSV(t)
	outDir := t.TempDir()
	out, _, code := run("ingest", "commodity-prices", "--file", csv, "--out", outDir)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"COMMODITY PRICE INGESTION", "Source file", "Output", "Rows", "Valid rows",
		"Skipped rows", "Commodities", "Date range", "Latest month", "synthetic",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ingest output missing %q\n---\n%s", want, out)
		}
	}

	// The normalized file is readable and scoreable downstream.
	file, err := commodityprices.Load(outDir)
	if err != nil {
		t.Fatalf("loading ingested file: %v", err)
	}
	if len(file.Records) != 8 {
		t.Errorf("ingested records = %d, want 8", len(file.Records))
	}
}

func TestIngestCommodityPricesSkipsInvalid(t *testing.T) {
	csv := "date,commodity_code,commodity_name,price_usd,unit,source\n" +
		"2024-01,crude_oil,crude oil,80,USD/barrel,synthetic\n" +
		"bad,crude_oil,crude oil,80,USD/barrel,synthetic\n" +
		"2024-02,crude_oil,crude oil,-1,USD/barrel,synthetic\n"
	path := filepath.Join(t.TempDir(), "p.csv")
	if err := os.WriteFile(path, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := run("ingest", "commodity-prices", "--file", path, "--out", t.TempDir())
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Skipped rows : 2") {
		t.Errorf("expected 2 skipped rows\n---\n%s", out)
	}
}

func TestIngestCommodityPricesRequiresFile(t *testing.T) {
	_, errOut, code := run("ingest", "commodity-prices", "--out", t.TempDir())
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "--file is required") {
		t.Errorf("expected required-file error, got %q", errOut)
	}
}

func TestScoreCommoditiesText(t *testing.T) {
	dir := seedCommodityPrices(t)
	out, _, code := run("score", "commodities", "--data", dir)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"COMMODITY STRESS SCORES", "COMMODITY", "LATEST PRICE", "3M CHANGE", "12M CHANGE",
		"VOLATILITY", "SCORE", "RISK", "crude oil", "lithium carbonate", "Risk bands",
		"not a prediction of future prices",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("score output missing %q\n---\n%s", want, out)
		}
	}
	// The collapsing lithium series is far more stressed than steady crude oil.
	if strings.Index(out, "lithium carbonate") > strings.Index(out, "crude oil") {
		t.Errorf("expected lithium ranked above crude oil\n---\n%s", out)
	}
}

func TestIngestCommodityPricesPinkSheet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pink.xlsx")
	if err := commodityprices.WritePinkSheetFixture(path); err != nil {
		t.Fatalf("WritePinkSheetFixture: %v", err)
	}
	outDir := t.TempDir()
	out, _, code := run("ingest", "commodity-prices", "--file", path, "--source", "worldbank-pinksheet", "--out", outDir)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"World Bank Pink Sheet", "Pink Sheet", "Missing GFIP"} {
		if !strings.Contains(out, want) {
			t.Errorf("ingest output missing %q\n---\n%s", want, out)
		}
	}
	file, err := commodityprices.Load(outDir)
	if err != nil {
		t.Fatalf("loading ingested file: %v", err)
	}
	if !commodityprices.IsRealPriceSource(file.Source) {
		t.Errorf("source = %q, want real Pink Sheet source", file.Source)
	}
}

func TestScoreCommoditiesJSON(t *testing.T) {
	dir := seedCommodityPrices(t)
	out, _, code := run("score", "commodities", "--data", dir, "--output", "json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, key := range []string{"weights", "risk_bands", "scores", "data_source", "real_price_data"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("commodity JSON missing %q", key)
		}
	}
	var scores []map[string]json.RawMessage
	if err := json.Unmarshal(parsed["scores"], &scores); err != nil {
		t.Fatalf("scores is not an array: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("expected 2 commodity scores, got %d", len(scores))
	}
	for _, key := range []string{
		"commodity_code", "commodity_name", "latest_price_usd", "volatility_pct",
		"commodity_stress_score", "risk_level", "components",
	} {
		if _, ok := scores[0][key]; !ok {
			t.Errorf("commodity score missing %q", key)
		}
	}
}

func TestScoreCommoditiesExplainFormula(t *testing.T) {
	out, _, code := run("score", "commodities", "--explain-formula")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"COMMODITY STRESS SCORE", "recent_change_score", "volatility_score", "momentum_score",
		"0.40", "0.20", "Risk bands", "not a prediction of future prices",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain-formula output missing %q\n---\n%s", want, out)
		}
	}
}

func TestScoreCommoditiesSave(t *testing.T) {
	dir := seedCommodityPrices(t)
	path := filepath.Join(t.TempDir(), "nested", "commodity_scores.json")
	out, _, code := run("score", "commodities", "--data", dir, "--save", path)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Saved commodity stress scores") {
		t.Errorf("expected save confirmation, got %q", out)
	}
	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("saved file not readable: %v", err)
	}
}

func TestScoreCommoditiesMissingData(t *testing.T) {
	_, errOut, code := run("score", "commodities", "--data", t.TempDir())
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "reading") {
		t.Errorf("expected a read error, got %q", errOut)
	}
}
