// Package models defines the core domain types for AtlasGraph: the nodes and
// edges that make up the economic dependency graph.
//
// The graph is deliberately heterogeneous. A single graph contains countries,
// commodities and industrial sectors as nodes, connected by typed, weighted
// edges that describe who depends on whom and how concentrated that dependency
// is. Keeping every entity in one graph lets the simulation engine trace a
// shock across domain boundaries, e.g.:
//
//	Taiwan (country) -> semiconductors (commodity) -> United States (country)
//	    -> AI hardware (sector) -> cloud infrastructure (sector)
package models

import "fmt"

// NodeType classifies a node in the dependency graph.
type NodeType string

const (
	// Country is a sovereign economic actor that produces and/or consumes.
	Country NodeType = "country"
	// Commodity is a traded good or resource (e.g. semiconductors, cobalt).
	Commodity NodeType = "commodity"
	// Sector is an industrial / economic sector that depends on inputs.
	Sector NodeType = "sector"
	// Route is a trade/transit route or chokepoint (e.g. the Suez Canal) that
	// a commodity flow depends on.
	Route NodeType = "route"
	// Company is a firm-level actor. Reserved for future modelling.
	Company NodeType = "company"
)

// EdgeType (a relationship type) classifies the dependency relationship an edge
// represents. The relationship type is central to milestone-3 propagation:
// shock profiles only travel along relationship types they recognise.
//
// In every case the edge direction means "target depends on source": an edge
// A --rel--> B says B is exposed to A through relationship rel.
type EdgeType string

const (
	// RelExports: a country's exports supply a commodity to the market.
	RelExports EdgeType = "exports"
	// RelImports: a commodity is imported/consumed by the target.
	RelImports EdgeType = "imports"
	// RelSupplies: a generic upstream supply relationship.
	RelSupplies EdgeType = "supplies"
	// RelDependsOn: a generic downstream dependency.
	RelDependsOn EdgeType = "depends_on"
	// RelUsedBy: an input (commodity/sector) is used by a downstream sector.
	RelUsedBy EdgeType = "used_by"
	// RelRouteExposure: a commodity flow depends on a transit route/chokepoint.
	RelRouteExposure EdgeType = "route_exposure"
	// RelPriceExposure: a downstream entity is exposed to a commodity's price.
	RelPriceExposure EdgeType = "price_exposure"
	// RelIndustryDependency: a sector depends on a country/commodity industrial base.
	RelIndustryDependency EdgeType = "industry_dependency"
	// RelCompanyDependency: a firm-level dependency (reserved for company nodes).
	RelCompanyDependency EdgeType = "company_dependency"
	// RelMacroExposure: a broad macroeconomic exposure.
	RelMacroExposure EdgeType = "macro_exposure"
	// RelRealExports: a country supplies a commodity per UN Comtrade dependency data.
	RelRealExports EdgeType = "real_exports"
	// RelRealImportDependency: an importer depends on a commodity per UN Comtrade data.
	RelRealImportDependency EdgeType = "real_import_dependency"
	// RelShippingDependency: a dependency on shipping/logistics capacity.
	RelShippingDependency EdgeType = "shipping_dependency"
)

// validRelationships is the closed set of relationship types the loader accepts.
var validRelationships = map[EdgeType]struct{}{
	RelExports: {}, RelImports: {}, RelSupplies: {}, RelDependsOn: {},
	RelUsedBy: {}, RelRouteExposure: {}, RelPriceExposure: {},
	RelIndustryDependency: {}, RelCompanyDependency: {}, RelMacroExposure: {},
	RelShippingDependency: {}, RelRealExports: {}, RelRealImportDependency: {},
}

// IsValidRelationship reports whether t is a recognised relationship type.
func IsValidRelationship(t EdgeType) bool {
	_, ok := validRelationships[t]
	return ok
}

// RelationshipTypes returns all valid relationship types (unordered).
func RelationshipTypes() []EdgeType {
	out := make([]EdgeType, 0, len(validRelationships))
	for t := range validRelationships {
		out = append(out, t)
	}
	return out
}

// ShockType names a category of economic shock. Each maps to a propagation
// profile in the simulation package.
type ShockType string

const (
	ShockExportCollapse  ShockType = "export_collapse"
	ShockSupplyCut       ShockType = "supply_cut"
	ShockPriceSpike      ShockType = "price_spike"
	ShockRouteDisruption ShockType = "route_disruption"
)

// validShockTypes is the closed set of shock types the engine understands.
var validShockTypes = map[ShockType]struct{}{
	ShockExportCollapse: {}, ShockSupplyCut: {},
	ShockPriceSpike: {}, ShockRouteDisruption: {},
}

// IsValidShockType reports whether s is a recognised shock type.
func IsValidShockType(s string) bool {
	_, ok := validShockTypes[ShockType(s)]
	return ok
}

// NodeID is a stable, unique identifier for a node. It is namespaced by type
// (e.g. "country:taiwan") so that a commodity and a country can never collide.
type NodeID string

// NewNodeID builds a namespaced identifier from a node type and display name.
func NewNodeID(t NodeType, name string) NodeID {
	return NodeID(fmt.Sprintf("%s:%s", t, slug(name)))
}

// Node is a single entity in the dependency graph.
type Node struct {
	ID   NodeID
	Name string
	Type NodeType
	// Source labels the provenance of a node when it was added from real data.
	Source string
	// GeneratedFromRealData marks nodes created during graph fusion from processed panels.
	GeneratedFromRealData bool
}

// NewNode constructs a Node with a derived, namespaced ID.
func NewNode(t NodeType, name string) Node {
	return Node{ID: NewNodeID(t, name), Name: name, Type: t}
}

// Edge is a directed, weighted dependency from one node to another.
//
// Weight expresses how strongly the target depends on this particular flow
// (0 = no reliance, 1 = total reliance). Concentration expresses how
// concentrated the supply behind this flow is (0 = highly diversified,
// 1 = single-source). Both are bounded to [0, 1].
//
// The remaining fields drive typed propagation (milestone 3):
//   - Commodity scopes the edge to a commodity; a shock will not cross into a
//     different commodity unless the profile or the edge explicitly permits it.
//   - Sector is optional context for sector-scoped reasoning.
//   - PropagationEnabled can switch an edge off for propagation entirely.
//   - AllowedShockTypes, when non-empty, restricts which shocks may travel the
//     edge.
//   - CrossCommodity marks an edge as an explicit cross-commodity bridge.
type Edge struct {
	From          NodeID
	To            NodeID
	Type          EdgeType
	Weight        float64
	Concentration float64

	Commodity          string
	Sector             string
	PropagationEnabled bool
	AllowedShockTypes  []string
	CrossCommodity     bool

	// Real-data fusion metadata (zero values = not set / demo edge).
	RealData      bool
	DataSource    string
	TradeValueUSD float64
	Year          int
	HSCode        string
	Importer      string
}

// slug normalizes a display name into a lowercase, space-free token suitable
// for embedding in an identifier.
func slug(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			out = append(out, r)
		case r == ' ' || r == '\t' || r == '-' || r == '_':
			out = append(out, '_')
		default:
			// Drop punctuation that would make IDs noisy.
		}
	}
	return string(out)
}
