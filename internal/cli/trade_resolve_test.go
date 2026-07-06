package cli

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atlasgraph/atlas/internal/ingest/trade"
)

func seedUNComtradeTrade(t *testing.T) string {
	t.Helper()
	csv := filepath.Join("..", "..", "data", "examples", "un_comtrade_sample.csv")
	outDir := t.TempDir()
	_, _, code := run("ingest", "trade", "--file", csv, "--out", outDir, "--source", "un-comtrade")
	if code != 0 {
		t.Fatalf("ingest trade un-comtrade exit = %d, want 0", code)
	}
	return outDir
}

func TestIngestTradeUNComtradeCLI(t *testing.T) {
	dir := seedUNComtradeTrade(t)
	loaded, err := trade.LoadDependencies(dir)
	if err != nil {
		t.Fatalf("LoadDependencies: %v", err)
	}
	if len(loaded.Dependencies) == 0 {
		t.Fatal("expected dependencies")
	}
	if loaded.Source != trade.ComtradeRealSourceName {
		t.Fatalf("source = %q", loaded.Source)
	}
}

func TestIngestTradeUNComtradeDirCLI(t *testing.T) {
	srcDir := t.TempDir()
	csv := filepath.Join("..", "..", "data", "examples", "un_comtrade_sample.csv")
	body, err := os.ReadFile(csv)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sample.csv"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	out, _, code := run("ingest", "trade", "--dir", srcDir, "--out", outDir, "--source", "un-comtrade")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "UN COMTRADE TRADE INGESTION") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestAPITradeDependencyRealData(t *testing.T) {
	h := newAPIServer(serverConfig{TradeData: seedUNComtradeTrade(t)})
	rec := do(h, http.MethodGet, "/api/trade/dependency?importer=United%20States&commodity=wheat", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	if string(parsed["source"]) != `"UN Comtrade"` {
		t.Fatalf("source = %s", parsed["source"])
	}
	if string(parsed["real_trade_data"]) != "true" {
		t.Fatalf("real_trade_data = %s", parsed["real_trade_data"])
	}
}

func TestAPITradeDependencyDemoFallback(t *testing.T) {
	h := newAPIServer(serverConfig{TradeData: seedProcessedTrade(t, tradeSampleCSV)})
	rec := do(h, http.MethodGet, "/api/trade/dependency?importer=USA&commodity=semiconductors", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	if string(parsed["real_trade_data"]) != "false" {
		t.Fatalf("real_trade_data = %s", parsed["real_trade_data"])
	}
}

func TestAPITradeDependencyImporterAlias(t *testing.T) {
	h := newAPIServer(serverConfig{TradeData: seedUNComtradeTrade(t)})
	rec := do(h, http.MethodGet, "/api/trade/dependency?importer=USA&commodity=wheat", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\n%s", rec.Code, rec.Body.String())
	}
}

func TestAPITradeDependencyReturnsAllSuppliers(t *testing.T) {
	dir := t.TempDir()
	deps := trade.DependencyFile{
		Source: trade.ComtradeRealSourceName,
		Dependencies: []trade.TradeDependency{
			{Importer: "United States", Exporter: "China", Commodity: "semiconductors", HSCode: "8542", Year: 2024, TradeValueUSD: 60, Share: 0.6, Source: trade.ComtradeRealSourceName},
			{Importer: "United States", Exporter: "Taiwan", Commodity: "semiconductors", HSCode: "8542", Year: 2024, TradeValueUSD: 30, Share: 0.3, Source: trade.ComtradeRealSourceName},
			{Importer: "United States", Exporter: "Japan", Commodity: "semiconductors", HSCode: "8542", Year: 2024, TradeValueUSD: 10, Share: 0.1, Source: trade.ComtradeRealSourceName},
		},
	}
	if _, err := trade.SaveDependencies(dir, deps); err != nil {
		t.Fatal(err)
	}

	h := newAPIServer(serverConfig{TradeData: dir})
	depRec := do(h, http.MethodGet, "/api/trade/dependency?importer=United%20States&commodity=semiconductors", "", nil)
	if depRec.Code != http.StatusOK {
		t.Fatalf("dependency status = %d\n%s", depRec.Code, depRec.Body.String())
	}
	depParsed := decodeBody(t, depRec)
	if string(depParsed["source"]) != `"UN Comtrade"` {
		t.Fatalf("source = %s", depParsed["source"])
	}
	if string(depParsed["real_trade_data"]) != "true" {
		t.Fatalf("real_trade_data = %s", depParsed["real_trade_data"])
	}
	var suppliers []map[string]json.RawMessage
	if err := json.Unmarshal(depParsed["suppliers"], &suppliers); err != nil {
		t.Fatalf("suppliers decode: %v", err)
	}
	if len(suppliers) != 3 {
		t.Fatalf("suppliers = %d, want 3", len(suppliers))
	}

	conRec := do(h, http.MethodGet, "/api/trade/concentration?importer=United%20States&commodity=semiconductors", "", nil)
	if conRec.Code != http.StatusOK {
		t.Fatalf("concentration status = %d\n%s", conRec.Code, conRec.Body.String())
	}
	conParsed := decodeBody(t, conRec)
	var hhi float64
	if err := json.Unmarshal(conParsed["hhi"], &hhi); err != nil {
		t.Fatalf("hhi decode: %v", err)
	}
	if hhi >= 1.0 {
		t.Fatalf("hhi = %v, want < 1 for multiple suppliers", hhi)
	}
	if math.Abs(hhi-0.46) > 1e-6 {
		t.Fatalf("hhi = %v, want 0.46", hhi)
	}
}
