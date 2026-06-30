package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/simulation"
)

func TestAPIGraphEntities(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/graph/entities", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	var ents struct {
		Countries   []string `json:"countries"`
		Commodities []string `json:"commodities"`
		Sectors     []string `json:"sectors"`
		Routes      []string `json:"routes"`
		Companies   []string `json:"companies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ents); err != nil {
		t.Fatalf("decode entities: %v\n%s", err, rec.Body.String())
	}
	if !contains(ents.Countries, "Taiwan") {
		t.Errorf("countries should include Taiwan, got %v", ents.Countries)
	}
	if !contains(ents.Commodities, "semiconductors") {
		t.Errorf("commodities should include semiconductors, got %v", ents.Commodities)
	}
	// The embedded sample has route chokepoints.
	if len(ents.Routes) == 0 {
		t.Errorf("expected route entities in the embedded sample, got none")
	}
}

func TestAPIGraphEntitiesRejectsPOST(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodPost, "/api/graph/entities", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestAPIShockOptions(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodGet, "/api/shock/options", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	var opts struct {
		Sources     []string `json:"sources"`
		Commodities []string `json:"commodities"`
		ShockTypes  []struct {
			Type           string   `json:"type"`
			Name           string   `json:"name"`
			Description    string   `json:"description"`
			RecommendedFor []string `json:"recommended_for"`
			Requires       []string `json:"requires"`
		} `json:"shock_types"`
		RecommendedScenarios []struct {
			Label     string  `json:"label"`
			Source    string  `json:"source"`
			Commodity string  `json:"commodity"`
			ShockType string  `json:"shock_type"`
			Drop      float64 `json:"drop"`
			Depth     int     `json:"depth"`
		} `json:"recommended_scenarios"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &opts); err != nil {
		t.Fatalf("decode options: %v\n%s", err, rec.Body.String())
	}
	if !contains(opts.Sources, "Taiwan") {
		t.Errorf("sources should include Taiwan, got %v", opts.Sources)
	}
	if !contains(opts.Commodities, "semiconductors") {
		t.Errorf("commodities should include semiconductors, got %v", opts.Commodities)
	}
	if len(opts.ShockTypes) != 4 {
		t.Fatalf("expected 4 shock types, got %d", len(opts.ShockTypes))
	}
	for _, st := range opts.ShockTypes {
		if st.Type == "" || st.Name == "" || st.Description == "" {
			t.Errorf("shock type missing fields: %+v", st)
		}
		if len(st.Requires) == 0 {
			t.Errorf("shock type %q missing requires", st.Type)
		}
	}
	if len(opts.RecommendedScenarios) == 0 {
		t.Fatal("expected at least one recommended scenario for the embedded graph")
	}
	for _, rs := range opts.RecommendedScenarios {
		if rs.Label == "" || rs.Source == "" || rs.Commodity == "" || rs.ShockType == "" {
			t.Errorf("recommended scenario missing fields: %+v", rs)
		}
		if !scenarioMakesSense(mustEmbeddedGraph(t), rs.Source, rs.Commodity, rs.ShockType) {
			t.Errorf("recommended scenario does not make sense for the graph: %+v", rs)
		}
	}
}

