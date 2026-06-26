package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/simulation"
)

func runScenario(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas scenario <list|run> [args]")
		return 2
	}
	switch args[0] {
	case "list":
		return scenarioList(args[1:], out, errOut)
	case "run":
		return scenarioRun(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown scenario subcommand %q (want list or run)\n", args[0])
		return 2
	}
}

func scenarioList(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("scenario list", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dataDir := fs.String("data", "", "dataset directory (default: embedded sample)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ds, err := loadDataset(*dataDir)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	renderScenarioList(out, data.SortScenarios(ds.Scenarios))
	return 0
}

func scenarioRun(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintln(errOut, "error: scenario id is required")
		fmt.Fprintln(errOut, "Usage: atlas scenario run <id> [--data dir] [--output text|json] [--save file]")
		return 2
	}
	id := args[0]

	fs := flag.NewFlagSet("scenario run", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dataDir := fs.String("data", "", "dataset directory (default: embedded sample)")
	output := fs.String("output", "text", "output format: text or json")
	save := fs.String("save", "", "write the JSON result to this file")
	explain := fs.Bool("explain", false, "print the propagation logic and blocked branches")
	if err := fs.Parse(args[1:]); err != nil {
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
	scen, ok := ds.Scenario(id)
	if !ok {
		fmt.Fprintf(errOut, "error: unknown scenario %q (try `atlas scenario list`)\n", id)
		return 1
	}

	cfg := config.Default()
	req := simulation.ShockRequest{
		Source:    scen.Source,
		Commodity: scen.Commodity,
		ShockType: scen.ShockType,
		DropPct:   scen.ShockPercent,
		Depth:     scen.Depth,
	}
	return executeShock(out, errOut, ds, cfg, req, &scen, *output, *save, *explain)
}
