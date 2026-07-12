package graphfusion

import (
	"testing"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/simulation"
)

func sampleDeps() *trade.DependencyFile {
	return &trade.DependencyFile{
		Source: "UN Comtrade",
		Dependencies: []trade.TradeDependency{
			{Importer: "United States", Exporter: "Taiwan", Commodity: "semiconductors", Share: 0.60, TradeValueUSD: 60, Year: 2023, HSCode: "8542"},
			{Importer: "United States", Exporter: "Korea Rep.", Commodity: "semiconductors", Share: 0.30, TradeValueUSD: 30, Year: 2023, HSCode: "8542"},
			{Importer: "United States", Exporter: "Malaysia", Commodity: "semiconductors", Share: 0.2377, TradeValueUSD: 24, Year: 2023, HSCode: "8542"},
			{Importer: "United States", Exporter: "Malaysia", Commodity: "semiconductors", Share: 0.10, TradeValueUSD: 10, Year: 2023, HSCode: "8542"},
		},
	}
}

func TestFuseWithoutRealData(t *testing.T) {
	base, err := data.Default()
	if err != nil {
		t.Fatal(err)
	}
	out := Fuse(Input{Base: base})
	if out.Meta.FusionEnabled {
		t.Fatal("expected fusion disabled without trade data")
	}
	if out.Dataset.Graph.NodeCount() != base.Graph.NodeCount() {
		t.Fatalf("nodes = %d, want %d", out.Dataset.Graph.NodeCount(), base.Graph.NodeCount())
	}
	if out.Dataset.Graph.EdgeCount() != base.Graph.EdgeCount() {
		t.Fatalf("edges = %d, want %d", out.Dataset.Graph.EdgeCount(), base.Graph.EdgeCount())
	}
}

func TestFuseAddsRealTradeEdges(t *testing.T) {
	base, err := data.Default()
	if err != nil {
		t.Fatal(err)
	}
	before := base.Graph.EdgeCount()
	out := Fuse(Input{Base: base, Trade: sampleDeps(), TradeReal: true})
	if !out.Meta.FusionEnabled {
		t.Fatal("expected fusion enabled")
	}
	if out.Meta.RealTradeEdges <= 0 {
		t.Fatalf("real trade edges = %d, want > 0", out.Meta.RealTradeEdges)
	}
	if out.Dataset.Graph.EdgeCount() <= before {
		t.Fatalf("fused edges = %d, want > base %d", out.Dataset.Graph.EdgeCount(), before)
	}
}

func TestFuseDeduplicatesEdges(t *testing.T) {
	g := graph.New()
	out := Fuse(Input{
		Base:      &data.Dataset{Graph: g, Scenarios: nil},
		Trade:     sampleDeps(),
		TradeReal: true,
	})
	malaysiaExport := 0
	for _, n := range out.Dataset.Graph.Nodes() {
		if n.Type != models.Country || n.Name != "Malaysia" {
			continue
		}
		for _, e := range out.Dataset.Graph.OutEdges(n.ID) {
			if e.Type == models.RelRealExports {
				malaysiaExport++
			}
		}
	}
	if malaysiaExport != 1 {
		t.Fatalf("malaysia real_exports edges = %d, want 1 (deduped)", malaysiaExport)
	}
}

func TestFuseCreatesMissingCountryNode(t *testing.T) {
	g := graph.New()
	g.AddNode(models.NewNode(models.Commodity, "semiconductors"))
	deps := &trade.DependencyFile{
		Source: "UN Comtrade",
		Dependencies: []trade.TradeDependency{
			{Importer: "United States", Exporter: "Malaysia", Commodity: "semiconductors", Share: 0.5, Year: 2023},
		},
	}
	out := Fuse(Input{Base: &data.Dataset{Graph: g}, Trade: deps, TradeReal: true})
	if _, ok := out.Dataset.Graph.NodeByName(models.Country, "Malaysia"); !ok {
		t.Fatal("expected Malaysia country node")
	}
	n, _ := out.Dataset.Graph.NodeByName(models.Country, "Malaysia")
	if !n.GeneratedFromRealData {
		t.Fatal("expected generated_from_real_data on fused country")
	}
}

func TestRealEdgeWeightEqualsShare(t *testing.T) {
	g := graph.New()
	g.AddNode(models.NewNode(models.Commodity, "semiconductors"))
	deps := &trade.DependencyFile{
		Source: "UN Comtrade",
		Dependencies: []trade.TradeDependency{
			{Importer: "United States", Exporter: "Malaysia", Commodity: "semiconductors", Share: 0.2377, Year: 2023},
		},
	}
	out := Fuse(Input{Base: &data.Dataset{Graph: g}, Trade: deps, TradeReal: true})
	malaysia, _ := out.Dataset.Graph.NodeByName(models.Country, "Malaysia")
	commodity, _ := out.Dataset.Graph.NodeByName(models.Commodity, "semiconductors")
	e, ok := out.Dataset.Graph.EdgeBetween(malaysia.ID, commodity.ID)
	if !ok {
		t.Fatal("expected export edge")
	}
	if e.Weight != 0.2377 {
		t.Fatalf("weight = %v, want 0.2377", e.Weight)
	}
	if !e.RealData {
		t.Fatal("expected real_data=true")
	}
}

func TestShockUsesRealTradeEdges(t *testing.T) {
	base, err := data.Load("data/strategic_global")
	if err != nil {
		t.Skip("strategic_global not available:", err)
	}
	out := Fuse(Input{Base: base, Trade: sampleDeps(), TradeReal: true})
	res, err := simulation.RunWithContext(out.Dataset.Graph, config.Default(), simulation.ShockRequest{
		Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", DropPct: 30, Depth: 3,
	}, &out.SimCtx)
	if err != nil {
		t.Fatalf("shock failed: %v", err)
	}
	if len(res.AllAffected) == 0 {
		t.Fatal("expected affected nodes from real trade shock")
	}
}
