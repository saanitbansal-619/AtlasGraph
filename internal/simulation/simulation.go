// Package simulation runs economic shock scenarios over the dependency graph.
//
// A shock originates at a (source, commodity) pair with a typed shock kind
// (e.g. export_collapse, route_disruption). The engine injects the resulting
// disruption at the commodity node and propagates it downstream — but only
// along edges that the shock's profile and the propagation rules permit. This
// keeps a semiconductor shock from leaking into unrelated commodities such as
// crude oil or lithium. It then recomputes fragility for every affected node
// and summarises the blast radius, including the branches that were blocked.
package simulation

import (
	"fmt"
	"sort"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/scoring"
)

// Distance bands used to classify how far an affected node sits from the
// source. The commodity itself is the origin of the disruption.
const (
	distCommodity   = 1 // the shocked commodity
	distDirect      = 2 // entities consuming the commodity directly
	distSecondOrder = 3 // entities depending on those consumers
)

// ShockRequest describes a scenario to simulate.
type ShockRequest struct {
	Source    string  // source entity name, e.g. "Taiwan" or "Suez Canal"
	Commodity string  // commodity name, e.g. "semiconductors"
	ShockType string  // e.g. "export_collapse" (must match a ShockProfile)
	DropPct   float64 // export/flow drop, 0..100
	Depth     int     // max edges to traverse from the source
}

// NodeImpact captures what a shock did to a single node.
type NodeImpact struct {
	Node           models.Node
	Distance       int     // hops from the source (commodity = 1)
	Impact         float64 // disruption fraction reaching this node, [0,1]
	BaseFragility  float64 // fragility before the shock
	ShockFragility float64 // fragility under the shock
	Delta          float64 // ShockFragility - BaseFragility
}

// PathEdge labels a single hop of a dependency path.
type PathEdge struct {
	Relationship string
	Commodity    string
	Weight       float64
}

// Path is an affected dependency chain from the source through the commodity.
// Edges[i] is the relationship from Nodes[i] to Nodes[i+1].
type Path struct {
	Nodes      []models.Node
	Edges      []PathEdge
	PathWeight float64 // product of edge weights along the path
	EndImpact  float64 // impact reaching the final node of the path
}

// BlockedEdge records a branch the rules prevented a shock from taking. These
// power the --explain view and make the engine's decisions auditable.
type BlockedEdge struct {
	From           models.Node
	To             models.Node
	Relationship   string
	Commodity      string
	Reason         string
	CrossCommodity bool // true when blocked because it crosses commodities
}

// Result is the structured outcome of a shock simulation.
type Result struct {
	Request         ShockRequest
	Profile         ShockProfile
	SourceNode      models.Node
	CommodityNode   models.Node
	ActiveCommodity string
	InitialImpact   float64 // disruption injected at the commodity node, [0,1]
	GraphNodeCount  int     // total nodes in the graph the shock ran against

	Direct      []NodeImpact // distance == 2
	SecondOrder []NodeImpact // distance == 3
	AllAffected []NodeImpact // every node with non-trivial impact, by Delta desc

	Paths        []Path        // affected dependency paths, strongest first
	BlockedEdges []BlockedEdge // branches the rules pruned, for transparency

	TopCountries   []NodeImpact
	TopCommodities []NodeImpact
	TopSectors     []NodeImpact
}

// BlockedCommodities returns the distinct commodities whose branches were
// blocked specifically because they were unrelated to the shock's commodity
// (cross-commodity blocks), not merely because of a relationship mismatch.
func (r Result) BlockedCommodities() []string {
	seen := map[string]struct{}{}
	var out []string
	for _, b := range r.BlockedEdges {
		if b.Commodity == "" || !b.CrossCommodity {
			continue
		}
		if _, ok := seen[b.Commodity]; ok {
			continue
		}
		seen[b.Commodity] = struct{}{}
		out = append(out, b.Commodity)
	}
	sort.Strings(out)
	return out
}

