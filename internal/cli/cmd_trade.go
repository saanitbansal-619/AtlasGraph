package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/atlasgraph/atlas/internal/ingest/trade"
)

func runTrade(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas trade <summary|dependency|concentration> [flags]")
		return 2
	}
	switch args[0] {
	case "summary":
		return tradeSummary(args[1:], out, errOut)
	case "dependency":
		return tradeDependency(args[1:], out, errOut)
	case "concentration":
		return tradeConcentration(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown trade subcommand %q (want summary, dependency or concentration)\n", args[0])
		return 2
	}
}

func tradeSummary(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("trade summary", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dataDir := fs.String("data", "data/processed/trade", "directory holding ingested trade data")
	output := fs.String("output", "text", "output format: text or json")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas trade summary [--data dir] [--output text|json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !validOutput(*output) {
		fmt.Fprintf(errOut, "error: invalid --output %q (want text or json)\n", *output)
		return 2
	}

	resolved, ok := loadTradeFile(*dataDir, errOut)
	if !ok {
		return 1
	}

	summary := trade.BuildSummary(resolved.File, 5)
	if *output == "json" {
		if err := writeJSON(out, buildTradeSummaryJSON(resolved, summary)); err != nil {
			fmt.Fprintf(errOut, "error: encoding json: %v\n", err)
			return 1
		}
		return 0
	}
	renderTradeSummary(out, summary)
	return 0
}

func tradeDependency(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("trade dependency", flag.ContinueOnError)
	fs.SetOutput(errOut)
	importer := fs.String("importer", "", "importer country code or name (e.g. USA)")
	commodity := fs.String("commodity", "", "commodity name or code (e.g. semiconductors)")
	dataDir := fs.String("data", "data/processed/trade", "directory holding ingested trade data")
	output := fs.String("output", "text", "output format: text or json")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas trade dependency --importer USA --commodity semiconductors [--data dir] [--output text|json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !validOutput(*output) {
		fmt.Fprintf(errOut, "error: invalid --output %q (want text or json)\n", *output)
		return 2
	}
	if strings.TrimSpace(*importer) == "" || strings.TrimSpace(*commodity) == "" {
		fmt.Fprintln(errOut, "error: --importer and --commodity are required")
		fs.Usage()
		return 2
	}

	resolved, ok := loadTradeFile(*dataDir, errOut)
	if !ok {
		return 1
	}

	dep := trade.BuildDependencyResolved(resolved, *importer, *commodity)
	if !dep.HasData {
		fmt.Fprintf(errOut, "error: no trade flows for importer %q and commodity %q in %s\n", *importer, *commodity, *dataDir)
		return 1
	}
	if *output == "json" {
		if err := writeJSON(out, buildTradeDependencyJSON(resolved, dep)); err != nil {
			fmt.Fprintf(errOut, "error: encoding json: %v\n", err)
			return 1
		}
		return 0
	}
	renderTradeDependency(out, dep)
	return 0
}

func tradeConcentration(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("trade concentration", flag.ContinueOnError)
	fs.SetOutput(errOut)
	importer := fs.String("importer", "", "importer country code or name (e.g. USA)")
	commodity := fs.String("commodity", "", "commodity name or code (e.g. semiconductors)")
	dataDir := fs.String("data", "data/processed/trade", "directory holding ingested trade data")
	output := fs.String("output", "text", "output format: text or json")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas trade concentration --importer USA --commodity semiconductors [--data dir] [--output text|json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !validOutput(*output) {
		fmt.Fprintf(errOut, "error: invalid --output %q (want text or json)\n", *output)
		return 2
	}
	if strings.TrimSpace(*importer) == "" || strings.TrimSpace(*commodity) == "" {
		fmt.Fprintln(errOut, "error: --importer and --commodity are required")
		fs.Usage()
		return 2
	}

	resolved, ok := loadTradeFile(*dataDir, errOut)
	if !ok {
		return 1
	}

	con := trade.BuildConcentrationResolved(resolved, *importer, *commodity)
	if !con.HasData {
		fmt.Fprintf(errOut, "error: no trade flows for importer %q and commodity %q in %s\n", *importer, *commodity, *dataDir)
		return 1
	}
	if *output == "json" {
		if err := writeJSON(out, buildTradeConcentrationJSON(resolved, con)); err != nil {
			fmt.Fprintf(errOut, "error: encoding json: %v\n", err)
			return 1
		}
		return 0
	}
	renderTradeConcentration(out, con)
	return 0
}

// loadTradeFile loads processed trade data, preferring trade_dependencies.json.
func loadTradeFile(dir string, errOut io.Writer) (trade.ResolvedTrade, bool) {
	resolved, err := trade.ResolveTrade(dir)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		fmt.Fprintln(errOut, "hint: run `atlas ingest trade --dir <dir> --source un-comtrade` or `atlas ingest trade --file <csv>` first")
		return trade.ResolvedTrade{}, false
	}
	return resolved, true
}
