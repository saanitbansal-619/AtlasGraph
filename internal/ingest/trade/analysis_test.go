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
	wantCommodities := []string{"crude oil", "semiconductors"}
	if len(s.AvailableCommodities) != len(wantCommodities) {
		t.Fatalf("available_commodities = %v, want %v", s.AvailableCommodities, wantCommodities)
	}
	for i, want := range wantCommodities {
		if s.AvailableCommodities[i] != want {
			t.Errorf("available_commodities[%d] = %q, want %q", i, s.AvailableCommodities[i], want)
		}
	}
	if len(s.AvailableImporters) < 2 {
		t.Fatalf("available_importers = %v, want at least USA and Germany", s.AvailableImporters)
	}
	if len(s.TopCommodities) != 2 {
		t.Errorf("top_commodities = %d, want 2 (all commodities in sample)", len(s.TopCommodities))
	}
}

func TestAvailableImportReportersFiltersExports(t *testing.T) {
	df := DependencyFile{
		Source: ComtradeRealSourceName,
		Dependencies: []TradeDependency{
			{Importer: "Germany", Exporter: "Norway", Commodity: "natural gas", Flow: FlowImport},
			{Importer: "China", Exporter: "Australia", Commodity: "iron ore", Flow: FlowImport},
			{Importer: "Ukraine", Exporter: "Algeria", Commodity: "crude oil", Flow: FlowExport},
		},
	}
	got := AvailableImportReporters(df)
	want := []string{"China", "Germany"}
	if len(got) != len(want) {
		t.Fatalf("AvailableImportReporters = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildTradeOptionsImporterCommodities(t *testing.T) {
	resolved := ResolvedTrade{
		DependencyFile: &DependencyFile{
			Source: ComtradeRealSourceName,
			Dependencies: []TradeDependency{
				{Importer: "India", Exporter: "Saudi Arabia", Commodity: "crude oil", Flow: FlowImport},
				{Importer: "India", Exporter: "Russia", Commodity: "fertilizer", Flow: FlowImport},
				{Importer: "United States", Exporter: "Taiwan", Commodity: "semiconductors", Flow: FlowImport},
				{Importer: "Ukraine", Exporter: "Algeria", Commodity: "crude oil", Flow: FlowExport},
			},
		},
	}
	opts := BuildTradeOptions(resolved)
	if len(opts.Importers) != 2 {
		t.Fatalf("importers = %d, want 2 (India, United States); got %+v", len(opts.Importers), opts.Importers)
	}
	byName := map[string][]string{}
	for _, im := range opts.Importers {
		byName[im.Name] = im.Commodities
	}
	india := byName["India"]
	if len(india) != 2 || india[0] != "crude oil" || india[1] != "fertilizer" {
		t.Errorf("India commodities = %v, want [crude oil fertilizer]", india)
	}
	if _, ok := byName["India"]; ok {
		for _, c := range byName["India"] {
			if c == "semiconductors" {
				t.Fatal("India must not list semiconductors")
			}
		}
	}
	usa := byName["United States"]
	if len(usa) != 1 || usa[0] != "semiconductors" {
		t.Errorf("United States commodities = %v, want [semiconductors]", usa)
	}
	if _, ok := byName["Ukraine"]; ok {
		t.Fatal("export-partner Ukraine must not appear as importer option")
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

func TestBuildDependencyGroupsByExporterNameWhenCodeMissing(t *testing.T) {
	file := TradeFile{Records: []TradeFlowRecord{
		{Year: 2024, ExporterName: "China", ImporterName: "United States", CommodityName: "semiconductors", TradeValueUSD: 60},
		{Year: 2024, ExporterName: "Taiwan", ImporterName: "United States", CommodityName: "semiconductors", TradeValueUSD: 30},
		{Year: 2024, ExporterName: "Japan", ImporterName: "United States", CommodityName: "semiconductors", TradeValueUSD: 10},
	}}
	dep := BuildDependency(file, "United States", "semiconductors")
	if len(dep.Suppliers) != 3 {
		t.Fatalf("suppliers = %d, want 3", len(dep.Suppliers))
	}
	approx(t, "total imports", dep.TotalImportsUSD, 100)
	approx(t, "hhi", concentrationFromDependency(dep).HHI, 0.46)
}

func TestBuildDependencyFromDependenciesThreeSuppliers(t *testing.T) {
	df := DependencyFile{
		Source: ComtradeRealSourceName,
		Dependencies: []TradeDependency{
			{Importer: "United States", Exporter: "China", Commodity: "semiconductors", HSCode: "8542", Year: 2024, TradeValueUSD: 60, Share: 0.6},
			{Importer: "United States", Exporter: "Taiwan", Commodity: "semiconductors", HSCode: "8542", Year: 2024, TradeValueUSD: 30, Share: 0.3},
			{Importer: "United States", Exporter: "Japan", Commodity: "semiconductors", HSCode: "8542", Year: 2024, TradeValueUSD: 10, Share: 0.1},
		},
	}
	dep := BuildDependencyFromDependencies(df, "United States", "semiconductors")
	if len(dep.Suppliers) != 3 {
		t.Fatalf("suppliers = %d, want 3", len(dep.Suppliers))
	}
	approx(t, "total imports", dep.TotalImportsUSD, 100)
	if dep.Suppliers[0].ExporterName != "China" {
		t.Fatalf("top supplier = %q, want China", dep.Suppliers[0].ExporterName)
	}
	con := concentrationFromDependency(dep)
	approx(t, "hhi", con.HHI, 0.46)
	if con.TopSupplier.ExporterName != "China" {
		t.Fatalf("top supplier = %q, want China", con.TopSupplier.ExporterName)
	}
}

func TestBuildDependencyResolvedUsesDependencyFile(t *testing.T) {
	df := DependencyFile{
		Source: ComtradeRealSourceName,
		Dependencies: []TradeDependency{
			{Importer: "United States", Exporter: "China", Commodity: "semiconductors", TradeValueUSD: 60, Share: 0.6},
			{Importer: "United States", Exporter: "Taiwan", Commodity: "semiconductors", TradeValueUSD: 30, Share: 0.3},
			{Importer: "United States", Exporter: "Japan", Commodity: "semiconductors", TradeValueUSD: 10, Share: 0.1},
		},
	}
	resolved := ResolvedTrade{
		Source: ComtradeRealSourceName, RealTradeData: true, DependencyFile: &df,
		File: DependenciesToTradeFile(df),
	}
	dep := BuildDependencyResolved(resolved, "USA", "semiconductors")
	if len(dep.Suppliers) != 3 {
		t.Fatalf("suppliers = %d, want 3", len(dep.Suppliers))
	}
}
