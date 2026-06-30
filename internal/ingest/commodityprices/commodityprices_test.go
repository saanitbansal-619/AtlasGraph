package commodityprices

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleCSV = `date,commodity_code,commodity_name,price_usd,unit,source
2024-01,crude_oil,crude oil,82.4,USD/barrel,synthetic
2024-02,crude_oil,crude oil,"1,085.0",USD/barrel,synthetic
2024-01-15,Copper,copper,8500,USD/metric ton,synthetic
bad-date,crude_oil,crude oil,80,USD/barrel,synthetic
2024-03,crude_oil,crude oil,-5,USD/barrel,synthetic
2024-04,crude_oil,,90,USD/barrel,synthetic

2024-05,nickel,nickel,16000,USD/metric ton,
`

func TestParseCSVNormalizesAndSkips(t *testing.T) {
	res, err := ParseCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}

	// 7 non-blank data rows; 3 are invalid (bad date, negative price, missing name).
	if res.TotalRows != 7 {
		t.Errorf("TotalRows = %d, want 7", res.TotalRows)
	}
	if res.ValidRows() != 4 {
		t.Errorf("ValidRows = %d, want 4", res.ValidRows())
	}
	if len(res.Skipped) != 3 {
		t.Fatalf("Skipped = %d, want 3 (%+v)", len(res.Skipped), res.Skipped)
	}

	// Thousands separators are tolerated.
	if got := res.Records[1].PriceUSD; got != 1085.0 {
		t.Errorf("price with comma = %v, want 1085", got)
	}
	// Date with a day component is reduced to YYYY-MM and code is normalised.
	var copper *PriceRecord
	for i := range res.Records {
		if res.Records[i].CommodityCode == "copper" {
			copper = &res.Records[i]
		}
	}
	if copper == nil {
		t.Fatal("expected a normalised 'copper' record")
	}
	if copper.Date != "2024-01" {
		t.Errorf("date = %q, want 2024-01", copper.Date)
	}
	// A blank source falls back to the package default.
	var nickel *PriceRecord
	for i := range res.Records {
		if res.Records[i].CommodityCode == "nickel" {
			nickel = &res.Records[i]
		}
	}
	if nickel == nil || nickel.Source != SourceName {
		t.Errorf("blank source should default to %q, got %+v", SourceName, nickel)
	}
}

func TestParseCSVMissingColumn(t *testing.T) {
	csv := "date,commodity_code,price_usd,unit,source\n2024-01,crude_oil,80,USD/barrel,synthetic\n"
	_, err := ParseCSV(strings.NewReader(csv))
	if err == nil {
		t.Fatal("expected an error for a missing required column")
	}
	if !strings.Contains(err.Error(), "commodity_name") {
		t.Errorf("error %q should name the missing column", err)
	}
}

func TestParseCSVEmpty(t *testing.T) {
	if _, err := ParseCSV(strings.NewReader("")); err == nil {
		t.Fatal("expected an error for an empty file")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := PriceFile{
		Source:     SourceName,
		IngestedAt: time.Now().UTC().Truncate(time.Second),
		SourceFile: "sample.csv",
		Records: []PriceRecord{
			{Date: "2024-01", CommodityCode: "crude_oil", CommodityName: "crude oil", PriceUSD: 80, Unit: "USD/barrel", Source: SourceName},
			{Date: "2024-02", CommodityCode: "crude_oil", CommodityName: "crude oil", PriceUSD: 82, Unit: "USD/barrel", Source: SourceName},
		},
	}
	path, err := Save(dir, in)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if filepath.Base(path) != OutputFileName {
		t.Errorf("saved file = %q, want %q", filepath.Base(path), OutputFileName)
	}

	out, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out.Records) != 2 || out.Records[0].CommodityCode != "crude_oil" {
		t.Errorf("round-trip mismatch: %+v", out.Records)
	}
}

func TestLoadMissingDir(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected an error loading a missing directory")
	}
}

func TestLoadRepoSample(t *testing.T) {
	res, err := LoadFile(filepath.Join("..", "..", "..", "data", "examples", "commodity_prices_sample.csv"))
	if err != nil {
		t.Fatalf("loading repo sample: %v", err)
	}
	if res.ValidRows() == 0 {
		t.Fatal("repo sample produced no valid rows")
	}
	if len(res.Skipped) != 0 {
		t.Errorf("repo sample should have no skipped rows, got %+v", res.Skipped)
	}
	s := BuildSummary(PriceFile{Records: res.Records})
	if s.Commodities != 10 {
		t.Errorf("repo sample commodities = %d, want 10", s.Commodities)
	}
	if s.FirstMonth != "2023-01" || s.LastMonth != "2024-12" {
		t.Errorf("repo sample range = %s..%s, want 2023-01..2024-12", s.FirstMonth, s.LastMonth)
	}
}

func TestLoadFileMissing(t *testing.T) {
	if _, err := os.Stat("definitely-not-here.csv"); err == nil {
		t.Skip("unexpected file present")
	}
	if _, err := LoadFile("definitely-not-here.csv"); err == nil {
		t.Fatal("expected an error opening a missing file")
	}
}
