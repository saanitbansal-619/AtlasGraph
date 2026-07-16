package trade

import (
	"strings"
	"testing"
)

const unComtradeV2Header = "Period,Trade Flow,Reporter,Partner,Commodity Code,Trade Value (US$),Net Weight (kg),Qty,Qty Unit\n"

func TestParseComtradeV2MonthlyAggregation(t *testing.T) {
	body := unComtradeV2Header +
		"202401,Import,USA,Canada,1001,1000000,500000,500000,kg\n" +
		"202402,Import,USA,Canada,1001,2000000,600000,600000,kg\n"
	res, flows, err := ParseComtradeV2CSV(strings.NewReader(body), "")
	if err != nil {
		t.Fatalf("ParseComtradeV2CSV: %v", err)
	}
	if res.ValidRows != 2 {
		t.Fatalf("valid rows = %d, want 2", res.ValidRows)
	}
	if len(flows) != 1 {
		t.Fatalf("flows = %d, want 1 annual aggregate", len(flows))
	}
	if flows[0].year != 2024 {
		t.Fatalf("year = %d, want 2024", flows[0].year)
	}
	if flows[0].tradeValueUSD != 3000000 {
		t.Fatalf("value = %v, want 3000000", flows[0].tradeValueUSD)
	}
	if flows[0].commodity != "wheat" {
		t.Fatalf("commodity = %q, want wheat", flows[0].commodity)
	}
}

func TestParseComtradeV2SkipsWorldAggregate(t *testing.T) {
	body := unComtradeV2Header +
		"202401,Import,USA,World,1001,999999,0,0,kg\n" +
		"202401,Import,USA,Canada,1001,1000,0,0,kg\n"
	res, flows, err := ParseComtradeV2CSV(strings.NewReader(body), "")
	if err != nil {
		t.Fatalf("ParseComtradeV2CSV: %v", err)
	}
	if res.SkippedAggregateRows != 1 {
		t.Fatalf("skipped aggregate = %d, want 1", res.SkippedAggregateRows)
	}
	if len(flows) != 1 {
		t.Fatalf("flows = %d, want 1", len(flows))
	}
}

func TestMapHSCodeCommodities(t *testing.T) {
	cases := map[string]string{
		"1001": "wheat", "2709": "crude oil", "2711": "natural gas",
		"7403": "copper", "7502": "nickel", "3105": "fertilizer",
		"8542": "semiconductors", "8507": "batteries",
		"283691": "lithium",
		"280530": "rare earths",
		"284690": "rare earths",
		"260500": "cobalt",
		"810520": "cobalt",
		"810530": "cobalt",
	}
	for code, want := range cases {
		got, ok := MapHSCode(code)
		if !ok || got != want {
			t.Fatalf("MapHSCode(%q) = %q,%v want %q", code, got, ok, want)
		}
	}
}

func TestMapCommodityFilenameStrategic(t *testing.T) {
	cases := map[string]string{
		"lithium_carbonates_2024.csv":   "lithium",
		"rare_earth_metals.csv":         "rare earths",
		"rare_earth_compounds_hs.csv":   "rare earths",
		"cobalt_ores.csv":               "cobalt",
		"cobalt_intermediate_export.csv": "cobalt",
		"cobalt_unwrought.csv":          "cobalt",
	}
	for name, want := range cases {
		got, ok := MapCommodityFilename(name)
		if !ok || got != want {
			t.Fatalf("MapCommodityFilename(%q) = %q,%v want %q", name, got, ok, want)
		}
	}
}

func TestParseComtradeV2StrategicHSCodes(t *testing.T) {
	body := unComtradeV2Header +
		"202401,Import,USA,Chile,283691,1000,0,0,kg\n" +
		"202401,Import,USA,China,280530,2000,0,0,kg\n" +
		"202401,Import,USA,China,284690,3000,0,0,kg\n" +
		"202401,Import,USA,Congo,260500,4000,0,0,kg\n" +
		"202401,Import,USA,Congo,810520,5000,0,0,kg\n" +
		"202401,Import,USA,Congo,810530,6000,0,0,kg\n"
	res, flows, err := ParseComtradeV2CSV(strings.NewReader(body), "")
	if err != nil {
		t.Fatalf("ParseComtradeV2CSV: %v", err)
	}
	wantCommodities := map[string]bool{"lithium": true, "rare earths": true, "cobalt": true}
	seen := map[string]bool{}
	for _, f := range flows {
		if !wantCommodities[f.commodity] {
			t.Fatalf("unexpected commodity %q", f.commodity)
		}
		seen[f.commodity] = true
	}
	for c := range wantCommodities {
		if !seen[c] {
			t.Fatalf("missing commodity %q in flows", c)
		}
	}
	for _, c := range []string{"lithium", "rare earths", "cobalt"} {
		if _, ok := res.CommoditiesMapped[c]; !ok {
			t.Fatalf("expected %q in CommoditiesMapped", c)
		}
	}
}

