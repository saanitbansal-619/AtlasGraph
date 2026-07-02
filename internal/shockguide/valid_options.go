// Package shockguide builds graph-aware shock UI guidance (valid source/commodity
// combinations, warnings) without changing propagation logic.
package shockguide

import (
	"sort"
	"strings"

	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/simulation"
)

// CommodityOption is a commodity directly linked to a shock source.
type CommodityOption struct {
	Commodity     string
	ShockTypes    []string
	Relationships []string
}

// SourceOption groups valid commodities for one shock source entity.
type SourceOption struct {
	Source      string
	Type        string
	Commodities []CommodityOption
}

// ValidOptions is the graph-derived set of runnable shock combinations.
type ValidOptions struct {
	Sources []SourceOption
}

// shockTypesForRelationship maps a direct source→commodity edge type to shock
// types the analyst can meaningfully originate from that link.
var shockTypesForRelationship = map[models.EdgeType][]string{
	models.RelExports:       {string(models.ShockExportCollapse), string(models.ShockSupplyCut)},
	models.RelSupplies:      {string(models.ShockSupplyCut)},
	models.RelRouteExposure: {string(models.ShockRouteDisruption)},
	models.RelPriceExposure: {string(models.ShockPriceSpike)},
}

// BuildValidOptions derives valid source/commodity/shock_type triples from the
// loaded graph. Only direct source→commodity edges with a meaningful origin
// relationship are included. Sources with no valid commodities are omitted.
func BuildValidOptions(g *graph.Graph, sourceFilter string) ValidOptions {
	filter := strings.TrimSpace(sourceFilter)
	out := ValidOptions{Sources: []SourceOption{}}

	for _, n := range g.Nodes() {
		if n.Type == models.Commodity || n.Type == models.Sector {
			continue
		}
		if filter != "" && !strings.EqualFold(n.Name, filter) {
			continue
		}

		commodities := commoditiesForSource(g, n)
		if len(commodities) == 0 {
			continue
		}
		out.Sources = append(out.Sources, SourceOption{
			Source:      n.Name,
			Type:        string(n.Type),
			Commodities: commodities,
		})
	}

	sort.Slice(out.Sources, func(i, j int) bool {
		return out.Sources[i].Source < out.Sources[j].Source
	})
	return out
}

func commoditiesForSource(g *graph.Graph, src models.Node) []CommodityOption {
	byCommodity := map[string]*commodityAcc{}

	for _, e := range g.OutEdges(src.ID) {
		to, ok := g.Node(e.To)
		if !ok || to.Type != models.Commodity {
			continue
		}
		shockTypes := shockTypesForRelationship[e.Type]
		if len(shockTypes) == 0 {
			continue
		}
		// Origin edge must be runnable: profile allows this relationship.
		if !originShockTypeAllowed(e.Type, shockTypes) {
			continue
		}

		acc, ok := byCommodity[to.Name]
		if !ok {
			acc = &commodityAcc{
				rels:       map[string]struct{}{},
				shockTypes: map[string]struct{}{},
			}
			byCommodity[to.Name] = acc
		}
		acc.rels[string(e.Type)] = struct{}{}
		for _, st := range shockTypes {
			if profileAllowsOrigin(e.Type, st) {
				acc.shockTypes[st] = struct{}{}
			}
		}
	}

	names := make([]string, 0, len(byCommodity))
	for name, acc := range byCommodity {
		if len(acc.shockTypes) == 0 {
			continue
		}
		names = append(names, name)
		_ = acc
	}
	sort.Strings(names)

	out := make([]CommodityOption, 0, len(names))
	for _, name := range names {
		acc := byCommodity[name]
		out = append(out, CommodityOption{
			Commodity:     name,
			ShockTypes:    sortedKeys(acc.shockTypes),
			Relationships: sortedKeys(acc.rels),
		})
	}
	return out
}

type commodityAcc struct {
	rels       map[string]struct{}
	shockTypes map[string]struct{}
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func originShockTypeAllowed(rel models.EdgeType, candidates []string) bool {
	for _, st := range candidates {
		if profileAllowsOrigin(rel, st) {
			return true
		}
	}
	return false
}

func profileAllowsOrigin(rel models.EdgeType, shockType string) bool {
	profile, ok := simulation.ProfileFor(shockType)
	if !ok {
		return false
	}
	return profile.Allows(rel)
}

// PairValid reports whether source, commodity, and shockType form a valid
// direct-edge combination on the graph.
func PairValid(g *graph.Graph, source, commodity, shockType string) bool {
	src, ok := g.FindByName(source)
	if !ok {
		return false
	}
	com, ok := g.NodeByName(models.Commodity, commodity)
	if !ok {
		return false
	}
	profile, ok := simulation.ProfileFor(shockType)
	if !ok {
		return false
	}

	for _, e := range g.OutEdges(src.ID) {
		if e.To != com.ID {
			continue
		}
		candidates := shockTypesForRelationship[e.Type]
		if !contains(candidates, shockType) {
			continue
		}
		if profile.Allows(e.Type) {
			return true
		}
	}
	return false
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
