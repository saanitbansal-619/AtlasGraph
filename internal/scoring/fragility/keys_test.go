package fragility

import (
	"testing"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/models"
)

func TestCountryKeyAliases(t *testing.T) {
	cases := []struct {
		code, name, want string
	}{
		{"", "Taiwan", "TWN"},
		{"TWN", "Taiwan", "TWN"},
		{"", "South Korea", "KOR"},
		{"KOR", "Korea, Rep.", "KOR"},
		{"", "Korea, Rep.", "KOR"},
		{"", "United States", "USA"},
		{"USA", "United States", "USA"},
		{"", "DRC", "COD"},
		{"", "Democratic Republic of the Congo", "COD"},
		{"", "Dem. Rep. of the Congo", "COD"},
		{"", "Germany", "DEU"},
		{"DEU", "Germany", "DEU"},
	}
	for _, tc := range cases {
		if got := countryKey(tc.code, tc.name); got != tc.want {
			t.Errorf("countryKey(%q, %q) = %q, want %q", tc.code, tc.name, got, tc.want)
		}
	}
}

func TestCommodityKeyNormalization(t *testing.T) {
	cases := []struct {
		code, name, want string
	}{
		{"", "crude oil", "crude_oil"},
		{"crude_oil", "", "crude_oil"},
		{"", "crude_oil", "crude_oil"},
		{"", "semiconductors", "semiconductors"},
		{"", "lithium carbonate", "lithium_carbonate"},
		{"", "lithium batteries", "lithium_batteries"},
		{"", "cobalt", "cobalt"},
		{"", "cobalt ores", "cobalt_ores"},
	}
	for _, tc := range cases {
		if got := commodityKey(tc.code, tc.name); got != tc.want {
			t.Errorf("commodityKey(%q, %q) = %q, want %q", tc.code, tc.name, got, tc.want)
		}
	}
}

func TestMergeCountryRefCombinesAliases(t *testing.T) {
	a := mergeCountryRef(countryRef{name: "South Korea"}, countryRef{code: "KOR", name: "Korea, Rep."})
	if a.code != "KOR" {
		t.Fatalf("code = %q, want KOR", a.code)
	}
	if a.name != "South Korea" {
		t.Fatalf("name = %q, want South Korea", a.name)
	}

	b := mergeCountryRef(countryRef{name: "DRC"}, countryRef{name: "Democratic Republic of the Congo"})
	if countryKey(b.code, b.name) != "COD" {
		t.Fatalf("DRC aliases should merge to COD, got %q/%q", b.code, b.name)
	}
}

func TestMergeCommodityRefCombinesAliases(t *testing.T) {
	a := mergeCommodityRef(commodityRef{name: "crude oil"}, commodityRef{code: "crude_oil"})
	if commodityKey(a.code, a.name) != "crude_oil" {
		t.Fatalf("commodity key = %q, want crude_oil", commodityKey(a.code, a.name))
	}
	if a.name != "crude oil" {
		t.Fatalf("display name = %q, want crude oil", a.name)
	}
}

func TestDuplicateCountriesMergeInScore(t *testing.T) {
	ds, err := dataDefault()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}

	tradeFile := tradeFileWithKoreaAliases()
	macroFile := macroFileWithKoreaAliases()

	res := Score(Sources{
		Graph:  ds.Graph,
		Trade:  &tradeFile,
		Macro:  &macroFile,
		Config: defaultCfg(),
	})

	koreaCount := 0
	for _, c := range res.Countries {
		if countryKey(c.CountryCode, c.CountryName) == "KOR" {
			koreaCount++
		}
	}
	if koreaCount != 1 {
		t.Fatalf("expected exactly one South Korea row, got %d", koreaCount)
	}
}

func TestDuplicateCommoditiesMergeInScore(t *testing.T) {
	g := graphWithCommodityAliases()

	res := Score(Sources{
		Graph: g,
		Trade: ptrTrade(tradeFileWithCrudeOilAliases()),
	})

	crudeCount := 0
	for _, c := range res.Commodities {
		if commodityKey(c.CommodityCode, c.CommodityName) == "crude_oil" {
			crudeCount++
		}
	}
	if crudeCount != 1 {
		t.Fatalf("expected exactly one crude oil row, got %d", crudeCount)
	}
}

