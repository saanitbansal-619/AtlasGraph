package scoring

import (
	"math"
	"testing"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
)

const eps = 1e-9

func TestFragilityFormula(t *testing.T) {
	got := Fragility(Factors{Dependency: 0.5, Concentration: 0.5, Exposure: 0.5}, 100)
	want := 0.5 * 0.5 * 0.5 * 100 // 12.5
	if math.Abs(got-want) > eps {
		t.Fatalf("Fragility = %v, want %v", got, want)
	}
}

func TestFragilityCaps(t *testing.T) {
	// Inputs above 1 must be clamped, and the result capped at max.
	got := Fragility(Factors{Dependency: 2, Concentration: 2, Exposure: 2}, 100)
	if got != 100 {
		t.Fatalf("Fragility over-range = %v, want 100", got)
	}
	if got := Fragility(Factors{Dependency: -1, Concentration: 1, Exposure: 1}, 100); got != 0 {
		t.Fatalf("Fragility under-range = %v, want 0", got)
	}
	// Custom cap is honoured.
	if got := Fragility(Factors{Dependency: 1, Concentration: 1, Exposure: 1}, 50); got != 50 {
		t.Fatalf("Fragility custom cap = %v, want 50", got)
	}
}

func TestFragilityMonotonicInExposure(t *testing.T) {
	low := Fragility(Factors{Dependency: 0.8, Concentration: 0.8, Exposure: 0.2}, 100)
	high := Fragility(Factors{Dependency: 0.8, Concentration: 0.8, Exposure: 0.9}, 100)
	if !(high > low) {
		t.Fatalf("expected fragility to increase with exposure: low=%v high=%v", low, high)
	}
}

// twoSupplierGraph: S1 ->(0.9/0.9) T, S2 ->(0.5/0.3) T.
func twoSupplierGraph() (*graph.Graph, models.NodeID) {
	g := graph.New()
	t1 := models.NewNode(models.Commodity, "T")
	s1 := models.NewNode(models.Country, "S1")
	s2 := models.NewNode(models.Country, "S2")
	for _, n := range []models.Node{t1, s1, s2} {
		g.AddNode(n)
	}
	g.AddEdge(models.Edge{From: s1.ID, To: t1.ID, Weight: 0.9, Concentration: 0.9})
	g.AddEdge(models.Edge{From: s2.ID, To: t1.ID, Weight: 0.5, Concentration: 0.3})
	return g, t1.ID
}

func TestDependencyScoreNoisyOr(t *testing.T) {
	g, target := twoSupplierGraph()
	got := DependencyScore(g, target)
	want := 1 - (1-0.9)*(1-0.5) // 0.95
	if math.Abs(got-want) > eps {
		t.Fatalf("DependencyScore = %v, want %v", got, want)
	}
}

func TestDependencyScoreNoInbound(t *testing.T) {
	g, _ := twoSupplierGraph()
	src := models.NewNodeID(models.Country, "S1")
	if got := DependencyScore(g, src); got != 0 {
		t.Fatalf("pure source DependencyScore = %v, want 0", got)
	}
}

func TestConcentrationScoreIsMax(t *testing.T) {
	g, target := twoSupplierGraph()
	if got := ConcentrationScore(g, target); math.Abs(got-0.9) > eps {
		t.Fatalf("ConcentrationScore = %v, want 0.9", got)
	}
}

func TestNodeFragilityShockRaisesScore(t *testing.T) {
	g, target := twoSupplierGraph()
	cfg := config.Default()
	base := NodeFragility(g, target, 0, cfg)
	shock := NodeFragility(g, target, 0.4, cfg)
	if !(shock > base) {
		t.Fatalf("shock fragility %.3f should exceed base %.3f", shock, base)
	}
}
