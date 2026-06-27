package trade

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const goodCSV = `year,exporter_code,exporter_name,importer_code,importer_name,commodity_code,commodity_name,trade_value_usd,quantity,unit
2023,TWN,Taiwan,USA,United States,8542,semiconductors,85000000000,0,USD
2023,KOR,Korea Rep.,USA,United States,8542,semiconductors,21000000000,0,USD
`

func TestParseCSVHappyPath(t *testing.T) {
	res, err := ParseCSV(strings.NewReader(goodCSV), time.Now().UTC())
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if res.TotalRows != 2 || res.ValidRows() != 2 {
		t.Fatalf("rows: total=%d valid=%d, want 2/2", res.TotalRows, res.ValidRows())
	}
	if len(res.Skipped) != 0 {
		t.Fatalf("expected no skipped rows, got %v", res.Skipped)
	}
	first := res.Records[0]
	if first.ExporterCode != "TWN" || first.ImporterCode != "USA" {
		t.Errorf("codes not normalised: %+v", first)
	}
	if first.CommodityName != "semiconductors" {
		t.Errorf("commodity = %q", first.CommodityName)
	}
	if first.TradeValueUSD != 85e9 {
		t.Errorf("trade value = %v, want 85e9", first.TradeValueUSD)
	}
	if first.Source != SourceName {
		t.Errorf("source = %q, want %q", first.Source, SourceName)
	}
}

func TestParseCSVLowercaseHeaderAndReordering(t *testing.T) {
	// Columns deliberately reordered and mixed-case; loader must map by name.
	csv := `Commodity_Name,IMPORTER_CODE,importer_name,exporter_code,exporter_name,commodity_code,trade_value_usd,quantity,unit,YEAR
semiconductors,USA,United States,TWN,Taiwan,8542,1000,0,USD,2023
`
	res, err := ParseCSV(strings.NewReader(csv), time.Now().UTC())
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if res.ValidRows() != 1 {
		t.Fatalf("valid rows = %d, want 1", res.ValidRows())
	}
	r := res.Records[0]
	if r.Year != 2023 || r.ExporterCode != "TWN" || r.TradeValueUSD != 1000 {
		t.Errorf("misaligned parse: %+v", r)
	}
}

func TestParseCSVMissingColumns(t *testing.T) {
	csv := "year,exporter_code,importer_code\n2023,TWN,USA\n"
	_, err := ParseCSV(strings.NewReader(csv), time.Now().UTC())
	if err == nil {
		t.Fatal("expected error for missing required columns")
	}
	if !strings.Contains(err.Error(), "missing required column") {
		t.Errorf("error = %v, want missing-column message", err)
	}
}

func TestParseCSVEmpty(t *testing.T) {
	_, err := ParseCSV(strings.NewReader(""), time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty-file error, got %v", err)
	}
}

func TestParseCSVSkipsMalformedRows(t *testing.T) {
	csv := `year,exporter_code,exporter_name,importer_code,importer_name,commodity_code,commodity_name,trade_value_usd,quantity,unit
2023,TWN,Taiwan,USA,United States,8542,semiconductors,85000000000,0,USD
notayear,TWN,Taiwan,USA,United States,8542,semiconductors,1,0,USD
2023,TWN,Taiwan,USA,United States,8542,semiconductors,notanumber,0,USD
2023,,Taiwan,USA,United States,8542,semiconductors,1,0,USD
2023,TWN,Taiwan,USA,United States,8542,semiconductors,-5,0,USD
`
	res, err := ParseCSV(strings.NewReader(csv), time.Now().UTC())
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if res.TotalRows != 5 {
		t.Fatalf("total rows = %d, want 5", res.TotalRows)
	}
	if res.ValidRows() != 1 {
		t.Fatalf("valid rows = %d, want 1", res.ValidRows())
	}
	if len(res.Skipped) != 4 {
		t.Fatalf("skipped = %d, want 4: %+v", len(res.Skipped), res.Skipped)
	}
	// Reasons should be specific and carry the source line number.
	joined := ""
	for _, s := range res.Skipped {
		joined += s.Reason + "\n"
	}
	for _, want := range []string{"invalid year", "invalid trade_value_usd", "missing exporter_code", "negative trade_value_usd"} {
		if !strings.Contains(joined, want) {
			t.Errorf("skip reasons missing %q; got:\n%s", want, joined)
		}
	}
	if res.Skipped[0].Line != 3 {
		t.Errorf("first malformed row should be line 3, got %d", res.Skipped[0].Line)
	}
}

func TestParseAmountHandlesCommasAndBlanks(t *testing.T) {
	v, reason := parseAmount("1,234,567", "trade_value_usd")
	if reason != "" || v != 1234567 {
		t.Errorf("parseAmount commas = %v (%q)", v, reason)
	}
	v, reason = parseAmount("", "quantity")
	if reason != "" || v != 0 {
		t.Errorf("blank should be 0, got %v (%q)", v, reason)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	res, err := ParseCSV(strings.NewReader(goodCSV), time.Now().UTC())
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	SortRecords(res.Records)
	in := TradeFile{Source: SourceName, IngestedAt: time.Now().UTC(), SourceFile: "x.csv", Records: res.Records}
	path, err := Save(dir, in)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if filepath.Base(path) != OutputFileName {
		t.Errorf("saved file = %q, want %q", filepath.Base(path), OutputFileName)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Records) != len(in.Records) {
		t.Fatalf("round-trip record count = %d, want %d", len(got.Records), len(in.Records))
	}
	if got.Records[0].ExporterCode != in.Records[0].ExporterCode {
		t.Errorf("round-trip mismatch: %+v vs %+v", got.Records[0], in.Records[0])
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "reading") {
		t.Fatalf("expected read error, got %v", err)
	}
}
