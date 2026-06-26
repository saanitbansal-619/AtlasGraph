// Package config centralises the tunable parameters of the engine and the
// build metadata embedded at link time. Keeping the model's knobs in one place
// makes the scoring and simulation behaviour explicit and easy to calibrate as
// real data lands.
package config

// Build metadata, overridden via -ldflags at build time (see Makefile).
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// Config holds engine-wide tunables. Use Default() for sensible values.
type Config struct {
	// AmbientExposureFactor scales a node's structural dependency into a
	// baseline ("peacetime") exposure. A shock then adds on top of this, so
	// the factor controls how much headroom a shock has to move fragility.
	AmbientExposureFactor float64

	// PropagationEpsilon is the smallest impact contribution worth
	// propagating. Below this, a downstream effect is treated as noise and
	// pruned, which keeps results focused and traversal bounded.
	PropagationEpsilon float64

	// MaxFragility is the upper bound applied to every fragility score so the
	// scale stays a clean 0..100.
	MaxFragility float64

	// TopN is how many entries each "top exposed" ranking returns.
	TopN int

	// DefaultDepth / DefaultDrop back-fill CLI flags when omitted.
	DefaultDepth int
	DefaultDrop  float64

	// MaxPathDepth bounds path-finding queries (graph paths) so traversal of
	// densely connected graphs stays fast and the output stays readable.
	MaxPathDepth int

	// DefaultShockType backs the shock command's --type flag when omitted.
	DefaultShockType string
}

// Default returns the calibrated default configuration used by the CLI.
func Default() Config {
	return Config{
		AmbientExposureFactor: 0.5,
		PropagationEpsilon:    0.01,
		MaxFragility:          100,
		TopN:                  5,
		DefaultDepth:          3,
		DefaultDrop:           30,
		MaxPathDepth:          6,
		DefaultShockType:      "export_collapse",
	}
}
