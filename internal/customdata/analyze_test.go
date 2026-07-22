package customdata

import (
	"math"
	"strings"
	"testing"
)

func TestAnalyzeValidCSVComputesHHI(t *testing.T) {
	csv := "importer,commodity,supplier,value_usd\nUnited States,Semiconductors,Taiwan,60\nUnited States, semiconductors ,Korea,40\n"
	got, err := Analyze(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if got.DatasetSummary.ValidRows != 2 || got.DatasetSummary.InvalidRows != 0 {
		t.Fatalf("summary = %+v", got.DatasetSummary)
	}
	if len(got.ConcentrationResults) != 1 {
		t.Fatalf("results = %d, want 1", len(got.ConcentrationResults))
	}
	result := got.ConcentrationResults[0]
	if math.Abs(result.HHI-0.52) > 1e-9 {
		t.Errorf("HHI = %.4f, want 0.52", result.HHI)
	}
	if result.TopSupplier != "Taiwan" || math.Abs(result.TopSupplierShare-0.60) > 1e-9 {
		t.Errorf("top supplier = %s %.3f", result.TopSupplier, result.TopSupplierShare)
	}
	if result.ConcentrationRisk != "High" {
		t.Errorf("risk = %s, want High", result.ConcentrationRisk)
	}
	if len(got.ValidRows) != 2 {
		t.Fatalf("normalized rows = %d, want 2", len(got.ValidRows))
	}
	if got.ValidRows[0].Commodity != "semiconductors" || got.ValidRows[0].Supplier != "Taiwan" {
		t.Errorf("normalized row = %+v", got.ValidRows[0])
	}
}

func TestAnalyzeInvalidRowsReturnsValidationErrors(t *testing.T) {
	csv := "importer,commodity,supplier,value_usd\n,oil,Saudi Arabia,abc\nUSA,,Canada,10\nUSA,oil,,20\nUSA,oil,Canada,-5\n"
	got, err := Analyze(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if got.DatasetSummary.InvalidRows != 4 || got.DatasetSummary.ValidRows != 0 {
		t.Fatalf("summary = %+v", got.DatasetSummary)
	}
	if len(got.ValidationErrors) < 4 {
		t.Fatalf("validation errors = %+v", got.ValidationErrors)
	}
}

func TestAnalyzeAcceptsAliases(t *testing.T) {
	csv := "importer_name,commodity,exporter,trade_value_usd\nGermany,Copper,Chile,100\n"
	got, err := Analyze(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if got.DatasetSummary.ValidRows != 1 {
		t.Fatalf("valid rows = %d", got.DatasetSummary.ValidRows)
	}
	result := got.ConcentrationResults[0]
	if result.Importer != "Germany" || result.Commodity != "copper" || result.TopSupplier != "Chile" {
		t.Errorf("alias result = %+v", result)
	}
}

func TestAnalyzeEmptyCSVFailsCleanly(t *testing.T) {
	if _, err := Analyze(strings.NewReader("")); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error = %v, want empty CSV error", err)
	}
	if _, err := Analyze(strings.NewReader("importer,commodity,supplier,value_usd\n")); err == nil ||
		!strings.Contains(err.Error(), "no data rows") {
		t.Fatalf("error = %v, want no data rows error", err)
	}
}

func TestAnalyzeMissingRequiredColumnsFailsCleanly(t *testing.T) {
	_, err := Analyze(strings.NewReader("importer,commodity,value_usd\nUSA,oil,10\n"))
	if err == nil || !strings.Contains(err.Error(), "supplier") {
		t.Fatalf("error = %v, want missing supplier column", err)
	}
}
