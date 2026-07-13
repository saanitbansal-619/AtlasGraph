package trade

import "testing"

func TestCountryCodeForNameKoreaTaiwanAliases(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Rep. of Korea", "KOR"},
		{"Republic of Korea", "KOR"},
		{"Korea, Rep.", "KOR"},
		{"South Korea", "KOR"},
		{"Other Asia, nes", "TWN"},
		{"Other Asia, not elsewhere specified", "TWN"},
		{"Taiwan", "TWN"},
		{"Taiwan, China", "TWN"},
	}
	for _, tc := range cases {
		if got := CountryCodeForName(tc.name); got != tc.want {
			t.Errorf("CountryCodeForName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNormalizeCountryNameKoreaTaiwanAliases(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"Rep. of Korea", "Korea, Rep."},
		{"Republic of Korea", "Korea, Rep."},
		{"Korea, Rep.", "Korea, Rep."},
		{"Other Asia, nes", "Taiwan"},
		{"Other Asia, not elsewhere specified", "Taiwan"},
		{"Taiwan, China", "Taiwan"},
		{"Taiwan", "Taiwan"},
	}
	for _, tc := range cases {
		if got := NormalizeCountryName(tc.raw); got != tc.want {
			t.Errorf("NormalizeCountryName(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestBuildSummaryFillsMissingCountryCodes(t *testing.T) {
	file := TradeFile{
		Records: []TradeFlowRecord{
			{
				Year: 2024, ExporterCode: "", ExporterName: "Rep. of Korea",
				ImporterCode: "", ImporterName: "United States",
				CommodityCode: "8542", CommodityName: "semiconductors", TradeValueUSD: 100,
			},
			{
				Year: 2024, ExporterCode: "", ExporterName: "Other Asia, nes",
				ImporterCode: "USA", ImporterName: "United States",
				CommodityCode: "8542", CommodityName: "semiconductors", TradeValueUSD: 50,
			},
		},
	}
	s := BuildSummary(file, 5)
	if len(s.TopExporters) < 2 {
		t.Fatalf("expected 2 exporters, got %d", len(s.TopExporters))
	}
	byName := map[string]string{}
	for _, e := range s.TopExporters {
		byName[e.Name] = e.Code
	}
	if byName["Korea, Rep."] != "KOR" {
		t.Errorf("Korea exporter code = %q, want KOR (name=%v)", byName["Korea, Rep."], byName)
	}
	if byName["Taiwan"] != "TWN" {
		t.Errorf("Taiwan exporter code = %q, want TWN (name=%v)", byName["Taiwan"], byName)
	}
	if s.TopImporters[0].Code != "USA" {
		t.Errorf("top importer code = %q, want USA", s.TopImporters[0].Code)
	}
}

func TestTradeDependencyToRecordNormalizesCodes(t *testing.T) {
	r := TradeDependencyToRecord(TradeDependency{
		Importer: "United States", Exporter: "Rep. of Korea",
		Commodity: "semiconductors", HSCode: "8542", Year: 2024, TradeValueUSD: 10,
	})
	if r.ExporterCode != "KOR" || r.ExporterName != "Korea, Rep." {
		t.Errorf("exporter = %s/%s, want KOR/Korea, Rep.", r.ExporterCode, r.ExporterName)
	}
	r2 := TradeDependencyToRecord(TradeDependency{
		Importer: "United States", Exporter: "Other Asia, nes",
		Commodity: "semiconductors", HSCode: "8542", Year: 2024, TradeValueUSD: 10,
	})
	if r2.ExporterCode != "TWN" || r2.ExporterName != "Taiwan" {
		t.Errorf("exporter = %s/%s, want TWN/Taiwan", r2.ExporterCode, r2.ExporterName)
	}
}
