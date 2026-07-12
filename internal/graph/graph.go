// Package graph implements an in-memory, directed, weighted dependency graph
// plus the traversal primitives the simulation engine relies on.
//
// The implementation favours clarity over raw performance: the dataset for the
// MVP is small and seeded, and the engine's correctness is far more valuable
// here than micro-optimisation. The public surface is intentionally small so
// it can later be backed by Neo4j without changing callers.
package graph

import (
	"sort"
	"strings"

	"github.com/atlasgraph/atlas/internal/models"
)

// Graph is a directed, weighted, heterogeneous dependency graph. It is not
// safe for concurrent mutation; build it once, then read concurrently.
type Graph struct {
	nodes map[models.NodeID]models.Node
	out   map[models.NodeID][]models.Edge
	in    map[models.NodeID][]models.Edge
}

// New returns an empty graph ready for population.
func New() *Graph {
	return &Graph{
		nodes: make(map[models.NodeID]models.Node),
		out:   make(map[models.NodeID][]models.Edge),
		in:    make(map[models.NodeID][]models.Edge),
	}
}

// AddNode inserts or replaces a node. Adding the same node twice is harmless.
func (g *Graph) AddNode(n models.Node) {
	g.nodes[n.ID] = n
}

// AddEdge inserts a directed edge. Endpoints are auto-registered as bare nodes
// if they were not added explicitly, which keeps seed code concise while still
// guaranteeing referential integrity for traversal.
func (g *Graph) AddEdge(e models.Edge) {
	if _, ok := g.nodes[e.From]; !ok {
		g.nodes[e.From] = models.Node{ID: e.From}
	}
	if _, ok := g.nodes[e.To]; !ok {
		g.nodes[e.To] = models.Node{ID: e.To}
	}
	g.out[e.From] = append(g.out[e.From], e)
	g.in[e.To] = append(g.in[e.To], e)
}

// Clone returns a deep copy of the graph for non-destructive fusion overlays.
func (g *Graph) Clone() *Graph {
	if g == nil {
		return New()
	}
	ng := New()
	for _, n := range g.Nodes() {
		ng.AddNode(n)
	}
	for _, n := range g.Nodes() {
		for _, e := range g.OutEdges(n.ID) {
			ng.AddEdge(e)
		}
	}
	return ng
}

// Node returns the node for an ID and whether it exists.
func (g *Graph) Node(id models.NodeID) (models.Node, bool) {
	n, ok := g.nodes[id]
	return n, ok
}

// NodeByName resolves a node by its display name and type. Resolution is
// O(1) because IDs are derived deterministically from (type, name).
func (g *Graph) NodeByName(t models.NodeType, name string) (models.Node, bool) {
	return g.Node(models.NewNodeID(t, name))
}

// typePriority orders node types when a bare name could match several. Lower
// wins. This makes FindByName deterministic and biases toward the most likely
// "source" of a shock (a country) over downstream entities.
func typePriority(t models.NodeType) int {
	switch t {
	case models.Country:
		return 0
	case models.Route:
		return 1
	case models.Commodity:
		return 2
	case models.Company:
		return 3
	case models.Sector:
		return 4
	default:
		return 5
	}
}

// FindByName resolves a node by display name across all node types
// (case-insensitive). When multiple types share a name it returns the
// highest-priority match (see typePriority). This lets callers accept a plain
// entity name without needing to know its type up front.
func (g *Graph) FindByName(name string) (models.Node, bool) {
	var best models.Node
	found := false
	for _, n := range g.Nodes() { // sorted, so iteration is deterministic
		if !strings.EqualFold(n.Name, name) {
			continue
		}
		if !found || typePriority(n.Type) < typePriority(best.Type) {
			best, found = n, true
		}
	}
	return best, found
}

