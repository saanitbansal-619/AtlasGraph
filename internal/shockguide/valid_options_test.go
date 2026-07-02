package shockguide

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
)

func loadStrategic(t *testing.T) *graph.Graph {
	t.Helper()
	dir := filepath.Join("..", "..", "data", "strategic_global")
	ds, err := data.Load(dir)
	if err != nil {
		t.Fatalf("load strategic_global: %v", err)
	}
	return ds.Graph
}

func findSource(opts ValidOptions, name string) (SourceOption, bool) {
	for _, s := range opts.Sources {
		if s.Source == name {
			return s, true
		}
	}
	return SourceOption{}, false
}

func findCommodity(src SourceOption, commodity string) (CommodityOption, bool) {
	for _, c := range src.Commodities {
		if c.Commodity == commodity {
			return c, true
		}
	}
	return CommodityOption{}, false
}

func TestRussiaDoesNotListBatteries(t *testing.T) {
	g := loadStrategic(t)
	opts := BuildValidOptions(g, "Russia")
	russia, ok := findSource(opts, "Russia")
	if !ok {
		t.Fatal("Russia should be a valid source")
	}
	if _, ok := findCommodity(russia, "batteries"); ok {
		t.Error("Russia should not list batteries without a direct edge")
	}
}

func TestRussiaListsNaturalGas(t *testing.T) {
	g := loadStrategic(t)
	opts := BuildValidOptions(g, "Russia")
	russia, ok := findSource(opts, "Russia")
	if !ok {
		t.Fatal("Russia should be a valid source")
	}
	ng, ok := findCommodity(russia, "natural gas")
	if !ok {
		t.Fatal("Russia should list natural gas")
	}
	if !containsStr(ng.ShockTypes, string(models.ShockSupplyCut)) {
		t.Errorf("natural gas shock types = %v, want supply_cut", ng.ShockTypes)
	}
}

func TestHormuzListsCrudeWithRouteDisruption(t *testing.T) {
	g := loadStrategic(t)
	opts := BuildValidOptions(g, "Strait of Hormuz")
	hormuz, ok := findSource(opts, "Strait of Hormuz")
	if !ok {
		t.Fatal("Strait of Hormuz should be a valid source")
	}
	oil, ok := findCommodity(hormuz, "crude oil")
	if !ok {
		t.Fatal("Hormuz should list crude oil")
	}
	if !containsStr(oil.ShockTypes, string(models.ShockRouteDisruption)) {
		t.Errorf("crude oil shock types = %v, want route_disruption", oil.ShockTypes)
	}
}

func TestTaiwanListsSemiconductorsExportCollapse(t *testing.T) {
	g := loadStrategic(t)
	opts := BuildValidOptions(g, "Taiwan")
	taiwan, ok := findSource(opts, "Taiwan")
	if !ok {
		t.Fatal("Taiwan should be a valid source")
	}
	chips, ok := findCommodity(taiwan, "semiconductors")
	if !ok {
		t.Fatal("Taiwan should list semiconductors")
	}
	if !containsStr(chips.ShockTypes, string(models.ShockExportCollapse)) {
		t.Errorf("semiconductors shock types = %v, want export_collapse", chips.ShockTypes)
	}
}

func TestNoDuplicateCommoditiesPerSource(t *testing.T) {
	g := loadStrategic(t)
	opts := BuildValidOptions(g, "")
	for _, src := range opts.Sources {
		seen := map[string]bool{}
		for _, c := range src.Commodities {
			if seen[c.Commodity] {
				t.Errorf("duplicate commodity %q for source %q", c.Commodity, src.Source)
			}
			seen[c.Commodity] = true
		}
	}
}

func TestShockTypesDedupedAndSorted(t *testing.T) {
	g := loadStrategic(t)
	opts := BuildValidOptions(g, "")
	for _, src := range opts.Sources {
		for _, c := range src.Commodities {
			if !slices.IsSorted(c.ShockTypes) {
				t.Errorf("%s/%s shock types not sorted: %v", src.Source, c.Commodity, c.ShockTypes)
			}
			seen := map[string]bool{}
			for _, st := range c.ShockTypes {
				if seen[st] {
					t.Errorf("duplicate shock type %q for %s/%s", st, src.Source, c.Commodity)
				}
				seen[st] = true
			}
		}
	}
}

func containsStr(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
