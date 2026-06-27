package graph

import (
	"testing"

	"github.com/atlasgraph/atlas/internal/models"
)

// buildLinear builds A -> B -> C -> D with unit weights for traversal tests.
func buildLinear() (*Graph, [4]models.NodeID) {
	g := New()
	ids := [4]models.NodeID{}
	names := []string{"A", "B", "C", "D"}
	for i, n := range names {
		node := models.NewNode(models.Sector, n)
		ids[i] = node.ID
		g.AddNode(node)
	}
	for i := 0; i < 3; i++ {
		g.AddEdge(models.Edge{From: ids[i], To: ids[i+1], Type: models.RelUsedBy, Weight: 1, Concentration: 1})
	}
	return g, ids
}

func TestAddNodeAndCounts(t *testing.T) {
	g, _ := buildLinear()
	if got := g.NodeCount(); got != 4 {
		t.Fatalf("NodeCount = %d, want 4", got)
	}
	if got := g.EdgeCount(); got != 3 {
		t.Fatalf("EdgeCount = %d, want 3", got)
	}
}

func TestAddEdgeAutoRegistersEndpoints(t *testing.T) {
	g := New()
	from := models.NewNodeID(models.Country, "X")
	to := models.NewNodeID(models.Commodity, "Y")
	g.AddEdge(models.Edge{From: from, To: to, Weight: 0.5})
	if g.NodeCount() != 2 {
		t.Fatalf("expected endpoints auto-registered, got %d nodes", g.NodeCount())
	}
	if _, ok := g.Node(from); !ok {
		t.Errorf("from node not registered")
	}
}

func TestNeighborsAndInOutEdges(t *testing.T) {
	g, ids := buildLinear()
	nb := g.Neighbors(ids[0])
	if len(nb) != 1 || nb[0] != ids[1] {
		t.Fatalf("Neighbors(A) = %v, want [B]", nb)
	}
	if len(g.OutEdges(ids[1])) != 1 {
		t.Errorf("OutEdges(B) = %d, want 1", len(g.OutEdges(ids[1])))
	}
	if len(g.InEdges(ids[1])) != 1 {
		t.Errorf("InEdges(B) = %d, want 1", len(g.InEdges(ids[1])))
	}
	if len(g.OutEdges(ids[3])) != 0 {
		t.Errorf("OutEdges(D) should be empty")
	}
}

func TestEdgeBetween(t *testing.T) {
	g, ids := buildLinear()
	if _, ok := g.EdgeBetween(ids[0], ids[1]); !ok {
		t.Errorf("expected edge A->B")
	}
	if _, ok := g.EdgeBetween(ids[0], ids[2]); ok {
		t.Errorf("did not expect edge A->C")
	}
}

func TestAllPathsRespectsDepth(t *testing.T) {
	g, ids := buildLinear()

	// Depth 1: only A->B.
	p1 := g.AllPaths(ids[0], 1)
	if len(p1) != 1 {
		t.Fatalf("depth 1 paths = %d, want 1", len(p1))
	}
	if len(p1[0]) != 2 {
		t.Fatalf("depth 1 path length = %d, want 2", len(p1[0]))
	}

	// Depth 3 from A: A->B, A->B->C, A->B->C->D = 3 paths.
	p3 := g.AllPaths(ids[0], 3)
	if len(p3) != 3 {
		t.Fatalf("depth 3 paths = %d, want 3", len(p3))
	}
	longest := p3[len(p3)-1]
	if len(longest) != 4 {
		t.Fatalf("longest path nodes = %d, want 4", len(longest))
	}
}

func TestPathsBetween(t *testing.T) {
	g, ids := buildLinear()

	full := g.PathsBetween(ids[0], ids[3], 3)
	if len(full) != 1 {
		t.Fatalf("PathsBetween(A,D,3) = %d paths, want 1", len(full))
	}
	if len(full[0]) != 4 {
		t.Fatalf("path length = %d, want 4", len(full[0]))
	}

	// Too shallow to reach C.
	if got := g.PathsBetween(ids[0], ids[2], 1); len(got) != 0 {
		t.Fatalf("PathsBetween(A,C,1) = %d, want 0", len(got))
	}
}

// buildLabeled builds a small heterogeneous graph with two commodity branches
// and a parallel edge, for edge-aware/filtered traversal tests:
//
//	A --exports/chips--> B --imports/chips--> C   (chips branch to the target)
//	                     B --used_by/chips--> C   (a second, parallel chips edge)
//	                     B --exports/oil----> D   (an unrelated commodity branch)
func buildLabeled() (*Graph, map[string]models.NodeID) {
	g := New()
	id := map[string]models.NodeID{}
	add := func(t models.NodeType, name string) {
		n := models.NewNode(t, name)
		id[name] = n.ID
		g.AddNode(n)
	}
	add(models.Country, "A")
	add(models.Commodity, "B")
	add(models.Sector, "C")
	add(models.Commodity, "D")
	g.AddEdge(models.Edge{From: id["A"], To: id["B"], Type: models.RelExports, Weight: 0.9, Commodity: "chips", PropagationEnabled: true})
	g.AddEdge(models.Edge{From: id["B"], To: id["C"], Type: models.RelImports, Weight: 0.8, Commodity: "chips", PropagationEnabled: true})
	g.AddEdge(models.Edge{From: id["B"], To: id["C"], Type: models.RelUsedBy, Weight: 0.5, Commodity: "chips", PropagationEnabled: true})
	g.AddEdge(models.Edge{From: id["B"], To: id["D"], Type: models.RelExports, Weight: 0.7, Commodity: "oil", PropagationEnabled: true})
	return g, id
}