func TestAPIShockOptionsRejectsPOST(t *testing.T) {
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock/options", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// A valid, on-profile export collapse should produce no warnings field.
func TestAPIShockNoWarningsForValidCombo(t *testing.T) {
	body := `{"source":"Taiwan","commodity":"semiconductors","drop":30,"depth":3,"shock_type":"export_collapse"}`
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	if raw, ok := parsed["warnings"]; ok {
		t.Errorf("expected no warnings for a valid export_collapse, got %s", raw)
	}
}

// A shock type whose profile does not travel along the source→commodity link
// (price_spike over an exports edge) should still run, but warn.
func TestAPIShockWarnsForMismatch(t *testing.T) {
	body := `{"source":"Taiwan","commodity":"semiconductors","drop":30,"depth":3,"shock_type":"price_spike"}`
	rec := do(fullTestServer(t), http.MethodPost, "/api/shock", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	parsed := decodeBody(t, rec)
	raw, ok := parsed["warnings"]
	if !ok {
		t.Fatalf("expected warnings for a mismatched price_spike, got none\n%s", rec.Body.String())
	}
	var warns []string
	if err := json.Unmarshal(raw, &warns); err != nil {
		t.Fatalf("warnings not a string array: %v", err)
	}
	if len(warns) == 0 {
		t.Fatal("warnings array is empty")
	}
}

// --- unit tests for shockWarnings against hand-built graphs ----------------

func profile(t *testing.T, shockType models.ShockType) simulation.ShockProfile {
	t.Helper()
	p, ok := simulation.ProfileFor(string(shockType))
	if !ok {
		t.Fatalf("no profile for %q", shockType)
	}
	return p
}

func edge(from, to models.Node, rel models.EdgeType) models.Edge {
	return models.Edge{From: from.ID, To: to.ID, Type: rel, Weight: 0.8, Concentration: 0.7, Commodity: to.Name}
}

func TestShockWarningsRouteDisruptionNoRoutes(t *testing.T) {
	g := graph.New()
	taiwan := models.NewNode(models.Country, "Taiwan")
	chips := models.NewNode(models.Commodity, "semiconductors")
	g.AddNode(taiwan)
	g.AddNode(chips)
	g.AddEdge(edge(taiwan, chips, models.RelExports))

	w := shockWarnings(g, profile(t, models.ShockRouteDisruption), "Taiwan", "semiconductors")
	if len(w) != 1 {
		t.Fatalf("expected exactly one (no-routes) warning, got %v", w)
	}
	if !containsSub(w, "no routes") {
		t.Errorf("warning should mention missing routes, got %v", w)
	}
}

func TestShockWarningsExportCollapseValidCombo(t *testing.T) {
	g := graph.New()
	taiwan := models.NewNode(models.Country, "Taiwan")
	chips := models.NewNode(models.Commodity, "semiconductors")
	g.AddNode(taiwan)
	g.AddNode(chips)
	g.AddEdge(edge(taiwan, chips, models.RelExports))

	w := shockWarnings(g, profile(t, models.ShockExportCollapse), "Taiwan", "semiconductors")
	if len(w) != 0 {
		t.Errorf("expected no warnings for a valid export_collapse, got %v", w)
	}
}

func TestShockWarningsSupplyCutNoSupplyEdge(t *testing.T) {
	g := graph.New()
	china := models.NewNode(models.Country, "China")
	oil := models.NewNode(models.Commodity, "crude oil")
	g.AddNode(china)
	g.AddNode(oil)
	// Only a depends_on edge links the pair: not exports/supplies.
	g.AddEdge(edge(china, oil, models.RelDependsOn))

	w := shockWarnings(g, profile(t, models.ShockSupplyCut), "China", "crude oil")
	if !containsSub(w, "exports/supplies") {
		t.Errorf("expected an exports/supplies warning, got %v", w)
	}
}

func TestShockWarningsSourceCommodityMismatch(t *testing.T) {
	g := graph.New()
	taiwan := models.NewNode(models.Country, "Taiwan")
	chips := models.NewNode(models.Commodity, "semiconductors")
	sector := models.NewNode(models.Sector, "AI hardware")
	g.AddNode(taiwan)
	g.AddNode(chips)
	g.AddNode(sector)
	g.AddEdge(edge(taiwan, chips, models.RelExports))
	// A price_exposure edge exists in the graph, so the graph-level price
	// warning is suppressed and only the link mismatch should fire.
	g.AddEdge(edge(chips, sector, models.RelPriceExposure))

	w := shockWarnings(g, profile(t, models.ShockPriceSpike), "Taiwan", "semiconductors")
	if !containsSub(w, "does not travel along") {
		t.Errorf("expected a link mismatch warning, got %v", w)
	}
}

func mustEmbeddedGraph(t *testing.T) *graph.Graph {
	t.Helper()
	ds, err := loadDataset("")
	if err != nil {
		t.Fatalf("load embedded dataset: %v", err)
	}
	return ds.Graph
}

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func containsSub(s []string, sub string) bool {
	for _, v := range s {
		if strings.Contains(v, sub) {
			return true
		}
	}
	return false
}
