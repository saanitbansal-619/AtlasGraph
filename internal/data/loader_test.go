package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atlasgraph/atlas/internal/models"
)

func TestDefaultLoads(t *testing.T) {
	ds, err := Default()
	if err != nil {
		t.Fatalf("Default() error: %v", err)
	}
	if ds.Graph.NodeCount() == 0 || ds.Graph.EdgeCount() == 0 {
		t.Fatalf("expected a populated graph, got %d nodes / %d edges",
			ds.Graph.NodeCount(), ds.Graph.EdgeCount())
	}
	if got := ds.Graph.CountByType(models.Country); got < 1 {
		t.Errorf("expected countries in graph, got %d", got)
	}
	if got := ds.Graph.CountByType(models.Commodity); got < 1 {
		t.Errorf("expected commodities in graph, got %d", got)
	}
}

func TestScenarioPresetsLoad(t *testing.T) {
	ds, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"taiwan_semiconductor_shock",
		"suez_route_disruption",
		"lithium_price_spike",
		"crude_oil_supply_shock",
	}
	for _, id := range want {
		if _, ok := ds.Scenario(id); !ok {
			t.Errorf("missing scenario preset %q", id)
		}
	}
	if _, ok := ds.Scenario("does_not_exist"); ok {
		t.Errorf("unexpected scenario found")
	}
}

func TestLoadFromDisk(t *testing.T) {
	// The canonical dataset lives at the repository root under data/sample.
	dir := filepath.Join("..", "..", "data", "sample")
	ds, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(%q) error: %v", dir, err)
	}
	if ds.Graph.NodeCount() == 0 {
		t.Fatalf("expected nodes loaded from disk")
	}
	// Disk and embedded data should describe the same graph.
	emb, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if ds.Graph.NodeCount() != emb.Graph.NodeCount() || ds.Graph.EdgeCount() != emb.Graph.EdgeCount() {
		t.Errorf("disk vs embedded mismatch: disk(%d/%d) embedded(%d/%d)",
			ds.Graph.NodeCount(), ds.Graph.EdgeCount(),
			emb.Graph.NodeCount(), emb.Graph.EdgeCount())
	}
}

func TestLoadMissingDirectory(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatalf("expected error for missing directory")
	}
}

// writeDataset writes a minimal valid dataset to dir, then applies overrides to
// individual files so tests can introduce specific corruption.
func writeDataset(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	return dir
}

const (
	validEntities = `{
		"countries": [{"name": "Alpha"}],
		"commodities": [{"name": "widget"}],
		"sectors": [{"name": "manufacturing"}]
	}`
	validDeps = `{
		"dependencies": [
			{"source": "Alpha", "target": "widget", "relationship_type": "exports", "weight": 0.9, "concentration": 0.8, "commodity": "widget"},
			{"source": "widget", "target": "manufacturing", "relationship_type": "used_by", "weight": 0.7, "commodity": "widget"}
		]
	}`
	validScenarios = `{
		"scenarios": [
			{"id": "s1", "name": "Test", "source": "Alpha", "commodity": "widget", "shock_type": "supply_cut", "shock_percent": 20, "depth": 2, "description": "x"}
		]
	}`
)

func TestInvalidData(t *testing.T) {
	cases := []struct {
		name      string
		entities  string
		deps      string
		scenarios string
		wantSubs  string
	}{
		{
			name:      "malformed entities json",
			entities:  `{ this is not json `,
			deps:      validDeps,
			scenarios: validScenarios,
			wantSubs:  "parsing entities.json",
		},
		{
			name:      "missing entity name",
			entities:  `{"countries": [{"description": "no name"}]}`,
			deps:      validDeps,
			scenarios: validScenarios,
			wantSubs:  "missing the required",
		},
		{
			name:      "duplicate entity across types",
			entities:  `{"countries": [{"name": "widget"}], "commodities": [{"name": "widget"}]}`,
			deps:      validDeps,
			scenarios: validScenarios,
			wantSubs:  "declared as both",
		},
		{
			name:      "dependency references unknown entity",
			entities:  validEntities,
			deps:      `{"dependencies": [{"source": "Ghost", "target": "widget", "relationship_type": "exports", "weight": 0.5}]}`,
			scenarios: validScenarios,
			wantSubs:  "unknown source entity",
		},
		{
			name:      "weight out of range",
			entities:  validEntities,
			deps:      `{"dependencies": [{"source": "Alpha", "target": "widget", "relationship_type": "exports", "weight": 1.5}]}`,
			scenarios: validScenarios,
			wantSubs:  "weight must be within",
		},
		{
			name:      "invalid relationship type",
			entities:  validEntities,
			deps:      `{"dependencies": [{"source": "Alpha", "target": "widget", "relationship_type": "wishes_for", "weight": 0.5}]}`,
			scenarios: validScenarios,
			wantSubs:  "invalid relationship_type",
		},
		{
			name:      "invalid allowed_shock_types entry",
			entities:  validEntities,
			deps:      `{"dependencies": [{"source": "Alpha", "target": "widget", "relationship_type": "exports", "weight": 0.5, "allowed_shock_types": ["meteor"]}]}`,
			scenarios: validScenarios,
			wantSubs:  "invalid allowed_shock_types",
		},
		{
			name:      "scenario references unknown commodity",
			entities:  validEntities,
			deps:      validDeps,
			scenarios: `{"scenarios": [{"id": "bad", "source": "Alpha", "commodity": "nonexistent", "shock_type": "supply_cut", "shock_percent": 10, "depth": 2}]}`,
			wantSubs:  "unknown commodity",
		},
		{
			name:      "scenario references unknown shock type",
			entities:  validEntities,
			deps:      validDeps,
			scenarios: `{"scenarios": [{"id": "bad", "source": "Alpha", "commodity": "widget", "shock_type": "meteor", "shock_percent": 10, "depth": 2}]}`,
			wantSubs:  "unknown shock_type",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := writeDataset(t, map[string]string{
				entitiesFileName:     c.entities,
				dependenciesFileName: c.deps,
				scenariosFileName:    c.scenarios,
			})
			_, err := Load(dir)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), c.wantSubs) {
				t.Errorf("error %q does not contain %q", err.Error(), c.wantSubs)
			}
		})
	}
}

func TestMissingFileGivesHelpfulError(t *testing.T) {
	dir := writeDataset(t, map[string]string{
		entitiesFileName: validEntities,
		// dependencies.json and scenarios.json intentionally absent
	})
	_, err := Load(dir)
	if err == nil {
		t.Fatalf("expected error for missing dependencies file")
	}
	if !strings.Contains(err.Error(), dependenciesFileName) {
		t.Errorf("error should name the missing file, got %q", err.Error())
	}
}
