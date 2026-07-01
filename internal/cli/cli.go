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
	case "ingest":
		return runIngest(args[1:], out, errOut)
	case "indicators":
		return runIndicators(args[1:], out, errOut)
	case "trade":
		return runTrade(args[1:], out, errOut)
	case "score":
		return runScore(args[1:], out, errOut)
	case "events":
		return runEvents(args[1:], out, errOut)
	case "serve":
		return runServe(args[1:], out, errOut)
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
  graph build-trade     Build a graph dataset from ingested trade flows
  risk leaderboard      Rank entities by baseline fragility
  ingest worldbank      Fetch real macro indicators from the World Bank API
  ingest trade          Ingest country-to-country trade flows from a local CSV
  ingest gdelt          Fetch real-world event/news risk from the GDELT API
  ingest commodity-prices  Ingest commodity price time series from a local CSV
  indicators country    Show ingested macro indicators for a country
  trade summary         Summarise an ingested trade-flow panel
  trade dependency      Show supplier dependency for an importer + commodity
  trade concentration   Show supplier concentration (HHI) for an importer + commodity
  score macro           Score macro exposure per country from macro indicators
  score commodities     Score commodity price stress from ingested price series
  score fragility       Unified country and commodity fragility from GFIP signals
  events risk           Score country event risk from ingested GDELT data
  serve                 Start the HTTP API server (JSON endpoints)
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
  atlas ingest worldbank --countries USA,CHN,JPN,DEU,KOR --start 2018 --end 2023 --out data/raw/worldbank
  atlas indicators country USA --data data/raw/worldbank
  atlas score macro --data data/raw/worldbank --year 2023 --verbose
  atlas ingest trade --file data/examples/trade_flows_sample.csv --out data/processed/trade
  atlas trade summary --data data/processed/trade
  atlas trade dependency --importer USA --commodity semiconductors --data data/processed/trade
  atlas trade concentration --importer USA --commodity semiconductors --data data/processed/trade
  atlas graph build-trade --trade-data data/processed/trade --out data/generated/trade_graph
  atlas graph summary --data data/generated/trade_graph
  atlas ingest gdelt --countries TWN,CHN,JPN,KOR,USA,DEU --days 7 --limit 25 --delay-seconds 6 --out data/raw/gdelt
  atlas ingest gdelt --fixture data/examples/gdelt_events_sample.json --out data/raw/gdelt
  atlas events risk --data data/raw/gdelt
  atlas ingest commodity-prices --file data/examples/commodity_prices_sample.csv --out data/processed/commodity_prices
  atlas score commodities --data data/processed/commodity_prices
  atlas score fragility --graph-data data/generated/trade_graph --trade-data data/processed/trade --macro-data data/raw/worldbank --event-data data/raw/gdelt --commodity-data data/processed/commodity_prices
  atlas serve --data data/generated/trade_graph --trade-data data/processed/trade --macro-data data/raw/worldbank --event-data data/raw/gdelt --commodity-data data/processed/commodity_prices --port 8080
`, "\n"))
}
