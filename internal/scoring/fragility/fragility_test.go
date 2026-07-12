package fragility

import (
	"strings"
	"testing"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/models"
)

func TestRiskLevelBands(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0, "Low"},
		{29.9, "Low"},
		{30, "Medium"},
		{59.9, "Medium"},
		{60, "High"},
		{79.9, "High"},
		{80, "Critical"},
		{100, "Critical"},
	}
	for _, tc := range cases {
		if got := RiskLevel(tc.score); got != tc.want {
			t.Errorf("RiskLevel(%v) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestBlendRenormalizesMissingComponents(t *testing.T) {
	comps := []Component{
		makeComponent("a", "alpha", 0.30, 60, true),
		makeComponent("b", "beta", 0.25, 0, false),
		makeComponent("c", "gamma", 0.25, 40, true),
		makeComponent("d", "delta", 0.20, 0, false),
	}
	got := blend(comps)
	// Only 0.30+0.25 = 0.55 weight available: (0.30*60 + 0.25*40) / 0.55
	want := (0.30*60 + 0.25*40) / 0.55
	if diff := got - want; diff < -0.01 || diff > 0.01 {
		t.Fatalf("blend = %v, want ~%v", got, want)
	}
}

func TestTopDrivers(t *testing.T) {
	comps := []Component{
		makeComponent("macro_exposure_score", "macro exposure", 0.30, 80, true),
		makeComponent("event_risk_score", "event risk", 0.25, 20, true),
		makeComponent("trade_concentration_score", "trade concentration", 0.25, 55, true),
		makeComponent("shock_exposure_score", "shock exposure", 0.20, 0, false),
	}
	drivers := topDrivers(comps, 2)
	if len(drivers) != 2 {
		t.Fatalf("expected 2 drivers, got %v", drivers)
	}
	if drivers[0] != "macro exposure" {
		t.Errorf("top driver = %q, want macro exposure", drivers[0])
	}
}

func TestCountryFragilityAllComponents(t *testing.T) {
	ds, err := data.Default()
	if err != nil {
		t.Fatalf("default dataset: %v", err)
	}

	macroFile := sampleMacroFile()
	eventFile := sampleEventFile()
	tradeFile := sampleTradeFile()

	res := Score(Sources{
		Graph:       ds.Graph,
		Scenarios:   ds.Scenarios,
		Trade:       &tradeFile,
		Macro:       &macroFile,
		Events:      &eventFile,
		Commodities: nil,
		Config:      config.Default(),
	})

	if len(res.Countries) == 0 {
		t.Fatal("expected country scores")
	}

	var usa *CountryScore
	for i := range res.Countries {
		if res.Countries[i].CountryCode == "USA" || res.Countries[i].CountryName == "United States" {
			usa = &res.Countries[i]
			break
		}
	}
	if usa == nil {
		t.Fatal("expected United States in country scores")
	}
	if len(usa.MissingComponents) != 0 {
		t.Errorf("expected all country components for USA, missing %v", usa.MissingComponents)
	}
	if usa.Score <= 0 {
		t.Errorf("expected positive unified score for USA, got %v", usa.Score)
	}
	if len(usa.TopDrivers) == 0 {
		t.Error("expected top drivers for USA")
	}
}

func TestCommodityFragilityAllComponents(t *testing.T) {
	ds, err := data.Default()
	if err != nil {
		t.Fatalf("default dataset: %v", err)
	}

	tradeFile := sampleTradeFile()
	eventFile := sampleEventFile()

	res := Score(Sources{
		Graph:       ds.Graph,
		Scenarios:   ds.Scenarios,
		Trade:       &tradeFile,
		Macro:       nil,
		Events:      &eventFile,
		Commodities: nil,
		Config:      config.Default(),
	})

	var semi *CommodityScore
	for i := range res.Commodities {
		if res.Commodities[i].CommodityName == "semiconductors" {
			semi = &res.Commodities[i]
			break
		}
	}
	if semi == nil {
		t.Fatal("expected semiconductors in commodity scores")
	}
	// Without commodity price data, stress is missing but graph + trade + events may be present.
	if semi.Score < 0 {
		t.Errorf("unexpected negative score: %v", semi.Score)
	}
	if len(semi.TopDrivers) == 0 && len(semi.MissingComponents) < 4 {
		t.Error("expected top drivers when some components are available")
	}
}

func TestMissingComponentNormalization(t *testing.T) {
	g := graph.New()
	taiwan := models.NewNode(models.Country, "Taiwan")
	g.AddNode(taiwan)

	res := Score(Sources{Graph: g, Config: config.Default()})
	if len(res.Countries) != 1 {
		t.Fatalf("expected 1 country, got %d", len(res.Countries))
	}
	c := res.Countries[0]
	if len(c.MissingComponents) == 0 {
		t.Fatal("expected missing components when only graph is provided")
	}
	if c.Score != 0 {
		t.Errorf("expected score 0 when all components missing, got %v", c.Score)
	}
}

func TestTradeConcentrationByImporter(t *testing.T) {
	file := sampleTradeFile()
	byImporter := tradeConcentrationByImporter(&file, nil)
	if len(byImporter) == 0 {
		t.Fatal("expected importer concentration scores")
	}
	if _, ok := byImporter["USA"]; !ok {
		t.Errorf("expected USA importer concentration, got %v", byImporter)
	}
}

func TestTradeConcentrationFromNameOnlyDeps(t *testing.T) {
	deps := &trade.DependencyFile{
		Source: trade.ComtradeRealSourceName,
		Dependencies: []trade.TradeDependency{
			{Importer: "United States", Exporter: "Taiwan", Commodity: "semiconductors", Share: 0.60, TradeValueUSD: 60},
			{Importer: "United States", Exporter: "Korea, Rep.", Commodity: "semiconductors", Share: 0.30, TradeValueUSD: 30},
			{Importer: "United States", Exporter: "Japan", Commodity: "semiconductors", Share: 0.10, TradeValueUSD: 10},
			{Importer: "United States", Exporter: "Saudi Arabia", Commodity: "crude oil", Share: 0.50, TradeValueUSD: 50},
			{Importer: "United States", Exporter: "Canada", Commodity: "crude oil", Share: 0.50, TradeValueUSD: 50},
		},
	}
	byImporter := tradeConcentrationByImporter(nil, deps)
	if _, ok := byImporter["USA"]; !ok {
		t.Fatalf("expected USA concentration from name-only deps, got %v", byImporter)
	}
	byCommodity := supplierConcentrationByCommodity(nil, deps)
	if _, ok := byCommodity[commodityKey("", "semiconductors")]; !ok {
		t.Fatalf("expected semiconductors supplier concentration, got %v", byCommodity)
	}
	// HHI semiconductors = 0.36+0.09+0.01 = 0.46 -> 46; crude oil = 0.25+0.25 = 0.50 -> 50; avg = 48
	approx := byImporter["USA"]
	if approx < 40 || approx > 55 {
		t.Errorf("USA trade concentration = %v, want ~48", approx)
	}
}

func TestSupplierConcentrationByCommodity(t *testing.T) {
	file := sampleTradeFile()
	byCommodity := supplierConcentrationByCommodity(&file, nil)
	key := commodityKey("", "semiconductors")
	score, ok := byCommodity[key]
	if !ok {
		t.Fatalf("expected semiconductors concentration, got %v", byCommodity)
	}
	// HHI = 0.46 -> 46
	if score < 45 || score > 47 {
		t.Errorf("supplier concentration = %v, want ~46", score)
	}
}

func TestTradeConcentrationMissingDataNoCrash(t *testing.T) {
	byImporter := tradeConcentrationByImporter(nil, nil)
	if len(byImporter) != 0 {
		t.Fatalf("expected empty map, got %v", byImporter)
	}
	byCommodity := supplierConcentrationByCommodity(nil, nil)
	if len(byCommodity) != 0 {
		t.Fatalf("expected empty map, got %v", byCommodity)
	}
	res := Score(Sources{Config: config.Default()})
	_ = res
}

func TestCountryTradeConcentrationAvailableFromDeps(t *testing.T) {
	g := graph.New()
	g.AddNode(models.NewNode(models.Country, "United States"))
	g.AddNode(models.NewNode(models.Commodity, "semiconductors"))

	deps := &trade.DependencyFile{
		Source: trade.ComtradeRealSourceName,
		Dependencies: []trade.TradeDependency{
			{Importer: "United States", Exporter: "Taiwan", Commodity: "semiconductors", Share: 0.60, TradeValueUSD: 60},
			{Importer: "United States", Exporter: "Korea, Rep.", Commodity: "semiconductors", Share: 0.40, TradeValueUSD: 40},
		},
	}
	file := trade.DependenciesToTradeFile(*deps)
	res := Score(Sources{
		Graph:     g,
		Trade:     &file,
		TradeDeps: deps,
		Config:    config.Default(),
	})
	if res.TradeConcentrationSource != trade.ComtradeRealSourceName {
		t.Errorf("source = %q, want UN Comtrade", res.TradeConcentrationSource)
	}
	if res.TradeConcentrationNote != "US import-based concentration" {
		t.Errorf("note = %q, want US import-based concentration", res.TradeConcentrationNote)
	}

	var usa *CountryScore
	for i := range res.Countries {
		if res.Countries[i].CountryName == "United States" || res.Countries[i].CountryCode == "USA" {
			usa = &res.Countries[i]
			break
		}
	}
	if usa == nil {
		t.Fatal("expected United States country score")
	}
	found := false
	for _, c := range usa.Components {
		if c.Key != "trade_concentration_score" {
			continue
		}
		found = true
		if !c.Available {
			t.Fatal("expected trade_concentration_score available")
		}
		if c.Source != trade.ComtradeRealSourceName {
			t.Errorf("component source = %q", c.Source)
		}
	}
	if !found {
		t.Fatal("missing trade_concentration_score component")
	}
	for _, m := range usa.MissingComponents {
		if m == "trade_concentration_score" {
			t.Fatal("trade_concentration_score should not be in missing_components")
		}
	}

	var semi *CommodityScore
	for i := range res.Commodities {
		if strings.EqualFold(res.Commodities[i].CommodityName, "semiconductors") {
			semi = &res.Commodities[i]
			break
		}
	}
	if semi == nil {
		t.Fatal("expected semiconductors commodity score")
	}
	for _, c := range semi.Components {
		if c.Key == "supplier_concentration_score" && !c.Available {
			t.Fatal("expected supplier_concentration_score available")
		}
	}
}

func TestHhiToScore(t *testing.T) {
	if hhiToScore(0.42) != 42 {
		t.Errorf("hhiToScore(0.42) = %v, want 42", hhiToScore(0.42))
	}
	if hhiToScore(1.5) != 100 {
		t.Errorf("hhiToScore(1.5) = %v, want 100", hhiToScore(1.5))
	}
}

func sampleMacroFile() worldbank.IndicatorFile {
	rec := func(code, name, ind string, year int, v float64) worldbank.CountryIndicatorRecord {
		val := v
		return worldbank.CountryIndicatorRecord{
			CountryCode: code, CountryName: name, IndicatorCode: ind,
			Year: year, Value: &val, Source: worldbank.SourceName,
		}
	}
	return worldbank.IndicatorFile{
		Source: worldbank.SourceName, StartYear: 2023, EndYear: 2023,
		Countries: []string{"USA", "TWN"},
		Records: []worldbank.CountryIndicatorRecord{
			rec("USA", "United States", "NY.GDP.MKTP.CD", 2023, 27e12),
			rec("USA", "United States", "NE.IMP.GNFS.ZS", 2023, 14.1),
			rec("USA", "United States", "NE.EXP.GNFS.ZS", 2023, 11.2),
			rec("USA", "United States", "NV.IND.MANF.ZS", 2023, 11.0),
			rec("USA", "United States", "FP.CPI.TOTL.ZG", 2023, 3.2),
			rec("USA", "United States", "TX.VAL.TECH.CD", 2023, 2e11),
			rec("TWN", "Taiwan", "NY.GDP.MKTP.CD", 2023, 8e11),
			rec("TWN", "Taiwan", "NE.IMP.GNFS.ZS", 2023, 62.0),
			rec("TWN", "Taiwan", "NE.EXP.GNFS.ZS", 2023, 70.0),
			rec("TWN", "Taiwan", "NV.IND.MANF.ZS", 2023, 32.0),
			rec("TWN", "Taiwan", "FP.CPI.TOTL.ZG", 2023, 2.5),
			rec("TWN", "Taiwan", "TX.VAL.TECH.CD", 2023, 1.5e11),
		},
	}
}

func sampleEventFile() gdelt.EventFile {
	return gdelt.EventFile{
		Source: gdelt.SourceName,
		Records: []gdelt.GDELTEventRecord{
			{CountryCode: "TWN", CountryName: "Taiwan", Tone: -4.0, RiskTermsMatched: []string{"sanctions", "semiconductor"}},
			{CountryCode: "TWN", CountryName: "Taiwan", Tone: -3.0, RiskTermsMatched: []string{"export"}},
			{CountryCode: "USA", CountryName: "United States", Tone: -1.0, RiskTermsMatched: []string{"trade"}},
		},
	}
}

func sampleTradeFile() trade.TradeFile {
	return trade.TradeFile{
		Records: []trade.TradeFlowRecord{
			{Year: 2023, ExporterCode: "TWN", ExporterName: "Taiwan", ImporterCode: "USA", ImporterName: "United States", CommodityCode: "8542", CommodityName: "semiconductors", TradeValueUSD: 60e9},
			{Year: 2023, ExporterCode: "KOR", ExporterName: "Korea Rep.", ImporterCode: "USA", ImporterName: "United States", CommodityCode: "8542", CommodityName: "semiconductors", TradeValueUSD: 30e9},
			{Year: 2023, ExporterCode: "JPN", ExporterName: "Japan", ImporterCode: "USA", ImporterName: "United States", CommodityCode: "8542", CommodityName: "semiconductors", TradeValueUSD: 10e9},
		},
	}
}

func TestCountryFragilityProcessedEventRisk(t *testing.T) {
	ds, err := data.Default()
	if err != nil {
		t.Fatalf("default dataset: %v", err)
	}

	processed := eventrisk.RiskFile{
		Source: eventrisk.SourceName,
		Countries: []eventrisk.CountryRisk{
			{Country: "Taiwan", CountryCode: "TWN", EventRiskScore: 85, RiskLevel: "Critical", EventCount: 4, RecentEventCount: 3, AverageTone: -6},
			{Country: "United States", CountryCode: "USA", EventRiskScore: 25, RiskLevel: "Low", EventCount: 2, RecentEventCount: 1, AverageTone: -1},
		},
	}
	legacyFile := sampleEventFile()

	withLegacy := Score(Sources{
		Graph: ds.Graph, Scenarios: ds.Scenarios, Trade: nil, Macro: nil,
		Events: &legacyFile, Config: config.Default(),
	})
	withProcessed := Score(Sources{
		Graph: ds.Graph, Scenarios: ds.Scenarios, Trade: nil, Macro: nil,
		ProcessedEventRisk: &processed, Events: &legacyFile, Config: config.Default(),
	})

	if len(withProcessed.Countries) == 0 {
		t.Fatal("expected country scores with processed event risk")
	}

	var twProcessed, twLegacy *CountryScore
	for i := range withProcessed.Countries {
		if withProcessed.Countries[i].CountryCode == "TWN" {
			twProcessed = &withProcessed.Countries[i]
		}
	}
	for i := range withLegacy.Countries {
		if withLegacy.Countries[i].CountryCode == "TWN" {
			twLegacy = &withLegacy.Countries[i]
		}
	}
	if twProcessed == nil || twLegacy == nil {
		t.Fatal("expected Taiwan in both score sets")
	}
	if twProcessed.Score == twLegacy.Score {
		t.Fatalf("expected processed event risk to change Taiwan score: processed=%v legacy=%v", twProcessed.Score, twLegacy.Score)
	}
}
