package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

func runIndicators(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas indicators country <ISO3> [--data dir]")
		return 2
	}
	switch args[0] {
	case "country":
		return indicatorsCountry(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown indicators subcommand %q (want country)\n", args[0])
		return 2
	}
}

func indicatorsCountry(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(errOut, "error: country ISO3 code is required")
		fmt.Fprintln(errOut, "Usage: atlas indicators country <ISO3> [--data dir]")
		return 2
	}
	iso3 := args[0]

	fs := flag.NewFlagSet("indicators country", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dataDir := fs.String("data", "data/raw/worldbank", "directory holding ingested World Bank data")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	file, err := worldbank.Load(*dataDir)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		fmt.Fprintln(errOut, "hint: run `atlas ingest worldbank --countries ...` first")
		return 1
	}

	summary := worldbank.BuildSummary(file, iso3)
	if !summary.HasData {
		fmt.Fprintf(errOut, "error: no data for country %q in %s\n", strings.ToUpper(iso3), *dataDir)
		return 1
	}
	renderCountryIndicators(out, summary)
	return 0
}
