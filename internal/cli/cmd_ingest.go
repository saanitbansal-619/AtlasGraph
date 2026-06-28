package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

func runIngest(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas ingest <worldbank|trade|trade-comtrade|gdelt> [flags]")
		return 2
	}
	switch args[0] {
	case "worldbank":
		return ingestWorldBank(args[1:], out, errOut)
	case "trade":
		return ingestTrade(args[1:], out, errOut)
	case "trade-comtrade":
		return ingestTradeComtrade(args[1:], out, errOut)
	case "gdelt":
		return ingestGDELT(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown ingest source %q (want worldbank, trade, trade-comtrade or gdelt)\n", args[0])
		return 2
	}
}

// ingestGDELT fetches recent risk-relevant news/event documents for the
// requested countries from the live GDELT DOC 2.0 API and normalises them to
// data/raw/gdelt/gdelt_events.json. --base-url is provided so the importer can
// be pointed at a local/fixture server; it defaults to the live API.
func ingestGDELT(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("ingest gdelt", flag.ContinueOnError)
	fs.SetOutput(errOut)
	countries := fs.String("countries", "", "comma-separated ISO3 country codes (e.g. TWN,CHN,USA)")
	days := fs.Int("days", gdelt.DefaultDays, "look-back window in days")
	outDir := fs.String("out", "data/raw/gdelt", "directory to write normalized output to")
	baseURL := fs.String("base-url", gdelt.DefaultBaseURL, "GDELT DOC 2.0 endpoint (override for testing)")
	timeout := fs.Duration("timeout", 2*time.Minute, "overall timeout for the fetch")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas ingest gdelt --countries TWN,CHN,USA [--days 7] [--out dir]")
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
	if *days < 1 {
		fmt.Fprintf(errOut, "error: --days must be >= 1, got %d\n", *days)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := gdelt.NewClient(60 * time.Second)
	client.BaseURL = *baseURL
	fmt.Fprintf(out, "Fetching GDELT events for %s (last %d days)…\n", strings.Join(codes, ", "), *days)

	records, err := client.Fetch(ctx, codes, *days)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	gdelt.SortRecords(records)

	file := gdelt.EventFile{
		Source:    gdelt.SourceName,
		FetchedAt: time.Now().UTC(),
		Days:      *days,
		Countries: codes,
		Records:   records,
	}
	path, err := gdelt.Save(*outDir, file)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	renderGDELTIngestReport(out, codes, *days, path, gdelt.BuildSummary(file, 5))
	return 0
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

// ingestTradeComtrade ingests a downloaded UN Comtrade-style CSV export and
// normalises it into the same trade_flows.json the rest of the trade pipeline
// consumes. It does not call the Comtrade API: this is a local importer for
// CSVs the user has already downloaded.
func ingestTradeComtrade(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("ingest trade-comtrade", flag.ContinueOnError)
	fs.SetOutput(errOut)
	file := fs.String("file", "", "path to a UN Comtrade-style CSV to ingest")
	outDir := fs.String("out", "data/processed/trade", "directory to write normalized output to")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas ingest trade-comtrade --file <csv> [--out dir]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*file) == "" {
		fmt.Fprintln(errOut, "error: --file is required (path to a UN Comtrade-style CSV)")
		fs.Usage()
		return 2
	}

	res, err := trade.LoadComtradeFile(*file)
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
		Source:     trade.ComtradeSourceName,
		IngestedAt: time.Now().UTC(),
		SourceFile: *file,
		Records:    res.Records,
	}
	path, err := trade.Save(*outDir, tf)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	renderComtradeIngestReport(out, *file, path, res, trade.BuildSummary(tf, 0))
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
