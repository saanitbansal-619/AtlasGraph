package tradegraph

import (
	"math"
	"testing"
	"time"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/simulation"
)

func rec(exCode, exName, imCode, imName, comName string, val float64) trade.TradeFlowRecord {
	return trade.TradeFlowRecord{
		Year: 2023, ExporterCode: exCode, ExporterName: exName,
		ImporterCode: imCode, ImporterName: imName, CommodityName: comName,
		TradeValueUSD: val, Unit: "USD", Source: trade.SourceName, IngestedAt: time.Now().UTC(),
	}
}

// testFile: USA imports semiconductors from TWN(60)/KOR(30)/JPN(10) and a crude
// oil flow from SAU to Germany so scenario triggers exist.
func testFile() trade.TradeFile {
	return trade.TradeFile{Records: []trade.TradeFlowRecord{
		rec("TWN", "Taiwan", "USA", "United States", "semiconductors", 60),
		rec("KOR", "Korea Rep.", "USA", "United States", "semiconductors", 30),
		rec("JPN", "Japan", "USA", "United States", "semiconductors", 10),
		rec("CHN", "China", "USA", "United States", "lithium batteries", 50),
		rec("SAU", "Saudi Arabia", "DEU", "Germany", "crude oil", 100),
	}}
}

func findDep(deps []Dependency, source, target, rel string) (Dependency, bool) {
	for _, d := range deps {
		if d.Source == source && d.Target == target && d.Relationship == rel {
			return d, true
		}
	}
	return Dependency{}, false
}

func approx(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-4 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestBuildEntities(t *testing.T) {
	r := Build(testFile())
	wantCountries := map[string]bool{
		"Taiwan": true, "Korea Rep.": true, "Japan": true,
		"China": true, "United States": true, "Saudi Arabia": true, "Germany": true,
	}
	if len(r.Entities.Countries) != len(wantCountries) {
		t.Fatalf("countries = %d, want %d", len(r.Entities.Countries), len(wantCountries))
	}
	for _, e := range r.Entities.Countries {
		if !wantCountries[e.Name] {
			t.Errorf("unexpected country %q", e.Name)
		}
	}
	wantCommodities := map[string]bool{"semiconductors": true, "lithium batteries": true, "crude oil": true}
	if len(r.Entities.Commodities) != len(wantCommodities) {
		t.Errorf("commodities = %d, want %d", len(r.Entities.Commodities), len(wantCommodities))
	}
	// Sectors come from the commodity->sector mapping (semiconductors + lithium
	// batteries + crude oil).
	if len(r.Entities.Sectors) == 0 {
		t.Errorf("expected mapped sectors, got 0")
	}
}

func TestSupplierShareWeights(t *testing.T) {
	r := Build(testFile())
	deps := r.Dependencies.Dependencies

	// Exports edge weight = exporter's global share of the commodity's exports.
	// Taiwan supplies 60 of 100 total semiconductor exports => 0.60.
	twn, ok := findDep(deps, "Taiwan", "semiconductors", "exports")
	if !ok {
		t.Fatal("missing Taiwan exports edge")
	}
	approx(t, "taiwan export weight", twn.Weight, 0.60)
	if twn.Concentration == nil {
		t.Fatal("exports edge should carry concentration")
	}
	approx(t, "taiwan export concentration", *twn.Concentration, 0.60)

	kor, _ := findDep(deps, "Korea Rep.", "semiconductors", "exports")
	approx(t, "korea export weight", kor.Weight, 0.30)

	// Imports edge weight = importer's top-supplier share (Taiwan = 0.60),
	// concentration = sourcing HHI = .6^2+.3^2+.1^2 = .46.
	imp, ok := findDep(deps, "semiconductors", "United States", "imports")
	if !ok {
		t.Fatal("missing semiconductors->USA imports edge")
	}
	approx(t, "usa import weight (top share)", imp.Weight, 0.60)
	if imp.Concentration == nil {
		t.Fatal("imports edge should carry concentration")
	}
	approx(t, "usa import concentration (hhi)", *imp.Concentration, 0.46)
}

func TestRelationshipTypesPresent(t *testing.T) {
	r := Build(testFile())
	seen := map[string]bool{}
	for _, d := range r.Dependencies.Dependencies {
		seen[d.Relationship] = true
		// Every generated edge must carry a valid, loader-acceptable weight.
		if d.Weight <= 0 || d.Weight > 1 {
			t.Errorf("edge %q->%q weight out of range: %v", d.Source, d.Target, d.Weight)
		}
	}
	for _, rel := range []string{"exports", "imports", "industry_dependency"} {
		if !seen[rel] {
			t.Errorf("missing %q dependencies", rel)
		}
	}
}

func TestScenariosGenerated(t *testing.T) {
	r := Build(testFile())
	got := map[string]Scenario{}
	for _, s := range r.Scenarios.Scenarios {
		got[s.ID] = s
	}
	// Taiwan exports semiconductors, China exports lithium batteries and Saudi
	// Arabia exports crude oil, so all three scenarios should be generated.
	if _, ok := got["taiwan_semiconductor_shock"]; !ok {
		t.Errorf("expected taiwan_semiconductor_shock scenario")
	}
	if _, ok := got["crude_oil_supply_shock"]; !ok {
		t.Errorf("expected crude_oil_supply_shock scenario")
	}
	if s, ok := got["lithium_battery_shock"]; !ok {
		t.Errorf("expected lithium_battery_shock scenario (China exports lithium batteries)")
	} else if s.Source != "China" {
		t.Errorf("lithium scenario source = %q, want China", s.Source)
	}
	if s := got["taiwan_semiconductor_shock"]; s.Source != "Taiwan" || s.Commodity != "semiconductors" {
		t.Errorf("taiwan scenario malformed: %+v", s)
	}
}

func TestGeneratedGraphLoadsAndSimulates(t *testing.T) {
	r := Build(testFile())
	dir := t.TempDir()
	if err := Write(dir, r); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// The generated dataset must load through the standard loader untouched.
	ds, err := data.Load(dir)
	if err != nil {
		t.Fatalf("data.Load on generated graph: %v", err)
	}
	if ds.Graph.NodeCount() == 0 || ds.Graph.EdgeCount() == 0 {
		t.Fatalf("generated graph empty: %d nodes / %d edges", ds.Graph.NodeCount(), ds.Graph.EdgeCount())
	}
	if _, ok := ds.Scenario("taiwan_semiconductor_shock"); !ok {
		t.Errorf("generated scenario not loaded")
	}

	// And it must support a shock simulation end to end.
	res, err := simulation.Run(ds.Graph, config.Default(), simulation.ShockRequest{
		Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 30, Depth: 3,
	})
	if err != nil {
		t.Fatalf("simulation.Run on generated graph: %v", err)
	}
	if res.InitialImpact <= 0 {
		t.Errorf("expected a non-zero initial impact, got %v", res.InitialImpact)
	}
	// The US should be reached as an importer of semiconductors.
	if _, ok := ds.Graph.NodeByName(models.Country, "United States"); !ok {
		t.Errorf("United States missing from generated graph")
	}
	if len(res.AllAffected) == 0 {
		t.Errorf("expected affected nodes from the shock")
	}
}
