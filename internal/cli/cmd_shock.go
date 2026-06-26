package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/simulation"
)

func runShock(args []string, out, errOut io.Writer) int {
	cfg := config.Default()

	fs := flag.NewFlagSet("shock", flag.ContinueOnError)
	fs.SetOutput(errOut)
	source := fs.String("source", "", "source entity whose exports/flow are shocked (e.g. Taiwan)")
	commodity := fs.String("commodity", "", "commodity that is disrupted (e.g. semiconductors)")
	drop := fs.Float64("drop", cfg.DefaultDrop, "export/flow drop as a percentage (0..100)")
	depth := fs.Int("depth", cfg.DefaultDepth, "max dependency hops to propagate from the source")
	shockType := fs.String("type", cfg.DefaultShockType, "shock type: export_collapse, supply_cut, price_spike or route_disruption")
	dataDir := fs.String("data", "", "dataset directory (default: embedded sample)")
	output := fs.String("output", "text", "output format: text or json")
	save := fs.String("save", "", "write the JSON result to this file")
	explain := fs.Bool("explain", false, "print the propagation logic and blocked branches")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas shock --source <entity> --commodity <name> [--type kind] [--drop N] [--depth N] [--data dir] [--output text|json] [--save file] [--explain]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *source == "" || *commodity == "" {
		fmt.Fprintln(errOut, "error: --source and --commodity are required")
		fs.Usage()
		return 2
	}
	if !validOutput(*output) {
		fmt.Fprintf(errOut, "error: invalid --output %q (want text or json)\n", *output)
		return 2
	}

	ds, err := loadDataset(*dataDir)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	req := simulation.ShockRequest{
		Source:    *source,
		Commodity: *commodity,
		ShockType: *shockType,
		DropPct:   *drop,
		Depth:     *depth,
	}
	return executeShock(out, errOut, ds, cfg, req, nil, *output, *save, *explain)
}

// executeShock runs a shock request and handles rendering and saving. scen is
// optional preset metadata used to enrich the output when running a scenario.
func executeShock(out, errOut io.Writer, ds *data.Dataset, cfg config.Config, req simulation.ShockRequest, scen *data.Scenario, output, save string, explain bool) int {
	res, err := simulation.Run(ds.Graph, cfg, req)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	if save != "" {
		if err := saveResultJSON(save, res, scen, explain); err != nil {
			fmt.Fprintf(errOut, "error: saving results: %v\n", err)
			return 1
		}
		fmt.Fprintf(out, "Saved JSON results to %s\n", save)
	}

	switch output {
	case "json":
		if err := writeResultJSON(out, res, scen, explain); err != nil {
			fmt.Fprintf(errOut, "error: encoding json: %v\n", err)
			return 1
		}
	default: // text
		if scen != nil {
			renderScenarioBanner(out, *scen)
		}
		renderResult(out, ds.Graph, res, explain)
	}
	return 0
}

func validOutput(o string) bool {
	switch o {
	case "", "text", "json":
		return true
	default:
		return false
	}
}
