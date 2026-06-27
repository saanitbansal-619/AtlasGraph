package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

func runScore(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas score <macro> [flags]")
		return 2
	}
	switch args[0] {
	case "macro":
		return scoreMacro(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown score subcommand %q (want macro)\n", args[0])
		return 2
	}
}

func scoreMacro(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("score macro", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dataDir := fs.String("data", "data/raw/worldbank", "directory holding ingested World Bank data")
	year := fs.Int("year", 0, "year lens (default: latest available year per country)")
	output := fs.String("output", "text", "output format: text or json")
	save := fs.String("save", "", "write the JSON result to this file")
	verbose := fs.Bool("verbose", false, "print a detailed component breakdown per country")
	explainFormula := fs.Bool("explain-formula", false, "print the score's formula, components, risk bands and limitations, then exit")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas score macro [--data dir] [--year Y] [--output text|json] [--save file] [--verbose] [--explain-formula]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !validOutput(*output) {
		fmt.Fprintf(errOut, "error: invalid --output %q (want text or json)\n", *output)
		return 2
	}

	// --explain-formula documents the methodology without needing ingested data.
	if *explainFormula {
		renderMacroFormula(out, macro.DefaultWeights())
		return 0
	}

	file, err := worldbank.Load(*dataDir)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		fmt.Fprintln(errOut, "hint: run `atlas ingest worldbank --countries ...` first")
		return 1
	}

	scores := macro.ScoreCountries(file, *year, macro.DefaultWeights())
	if len(scores) == 0 {
		fmt.Fprintf(errOut, "error: no country data found in %s\n", *dataDir)
		return 1
	}

	if *save != "" {
		if err := saveMacroJSON(*save, scores, *year); err != nil {
			fmt.Fprintf(errOut, "error: saving results: %v\n", err)
			return 1
		}
		fmt.Fprintf(out, "Saved macro exposure scores to %s\n", *save)
	}

	switch *output {
	case "json":
		if err := writeMacroJSON(out, scores, *year); err != nil {
			fmt.Fprintf(errOut, "error: encoding json: %v\n", err)
			return 1
		}
	default:
		renderMacroScores(out, scores, *year, *verbose)
	}
	return 0
}
