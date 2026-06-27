package trade

import (
	"strings"
	"testing"
	"time"
)

const comtradeHeader = "refYear,flowDesc,reporterISO,reporterDesc,partnerISO,partnerDesc,cmdCode,cmdDesc,primaryValue,qty,qtyUnitAbbr\n"

func parseComtrade(t *testing.T, body string) ComtradeLoadResult {
	t.Helper()
	res, err := ParseComtradeCSV(strings.NewReader(comtradeHeader+body), time.Now().UTC())
	if err != nil {
		t.Fatalf("ParseComtradeCSV: %v", err)
	}
	return res
}

func TestComtradeImportFlowMapsReporterAsImporter(t *testing.T) {
	// Import: reporter is the importer, partner is the exporter.
	res := parseComtrade(t, "2023,Import,USA,United States,TWN,Taiwan,8542,Electronic integrated circuits,85000000000,0,N/A\n")
	if res.ValidRows() != 1 {
		t.Fatalf("valid rows = %d, want 1", res.ValidRows())
	}
	if res.FlowsImported != 1 || res.FlowsExported != 0 {
		t.Fatalf("flow counts = imp %d/exp %d, want 1/0", res.FlowsImported, res.FlowsExported)
	}
	r := res.Records[0]
	if r.ImporterCode != "USA" || r.ImporterName != "United States" {
		t.Errorf("importer = %s/%s, want USA/United States", r.ImporterCode, r.ImporterName)
	}
	if r.ExporterCode != "TWN" || r.ExporterName != "Taiwan" {
		t.Errorf("exporter = %s/%s, want TWN/Taiwan", r.ExporterCode, r.ExporterName)
	}
	if r.Source != ComtradeSourceName {
		t.Errorf("source = %q, want %q", r.Source, ComtradeSourceName)
	}
}

func TestComtradeExportFlowMapsReporterAsExporter(t *testing.T) {
	// Export: reporter is the exporter, partner is the importer.
	res := parseComtrade(t, "2023,Export,TWN,Taiwan,JPN,Japan,8542,Electronic integrated circuits,32000000000,0,N/A\n")
	if res.ValidRows() != 1 {
		t.Fatalf("valid rows = %d, want 1", res.ValidRows())
	}
	if res.FlowsExported != 1 || res.FlowsImported != 0 {
		t.Fatalf("flow counts = imp %d/exp %d, want 0/1", res.FlowsImported, res.FlowsExported)
	}
	r := res.Records[0]
	if r.ExporterCode != "TWN" || r.ExporterName != "Taiwan" {
		t.Errorf("exporter = %s/%s, want TWN/Taiwan", r.ExporterCode, r.ExporterName)
	}
	if r.ImporterCode != "JPN" || r.ImporterName != "Japan" {
		t.Errorf("importer = %s/%s, want JPN/Japan", r.ImporterCode, r.ImporterName)
	}
}

func TestComtradeCommodityNormalization(t *testing.T) {
	cases := []struct {
		name    string
		cmdCode string
		cmdDesc string
		want    string
	}{
		{"code 8542 -> semiconductors", "8542", "Some unrelated text", "semiconductors"},
		{"desc integrated circuits -> semiconductors", "9999", "Electronic integrated circuits and parts", "semiconductors"},
		{"lithium batteries", "8507", "Electric accumulators; lithium-ion batteries", "lithium batteries"},
		{"cobalt ores", "2605", "Cobalt ores and concentrates", "cobalt ores"},
		{"crude oil", "2709", "Petroleum oils, crude", "crude oil"},
		{"rare earths", "2805", "Rare earth metals and compounds", "rare earths"},
		{"fallback cleaned lowercase", "7108", "  Gold,   Unwrought  ", "gold, unwrought"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeComtradeCommodity(c.cmdDesc, c.cmdCode); got != c.want {
				t.Errorf("normalizeComtradeCommodity(%q,%q) = %q, want %q", c.cmdDesc, c.cmdCode, got, c.want)
			}
		})
	}
}

