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

// gdeltSleepOverride lets tests replace the live client's rate-limit/back-off
// sleeps with a no-op so 429 retry and inter-country spacing can be exercised
// without real waits. It is nil in normal use (the client sleeps for real).
var gdeltSleepOverride func(time.Duration)

// ingestGDELT fetches recent risk-relevant news/event documents and normalises
// them to data/raw/gdelt/gdelt_events.json. It has two modes:
//
//   - live mode (default): query the GDELT DOC 2.0 API per country, with
//     rate-limit spacing and 429 retry/back-off, saving partial results when
//     some countries fail.
//   - fixture mode (--fixture <file>): load a local synthetic fixture for a
//     fully offline, reproducible demo.
//
// --base-url is provided so the importer can be pointed at a local/fixture
// server; it defaults to the live API.
func ingestGDELT(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("ingest gdelt", flag.ContinueOnError)
	fs.SetOutput(errOut)
	countries := fs.String("countries", "", "comma-separated ISO3 country codes (e.g. TWN,CHN,USA)")
	days := fs.Int("days", gdelt.DefaultDays, "look-back window in days")
	limit := fs.Int("limit", gdelt.DefaultLimit, "max results per country (live mode)")
	delaySeconds := fs.Int("delay-seconds", gdelt.DefaultDelaySeconds, "seconds between per-country requests (clamped to a 5s minimum)")
	fixture := fs.String("fixture", "", "path to a local GDELT fixture JSON for offline demo mode")
	outDir := fs.String("out", "data/raw/gdelt", "directory to write normalized output to")
	baseURL := fs.String("base-url", gdelt.DefaultBaseURL, "GDELT DOC 2.0 endpoint (override for testing)")
	timeout := fs.Duration("timeout", 5*time.Minute, "overall timeout for the fetch")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas ingest gdelt --countries TWN,CHN,USA [--days 7] [--limit 25] [--delay-seconds 6] [--out dir]")
		fmt.Fprintln(errOut, "       atlas ingest gdelt --fixture data/examples/gdelt_events_sample.json [--out dir]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Fixture mode: fully offline, no countries/limit/delay required.
	if strings.TrimSpace(*fixture) != "" {
		return ingestGDELTFixture(*fixture, *outDir, out, errOut)
	}

	codes := splitCodes(*countries)
	if len(codes) == 0 {
		fmt.Fprintln(errOut, "error: --countries is required (comma-separated ISO3 codes), or use --fixture for offline mode")
		fs.Usage()
		return 2
	}
	if *days < 1 {
		fmt.Fprintf(errOut, "error: --days must be >= 1, got %d\n", *days)
		return 2
	}
	if *limit < 1 {
		fmt.Fprintf(errOut, "error: --limit must be >= 1, got %d\n", *limit)
		return 2
	}
	delay := gdelt.ClampDelaySeconds(*delaySeconds)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := gdelt.NewClient(60 * time.Second)
	client.BaseURL = *baseURL
	client.MaxRecords = *limit
	client.DelaySeconds = delay
	if gdeltSleepOverride != nil {
		client.Sleep = gdeltSleepOverride
	}
	fmt.Fprintf(out, "Fetching GDELT events for %s (last %d days, limit %d/country, %ds spacing)…\n",
		strings.Join(codes, ", "), *days, *limit, delay)

	res, err := client.FetchPartial(ctx, codes, *days)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	// Every country failed: nothing to save, point the user at offline mode.
	if len(res.Succeeded) == 0 {
		for _, f := range res.Failed {
			fmt.Fprintf(errOut, "  failed %s: %s\n", f.Code, f.Reason)
		}
		fmt.Fprintln(errOut, "Live GDELT ingestion failed for all countries. Try again later or use --fixture data/examples/gdelt_events_sample.json for offline demo mode.")
		return 1
	}

	gdelt.SortRecords(res.Records)
	file := gdelt.EventFile{
		Source:    gdelt.SourceName,
		FetchedAt: time.Now().UTC(),
		Days:      *days,
		Countries: res.Succeeded,
		Records:   res.Records,
	}
	path, err := gdelt.Save(*outDir, file)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	renderGDELTLiveReport(out, codes, *days, *limit, delay, res, path, gdelt.BuildSummary(file, 5))
	return 0
}

// ingestGDELTFixture loads a local synthetic fixture, normalises it into the
// same schema as a live pull, saves it to outDir and prints a fixture-mode
// report. It never touches the network.
func ingestGDELTFixture(fixturePath, outDir string, out, errOut io.Writer) int {
	records, err := gdelt.LoadFixture(fixturePath)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	if len(records) == 0 {
		fmt.Fprintf(errOut, "error: no events found in fixture %s\n", fixturePath)
		return 1
	}

	gdelt.SortRecords(records)
	countries := distinctCountryCodes(records)
	file := gdelt.EventFile{
		Source:    gdelt.FixtureSourceName,
		FetchedAt: time.Now().UTC(),
		Days:      0,
		Countries: countries,
		Records:   records,
	}
	path, err := gdelt.Save(outDir, file)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	renderGDELTFixtureReport(out, fixturePath, path, countries, gdelt.BuildSummary(file, 5))
	return 0
}

// distinctCountryCodes returns the unique country codes present in records, in
// first-seen order.
func distinctCountryCodes(records []gdelt.GDELTEventRecord) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, r := range records {
		if r.CountryCode == "" {
			continue
		}
		if _, ok := seen[r.CountryCode]; ok {
			continue
		}
		seen[r.CountryCode] = struct{}{}
		out = append(out, r.CountryCode)
	}
	return out
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
