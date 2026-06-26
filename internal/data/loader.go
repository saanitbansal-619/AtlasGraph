// Package data loads the AtlasGraph dependency graph and scenario presets from
// JSON. The JSON files are the single source of truth; this package turns them
// into typed, validated Go structs and an in-memory graph.
//
// Data can come from two places:
//
//   - Default(): the dataset embedded in the binary (data/sample), so atlas
//     runs anywhere with no external files.
//   - Load(dir): a dataset on disk, selected via the CLI `--data` flag.
//
// Both share the same loader and validation, so behaviour is identical.
package data

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	sampledata "github.com/atlasgraph/atlas/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
)

// Standard dataset file names.
const (
	entitiesFileName     = "entities.json"
	dependenciesFileName = "dependencies.json"
	scenariosFileName    = "scenarios.json"
)

// Scenario is a saved shock preset.
type Scenario struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Source       string  `json:"source"`
	Commodity    string  `json:"commodity"`
	ShockType    string  `json:"shock_type"`
	ShockPercent float64 `json:"shock_percent"`
	Depth        int     `json:"depth"`
	Description  string  `json:"description"`
}

// Dataset is a fully loaded, validated dataset: the dependency graph plus the
// scenario presets that ship alongside it.
type Dataset struct {
	Graph     *graph.Graph
	Scenarios []Scenario
}

// Scenario looks up a preset by id.
func (d *Dataset) Scenario(id string) (Scenario, bool) {
	for _, s := range d.Scenarios {
		if s.ID == id {
			return s, true
		}
	}
	return Scenario{}, false
}

// --- JSON wire formats (decoupled from the internal model) -----------------

type entityDTO struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type entitiesFile struct {
	Countries   []entityDTO `json:"countries"`
	Commodities []entityDTO `json:"commodities"`
	Sectors     []entityDTO `json:"sectors"`
	Routes      []entityDTO `json:"routes"`
	Companies   []entityDTO `json:"companies"`
}

type dependencyDTO struct {
	Source             string   `json:"source"`
	Target             string   `json:"target"`
	Relationship       string   `json:"relationship_type"`
	Weight             float64  `json:"weight"`
	Concentration      *float64 `json:"concentration,omitempty"`
	Commodity          string   `json:"commodity,omitempty"`
	Sector             string   `json:"sector,omitempty"`
	PropagationEnabled *bool    `json:"propagation_enabled,omitempty"`
	AllowedShockTypes  []string `json:"allowed_shock_types,omitempty"`
	CrossCommodity     bool     `json:"cross_commodity,omitempty"`
	Description        string   `json:"description,omitempty"`
}

type dependenciesFile struct {
	Dependencies []dependencyDTO `json:"dependencies"`
}

type scenariosFile struct {
	Scenarios []Scenario `json:"scenarios"`
}

// --- Entry points ----------------------------------------------------------

// Default loads the dataset embedded in the binary.
func Default() (*Dataset, error) {
	return LoadFS(sampledata.FS, sampledata.Dir)
}

// Load reads a dataset from a directory on disk.
func Load(dir string) (*Dataset, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("data directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("data path %q is not a directory", dir)
	}
	return LoadFS(os.DirFS(dir), ".")
}

// LoadFS reads a dataset from any filesystem (used for both embedded and
// on-disk data). dir is the path within fsys that holds the JSON files.
func LoadFS(fsys fs.FS, dir string) (*Dataset, error) {
	var ents entitiesFile
	if err := readJSON(fsys, path.Join(dir, entitiesFileName), &ents); err != nil {
		return nil, err
	}
	var deps dependenciesFile
	if err := readJSON(fsys, path.Join(dir, dependenciesFileName), &deps); err != nil {
		return nil, err
	}
	var scen scenariosFile
	if err := readJSON(fsys, path.Join(dir, scenariosFileName), &scen); err != nil {
		return nil, err
	}

	g, err := buildGraph(ents, deps)
	if err != nil {
		return nil, err
	}
	if err := validateScenarios(scen.Scenarios, g); err != nil {
		return nil, err
	}

	return &Dataset{Graph: g, Scenarios: scen.Scenarios}, nil
}

// --- Internal helpers -------------------------------------------------------

func readJSON(fsys fs.FS, name string, v any) error {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path.Base(name), err)
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("parsing %s: %w", path.Base(name), err)
	}
	return nil
}

// typedEntity pairs an entity name with the node type its category implies.
type typedEntity struct {
	name string
	typ  models.NodeType
}

