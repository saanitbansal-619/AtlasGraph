package simulation

import (
	"fmt"
	"strings"

	"github.com/atlasgraph/atlas/internal/models"
)

// Decision is the outcome of evaluating whether a shock may travel an edge.
type Decision struct {
	Allowed        bool
	Reason         string // human-readable explanation when blocked
	CrossCommodity bool   // blocked because the edge crosses commodity boundaries
}

// Block reason categories, surfaced in explain output.
const (
	reasonDisabled       = "propagation disabled on edge"
	reasonRelationship   = "relationship not propagated by this shock type"
	reasonShockType      = "edge restricted to other shock types"
	reasonCrossCommodity = "cross-commodity branch blocked"
)

// Evaluate decides whether a shock described by `profile` can propagate along
// edge `e`, given the commodity currently driving the shock. The checks, in
// order, are:
//
//  1. the edge must have propagation enabled;
//  2. the shock profile must recognise the edge's relationship type;
//  3. if the edge restricts shock types, the shock type must be listed;
//  4. unless cross-commodity propagation is permitted (by the profile or the
//     edge), the edge's commodity must match the shock's active commodity.
//
// Commodity-agnostic edges (empty Commodity) always pass the commodity check.
func Evaluate(profile ShockProfile, e models.Edge, activeCommodity string) Decision {
	if !e.PropagationEnabled {
		return Decision{Allowed: false, Reason: reasonDisabled}
	}
	if !profile.Allows(e.Type) {
		return Decision{
			Allowed: false,
			Reason:  fmt.Sprintf("%s: %q not in %s profile", reasonRelationship, e.Type, profile.Type),
		}
	}
	if len(e.AllowedShockTypes) > 0 && !containsFold(e.AllowedShockTypes, profile.Type) {
		return Decision{
			Allowed: false,
			Reason:  fmt.Sprintf("%s: edge allows %s", reasonShockType, strings.Join(e.AllowedShockTypes, ", ")),
		}
	}
	if e.Commodity != "" && !profile.CrossCommodity && !e.CrossCommodity &&
		!strings.EqualFold(e.Commodity, activeCommodity) {
		return Decision{
			Allowed:        false,
			CrossCommodity: true,
			Reason:         fmt.Sprintf("%s: edge commodity %q != shock commodity %q", reasonCrossCommodity, e.Commodity, activeCommodity),
		}
	}
	return Decision{Allowed: true}
}

func containsFold(list []string, want string) bool {
	for _, v := range list {
		if strings.EqualFold(v, want) {
			return true
		}
	}
	return false
}
