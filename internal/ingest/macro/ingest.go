package macroingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

// Options controls macro ingestion.
type Options struct {
	Source    string // worldbank or csv
	RawDir    string
	OutDir    string
	Countries []string
	StartYear int
	EndYear   int
	Timeout   time.Duration
}

// Result summarizes a completed ingest run.
type Result struct {
	RawPath       string
	ScoresPath    string
	RecordCount   int
	CountryCount  int
	ScoreCount    int
	FetchSource   string
	Warnings      []worldbank.FetchWarning
	Indicators    worldbank.IndicatorFile
	Scores        macro.ProcessedScoreFile
}

// Ingest fetches or loads World Bank indicators, scores them, and writes
// data/processed/macro/macro_scores.json.
func Ingest(ctx context.Context, opt Options) (Result, error) {
	if strings.TrimSpace(opt.OutDir) == "" {
		return Result{}, fmt.Errorf("output directory is required")
	}
	if opt.StartYear > 0 && opt.EndYear > 0 && opt.StartYear > opt.EndYear {
		return Result{}, fmt.Errorf("start year %d is after end year %d", opt.StartYear, opt.EndYear)
	}
	codes := cleanCodes(opt.Countries)
	if len(codes) == 0 {
		codes = append([]string{}, DefaultCountries...)
	}

	source := strings.ToLower(strings.TrimSpace(opt.Source))
	if source == "" {
		source = "worldbank"
	}

	var indicators worldbank.IndicatorFile
	var fetchSource string
	var warnings []worldbank.FetchWarning
	var err error

	switch source {
	case "worldbank":
		indicators, fetchSource, warnings, err = loadWorldBank(ctx, opt, codes)
	case "csv":
		indicators, err = LoadRaw(opt.RawDir)
		fetchSource = "World Bank (CSV)"
	default:
		return Result{}, fmt.Errorf("unsupported source %q (want worldbank or csv)", opt.Source)
	}
	if err != nil {
		return Result{}, err
	}
	if len(indicators.Records) == 0 {
		return Result{}, fmt.Errorf("no macro indicator records available")
	}

	rawDir := strings.TrimSpace(opt.RawDir)
	if rawDir == "" {
		rawDir = RawDirName
	}
	rawPath, err := worldbank.Save(rawDir, indicators)
	if err != nil {
		return Result{}, fmt.Errorf("saving raw indicators: %w", err)
	}

	scores := macro.ScorePipelineCountries(indicators, 0, codes)
	scoresPath, err := macro.SaveProcessed(opt.OutDir, scores)
	if err != nil {
		return Result{}, fmt.Errorf("saving macro scores: %w", err)
	}

	countries := map[string]struct{}{}
	for _, code := range codes {
		countries[code] = struct{}{}
	}

	return Result{
		RawPath:      rawPath,
		ScoresPath:   scoresPath,
		RecordCount:  len(indicators.Records),
		CountryCount: len(countries),
		ScoreCount:   len(scores.Scores),
		FetchSource:  fetchSource,
		Warnings:     warnings,
		Indicators:   indicators,
		Scores:       scores,
	}, nil
}

func loadWorldBank(ctx context.Context, opt Options, codes []string) (worldbank.IndicatorFile, string, []worldbank.FetchWarning, error) {
	apiCodes, skipped := APICountryCodes(codes)
	client := worldbank.NewClient(30 * time.Second)

	query := worldbank.FetchQueryOpts{PerPage: worldbank.MacroPipelinePerPage}
	if opt.StartYear > 0 && opt.EndYear > 0 {
		query.StartYear = opt.StartYear
		query.EndYear = opt.EndYear
	}

	fetch, err := client.FetchBestEffort(ctx, apiCodes, PipelineIndicators, query)
	if err == nil && len(fetch.Records) > 0 {
		records := append([]worldbank.CountryIndicatorRecord{}, fetch.Records...)
		records = append(records, PlaceholderRecords(skipped)...)
		worldbank.SortRecords(records)
		return worldbank.IndicatorFile{
			Source:    macro.ProcessedSourceName,
			FetchedAt: time.Now().UTC(),
			StartYear: opt.StartYear,
			EndYear:   opt.EndYear,
			Countries: codes,
			Records:   records,
		}, "World Bank API", fetch.Warnings, nil
	}

	rawDir := strings.TrimSpace(opt.RawDir)
	if rawDir == "" {
		rawDir = RawDirName
	}
	file, loadErr := LoadRaw(rawDir)
	if loadErr != nil {
		if err != nil {
			return worldbank.IndicatorFile{}, "", fetch.Warnings, fmt.Errorf("World Bank API fetch failed (%v) and no local raw data in %q (%v)", err, rawDir, loadErr)
		}
		return worldbank.IndicatorFile{}, "", fetch.Warnings, loadErr
	}
	if err != nil {
		return file, "World Bank (local CSV/JSON fallback)", fetch.Warnings, nil
	}
	return file, "World Bank (local CSV/JSON fallback)", fetch.Warnings, nil
}

func cleanCodes(codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}
