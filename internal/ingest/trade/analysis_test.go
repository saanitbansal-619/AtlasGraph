package trade

import (
	"math"
	"testing"
	"time"
)

func approx(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// sampleFile builds a small panel: USA imports semiconductors from three
// suppliers (60/30/10 split) plus an unrelated crude-oil flow.
func sampleFile() TradeFile {
	rec := func(exCode, exName, imCode, imName, comCode, comName string, val float64) TradeFlowRecord {
		return TradeFlowRecord{
			Year: 2023, ExporterCode: exCode, ExporterName: exName,
			ImporterCode: imCode, ImporterName: imName,
			CommodityCode: comCode, CommodityName: comName,
			TradeValueUSD: val, Unit: "USD", Source: SourceName, IngestedAt: time.Now().UTC(),
		}
	}
	return TradeFile{Records: []TradeFlowRecord{
		rec("TWN", "Taiwan", "USA", "United States", "8542", "semiconductors", 60),
		rec("KOR", "Korea Rep.", "USA", "United States", "8542", "semiconductors", 30),
		rec("JPN", "Japan", "USA", "United States", "8542", "semiconductors", 10),
		rec("SAU", "Saudi Arabia", "DEU", "Germany", "2709", "crude oil", 50),
	}}
}

func TestBuildSummary(t *testing.T) {
	s := BuildSummary(sampleFile(), 5)
	if s.Records != 4 {
		t.Errorf("records = %d, want 4", s.Records)
	}
	approx(t, "total value", s.TotalValueUSD, 150)
	// Countries: TWN, KOR, JPN, USA, SAU, DEU = 6 unique codes.
	if s.Countries != 6 {
		t.Errorf("countries = %d, want 6", s.Countries)
	}
	if s.Commodities != 2 {
		t.Errorf("commodities = %d, want 2", s.Commodities)
	}
	if len(s.Years) != 1 || s.Years[0] != 2023 {
		t.Errorf("years = %v, want [2023]", s.Years)
	}
	// USA imports 100 (semiconductors) > DEU's 50 (crude oil).
	if s.TopImporters[0].Code != "USA" {
		t.Errorf("top importer = %q, want USA", s.TopImporters[0].Code)
	}
	// semiconductors total 100 > crude oil 50.
	if s.TopCommodities[0].Code != "8542" {
		t.Errorf("top commodity unexpected: %+v", s.TopCommodities[0])
	}
}

func TestBuildDependencyShares(t *testing.T) {
	dep := BuildDependency(sampleFile(), "USA", "semiconductors")
	if !dep.HasData {
		t.Fatal("expected data")
	}
	approx(t, "total imports", dep.TotalImportsUSD, 100)
	if len(dep.Suppliers) != 3 {
		t.Fatalf("suppliers = %d, want 3", len(dep.Suppliers))
	}
	// Sorted by value desc: Taiwan, Korea, Japan.
	if dep.Suppliers[0].ExporterCode != "TWN" {
		t.Errorf("top supplier = %q, want TWN", dep.Suppliers[0].ExporterCode)
	}
	approx(t, "taiwan share", dep.Suppliers[0].Share, 0.60)
	approx(t, "korea share", dep.Suppliers[1].Share, 0.30)
	approx(t, "japan share", dep.Suppliers[2].Share, 0.10)
	if dep.Suppliers[0].Dependency != "High" {
		t.Errorf("taiwan dependency = %q, want High", dep.Suppliers[0].Dependency)
	}
	if dep.Suppliers[1].Dependency != "Medium" {
		t.Errorf("korea dependency = %q, want Medium", dep.Suppliers[1].Dependency)
	}
	// A 0.10 share sits exactly on the Medium floor (Low is strictly < 0.10).
	if dep.Suppliers[2].Dependency != "Medium" {
		t.Errorf("japan dependency = %q, want Medium", dep.Suppliers[2].Dependency)
	}
}

func TestBuildDependencyMatchesByNameAndCode(t *testing.T) {
	// Importer by name, commodity by HS code.
	dep := BuildDependency(sampleFile(), "United States", "8542")
	if !dep.HasData || len(dep.Suppliers) != 3 {
		t.Fatalf("name/code match failed: %+v", dep)
	}
}

func TestBuildDependencyNoMatch(t *testing.T) {
	dep := BuildDependency(sampleFile(), "BRA", "semiconductors")
	if dep.HasData {
		t.Errorf("expected no data for unknown importer")
	}
}

func TestBuildConcentrationHHI(t *testing.T) {
	con := BuildConcentration(sampleFile(), "USA", "semiconductors")
	if !con.HasData {
		t.Fatal("expected data")
	}
	// HHI = 0.6^2 + 0.3^2 + 0.1^2 = 0.36 + 0.09 + 0.01 = 0.46.
	approx(t, "hhi", con.HHI, 0.46)
	if con.RiskLevel != "High" {
		t.Errorf("risk = %q, want High", con.RiskLevel)
	}
	if con.TopSupplier.ExporterCode != "TWN" {
		t.Errorf("top supplier = %q, want TWN", con.TopSupplier.ExporterCode)
	}
}

func TestConcentrationRiskBands(t *testing.T) {
	cases := []struct {
		hhi  float64
		want string
	}{
		{0.0, "Low"}, {0.149, "Low"}, {0.15, "Medium"}, {0.25, "Medium"},
		{0.2501, "High"}, {1.0, "High"},
	}
	for _, c := range cases {
		if got := ConcentrationRisk(c.hhi); got != c.want {
			t.Errorf("ConcentrationRisk(%.4f) = %q, want %q", c.hhi, got, c.want)
		}
	}
}

func TestSupplierDependencyBands(t *testing.T) {
	cases := []struct {
		share float64
		want  string
	}{
		{0.0, "Low"}, {0.099, "Low"}, {0.10, "Medium"}, {0.39, "Medium"},
		{0.40, "High"}, {1.0, "High"},
	}
	for _, c := range cases {
		if got := SupplierDependencyBand(c.share); got != c.want {
			t.Errorf("SupplierDependencyBand(%.3f) = %q, want %q", c.share, got, c.want)
		}
	}
}
