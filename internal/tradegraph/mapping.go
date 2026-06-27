package tradegraph

import "strings"

// sectorDep maps a commodity to a downstream industrial sector with a default
// dependency weight (0..1). These are deliberately coarse, hand-set defaults:
// they describe which industries are structurally exposed to a commodity, not a
// measured elasticity. They give the generated graph realistic downstream
// reach without claiming empirical precision.
type sectorDep struct {
	Sector string
	Weight float64
}

// commoditySectors maps a (lower-cased) commodity name to the sectors that
// depend on it. Commodities not listed here simply produce no sector edges.
var commoditySectors = map[string][]sectorDep{
	"semiconductors": {
		{Sector: "AI hardware", Weight: 0.85},
		{Sector: "cloud infrastructure", Weight: 0.80},
		{Sector: "automotive electronics", Weight: 0.70},
		{Sector: "consumer devices", Weight: 0.65},
	},
	"lithium batteries": {
		{Sector: "EV batteries", Weight: 0.85},
		{Sector: "automotive electronics", Weight: 0.60},
	},
	"cobalt ores": {
		{Sector: "EV batteries", Weight: 0.80},
	},
	"crude oil": {
		{Sector: "shipping logistics", Weight: 0.72},
		{Sector: "energy-intensive manufacturing", Weight: 0.70},
	},
	"rare earths": {
		{Sector: "EV batteries", Weight: 0.55},
		{Sector: "electronics manufacturing", Weight: 0.60},
	},
}

// sectorsFor returns the sector dependencies for a commodity name (matched
// case-insensitively), or nil when the commodity has no mapping.
func sectorsFor(commodity string) []sectorDep {
	return commoditySectors[strings.ToLower(strings.TrimSpace(commodity))]
}
