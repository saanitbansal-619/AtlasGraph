package simulation

import (
	"sort"

	"github.com/atlasgraph/atlas/internal/models"
)

// ShockProfile is a typed description of how a category of shock behaves. It
// declares which relationship types a shock of this kind can travel along, how
// quickly it attenuates, how deep it is sensible to propagate, and whether it
// is allowed to jump between commodities.
//
// Profiles are the heart of the realism layer: an export_collapse in
// semiconductors and a route_disruption in crude oil traverse the same graph
// very differently because their profiles permit different relationships.
type ShockProfile struct {
	Type                 string            // matches models.ShockType
	Name                 string            // human-friendly name
	Description          string            // one-line explanation
	AllowedRelationships []models.EdgeType // relationship types the shock may use
	Attenuation          float64           // per-hop decay multiplier in (0,1]
	RecommendedDepth     int               // suggested max propagation depth
	CrossCommodity       bool              // may the shock cross commodity boundaries?

	allowed map[models.EdgeType]bool // derived lookup set
}

// Allows reports whether the profile propagates along a relationship type.
func (p ShockProfile) Allows(rel models.EdgeType) bool {
	return p.allowed[rel]
}

// AllowedRelationshipStrings returns the allowed relationships as strings, in a
// stable order, for display and JSON output.
func (p ShockProfile) AllowedRelationshipStrings() []string {
	out := make([]string, 0, len(p.AllowedRelationships))
	for _, r := range p.AllowedRelationships {
		out = append(out, string(r))
	}
	sort.Strings(out)
	return out
}

// shockProfiles is the registry of supported shock profiles, keyed by type.
var shockProfiles = buildProfiles(
	ShockProfile{
		Type:        string(models.ShockExportCollapse),
		Name:        "Export Collapse",
		Description: "A producer's exports of a commodity collapse, cascading through importers and the industries that rely on them.",
		AllowedRelationships: []models.EdgeType{
			models.RelExports, models.RelImports, models.RelSupplies,
			models.RelDependsOn, models.RelUsedBy,
			models.RelIndustryDependency, models.RelCompanyDependency,
			models.RelRealExports, models.RelRealImportDependency,
		},
		Attenuation:      0.85,
		RecommendedDepth: 3,
		CrossCommodity:   false,
	},
	ShockProfile{
		Type:        string(models.ShockSupplyCut),
		Name:        "Supply Cut",
		Description: "Physical supply of a commodity is cut, propagating through trade and industrial use.",
		AllowedRelationships: []models.EdgeType{
			models.RelExports, models.RelImports, models.RelSupplies,
			models.RelDependsOn, models.RelUsedBy,
			models.RelRealExports, models.RelRealImportDependency,
		},
		Attenuation:      0.80,
		RecommendedDepth: 3,
		CrossCommodity:   false,
	},
	ShockProfile{
		Type:        string(models.ShockPriceSpike),
		Name:        "Price Spike",
		Description: "A commodity's price spikes, propagating through price-sensitive and industrial dependencies.",
		AllowedRelationships: []models.EdgeType{
			models.RelPriceExposure, models.RelDependsOn,
			models.RelUsedBy, models.RelIndustryDependency,
		},
		Attenuation:      0.75,
		RecommendedDepth: 3,
		CrossCommodity:   false,
	},
	ShockProfile{
		Type:        string(models.ShockRouteDisruption),
		Name:        "Route Disruption",
		Description: "A trade route/chokepoint is disrupted, propagating through route exposure, trade flows and shipping.",
		AllowedRelationships: []models.EdgeType{
			models.RelRouteExposure, models.RelImports,
			models.RelExports, models.RelShippingDependency,
		},
		Attenuation:      0.80,
		RecommendedDepth: 3,
		CrossCommodity:   false,
	},
)

// buildProfiles indexes profiles by type and derives their lookup sets.
func buildProfiles(profiles ...ShockProfile) map[string]ShockProfile {
	m := make(map[string]ShockProfile, len(profiles))
	for _, p := range profiles {
		p.allowed = make(map[models.EdgeType]bool, len(p.AllowedRelationships))
		for _, r := range p.AllowedRelationships {
			p.allowed[r] = true
		}
		m[p.Type] = p
	}
	return m
}

// ProfileFor returns the profile for a shock type and whether it exists.
func ProfileFor(shockType string) (ShockProfile, bool) {
	p, ok := shockProfiles[shockType]
	return p, ok
}

// AllProfiles returns every registered profile, sorted by type.
func AllProfiles() []ShockProfile {
	out := make([]ShockProfile, 0, len(shockProfiles))
	for _, p := range shockProfiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// ProfileTypes returns the supported shock-type identifiers, sorted.
func ProfileTypes() []string {
	out := make([]string, 0, len(shockProfiles))
	for t := range shockProfiles {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
