package commodityprices

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMapPinkSheetSeries(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"Crude oil, average", "crude oil", true},
		{"Natural gas, Europe", "natural gas", true},
		{"Natural gas, U.S.", "natural gas", true},
		{"Liquefied natural gas, Japan", "LNG", true},
		{"Aluminum", "aluminum", true},
		{"Copper", "copper", true},
		{"Nickel", "nickel", true},
		{"Wheat, US HRW", "wheat", true},
		{"Maize", "corn", true},
		{"Rice, Thai 5%", "rice", true},
		{"Urea", "fertilizer", true},
		{"DAP", "fertilizer", true},
		{"Potassium chloride", "fertilizer", true},
		{"Semiconductors", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := MapPinkSheetSeries(tc.in)
		if ok != tc.ok {
			t.Errorf("MapPinkSheetSeries(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if !ok {
			continue
		}
		if got.Name != tc.want {
			t.Errorf("MapPinkSheetSeries(%q) name = %q, want %q", tc.in, got.Name, tc.want)
		}
	}
}

func TestStrategicGFIPCommoditiesMissing(t *testing.T) {
	missing := StrategicGFIPCommoditiesMissing(map[string]struct{}{"crude_oil": {}})
	if len(missing) == 0 {
		t.Fatal("expected missing strategic commodities")
	}
	found := false
	for _, m := range missing {
		if m == "semiconductors" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected semiconductors in missing list, got %v", missing)
	}
}

func TestParsePinkSheetXLSXFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pink.xlsx")
	if err := WritePinkSheetFixture(path); err != nil {
		t.Fatalf("WritePinkSheetFixture: %v", err)
	}

	res, err := LoadPinkSheetFile(path)
	if err != nil {
		t.Fatalf("LoadPinkSheetFile: %v", err)
	}
	if res.ValidRows() == 0 {
		t.Fatal("expected valid rows from fixture")
	}
	if res.Meta.MappedSeries < 4 {
		t.Errorf("MappedSeries = %d, want >= 4", res.Meta.MappedSeries)
	}
	if len(res.Meta.MissingGFIP) == 0 {
		t.Error("expected some strategic GFIP commodities to be reported missing")
	}

	var crude bool
	for _, r := range res.Records {
		if r.CommodityCode == "crude_oil" && r.Date == "2023-01" {
			crude = true
			if r.Source != PinkSheetSourceName {
				t.Errorf("source = %q, want %q", r.Source, PinkSheetSourceName)
			}
		}
	}
	if !crude {
		t.Error("expected crude oil records in parsed fixture")
	}
}

func TestParsePinkSheetRowOrientedFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pink-row.xlsx")
	if err := WritePinkSheetRowFixture(path); err != nil {
		t.Fatalf("WritePinkSheetRowFixture: %v", err)
	}
	res, err := LoadPinkSheetFile(path)
	if err != nil {
		t.Fatalf("LoadPinkSheetFile: %v", err)
	}
	if res.Meta.Layout != "row-oriented" {
		t.Errorf("layout = %q, want row-oriented", res.Meta.Layout)
	}
	if res.ValidRows() == 0 {
		t.Fatal("expected valid rows from row-oriented fixture")
	}
}

func TestParseRealPinkSheetFile(t *testing.T) {
	path := filepath.Join("..", "..", "..", RealPinkSheetPath())
	if _, err := os.Stat(path); err != nil {
		t.Skip("real Pink Sheet XLSX not present locally")
	}
	res, err := LoadPinkSheetFile(path)
	if err != nil {
		t.Fatalf("LoadPinkSheetFile: %v", err)
	}
	if res.Meta.Layout != "column-oriented" {
		t.Errorf("layout = %q, want column-oriented", res.Meta.Layout)
	}
	if res.Meta.MappedSeries < 10 {
		t.Errorf("MappedSeries = %d, want >= 10", res.Meta.MappedSeries)
	}
	if res.ValidRows() == 0 {
		t.Fatal("expected price records from real Pink Sheet file")
	}
}

func TestHistoryForCommodity(t *testing.T) {
	file := PriceFile{
		Source: PinkSheetSourceName,
		Records: []PriceRecord{
			{Date: "2023-01", CommodityCode: "crude_oil", CommodityName: "crude oil", PriceUSD: 80},
			{Date: "2023-02", CommodityCode: "crude_oil", CommodityName: "crude oil", PriceUSD: 82},
		},
	}
	h, err := HistoryForCommodity(file, "crude oil")
	if err != nil {
		t.Fatalf("HistoryForCommodity: %v", err)
	}
	if len(h.Points) != 2 || h.Source != PinkSheetSourceName {
		t.Fatalf("unexpected history: %+v", h)
	}
	if _, err := HistoryForCommodity(file, "semiconductors"); err == nil {
		t.Fatal("expected error for missing commodity history")
	}
}

func TestIsRealPriceSource(t *testing.T) {
	if !IsRealPriceSource(PinkSheetSourceName) {
		t.Error("Pink Sheet should be real price source")
	}
	if IsRealPriceSource(SourceName) {
		t.Error("CSV demo source should not be real")
	}
}

func TestDetectSource(t *testing.T) {
	if got := DetectSource("data/raw/worldbank_pinksheet/CMO-Historical-Data-Monthly.xlsx"); got != "worldbank-pinksheet" {
		t.Errorf("DetectSource xlsx = %q", got)
	}
	if got := DetectSource("data/examples/commodity_prices_sample.csv"); got != "csv" {
		t.Errorf("DetectSource csv = %q", got)
	}
}

func TestIngestFromFileCSVStillWorks(t *testing.T) {
	res, source, _, err := IngestFromFile(filepath.Join("..", "..", "..", "data", "examples", "commodity_prices_sample.csv"), "csv")
	if err != nil {
		t.Fatalf("IngestFromFile csv: %v", err)
	}
	if res.ValidRows() == 0 {
		t.Fatal("expected valid CSV rows")
	}
	if strings.Contains(source, "World Bank") {
		t.Errorf("csv source should not be Pink Sheet, got %q", source)
	}
}