func TestPathsBetweenFuncKeepsParallelEdges(t *testing.T) {
	g, id := buildLabeled()
	// With no predicate the two parallel B->C edges yield two distinct paths,
	// each carrying its own relationship label.
	paths := g.PathsBetweenFunc(id["A"], id["C"], 6, nil, nil)
	if len(paths) != 2 {
		t.Fatalf("PathsBetweenFunc(A,C) = %d paths, want 2 (parallel edges)", len(paths))
	}
	rels := map[models.EdgeType]bool{}
	for _, p := range paths {
		if len(p.Edges) != 2 {
			t.Fatalf("path has %d edges, want 2", len(p.Edges))
		}
		rels[p.Edges[len(p.Edges)-1].Type] = true
	}
	if !rels[models.RelImports] || !rels[models.RelUsedBy] {
		t.Errorf("expected both imports and used_by final hops, got %v", rels)
	}
}

func TestPathsBetweenFuncAllowAndBlock(t *testing.T) {
	g, id := buildLabeled()
	// Allow only edges on the chips commodity; the oil branch must be pruned
	// and reported through onBlock.
	var blocked []models.Edge
	allow := func(e models.Edge) bool { return e.Commodity == "chips" }
	onBlock := func(e models.Edge) { blocked = append(blocked, e) }

	paths := g.PathsBetweenFunc(id["A"], id["C"], 6, allow, onBlock)
	if len(paths) != 2 {
		t.Fatalf("filtered PathsBetweenFunc(A,C) = %d paths, want 2", len(paths))
	}
	if len(blocked) == 0 {
		t.Fatal("expected the oil branch to be reported as blocked")
	}
	for _, e := range blocked {
		if e.To != id["D"] {
			t.Errorf("unexpected blocked edge to %s, want D", e.To)
		}
	}
}

func TestPathsBetweenFuncRespectsDepth(t *testing.T) {
	g, id := buildLabeled()
	// A->B->C needs two edges; a one-edge horizon must find nothing.
	if got := g.PathsBetweenFunc(id["A"], id["C"], 1, nil, nil); len(got) != 0 {
		t.Fatalf("PathsBetweenFunc(A,C,1) = %d, want 0", len(got))
	}
}

func TestCountByTypeAndDegree(t *testing.T) {
	g, ids := buildLinear()
	if got := g.CountByType(models.Sector); got != 4 {
		t.Errorf("CountByType(Sector) = %d, want 4", got)
	}
	if got := g.CountByType(models.Country); got != 0 {
		t.Errorf("CountByType(Country) = %d, want 0", got)
	}
	// B sits in the middle: one inbound, one outbound.
	if got := g.Degree(ids[1]); got != 2 {
		t.Errorf("Degree(B) = %d, want 2", got)
	}
	if got := g.InDegree(ids[0]); got != 0 {
		t.Errorf("InDegree(A) = %d, want 0", got)
	}
	if got := g.OutDegree(ids[3]); got != 0 {
		t.Errorf("OutDegree(D) = %d, want 0", got)
	}
}

func TestFindByName(t *testing.T) {
	g, _ := buildLinear()
	n, ok := g.FindByName("b") // case-insensitive
	if !ok || n.Name != "B" {
		t.Fatalf("FindByName(\"b\") = %v, %v; want node B", n, ok)
	}
	if _, ok := g.FindByName("nonexistent"); ok {
		t.Errorf("FindByName should not resolve an unknown name")
	}
}

func TestFindByNamePrefersCountry(t *testing.T) {
	g := New()
	country := models.NewNode(models.Country, "Shared")
	sector := models.NewNode(models.Sector, "Shared")
	g.AddNode(sector)
	g.AddNode(country)
	n, ok := g.FindByName("Shared")
	if !ok {
		t.Fatal("expected to resolve Shared")
	}
	if n.Type != models.Country {
		t.Errorf("FindByName tie-break = %s, want country", n.Type)
	}
}

func TestAllPathsAvoidsCycles(t *testing.T) {
	g := New()
	a := models.NewNode(models.Sector, "A")
	b := models.NewNode(models.Sector, "B")
	g.AddNode(a)
	g.AddNode(b)
	g.AddEdge(models.Edge{From: a.ID, To: b.ID, Weight: 1})
	g.AddEdge(models.Edge{From: b.ID, To: a.ID, Weight: 1}) // cycle

	// With a cycle, a large depth must still terminate and stay simple.
	paths := g.AllPaths(a.ID, 10)
	for _, p := range paths {
		seen := map[models.NodeID]bool{}
		for _, id := range p {
			if seen[id] {
				t.Fatalf("path %v revisits a node", p)
			}
			seen[id] = true
		}
	}
}
