package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
	"github.com/atlasgraph/atlas/internal/scoring/events"
)

func runEvents(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas events <risk> [flags]")
		return 2
	}
	switch args[0] {
	case "risk":
		return eventsRisk(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown events subcommand %q (want risk)\n", args[0])
		return 2
	}
}

func eventsRisk(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("events risk", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dataDir := fs.String("data", "data/raw/gdelt", "directory holding ingested GDELT event data")
	output := fs.String("output", "text", "output format: text or json")
	save := fs.String("save", "", "write the JSON result to this file")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas events risk [--data dir] [--output text|json] [--save file]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !validOutput(*output) {
		fmt.Fprintf(errOut, "error: invalid --output %q (want text or json)\n", *output)
		return 2
	}

	file, err := gdelt.Load(*dataDir)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		fmt.Fprintln(errOut, "hint: run `atlas ingest gdelt --countries ...` first")
		return 1
	}

	scores := events.ScoreCountries(file, events.DefaultWeights())
	if len(scores) == 0 {
		fmt.Fprintf(errOut, "error: no event data found in %s\n", *dataDir)
		return 1
	}

	if *save != "" {
		if err := saveEventRiskJSON(*save, scores); err != nil {
			fmt.Fprintf(errOut, "error: saving results: %v\n", err)
			return 1
		}
		fmt.Fprintf(out, "Saved event risk scores to %s\n", *save)
	}

	switch *output {
	case "json":
		if err := writeEventRiskJSON(out, scores); err != nil {
			fmt.Fprintf(errOut, "error: encoding json: %v\n", err)
			return 1
		}
	default:
		renderEventRiskScores(out, scores)
	}
	return 0
}
