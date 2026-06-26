package simulation

import (
	"math"
	"testing"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
)

// seedGraph loads the embedded sample dataset for tests.
func seedGraph(t *testing.T) *graph.Graph {
	t.Helper()
	ds, err := data.Default()
	if err != nil {
		t.Fatalf("loading sample dataset: %v", err)
	}
	return ds.Graph
}

func baseReq() ShockRequest {
	return ShockRequest{Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 30, Depth: 3}
}

func findImpact(items []NodeImpact, name string) (NodeImpact, bool) {
	for _, ni := range items {
		if ni.Node.Name == name {
			return ni, true
		}
	}
	return NodeImpact{}, false
}

func TestRunValidationErrors(t *testing.T) {
	g := seedGraph(t)
	cfg := config.Default()

	cases := []struct {
		name string
		req  ShockRequest
	}{
		{"unknown shock type", ShockRequest{Source: "Taiwan", Commodity: "semiconductors", ShockType: "meteor_strike", DropPct: 30, Depth: 3}},
		{"unknown source", ShockRequest{Source: "Atlantis", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 30, Depth: 3}},
		{"unknown commodity", ShockRequest{Source: "Taiwan", Commodity: "unobtainium", ShockType: "export_collapse", DropPct: 30, Depth: 3}},
		{"source does not produce", ShockRequest{Source: "Germany", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 30, Depth: 3}},
		{"bad depth", ShockRequest{Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 30, Depth: 0}},
		{"bad drop", ShockRequest{Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 150, Depth: 3}},
	}
	for _, c := range cases {
		if _, err := Run(g, cfg, c.req); err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}
}

func TestInitialImpactIsDropTimesConcentration(t *testing.T) {
	g := seedGraph(t)
	res, err := Run(g, config.Default(), baseReq())
	if err != nil {
		t.Fatal(err)
	}
	want := 0.30 * 0.92 // Taiwan -> semiconductors concentration
	if math.Abs(res.InitialImpact-want) > 1e-9 {
		t.Fatalf("InitialImpact = %v, want %v", res.InitialImpact, want)
	}
}

func TestDirectAndSecondOrderClassification(t *testing.T) {
	g := seedGraph(t)
	res, err := Run(g, config.Default(), baseReq())
	if err != nil {
		t.Fatal(err)
	}

	// United States imports semiconductors directly (distance 2).
	us, ok := findImpact(res.Direct, "United States")
	if !ok {
		t.Fatalf("expected United States in direct exposure")
	}
	if us.Distance != distDirect {
		t.Errorf("US distance = %d, want %d", us.Distance, distDirect)
	}
	if us.Impact <= 0 {
		t.Errorf("US impact should be > 0, got %v", us.Impact)
	}

	// AI hardware depends on the US (distance 3 from Taiwan).
	if _, ok := findImpact(res.SecondOrder, "AI hardware"); !ok {
		t.Fatalf("expected AI hardware in second-order exposure")
	}
}

func TestShockRaisesFragility(t *testing.T) {
	g := seedGraph(t)
	res, err := Run(g, config.Default(), baseReq())
	if err != nil {
		t.Fatal(err)
	}
	for _, ni := range res.AllAffected {
		if ni.ShockFragility < ni.BaseFragility {
			t.Errorf("%s: shock fragility %.2f below base %.2f", ni.Node.Name, ni.ShockFragility, ni.BaseFragility)
		}
		if ni.Delta < 0 {
			t.Errorf("%s: negative delta %.2f", ni.Node.Name, ni.Delta)
		}
	}
}

func TestAffectedPathsRouteThroughCommodity(t *testing.T) {
	g := seedGraph(t)
	res, err := Run(g, config.Default(), baseReq())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) == 0 {
		t.Fatal("expected at least one affected path")
	}
	for _, p := range res.Paths {
		if len(p.Nodes) < 2 {
			t.Fatalf("path too short: %v", p.Nodes)
		}
		if p.Nodes[0].Name != "Taiwan" {
			t.Errorf("path should start at Taiwan, got %s", p.Nodes[0].Name)
		}
		if p.Nodes[1].Name != "semiconductors" {
			t.Errorf("path should route through semiconductors, got %s", p.Nodes[1].Name)
		}
	}
}

func TestHigherDropMeansHigherImpact(t *testing.T) {
	g := seedGraph(t)
	cfg := config.Default()

	low, err := Run(g, cfg, ShockRequest{Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 10, Depth: 3})
	if err != nil {
		t.Fatal(err)
	}
	high, err := Run(g, cfg, ShockRequest{Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 60, Depth: 3})
	if err != nil {
		t.Fatal(err)
	}
	lowUS, _ := findImpact(low.Direct, "United States")
	highUS, _ := findImpact(high.Direct, "United States")
	if !(highUS.Impact > lowUS.Impact) {
		t.Fatalf("expected higher drop to raise US impact: low=%v high=%v", lowUS.Impact, highUS.Impact)
	}
}