func TestParseComtradeV2FilenameHint(t *testing.T) {
	// Unmapped HS code; filename should provide commodity.
	body := unComtradeV2Header +
		"202401,Import,USA,Chile,999999,1000,0,0,kg\n"
	_, flows, err := ParseComtradeV2CSV(strings.NewReader(body), "lithium_carbonates_2024.csv")
	if err != nil {
		t.Fatalf("ParseComtradeV2CSV: %v", err)
	}
	if len(flows) != 1 || flows[0].commodity != "lithium" {
		t.Fatalf("flows = %#v, want lithium from filename", flows)
	}
}

func TestBuildDependenciesWithShares(t *testing.T) {
	flows := []aggregatedFlow{
		{importer: "United States", exporter: "Canada", commodity: "wheat", hsCode: "1001", year: 2024, tradeValueUSD: 80},
		{importer: "United States", exporter: "Mexico", commodity: "wheat", hsCode: "1001", year: 2024, tradeValueUSD: 20},
	}
	deps := buildDependenciesWithShares(flows, ComtradeRealSourceName)
	if len(deps) != 2 {
		t.Fatalf("deps = %d, want 2", len(deps))
	}
	if deps[0].Share < 0.79 || deps[0].Share > 0.81 {
		t.Fatalf("top share = %v, want ~0.8", deps[0].Share)
	}
}

func TestParseComtradeV2SetsFlowDirection(t *testing.T) {
	body := unComtradeV2Header +
		"202401,Import,Germany,Norway,2711,1000,0,0,kg\n" +
		"202401,Export,Algeria,Ukraine,2709,2000,0,0,kg\n"
	_, flows, err := ParseComtradeV2CSV(strings.NewReader(body), "")
	if err != nil {
		t.Fatal(err)
	}
	deps := buildDependenciesWithShares(flows, ComtradeRealSourceName)
	var sawImport, sawExport bool
	for _, d := range deps {
		switch d.Flow {
		case FlowImport:
			sawImport = true
			if d.Importer != "Germany" {
				t.Errorf("import importer = %q, want Germany", d.Importer)
			}
		case FlowExport:
			sawExport = true
			if d.Importer != "Ukraine" || d.Exporter != "Algeria" {
				t.Errorf("export row = %s←%s, want Ukraine←Algeria", d.Importer, d.Exporter)
			}
		default:
			t.Errorf("unexpected flow %q", d.Flow)
		}
	}
	if !sawImport || !sawExport {
		t.Fatalf("expected both import and export rows, got %#v", deps)
	}
}

func TestMatchImporterAliases(t *testing.T) {
	rec := TradeFlowRecord{ImporterName: "United States", CommodityName: "wheat", TradeValueUSD: 1}
	dep := BuildDependency(TradeFile{Records: []TradeFlowRecord{rec}}, "USA", "wheat")
	if !dep.HasData {
		t.Fatal("expected USA alias to match United States importer")
	}
}

func TestIngestUNComtradeSampleFile(t *testing.T) {
	path := "../../../data/examples/un_comtrade_sample.csv"
	deps, stats, err := IngestUNComtradeFiles([]string{path})
	if err != nil {
		t.Fatalf("IngestUNComtradeFiles: %v", err)
	}
	if len(deps.Dependencies) == 0 {
		t.Fatal("expected dependencies")
	}
	if stats.SkippedAggregateRows < 1 {
		t.Fatal("expected World row to be skipped")
	}
	if _, ok := stats.CommoditiesMapped["wheat"]; !ok {
		t.Fatal("expected wheat mapped")
	}
	if _, ok := stats.CommoditiesMapped["semiconductors"]; !ok {
		t.Fatal("expected semiconductors mapped")
	}
}

func TestResolveTradePrefersDependencies(t *testing.T) {
	dir := t.TempDir()
	deps := DependencyFile{
		Source: ComtradeRealSourceName, Year: 2024,
		Dependencies: []TradeDependency{{
			Importer: "United States", Exporter: "Canada", Commodity: "wheat",
			HSCode: "1001", Year: 2024, TradeValueUSD: 100, Share: 1, Source: ComtradeRealSourceName,
		}},
	}
	if _, err := SaveDependencies(dir, deps); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveTrade(dir)
	if err != nil {
		t.Fatalf("ResolveTrade: %v", err)
	}
	if !resolved.RealTradeData {
		t.Fatal("expected real trade data")
	}
	if resolved.Source != ComtradeRealSourceName {
		t.Fatalf("source = %q", resolved.Source)
	}
}

func TestResolveTradeDemoFallback(t *testing.T) {
	dir := t.TempDir()
	tf := TradeFile{Source: SourceName, Records: []TradeFlowRecord{
		{ImporterName: "United States", ExporterName: "Canada", CommodityName: "wheat", TradeValueUSD: 1, Year: 2024},
	}}
	if _, err := Save(dir, tf); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveTrade(dir)
	if err != nil {
		t.Fatalf("ResolveTrade: %v", err)
	}
	if resolved.RealTradeData {
		t.Fatal("expected demo fallback")
	}
	if resolved.Source != "demo" {
		t.Fatalf("source = %q, want demo", resolved.Source)
	}
}

func TestNormalizeImporterQuery(t *testing.T) {
	if got := NormalizeImporterQuery("USA"); got != "United States" {
		t.Fatalf("got %q", got)
	}
}
