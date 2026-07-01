package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/atlasgraph/atlas/internal/scoring/fragility"
)

func scoreFragility(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("score fragility", flag.ContinueOnError)
	fs.SetOutput(errOut)
	graphData := fs.String("graph-data", "data/generated/trade_graph", "directory holding graph entities/dependencies/scenarios")
	tradeData := fs.String("trade-data", "data/processed/trade", "directory holding ingested trade data")
	macroData := fs.String("macro-data", "data/raw/worldbank", "directory holding ingested World Bank macro data")
	eventData := fs.String("event-data", "data/raw/gdelt", "directory holding ingested GDELT event data")
	commodityData := fs.String("commodity-data", "data/processed/commodity_prices", "directory holding ingested commodity price data")
	output := fs.String("output", "text", "output format: text or json")
	save := fs.String("save", "", "write the JSON result to this file")
	explainFormula := fs.Bool("explain-formula", false, "print the score's formula, components, risk bands and limitations, then exit")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas score fragility [--graph-data dir] [--trade-data dir] [--macro-data dir] [--event-data dir] [--commodity-data dir] [--output text|json] [--save file] [--explain-formula]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !validOutput(*output) {
		fmt.Fprintf(errOut, "error: invalid --output %q (want text or json)\n", *output)
		return 2
	}

	if *explainFormula {
		renderFragilityFormula(out)
		return 0
	}

	src := loadFragilitySources(*graphData, *tradeData, *macroData, *eventData, *commodityData)
	res := fragility.Score(src)
	if len(res.Countries) == 0 && len(res.Commodities) == 0 {
		fmt.Fprintln(errOut, "error: no fragility scores could be computed from the provided data paths")
		fmt.Fprintln(errOut, "hint: pass at least one valid --graph-data, --trade-data, --macro-data, --event-data, or --commodity-data directory")
		return 1
	}

	if *save != "" {
		if err := saveFragilityJSON(*save, res); err != nil {
			fmt.Fprintf(errOut, "error: saving results: %v\n", err)
			return 1
		}
		fmt.Fprintf(out, "Saved unified fragility scores to %s\n", *save)
	}

	switch *output {
	case "json":
		if err := writeFragilityJSON(out, res); err != nil {
			fmt.Fprintf(errOut, "error: encoding json: %v\n", err)
			return 1
		}
	default:
		renderFragilityScores(out, res)
	}
	return 0
}
