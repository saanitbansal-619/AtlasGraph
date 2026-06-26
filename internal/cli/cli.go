// Package cli wires the engine to the command line. main.go stays a thin shell
// around Run so the command surface is testable and the rendering lives next
// to the logic it formats.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
)

// Run dispatches a subcommand. It returns a process exit code and writes all
// human-facing output to out / errors to errOut.
func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		printUsage(out)
		return 0
	}

	switch args[0] {
	case "shock":
		return runShock(args[1:], out, errOut)
	case "scenario":
		return runScenario(args[1:], out, errOut)
	case "graph":
		return runGraph(args[1:], out, errOut)
	case "risk":
		return runRisk(args[1:], out, errOut)
	case "version", "--version", "-v":
		fmt.Fprintf(out, "atlas %s (commit %s, built %s)\n", config.Version, config.Commit, config.BuildDate)
		return 0
	case "help", "--help", "-h":
		printUsage(out)
		return 0
	default:
		fmt.Fprintf(errOut, "unknown command %q\n\n", args[0])
		printUsage(errOut)
		return 2
	}
}

// loadDataset resolves the dataset for a command. An empty dir means "use the
// dataset embedded in the binary"; otherwise it is loaded from disk.
func loadDataset(dir string) (*data.Dataset, error) {
	if strings.TrimSpace(dir) == "" {
		return data.Default()
	}
	return data.Load(dir)
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, strings.TrimLeft(`
AtlasGraph - Economic Shock Propagation Engine

Usage:
  atlas <command> [flags]

Commands:
  shock                 Simulate an economic shock and trace its propagation
  scenario list         List saved scenario presets
  scenario run <id>     Run a saved scenario preset
  graph summary         Show high-level graph statistics
  graph paths           Show dependency paths between two entities
  graph dump            Print every dependency edge
  risk leaderboard      Rank entities by baseline fragility
  version               Print version information
  help                  Show this help

Common flags:
  --data <dir>          Load dataset from a directory (default: embedded sample)
  --type <kind>         Shock type: export_collapse, supply_cut, price_spike, route_disruption
  --output text|json    Output format for shock results (default: text)
  --save <file>         Write the JSON result to a file
  --explain             Print the propagation logic and blocked branches

Examples:
  atlas shock --source Taiwan --commodity semiconductors --drop 30 --depth 3
  atlas shock --source Taiwan --commodity semiconductors --type export_collapse --explain
  atlas shock --source Taiwan --commodity semiconductors --drop 30 --output json
  atlas shock --source Taiwan --commodity semiconductors --save results/taiwan_shock.json
  atlas scenario list --data data/sample
  atlas scenario run taiwan_semiconductor_shock --data data/sample --explain
  atlas graph summary --data data/sample
  atlas graph paths --from Taiwan --to "cloud infrastructure" --data data/sample
  atlas risk leaderboard --data data/sample
`, "\n"))
}