func TestZeroDropProducesNoAffectedNodes(t *testing.T) {
	g := seedGraph(t)
	res, err := Run(g, config.Default(), ShockRequest{Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 0, Depth: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.AllAffected) != 0 {
		t.Fatalf("zero drop should affect no nodes, got %d", len(res.AllAffected))
	}
}

func TestDepthLimitsPropagation(t *testing.T) {
	g := seedGraph(t)
	cfg := config.Default()

	d1, err := Run(g, cfg, ShockRequest{Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 30, Depth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(d1.Direct) != 0 {
		t.Errorf("depth 1 should have no direct exposure, got %d", len(d1.Direct))
	}

	d2, err := Run(g, cfg, ShockRequest{Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 30, Depth: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(d2.Direct) == 0 {
		t.Errorf("depth 2 should expose direct importers")
	}
	if len(d2.SecondOrder) != 0 {
		t.Errorf("depth 2 should not reach second-order nodes, got %d", len(d2.SecondOrder))
	}
}

func affected(res Result, name string) bool {
	_, ok := findImpact(res.AllAffected, name)
	return ok
}

// A semiconductor export collapse must not leak into unrelated commodities.
func TestSemiconductorShockDoesNotReachUnrelatedCommodities(t *testing.T) {
	g := seedGraph(t)
	res, err := Run(g, config.Default(), baseReq())
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{"crude oil", "lithium", "cobalt", "EV batteries"} {
		if affected(res, leaked) {
			t.Errorf("semiconductor shock should not reach %q", leaked)
		}
	}
	for _, want := range []string{"United States", "Japan", "China", "Germany", "AI hardware", "cloud infrastructure", "automotive electronics", "consumer devices"} {
		if !affected(res, want) {
			t.Errorf("semiconductor shock should reach %q", want)
		}
	}
	// The unrelated commodities should show up as blocked branches instead.
	blocked := map[string]bool{}
	for _, c := range res.BlockedCommodities() {
		blocked[c] = true
	}
	for _, c := range []string{"crude oil", "lithium", "cobalt"} {
		if !blocked[c] {
			t.Errorf("expected %q among blocked commodity branches, got %v", c, res.BlockedCommodities())
		}
	}
}

func TestPriceSpikePropagatesThroughPriceExposure(t *testing.T) {
	g := seedGraph(t)
	req := ShockRequest{Source: "China", Commodity: "lithium", ShockType: "price_spike", DropPct: 40, Depth: 3}
	res, err := Run(g, config.Default(), req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"EV batteries", "automotive electronics"} {
		if !affected(res, want) {
			t.Errorf("lithium price spike should reach %q", want)
		}
	}
	// automotive electronics is reached via a price_exposure edge.
	found := false
	for _, p := range res.Paths {
		for _, e := range p.Edges {
			if e.Relationship == "price_exposure" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected a price_exposure hop in the affected paths")
	}
}

func TestRouteDisruptionPropagatesThroughRouteExposure(t *testing.T) {
	g := seedGraph(t)
	req := ShockRequest{Source: "Suez Canal", Commodity: "crude oil", ShockType: "route_disruption", DropPct: 35, Depth: 3}
	res, err := Run(g, config.Default(), req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Europe", "shipping logistics"} {
		if !affected(res, want) {
			t.Errorf("route disruption should reach %q", want)
		}
	}
	// The origin hop is a route_exposure edge.
	if len(res.Paths) == 0 || res.Paths[0].Edges[0].Relationship != "route_exposure" {
		t.Errorf("expected the origin hop to be route_exposure")
	}
}

func TestSupplyCutDiffersFromRouteDisruption(t *testing.T) {
	g := seedGraph(t)
	cfg := config.Default()
	// supply_cut reaches energy-intensive manufacturing (via used_by), which
	// route_disruption does not (used_by is not in its profile).
	sc, err := Run(g, cfg, ShockRequest{Source: "Saudi Arabia", Commodity: "crude oil", ShockType: "supply_cut", DropPct: 25, Depth: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !affected(sc, "energy-intensive manufacturing") {
		t.Errorf("supply cut should reach energy-intensive manufacturing")
	}
}

func TestPathsCarryRelationshipLabels(t *testing.T) {
	g := seedGraph(t)
	res, err := Run(g, config.Default(), baseReq())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Paths) == 0 {
		t.Fatal("expected affected paths")
	}
	for _, p := range res.Paths {
		if len(p.Edges) != len(p.Nodes)-1 {
			t.Fatalf("path has %d nodes but %d edges", len(p.Nodes), len(p.Edges))
		}
		if p.Edges[0].Relationship != "exports" {
			t.Errorf("first hop should be 'exports', got %q", p.Edges[0].Relationship)
		}
	}
}

func TestUnknownShockTypeFailsCleanly(t *testing.T) {
	g := seedGraph(t)
	_, err := Run(g, config.Default(), ShockRequest{Source: "Taiwan", Commodity: "semiconductors", ShockType: "asteroid", DropPct: 30, Depth: 3})
	if err == nil {
		t.Fatal("expected an error for an unknown shock type")
	}
}

func TestProfilesCoverAllShockTypes(t *testing.T) {
	for _, st := range []models.ShockType{
		models.ShockExportCollapse, models.ShockSupplyCut,
		models.ShockPriceSpike, models.ShockRouteDisruption,
	} {
		if _, ok := ProfileFor(string(st)); !ok {
			t.Errorf("no profile registered for shock type %q", st)
		}
	}
	if len(AllProfiles()) != 4 {
		t.Errorf("expected 4 profiles, got %d", len(AllProfiles()))
	}
}

// Guard against accidental graph drift breaking the documented example.
func TestSeedGraphContainsExpectedNodes(t *testing.T) {
	g := seedGraph(t)
	checks := []struct {
		t    models.NodeType
		name string
	}{
		{models.Country, "Taiwan"},
		{models.Commodity, "semiconductors"},
		{models.Sector, "AI hardware"},
	}
	for _, c := range checks {
		if _, ok := g.NodeByName(c.t, c.name); !ok {
			t.Errorf("seed graph missing %s %q", c.t, c.name)
		}
	}
}
