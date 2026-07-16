package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
	macroingest "github.com/atlasgraph/atlas/internal/ingest/macro"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

func runIngest(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas ingest <worldbank|macro|trade|trade-comtrade|gdelt|commodity-prices|events> [flags]")
		return 2
	}
	switch args[0] {
	case "worldbank":
		return ingestWorldBank(args[1:], out, errOut)
	case "macro":
		return ingestMacro(args[1:], out, errOut)
	case "trade":
		return ingestTrade(args[1:], out, errOut)
	case "trade-comtrade":
		return ingestTradeComtrade(args[1:], out, errOut)
	case "gdelt":
		return ingestGDELT(args[1:], out, errOut)
	case "commodity-prices":
		return ingestCommodityPrices(args[1:], out, errOut)
	case "events":
		return ingestEvents(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown ingest source %q (want worldbank, macro, trade, trade-comtrade, gdelt, commodity-prices or events)\n", args[0])
		return 2
	}
}

// ingestCommodityPrices loads a local commodity price time-series CSV or World
// Bank Pink Sheet monthly XLSX, normalises it into commodity_prices.json, and
// prints an ingestion report.
func ingestCommodityPrices(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("ingest commodity-prices", flag.ContinueOnError)
	fs.SetOutput(errOut)
	file := fs.String("file", "", "path to a commodity price CSV or Pink Sheet XLSX to ingest")
	source := fs.String("source", "", "ingest source: csv or worldbank-pinksheet (auto-detected from extension when omitted)")
	outDir := fs.String("out", "data/processed/commodity_prices", "directory to write normalized output to")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas ingest commodity-prices --file <csv|xlsx> [--source csv|worldbank-pinksheet] [--out dir]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*file) == "" {
		fmt.Fprintln(errOut, "error: --file is required (path to a commodity price CSV or Pink Sheet XLSX)")
		fs.Usage()
		return 2
	}

	res, sourceName, meta, err := commodityprices.IngestFromFile(*file, *source)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	if res.ValidRows() == 0 {
		fmt.Fprintf(errOut, "error: no valid commodity price rows in %s\n", *file)
		return 1
	}

	commodityprices.SortRecords(res.Records)
	pf := commodityprices.PriceFile{
		Source:     sourceName,
		IngestedAt: time.Now().UTC(),
		SourceFile: *file,
		Records:    res.Records,
	}
	path, err := commodityprices.Save(*outDir, pf)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	renderCommodityIngestReport(out, *file, path, res, commodityprices.BuildSummary(pf), sourceName, meta)
	return 0
}

// ingestEvents loads a local GDELT-style CSV/JSON file, scores country event
// risk, and writes data/processed/events/event_risk.json.
func ingestEvents(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("ingest events", flag.ContinueOnError)
	fs.SetOutput(errOut)
	file := fs.String("file", "", "path to a GDELT-style event CSV or JSON file")
	source := fs.String("source", eventrisk.SourceName, "ingest source label (e.g. gdelt)")
	outDir := fs.String("out", "data/processed/events", "directory to write normalized output to")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas ingest events --file <csv|json> [--source gdelt] [--out dir]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*file) == "" {
		fmt.Fprintln(errOut, "error: --file is required (path to a GDELT-style event CSV or JSON)")
		fs.Usage()
		return 2
	}

	riskFile, warnings, err := eventrisk.IngestFromFile(*file, *source)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		for _, w := range warnings {
			fmt.Fprintf(errOut, "  warning: %s\n", w)
		}
		return 1
	}

	path, err := eventrisk.Save(*outDir, riskFile)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	renderEventIngestReport(out, *file, path, riskFile, warnings)
	return 0
}

