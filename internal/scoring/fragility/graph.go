package fragility

import (
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
)

// commodityCentrality scores each commodity node by its graph degree relative to
// the highest-degree commodity in the loaded graph.
func commodityCentrality(g *graph.Graph) map[string]float64 {
	out := map[string]float64{}
	if g == nil {
		return out
	}
	maxDeg := 0
	degrees := map[string]int{}
	for _, n := range g.Nodes() {
		if n.Type != models.Commodity {
			continue
		}
		d := g.Degree(n.ID)
		degrees[n.Name] = d
		if d > maxDeg {
			maxDeg = d
		}
	}
	if maxDeg == 0 {
		return out
	}
	for name, d := range degrees {
		out[normalizeCommodityKey(name)] = float64(d) / float64(maxDeg) * 100
	}
	return out
}