// Nodes returns all nodes sorted by ID for deterministic iteration/output.
func (g *Graph) Nodes() []models.Node {
	out := make([]models.Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int { return len(g.nodes) }

// CountByType returns the number of nodes of a given type.
func (g *Graph) CountByType(t models.NodeType) int {
	n := 0
	for _, node := range g.nodes {
		if node.Type == t {
			n++
		}
	}
	return n
}

// OutDegree / InDegree / Degree report connectivity for a node.
func (g *Graph) OutDegree(id models.NodeID) int { return len(g.out[id]) }
func (g *Graph) InDegree(id models.NodeID) int  { return len(g.in[id]) }
func (g *Graph) Degree(id models.NodeID) int    { return len(g.out[id]) + len(g.in[id]) }

// EdgeCount returns the number of directed edges in the graph.
func (g *Graph) EdgeCount() int {
	total := 0
	for _, edges := range g.out {
		total += len(edges)
	}
	return total
}

// OutEdges returns outgoing edges from a node (the things that depend on it).
func (g *Graph) OutEdges(id models.NodeID) []models.Edge { return g.out[id] }

// InEdges returns incoming edges to a node (the things it depends on).
func (g *Graph) InEdges(id models.NodeID) []models.Edge { return g.in[id] }

// EdgeBetween returns the first edge from -> to and whether one exists.
func (g *Graph) EdgeBetween(from, to models.NodeID) (models.Edge, bool) {
	for _, e := range g.out[from] {
		if e.To == to {
			return e, true
		}
	}
	return models.Edge{}, false
}

// Neighbors returns the unique downstream node IDs reachable in one hop.
func (g *Graph) Neighbors(id models.NodeID) []models.NodeID {
	seen := make(map[models.NodeID]struct{})
	out := make([]models.NodeID, 0, len(g.out[id]))
	for _, e := range g.out[id] {
		if _, ok := seen[e.To]; ok {
			continue
		}
		seen[e.To] = struct{}{}
		out = append(out, e.To)
	}
	return out
}

// AllPaths enumerates every simple (cycle-free) downstream path starting at
// `from`, traversing at most maxDepth edges. Each returned path is a slice of
// node IDs beginning with `from`. Paths of length 1 (just the source) are not
// returned; only paths that traverse at least one edge are meaningful here.
//
// maxDepth is the number of edges, so a path may contain up to maxDepth+1
// nodes. The traversal is bounded and deterministic.
func (g *Graph) AllPaths(from models.NodeID, maxDepth int) [][]models.NodeID {
	var paths [][]models.NodeID
	if maxDepth < 1 {
		return paths
	}
	visited := map[models.NodeID]bool{from: true}
	current := []models.NodeID{from}

	var walk func()
	walk = func() {
		if len(current)-1 >= maxDepth {
			return
		}
		for _, nid := range g.Neighbors(current[len(current)-1]) {
			if visited[nid] {
				continue // keep paths simple to avoid cycles
			}
			visited[nid] = true
			current = append(current, nid)

			// Record this path (a copy, since `current` is reused).
			path := make([]models.NodeID, len(current))
			copy(path, current)
			paths = append(paths, path)

			walk()

			current = current[:len(current)-1]
			delete(visited, nid)
		}
	}
	walk()
	return paths
}

// PathsBetween returns every simple path from `from` to `to` traversing at
// most maxDepth edges. Paths are returned shortest-first.
func (g *Graph) PathsBetween(from, to models.NodeID, maxDepth int) [][]models.NodeID {
	var out [][]models.NodeID
	for _, p := range g.AllPaths(from, maxDepth) {
		if p[len(p)-1] == to {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i]) < len(out[j]) })
	return out
}

// EdgePath is a simple path expressed as alternating nodes and the concrete
// edges that connect them: Edges[i] runs from Nodes[i] to Nodes[i+1]. Unlike a
// bare []NodeID path it preserves edge identity (relationship type, commodity,
// weight), which callers need to label hops and to apply per-edge propagation
// rules.
type EdgePath struct {
	Nodes []models.Node
	Edges []models.Edge
}

// Weight returns the product of the path's edge weights, mirroring how the
// simulation engine scores a dependency chain.
func (p EdgePath) Weight() float64 {
	w := 1.0
	for _, e := range p.Edges {
		w *= e.Weight
	}
	return w
}

// PathsBetweenFunc enumerates every simple path from `from` to `to` traversing
// at most maxDepth edges, following only edges for which allow returns true. A
// nil allow permits all edges. Each edge that allow rejects is passed to
// onBlock (when non-nil) so callers can report which branches were pruned;
// onBlock may be invoked more than once for the same edge across branches, so
// callers that need uniqueness should de-duplicate.
//
// Because it walks concrete edges rather than collapsing to neighbours,
// parallel edges of different relationship types between the same pair yield
// distinct paths. Paths are returned shortest-first, then by descending path
// weight, so the most direct, strongest dependency chains lead.
func (g *Graph) PathsBetweenFunc(from, to models.NodeID, maxDepth int, allow func(models.Edge) bool, onBlock func(models.Edge)) []EdgePath {
	var paths []EdgePath
	if maxDepth < 1 {
		return paths
	}
	fromNode, ok := g.Node(from)
	if !ok {
		return paths
	}

	visited := map[models.NodeID]bool{from: true}
	nodes := []models.Node{fromNode}
	var edges []models.Edge

	var walk func()
	walk = func() {
		last := nodes[len(nodes)-1]
		if last.ID == to && len(edges) > 0 {
			paths = append(paths, snapshotEdgePath(nodes, edges))
			return // a simple path: do not extend through the target
		}
		if len(edges) >= maxDepth {
			return
		}
		for _, e := range g.OutEdges(last.ID) {
			if visited[e.To] {
				continue // keep paths simple to avoid cycles
			}
			if allow != nil && !allow(e) {
				if onBlock != nil {
					onBlock(e)
				}
				continue
			}
			toNode, _ := g.Node(e.To)
			visited[e.To] = true
			nodes = append(nodes, toNode)
			edges = append(edges, e)

			walk()

			nodes = nodes[:len(nodes)-1]
			edges = edges[:len(edges)-1]
			delete(visited, e.To)
		}
	}
	walk()

	sort.SliceStable(paths, func(i, j int) bool {
		if len(paths[i].Nodes) != len(paths[j].Nodes) {
			return len(paths[i].Nodes) < len(paths[j].Nodes)
		}
		return paths[i].Weight() > paths[j].Weight()
	})
	return paths
}

func snapshotEdgePath(nodes []models.Node, edges []models.Edge) EdgePath {
	n := make([]models.Node, len(nodes))
	copy(n, nodes)
	e := make([]models.Edge, len(edges))
	copy(e, edges)
	return EdgePath{Nodes: n, Edges: e}
}