// buildGraph validates the entities and dependencies and assembles the graph.
func buildGraph(ents entitiesFile, deps dependenciesFile) (*graph.Graph, error) {
	categories := []struct {
		list []entityDTO
		typ  models.NodeType
	}{
		{ents.Countries, models.Country},
		{ents.Commodities, models.Commodity},
		{ents.Sectors, models.Sector},
		{ents.Routes, models.Route},
		{ents.Companies, models.Company},
	}

	// Resolve every entity name to its type, rejecting blanks and duplicates.
	byName := make(map[string]typedEntity)
	g := graph.New()
	for _, c := range categories {
		for _, e := range c.list {
			name := strings.TrimSpace(e.Name)
			if name == "" {
				return nil, fmt.Errorf("entities: a %s entry is missing the required \"name\" field", c.typ)
			}
			key := strings.ToLower(name)
			if existing, ok := byName[key]; ok {
				return nil, fmt.Errorf("entities: %q is declared as both %s and %s", name, existing.typ, c.typ)
			}
			byName[key] = typedEntity{name: name, typ: c.typ}
			g.AddNode(models.NewNode(c.typ, name))
		}
	}
	if g.NodeCount() == 0 {
		return nil, fmt.Errorf("entities: no entities defined")
	}

	resolve := func(name string) (typedEntity, bool) {
		te, ok := byName[strings.ToLower(strings.TrimSpace(name))]
		return te, ok
	}

	for i, d := range deps.Dependencies {
		where := fmt.Sprintf("dependency %d (%q -> %q)", i+1, d.Source, d.Target)
		if strings.TrimSpace(d.Source) == "" || strings.TrimSpace(d.Target) == "" {
			return nil, fmt.Errorf("%s: source and target are required", where)
		}
		if strings.TrimSpace(d.Relationship) == "" {
			return nil, fmt.Errorf("%s: relationship_type is required", where)
		}
		if !models.IsValidRelationship(models.EdgeType(d.Relationship)) {
			return nil, fmt.Errorf("%s: invalid relationship_type %q (valid: %v)", where, d.Relationship, models.RelationshipTypes())
		}
		if d.Weight <= 0 || d.Weight > 1 {
			return nil, fmt.Errorf("%s: weight must be within (0,1], got %v", where, d.Weight)
		}
		for _, st := range d.AllowedShockTypes {
			if !models.IsValidShockType(st) {
				return nil, fmt.Errorf("%s: invalid allowed_shock_types entry %q", where, st)
			}
		}
		from, ok := resolve(d.Source)
		if !ok {
			return nil, fmt.Errorf("%s: unknown source entity %q", where, d.Source)
		}
		to, ok := resolve(d.Target)
		if !ok {
			return nil, fmt.Errorf("%s: unknown target entity %q", where, d.Target)
		}

		// Concentration defaults to the dependency weight when omitted.
		concentration := d.Weight
		if d.Concentration != nil {
			concentration = *d.Concentration
		}
		if concentration < 0 || concentration > 1 {
			return nil, fmt.Errorf("%s: concentration must be within [0,1], got %v", where, concentration)
		}

		// Propagation is enabled by default unless explicitly disabled.
		propagationEnabled := true
		if d.PropagationEnabled != nil {
			propagationEnabled = *d.PropagationEnabled
		}

		g.AddEdge(models.Edge{
			From:               models.NewNodeID(from.typ, from.name),
			To:                 models.NewNodeID(to.typ, to.name),
			Type:               models.EdgeType(d.Relationship),
			Weight:             d.Weight,
			Concentration:      concentration,
			Commodity:          strings.TrimSpace(d.Commodity),
			Sector:             strings.TrimSpace(d.Sector),
			PropagationEnabled: propagationEnabled,
			AllowedShockTypes:  d.AllowedShockTypes,
			CrossCommodity:     d.CrossCommodity,
		})
	}
	if g.EdgeCount() == 0 {
		return nil, fmt.Errorf("dependencies: no dependencies defined")
	}

	return g, nil
}

// validateScenarios checks that every preset references entities that exist
// and carries sane numeric ranges. This catches drift between the scenario and
// entity/dependency files early, with a helpful message.
func validateScenarios(scenarios []Scenario, g *graph.Graph) error {
	seen := make(map[string]struct{})
	for _, s := range scenarios {
		if strings.TrimSpace(s.ID) == "" {
			return fmt.Errorf("scenarios: a scenario is missing the required \"id\" field")
		}
		if _, dup := seen[s.ID]; dup {
			return fmt.Errorf("scenarios: duplicate scenario id %q", s.ID)
		}
		seen[s.ID] = struct{}{}

		if _, ok := g.FindByName(s.Source); !ok {
			return fmt.Errorf("scenario %q: unknown source entity %q", s.ID, s.Source)
		}
		if _, ok := g.NodeByName(models.Commodity, s.Commodity); !ok {
			return fmt.Errorf("scenario %q: unknown commodity %q", s.ID, s.Commodity)
		}
		if !models.IsValidShockType(s.ShockType) {
			return fmt.Errorf("scenario %q: unknown shock_type %q", s.ID, s.ShockType)
		}
		if s.ShockPercent < 0 || s.ShockPercent > 100 {
			return fmt.Errorf("scenario %q: shock_percent must be within 0..100, got %v", s.ID, s.ShockPercent)
		}
		if s.Depth < 1 {
			return fmt.Errorf("scenario %q: depth must be >= 1, got %d", s.ID, s.Depth)
		}
	}
	return nil
}

// SortScenarios returns the scenarios sorted by id (used for stable listings).
func SortScenarios(s []Scenario) []Scenario {
	out := append([]Scenario(nil), s...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