// Run executes a shock scenario against the graph and returns a Result.
func Run(g *graph.Graph, cfg config.Config, req ShockRequest) (Result, error) {
	profile, ok := ProfileFor(req.ShockType)
	if !ok {
		return Result{}, fmt.Errorf("unknown shock type %q (valid: %v)", req.ShockType, ProfileTypes())
	}
	source, ok := g.FindByName(req.Source)
	if !ok {
		return Result{}, fmt.Errorf("unknown source entity %q", req.Source)
	}
	commodity, ok := g.NodeByName(models.Commodity, req.Commodity)
	if !ok {
		return Result{}, fmt.Errorf("unknown commodity %q", req.Commodity)
	}
	originEdge, ok := g.EdgeBetween(source.ID, commodity.ID)
	if !ok {
		return Result{}, fmt.Errorf("%s has no direct link to %s in the current graph", req.Source, req.Commodity)
	}
	if req.Depth < 1 {
		return Result{}, fmt.Errorf("depth must be >= 1, got %d", req.Depth)
	}
	if req.DropPct < 0 || req.DropPct > 100 {
		return Result{}, fmt.Errorf("drop must be within 0..100, got %v", req.DropPct)
	}

	activeCommodity := commodity.Name

	// The disruption that actually hits the commodity market is the export/flow
	// drop scaled by the source's share (concentration) of that commodity.
	drop := req.DropPct / 100.0
	initialImpact := clamp01(drop * originEdge.Concentration)

	impact, dist, blocked := propagate(g, cfg, profile, commodity.ID, activeCommodity, initialImpact, req.Depth)

	res := Result{
		Request:         req,
		Profile:         profile,
		SourceNode:      source,
		CommodityNode:   commodity,
		ActiveCommodity: activeCommodity,
		InitialImpact:   initialImpact,
		GraphNodeCount:  g.NodeCount(),
		BlockedEdges:    blocked,
	}

	for id, imp := range impact {
		if imp < cfg.PropagationEpsilon {
			continue
		}
		node, _ := g.Node(id)
		base := scoring.NodeFragility(g, id, 0, cfg)
		shock := scoring.NodeFragility(g, id, imp, cfg)
		ni := NodeImpact{
			Node:           node,
			Distance:       dist[id],
			Impact:         imp,
			BaseFragility:  base,
			ShockFragility: shock,
			Delta:          shock - base,
		}
		res.AllAffected = append(res.AllAffected, ni)
		switch ni.Distance {
		case distDirect:
			res.Direct = append(res.Direct, ni)
		case distSecondOrder:
			res.SecondOrder = append(res.SecondOrder, ni)
		}
	}

	sortByDelta(res.AllAffected)
	sortByDelta(res.Direct)
	sortByDelta(res.SecondOrder)

	res.TopCountries = topByType(res.AllAffected, models.Country, cfg.TopN)
	res.TopCommodities = topByType(res.AllAffected, models.Commodity, cfg.TopN)
	res.TopSectors = topByType(res.AllAffected, models.Sector, cfg.TopN)
	res.Paths = affectedPaths(g, profile, source, commodity, activeCommodity, originEdge, impact, cfg, req.Depth)

	return res, nil
}

