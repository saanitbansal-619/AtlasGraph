package data

import (
	"path/filepath"
	"testing"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/simulation"
)

func strategicGlobalDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "data", "strategic_global")
}

func loadStrategicGlobal(t *testing.T) *Dataset {
	t.Helper()
	ds, err := Load(strategicGlobalDir(t))
	if err != nil {
		t.Fatalf("Load(strategic_global): %v", err)
	}
	return ds
}

func TestStrategicGlobalEntitiesLoad(t *testing.T) {
	ds := loadStrategicGlobal(t)
	if got := ds.Graph.CountByType(models.Country); got < 20 {
		t.Errorf("countries = %d, want >= 20", got)
	}
	if got := ds.Graph.CountByType(models.Commodity); got < 15 {
		t.Errorf("commodities = %d, want >= 15", got)
	}
	if got := ds.Graph.CountByType(models.Sector); got < 15 {
		t.Errorf("sectors = %d, want >= 15", got)
	}
	if got := ds.Graph.CountByType(models.Route); got < 5 {
		t.Errorf("routes = %d, want >= 5", got)
	}
}

func TestStrategicGlobalDependenciesLoad(t *testing.T) {
	ds := loadStrategicGlobal(t)
	if ds.Graph.EdgeCount() < 150 {
		t.Errorf("dependencies = %d, want >= 150", ds.Graph.EdgeCount())
	}
}

func TestStrategicGlobalScenariosLoad(t *testing.T) {
	ds := loadStrategicGlobal(t)
	if len(ds.Scenarios) < 8 {
		t.Fatalf("scenarios = %d, want >= 8", len(ds.Scenarios))
	}
	want := []string{
		"taiwan_semiconductor_shock",
		"china_rare_earth_export_control",
		"saudi_crude_oil_supply_cut",
		"russia_natural_gas_disruption",
		"ukraine_wheat_export_disruption",
		"drc_cobalt_supply_disruption",
		"chile_lithium_export_disruption",
		"hormuz_crude_route_disruption",
		"panama_canal_shipping_disruption",
		"south_china_sea_electronics_disruption",
	}
	for _, id := range want {
		if _, ok := ds.Scenario(id); !ok {
			t.Errorf("missing scenario %q", id)
		}
	}
}

func TestStrategicGlobalNoDuplicateEntityIDs(t *testing.T) {
	ds := loadStrategicGlobal(t)
	seen := make(map[models.NodeID]models.Node)
	for _, n := range ds.Graph.Nodes() {
		if prev, dup := seen[n.ID]; dup {
			t.Errorf("duplicate node id %q: %q and %q", n.ID, prev.Name, n.Name)
		}
		seen[n.ID] = n
	}
}

func TestStrategicGlobalTaiwanSemiconductorShock(t *testing.T) {
	ds := loadStrategicGlobal(t)
	cfg := config.Default()
	res, err := simulation.Run(ds.Graph, cfg, simulation.ShockRequest{
		Source: "Taiwan", Commodity: "semiconductors",
		ShockType: "export_collapse", DropPct: 30, Depth: 3,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.AllAffected) == 0 {
		t.Fatal("expected affected nodes from Taiwan semiconductor shock")
	}
}

func TestStrategicGlobalRouteDisruptionProducesAffectedNodes(t *testing.T) {
	ds := loadStrategicGlobal(t)
	scen, ok := ds.Scenario("hormuz_crude_route_disruption")
	if !ok {
		t.Fatal("missing hormuz_crude_route_disruption scenario")
	}
	cfg := config.Default()
	res, err := simulation.Run(ds.Graph, cfg, simulation.ShockRequest{
		Source: scen.Source, Commodity: scen.Commodity,
		ShockType: scen.ShockType, DropPct: scen.ShockPercent, Depth: scen.Depth,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.AllAffected) == 0 {
		t.Fatal("expected affected nodes from route disruption scenario")
	}
}