func tradeFileWithKoreaAliases() trade.TradeFile {
	f := sampleTradeFile()
	f.Records = append(f.Records, trade.TradeFlowRecord{
		Year: 2023, ExporterCode: "KOR", ExporterName: "Korea, Rep.",
		ImporterCode: "DEU", ImporterName: "Germany",
		CommodityCode: "8542", CommodityName: "semiconductors", TradeValueUSD: 5e9,
	})
	return f
}

func macroFileWithKoreaAliases() worldbank.IndicatorFile {
	f := sampleMacroFile()
	rec := func(code, name, ind string, year int, v float64) worldbank.CountryIndicatorRecord {
		val := v
		return worldbank.CountryIndicatorRecord{
			CountryCode: code, CountryName: name, IndicatorCode: ind,
			Year: year, Value: &val, Source: worldbank.SourceName,
		}
	}
	f.Records = append(f.Records,
		rec("KOR", "Korea, Rep.", "NY.GDP.MKTP.CD", 2023, 1.7e12),
		rec("KOR", "Korea, Rep.", "NE.IMP.GNFS.ZS", 2023, 38.0),
		rec("KOR", "Korea, Rep.", "NE.EXP.GNFS.ZS", 2023, 44.0),
		rec("KOR", "Korea, Rep.", "NV.IND.MANF.ZS", 2023, 25.0),
		rec("KOR", "Korea, Rep.", "FP.CPI.TOTL.ZG", 2023, 3.0),
		rec("KOR", "Korea, Rep.", "TX.VAL.TECH.CD", 2023, 1.2e11),
	)
	return f
}

func tradeFileWithCrudeOilAliases() trade.TradeFile {
	return trade.TradeFile{
		Records: []trade.TradeFlowRecord{
			{Year: 2023, ExporterCode: "SAU", ExporterName: "Saudi Arabia", ImporterCode: "USA", ImporterName: "United States", CommodityCode: "2709", CommodityName: "crude oil", TradeValueUSD: 10e9},
		},
	}
}

func graphWithCommodityAliases() *graph.Graph {
	g := graph.New()
	g.AddNode(models.NewNode(models.Commodity, "crude oil"))
	g.AddNode(models.NewNode(models.Commodity, "crude_oil"))
	return g
}

func dataDefault() (*data.Dataset, error) {
	return data.Default()
}

func defaultCfg() config.Config {
	return config.Default()
}

func ptrTrade(f trade.TradeFile) *trade.TradeFile { return &f }
func ptrMacro(f worldbank.IndicatorFile) *worldbank.IndicatorFile { return &f }
func ptrEvents(f gdelt.EventFile) *gdelt.EventFile { return &f }

func TestOutputNoDuplicateCanonicalKeys(t *testing.T) {
	ds, err := dataDefault()
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}

	res := Score(Sources{
		Graph:       ds.Graph,
		Scenarios:   ds.Scenarios,
		Trade:       ptrTrade(sampleTradeFile()),
		Macro:       ptrMacro(sampleMacroFile()),
		Events:      ptrEvents(sampleEventFile()),
		Config:      defaultCfg(),
	})

	seenCountries := map[string]bool{}
	for _, c := range res.Countries {
		key := countryKey(c.CountryCode, c.CountryName)
		if seenCountries[key] {
			t.Errorf("duplicate country canonical key %q (%s)", key, c.CountryName)
		}
		seenCountries[key] = true
	}

	seenCommodities := map[string]bool{}
	for _, c := range res.Commodities {
		key := commodityKey(c.CommodityCode, c.CommodityName)
		if seenCommodities[key] {
			t.Errorf("duplicate commodity canonical key %q (%s)", key, c.CommodityName)
		}
		seenCommodities[key] = true
	}
}