// propagate performs a bounded, layered breadth-first spread of disruption from
// the commodity node, following only edges the rules permit. Impact attenuates
// by edge weight and the profile's attenuation factor at each hop and is capped
// at 1. It returns the accumulated impact per node, the BFS distance from the
// source (commodity = distCommodity), and the edges that were blocked.
func propagate(g *graph.Graph, cfg config.Config, profile ShockProfile, commodity models.NodeID, activeCommodity string, initial float64, depth int) (map[models.NodeID]float64, map[models.NodeID]int, []BlockedEdge) {
	impact := map[models.NodeID]float64{commodity: initial}
	dist := map[models.NodeID]int{commodity: distCommodity}
	frontier := []models.NodeID{commodity}

	var blocked []BlockedEdge
	blockedSeen := map[string]bool{}

	for len(frontier) > 0 {
		var next []models.NodeID
		for _, u := range frontier {
			if dist[u] >= depth {
				continue // reached the requested traversal horizon
			}
			for _, e := range g.OutEdges(u) {
				if dec := Evaluate(profile, e, activeCommodity); !dec.Allowed {
					key := string(e.From) + "|" + string(e.To) + "|" + string(e.Type)
					if !blockedSeen[key] {
						blockedSeen[key] = true
						from, _ := g.Node(e.From)
						to, _ := g.Node(e.To)
						blocked = append(blocked, BlockedEdge{
							From: from, To: to, Relationship: string(e.Type),
							Commodity: e.Commodity, Reason: dec.Reason,
							CrossCommodity: dec.CrossCommodity,
						})
					}
					continue
				}
				contrib := impact[u] * e.Weight * profile.Attenuation
				if contrib < cfg.PropagationEpsilon {
					continue // prune negligible ripples
				}
				if _, seen := dist[e.To]; !seen {
					dist[e.To] = dist[u] + 1
					next = append(next, e.To)
				}
				impact[e.To] = clamp01(impact[e.To] + contrib)
			}
		}
		frontier = next
	}
	return impact, dist, blocked
}

// affectedPaths enumerates the dependency chains that begin at the source, pass
// through the shocked commodity via the origin edge, and continue only along
// rule-permitted edges to nodes that absorbed impact. Each hop is labelled with
// its relationship type, so the output is fully auditable. Paths are ranked by
// the impact reaching their endpoint.
func affectedPaths(g *graph.Graph, profile ShockProfile, source, commodity models.Node, activeCommodity string, originEdge models.Edge, impact map[models.NodeID]float64, cfg config.Config, depth int) []Path {
	var paths []Path

	visited := map[models.NodeID]bool{source.ID: true, commodity.ID: true}
	nodes := []models.Node{source, commodity}
	edges := []PathEdge{toPathEdge(originEdge)}

	var dfs func()
	dfs = func() {
		last := nodes[len(nodes)-1]
		if impact[last.ID] >= cfg.PropagationEpsilon {
			p := snapshotPath(nodes, edges)
			p.EndImpact = impact[last.ID]
			paths = append(paths, p)
		}
		if len(nodes)-1 >= depth {
			return
		}
		for _, e := range g.OutEdges(last.ID) {
			if visited[e.To] {
				continue
			}
			if dec := Evaluate(profile, e, activeCommodity); !dec.Allowed {
				continue
			}
			if impact[e.To] < cfg.PropagationEpsilon {
				continue
			}
			to, _ := g.Node(e.To)
			visited[e.To] = true
			nodes = append(nodes, to)
			edges = append(edges, toPathEdge(e))

			dfs()

			nodes = nodes[:len(nodes)-1]
			edges = edges[:len(edges)-1]
			delete(visited, e.To)
		}
	}
	dfs()

	sort.SliceStable(paths, func(i, j int) bool {
		if paths[i].EndImpact != paths[j].EndImpact {
			return paths[i].EndImpact > paths[j].EndImpact
		}
		return len(paths[i].Nodes) < len(paths[j].Nodes)
	})
	return paths
}

func snapshotPath(nodes []models.Node, edges []PathEdge) Path {
	n := make([]models.Node, len(nodes))
	copy(n, nodes)
	e := make([]PathEdge, len(edges))
	copy(e, edges)
	weight := 1.0
	for _, pe := range e {
		weight *= pe.Weight
	}
	return Path{Nodes: n, Edges: e, PathWeight: weight}
}

func toPathEdge(e models.Edge) PathEdge {
	return PathEdge{Relationship: string(e.Type), Commodity: e.Commodity, Weight: e.Weight}
}

func topByType(all []NodeImpact, t models.NodeType, n int) []NodeImpact {
	var out []NodeImpact
	for _, ni := range all { // `all` is already sorted by Delta desc
		if ni.Node.Type == t {
			out = append(out, ni)
			if len(out) == n {
				break
			}
		}
	}
	return out
}

func sortByDelta(s []NodeImpact) {
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].Delta != s[j].Delta {
			return s[i].Delta > s[j].Delta
		}
		return s[i].Node.Name < s[j].Node.Name
	})
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
