// Package scoring turns the structure of the dependency graph into fragility
// scores. Fragility answers the question: "how badly could this node be hurt
// by a disruption to the things it relies on?"
//
// The headline model is intentionally simple and inspectable:
//
//		fragility = dependency * concentration * exposure   (each in [0,1])
//		scaled to a 0..100 scale and capped.
//
//	  - dependency    – how reliant the node is on inbound flows (structural).
//	  - concentration – how concentrated its critical inputs are (structural).
//	  - exposure      – how much disruption is actually reaching it. In a calm
//	    baseline this is a fraction of its dependency (ambient risk); under a
//	    shock the propagated impact is added on top.
//
// Because all three factors are structural except exposure, a shock moves
// fragility purely through the exposure term, which makes the delta between
// baseline and shocked fragility easy to reason about.
package scoring

import (
	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
)

// Factors are the three normalised [0,1] inputs to the fragility formula.
type Factors struct {
	Dependency    float64
	Concentration float64
	Exposure      float64
}

// Fragility computes the headline fragility score on a 0..max scale.
func Fragility(f Factors, max float64) float64 {
	score := clamp01(f.Dependency) * clamp01(f.Concentration) * clamp01(f.Exposure) * 100
	return clamp(score, 0, max)
}

// DependencyScore measures how reliant a node is on its inbound dependencies.
//
// It combines every incoming edge into a single [0,1) reliance value using a
// "noisy-or" style aggregation: 1 - Π(1 - weight). A node with many strong
// suppliers scores high; a pure source with no inbound edges scores 0.
func DependencyScore(g *graph.Graph, id models.NodeID) float64 {
	in := g.InEdges(id)
	if len(in) == 0 {
		return 0
	}
	survive := 1.0
	for _, e := range in {
		survive *= 1 - clamp01(e.Weight)
	}
	return 1 - survive
}

// ConcentrationScore is the worst (highest) supplier concentration among a
// node's inbound dependencies. A single dominant supplier drives this toward 1.
func ConcentrationScore(g *graph.Graph, id models.NodeID) float64 {
	in := g.InEdges(id)
	max := 0.0
	for _, e := range in {
		if c := clamp01(e.Concentration); c > max {
			max = c
		}
	}
	return max
}

// AmbientExposure is a node's baseline ("peacetime") exposure, derived from its
// structural dependency. It gives baseline fragility a non-zero value so that a
// shock produces a meaningful delta rather than appearing out of nowhere.
func AmbientExposure(g *graph.Graph, id models.NodeID, cfg config.Config) float64 {
	return clamp01(DependencyScore(g, id) * cfg.AmbientExposureFactor)
}

// NodeFragility computes the fragility of a node given an additional shock
// impact in [0,1]. Passing impact = 0 yields the baseline fragility.
func NodeFragility(g *graph.Graph, id models.NodeID, impact float64, cfg config.Config) float64 {
	exposure := clamp01(AmbientExposure(g, id, cfg) + clamp01(impact))
	return Fragility(Factors{
		Dependency:    DependencyScore(g, id),
		Concentration: ConcentrationScore(g, id),
		Exposure:      exposure,
	}, cfg.MaxFragility)
}

func clamp01(v float64) float64 { return clamp(v, 0, 1) }

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