// renderEventIngestReport prints a short summary after event-risk ingest.
func renderEventIngestReport(out io.Writer, srcFile, outPath string, file eventrisk.RiskFile, warnings []string) {
	fmt.Fprintf(out, "Ingested %d events across %d countries from %s\n", file.EventCount, file.CountriesCovered, srcFile)
	if file.RowsProcessed > 0 {
		fmt.Fprintf(out, "Rows processed: %d\n", file.RowsProcessed)
	}
	if file.DateFrom != "" && file.DateTo != "" {
		fmt.Fprintf(out, "Date range: %s to %s\n", file.DateFrom, file.DateTo)
	}
	if file.LatestEventDate != "" {
		fmt.Fprintf(out, "Latest event date: %s\n", file.LatestEventDate)
	}
	if len(file.EventTypeBreakdown) > 0 {
		fmt.Fprintln(out, "Event types:")
		for _, t := range eventrisk.SortedEventTypeKeys(file.EventTypeBreakdown) {
			fmt.Fprintf(out, "  - %s: %d\n", t, file.EventTypeBreakdown[t])
		}
	}
	fmt.Fprintf(out, "Saved event risk panel to %s\n", outPath)
	for _, w := range warnings {
		fmt.Fprintf(out, "  warning: %s\n", w)
	}
	if len(file.Countries) > 0 {
		top := file.Countries[0]
		fmt.Fprintf(out, "Top event-risk country: %s (%.1f, %s)\n", top.Country, top.EventRiskScore, top.RiskLevel)
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
	file := fs.String("file", "", "path to a trade-flow or UN Comtrade CSV to ingest")
	dir := fs.String("dir", "", "directory of UN Comtrade CSV files to ingest")
	source := fs.String("source", "", "ingest source: un-comtrade for downloaded UN Comtrade CSV exports")
	outDir := fs.String("out", "data/processed/trade", "directory to write normalized output to")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas ingest trade --file <csv> [--out dir]")
		fmt.Fprintln(errOut, "       atlas ingest trade --dir <dir> --source un-comtrade [--out dir]")
		fmt.Fprintln(errOut, "       atlas ingest trade --file <csv> --source un-comtrade [--out dir]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	sourceName := strings.ToLower(strings.TrimSpace(*source))
	if sourceName == "un-comtrade" || strings.TrimSpace(*dir) != "" {
		return ingestUNComtradeTrade(*file, *dir, *outDir, out, errOut)
	}

	if strings.TrimSpace(*file) == "" {
		fmt.Fprintln(errOut, "error: --file is required (path to a trade-flow CSV), or use --dir --source un-comtrade")
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

func ingestUNComtradeTrade(file, dir, outDir string, out, errOut io.Writer) int {
	var paths []string
	if strings.TrimSpace(dir) != "" {
		entries, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintf(errOut, "error: %v\n", err)
			return 1
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.EqualFold(filepath.Ext(e.Name()), ".csv") {
				paths = append(paths, filepath.Join(dir, e.Name()))
			}
		}
		if len(paths) == 0 {
			fmt.Fprintf(errOut, "error: no CSV files found in %s\n", dir)
			return 1
		}
	} else if strings.TrimSpace(file) != "" {
		paths = []string{file}
	} else {
		fmt.Fprintln(errOut, "error: --file or --dir is required for --source un-comtrade")
		return 2
	}

	deps, stats, err := trade.IngestUNComtradeFiles(paths)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	path, err := trade.SaveDependencies(outDir, deps)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	renderUNComtradeIngestReport(out, paths, path, deps, stats)
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

// ingestMacro fetches or loads World Bank macro indicators, scores them, and
// writes data/processed/macro/macro_scores.json.
func ingestMacro(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("ingest macro", flag.ContinueOnError)
	fs.SetOutput(errOut)
	source := fs.String("source", "worldbank", "data source: worldbank (API with CSV fallback) or csv")
	rawDir := fs.String("raw", macroingest.RawDirName, "directory for raw CSV/JSON macro indicators")
	outDir := fs.String("out", "data/processed/macro", "directory to write macro_scores.json")
	countries := fs.String("countries", strings.Join(macroingest.DefaultCountries, ","), "comma-separated ISO3 country codes")
	start := fs.Int("start", worldbank.DefaultStartYear, "first year to fetch")
	end := fs.Int("end", defaultEndYear(), "last year to fetch")
	timeout := fs.Duration("timeout", 3*time.Minute, "overall timeout for API fetch")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas ingest macro [--source worldbank|csv] [--raw dir] [--out dir] [--countries ISO3,...] [--start Y] [--end Y]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *start > *end {
		fmt.Fprintf(errOut, "error: --start (%d) must not be after --end (%d)\n", *start, *end)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Fprintf(out, "Ingesting World Bank macro indicators (%s)…\n", *source)
	res, err := macroingest.Ingest(ctx, macroingest.Options{
		Source:    *source,
		RawDir:    *rawDir,
		OutDir:    *outDir,
		Countries: splitCodes(*countries),
		StartYear: *start,
		EndYear:   *end,
		Timeout:   *timeout,
	})
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	fmt.Fprintf(out, "Macro ingest complete (%s)\n", res.FetchSource)
	if len(res.Warnings) > 0 {
		fmt.Fprintf(out, "  Warnings (%d):\n", len(res.Warnings))
		for _, w := range res.Warnings {
			fmt.Fprintf(out, "    - %s: %s\n", w.Indicator, w.Message)
		}
	}
	fmt.Fprintf(out, "  Raw indicators : %s (%d records, %d countries)\n", res.RawPath, res.RecordCount, res.CountryCount)
	fmt.Fprintf(out, "  Macro scores   : %s (%d countries)\n", res.ScoresPath, res.ScoreCount)
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