func TestComtradeNormalizesCode8542InFullRow(t *testing.T) {
	res := parseComtrade(t, "2023,Export,TWN,Taiwan,JPN,Japan,8542,Goods not described,1000,0,N/A\n")
	if res.ValidRows() != 1 {
		t.Fatalf("valid rows = %d, want 1", res.ValidRows())
	}
	if got := res.Records[0].CommodityName; got != "semiconductors" {
		t.Errorf("commodity = %q, want semiconductors", got)
	}
	if got := res.Records[0].CommodityCode; got != "8542" {
		t.Errorf("commodity code = %q, want 8542", got)
	}
}

func TestComtradeSkipsMissingAndUnsupportedRows(t *testing.T) {
	body := "" +
		"2023,Export,TWN,Taiwan,JPN,Japan,8542,Electronic integrated circuits,1000,0,N/A\n" + // valid
		"2023,Re-export,TWN,Taiwan,JPN,Japan,8542,Electronic integrated circuits,1000,0,N/A\n" + // unsupported flow
		"2023,Export,,Taiwan,JPN,Japan,8542,Electronic integrated circuits,1000,0,N/A\n" + // missing reporterISO
		"2023,Import,USA,United States,,Taiwan,8542,Electronic integrated circuits,1000,0,N/A\n" + // missing partnerISO
		"2023,Export,TWN,Taiwan,JPN,Japan,,Electronic integrated circuits,1000,0,N/A\n" + // missing cmdCode
		"2023,Export,TWN,Taiwan,JPN,Japan,8542,Electronic integrated circuits,,0,N/A\n" + // missing primaryValue
		"notayear,Export,TWN,Taiwan,JPN,Japan,8542,Electronic integrated circuits,1000,0,N/A\n" // bad year

	res := parseComtrade(t, body)
	if res.TotalRows != 7 {
		t.Fatalf("total rows = %d, want 7", res.TotalRows)
	}
	if res.ValidRows() != 1 {
		t.Fatalf("valid rows = %d, want 1", res.ValidRows())
	}
	if len(res.Skipped) != 6 {
		t.Fatalf("skipped = %d, want 6: %+v", len(res.Skipped), res.Skipped)
	}
	joined := ""
	for _, s := range res.Skipped {
		joined += s.Reason + "\n"
	}
	for _, want := range []string{
		"unsupported flow", "missing reporterISO or partnerISO", "missing cmdCode",
		"missing primaryValue", "invalid refYear",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("skip reasons missing %q; got:\n%s", want, joined)
		}
	}
}

func TestComtradeMissingColumns(t *testing.T) {
	csv := "refYear,flowDesc,reporterISO\n2023,Export,TWN\n"
	_, err := ParseComtradeCSV(strings.NewReader(csv), time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "missing required column") {
		t.Fatalf("expected missing-column error, got %v", err)
	}
}

func TestComtradeEmpty(t *testing.T) {
	_, err := ParseComtradeCSV(strings.NewReader(""), time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty-file error, got %v", err)
	}
}

// TestComtradeOutputFeedsAnalysis verifies normalised Comtrade records flow
// straight into the existing supplier-dependency analysis: an import-reported
// flow and an export-reported flow into the same importer aggregate correctly.
func TestComtradeOutputFeedsAnalysis(t *testing.T) {
	body := "" +
		"2023,Import,USA,United States,TWN,Taiwan,8542,Electronic integrated circuits,80000000000,0,N/A\n" +
		"2023,Export,KOR,Korea,USA,United States,8542,Electronic integrated circuits,20000000000,0,N/A\n"
	res := parseComtrade(t, body)
	SortRecords(res.Records)
	tf := TradeFile{Source: ComtradeSourceName, Records: res.Records}

	dep := BuildDependency(tf, "USA", "semiconductors")
	if !dep.HasData {
		t.Fatal("expected dependency data for USA/semiconductors")
	}
	if dep.TotalImportsUSD != 100e9 {
		t.Fatalf("total imports = %v, want 100e9", dep.TotalImportsUSD)
	}
	if len(dep.Suppliers) != 2 || dep.Suppliers[0].ExporterCode != "TWN" {
		t.Fatalf("top supplier = %+v, want TWN first", dep.Suppliers)
	}
	if got := dep.Suppliers[0].Share; got < 0.79 || got > 0.81 {
		t.Errorf("Taiwan share = %.3f, want ~0.80", got)
	}
}
