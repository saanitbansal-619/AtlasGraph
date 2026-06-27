package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

func runIngest(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas ingest <worldbank|trade> [flags]")
		return 2
	}
	switch args[0] {
	case "worldbank":
		return ingestWorldBank(args[1:], out, errOut)
	case "trade":
		return ingestTrade(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown ingest source %q (want worldbank or trade)\n", args[0])
		return 2
	}
}

func ingestTrade(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("ingest trade", flag.ContinueOnError)
	fs.SetOutput(errOut)
	file := fs.String("file", "", "path to a trade-flow CSV to ingest")
	outDir := fs.String("out", "data/processed/trade", "directory to write normalized output to")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas ingest trade --file <csv> [--out dir]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*file) == "" {
		fmt.Fprintln(errOut, "error: --file is required (path to a trade-flow CSV)")
		fs.Usage()
		return 2
	}

	res, err := trade.LoadFile(*file)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	if res.ValidRows() == 0 {
		fmt.Fprintf(errOut, "error: no valid trade rows in %s\n", *file)
		return 1
	}

	trade.SortRecords(res.Records)
	tf := trade.TradeFile{
		Source:     trade.SourceName,
		IngestedAt: time.Now().UTC(),
		SourceFile: *file,
		Records:    res.Records,
	}
	path, err := trade.Save(*outDir, tf)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	renderTradeIngestReport(out, *file, path, res, trade.BuildSummary(tf, 0))
	return 0
}

func ingestWorldBank(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("ingest worldbank", flag.ContinueOnError)
	fs.SetOutput(errOut)
	countries := fs.String("countries", "", "comma-separated ISO3 country codes (e.g. USA,CHN,JPN)")
	start := fs.Int("start", worldbank.DefaultStartYear, "first year to fetch")
	end := fs.Int("end", defaultEndYear(), "last year to fetch")
	outDir := fs.String("out", "data/raw/worldbank", "directory to write normalized output to")
	timeout := fs.Duration("timeout", 3*time.Minute, "overall timeout for the fetch")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas ingest worldbank --countries USA,CHN,JPN [--start 2018] [--end 2023] [--out dir]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	codes := splitCodes(*countries)
	if len(codes) == 0 {
		fmt.Fprintln(errOut, "error: --countries is required (comma-separated ISO3 codes)")
		fs.Usage()
		return 2
	}
	if *start > *end {
		fmt.Fprintf(errOut, "error: --start (%d) must not be after --end (%d)\n", *start, *end)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := worldbank.NewClient(30 * time.Second)
	fmt.Fprintf(out, "Fetching World Bank indicators for %s (%d-%d)…\n", strings.Join(codes, ", "), *start, *end)

	records, err := client.Fetch(ctx, codes, worldbank.DefaultIndicators, *start, *end)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	worldbank.SortRecords(records)

	file := worldbank.IndicatorFile{
		Source:    worldbank.SourceName,
		FetchedAt: time.Now().UTC(),
		StartYear: *start,
		EndYear:   *end,
		Countries: codes,
		Records:   records,
	}
	path, err := worldbank.Save(*outDir, file)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	withValues := 0
	for _, r := range records {
		if r.Value != nil {
			withValues++
		}
	}
	fmt.Fprintf(out, "Saved %d records (%d with values) across %d countries and %d indicators to %s\n",
		len(records), withValues, len(codes), len(worldbank.DefaultIndicators), path)
	return 0
}

// defaultEndYear is the most recent year likely to have World Bank data:
// the previous calendar year.
func defaultEndYear() int {
	return time.Now().Year() - 1
}

// splitCodes parses a comma-separated list into trimmed, upper-cased codes.
func splitCodes(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		code := strings.ToUpper(strings.TrimSpace(part))
		if code != "" {
			out = append(out, code)
		}
	}
	return out
}
