package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	analyticsdb "github.com/atlasgraph/atlas/internal/db"
	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

// SourceRow captures per-source ETL metrics for one dataset.
type SourceRow struct {
	Name          string `json:"name"`
	Source        string `json:"source"`
	RowsProcessed int    `json:"rows_processed"`
	RowsLoaded    int    `json:"rows_loaded"`
}

// PipelineRunSummary is the v1 ETL run snapshot exposed by the API.
type PipelineRunSummary struct {
	RunID                    string            `json:"run_id"`
	StartedAt                *time.Time        `json:"started_at,omitempty"`
	CompletedAt              *time.Time        `json:"completed_at,omitempty"`
	Status                   string            `json:"status"`
	SourcesProcessed         []SourceRow       `json:"sources_processed"`
	TotalRowsProcessed       int               `json:"total_rows_processed"`
	TotalRowsLoaded          int               `json:"total_rows_loaded"`
	InvalidRows              int               `json:"invalid_rows"`
	ValidationChecks         []ValidationCheck `json:"validation_checks"`
	ValidationChecksPassed   int               `json:"validation_checks_passed"`
	ValidationChecksWarnings int               `json:"validation_checks_warnings"`
	ValidationChecksFailed   int               `json:"validation_checks_failed"`
	OutputTables             []string          `json:"output_tables"`
	Notes                    []string          `json:"notes,omitempty"`
}

// Config points the summary builder at the same processed paths the server uses.
type Config struct {
	TradeData          string
	ProcessedMacroData string
	MacroData          string
	ProcessedEventData string
	EventData          string
	CommodityData      string
	GraphData          string
}

// Compute builds a pipeline summary from processed files and optional PostgreSQL counts.
func Compute(ctx context.Context, cfg Config, db *analyticsdb.DB) (PipelineRunSummary, error) {
	summary := PipelineRunSummary{
		Status:           "idle",
		SourcesProcessed: []SourceRow{},
		ValidationChecks: []ValidationCheck{},
		OutputTables:     []string{},
		Notes:            []string{},
	}
	var timestamps []time.Time
	dbEnabled := db != nil

	addSource := func(name, source string, processed int) {
		if processed <= 0 {
			return
		}
		summary.SourcesProcessed = append(summary.SourcesProcessed, SourceRow{
			Name: name, Source: source, RowsProcessed: processed,
		})
		summary.TotalRowsProcessed += processed
	}

	if resolved, err := trade.ResolveTrade(cfg.TradeData); err == nil {
		tradeRows := 0
		if resolved.DependencyFile != nil {
			tradeRows = len(resolved.DependencyFile.Dependencies)
		} else {
			tradeRows = len(resolved.File.Records)
		}
		source := resolved.Source
		if source == "" {
			source = "UN Comtrade"
		}
		addSource("UN Comtrade trade rows", source, tradeRows)
		if !resolved.File.IngestedAt.IsZero() {
			timestamps = append(timestamps, resolved.File.IngestedAt)
		}
	} else if cfg.TradeData != "" {
		summary.Notes = append(summary.Notes, fmt.Sprintf("trade data unavailable: %v", err))
	}

	macroFile, macroErr := macro.LoadProcessed(cfg.ProcessedMacroData)
	if macroErr != nil {
		if processed, ok := macro.TryLoadProcessed(cfg.ProcessedMacroData); ok {
			macroFile = processed
			macroErr = nil
		}
	}
	if macroErr == nil {
		source := macroFile.Source
		if source == "" {
			source = "World Bank Macro"
		}
		addSource("World Bank Macro rows", source, len(macroFile.Scores))
		if !macroFile.GeneratedAt.IsZero() {
			timestamps = append(timestamps, macroFile.GeneratedAt)
		}
	} else if cfg.MacroData != "" {
		if file, err := worldbank.Load(cfg.MacroData); err == nil {
			scores := macro.ScoreCountries(file, 0, macro.DefaultWeights())
			if len(scores) > 0 {
				addSource("World Bank Macro rows", worldbank.SourceName, len(scores))
			}
		} else {
			summary.Notes = append(summary.Notes, fmt.Sprintf("macro data unavailable: %v", err))
		}
	} else if cfg.ProcessedMacroData != "" {
		summary.Notes = append(summary.Notes, fmt.Sprintf("macro data unavailable: %v", macroErr))
	}

	eventDir := cfg.ProcessedEventData
	if eventDir == "" {
		eventDir = cfg.EventData
	}
	if eventFile, err := eventrisk.Load(eventDir); err == nil {
		source := eventFile.Source
		if source == "" {
			source = "GDELT"
		}
		addSource("GDELT event-risk rows", source, len(eventFile.Countries))
		if !eventFile.IngestedAt.IsZero() {
			timestamps = append(timestamps, eventFile.IngestedAt)
		}
	} else if eventDir != "" {
		summary.Notes = append(summary.Notes, fmt.Sprintf("event data unavailable: %v", err))
	}

	if priceFile, err := commodityprices.Load(cfg.CommodityData); err == nil {
		source := priceFile.Source
		if source == "" {
			source = "World Bank Pink Sheet"
		}
		addSource("World Bank Pink Sheet commodity price rows", source, len(priceFile.Records))
		if !priceFile.IngestedAt.IsZero() {
			timestamps = append(timestamps, priceFile.IngestedAt)
		}
	} else if cfg.CommodityData != "" {
		summary.Notes = append(summary.Notes, fmt.Sprintf("commodity price data unavailable: %v", err))
	}

	if dataset, err := loadGraphDataset(cfg.GraphData); err == nil {
		addSource("Dependency graph edges", "Baseline dependency graph", graphEdgeCount(dataset))
	} else if strings.TrimSpace(cfg.GraphData) != "" {
		summary.Notes = append(summary.Notes, fmt.Sprintf("graph data unavailable: %v", err))
	}

	if db != nil {
		customRows, _ := db.CountCustomTradeFlows(ctx)
		if customRows > 0 {
			summary.SourcesProcessed = append(summary.SourcesProcessed, SourceRow{
				Name: "Custom client data rows", Source: "Client CSV upload",
				RowsProcessed: int(customRows), RowsLoaded: int(customRows),
			})
			summary.TotalRowsProcessed += int(customRows)
		}

		dbSummary, err := db.Summary(ctx)
		if err != nil {
			return summary, fmt.Errorf("query postgres summary: %w", err)
		}
		summary.TotalRowsLoaded = int(dbSummary.TradeFlows + dbSummary.EventRiskSignals +
			dbSummary.MacroScores + dbSummary.CommodityPrices + dbSummary.DependencyEdges + customRows)

		applyLoadedCounts(&summary, dbSummary, customRows)

		checks, err := db.ListValidationChecks(ctx)
		if err != nil {
			return summary, fmt.Errorf("query validation checks: %w", err)
		}
		if len(checks) == 0 {
			fileChecks, fileErr := buildFileBackedValidationChecks(cfg)
			if fileErr != nil {
				summary.Notes = append(summary.Notes, fmt.Sprintf("validation checks unavailable: %v", fileErr))
			} else {
				summary.ValidationChecks = fileChecks
			}
		} else {
			summary.ValidationChecks = normalizeDBValidationChecks(checks)
			if !checksLatest(checks).IsZero() {
				timestamps = append(timestamps, checksLatest(checks))
			}
		}

		summary.OutputTables = outputTablesFromDB(dbSummary, customRows)
	} else {
		summary.Notes = append(summary.Notes,
			"PostgreSQL analytics is disabled; rows_loaded and validation checks reflect file-backed processing only.")
		checks, err := buildFileBackedValidationChecks(cfg)
		if err != nil {
			summary.Notes = append(summary.Notes, fmt.Sprintf("validation checks unavailable: %v", err))
		} else {
			summary.ValidationChecks = checks
		}
		summary.OutputTables = fileBackedOutputTables(summary)
	}

	passed, warnings, failed, invalidRows := summarizeValidationChecks(summary.ValidationChecks)
	summary.ValidationChecksPassed = passed
	summary.ValidationChecksWarnings = warnings
	summary.ValidationChecksFailed = failed
	summary.InvalidRows = invalidRows

	if len(timestamps) > 0 {
		started := timestamps[0]
		completed := timestamps[0]
		for _, ts := range timestamps[1:] {
			if ts.Before(started) {
				started = ts
			}
			if ts.After(completed) {
				completed = ts
			}
		}
		summary.StartedAt = &started
		summary.CompletedAt = &completed
		summary.RunID = fmt.Sprintf("pipeline-%s", completed.UTC().Format("20060102T150405Z"))
	} else {
		summary.RunID = "pipeline-pending"
	}

	summary.Status = deriveStatus(summary, dbEnabled)
	return summary, nil
}

func checksLatest(checks []analyticsdb.PipelineValidationCheck) time.Time {
	var latest time.Time
	for _, check := range checks {
		if check.CreatedAt.After(latest) {
			latest = check.CreatedAt
		}
	}
	return latest
}

func applyLoadedCounts(summary *PipelineRunSummary, dbSummary analyticsdb.Summary, customRows int64) {
	loaded := map[string]int64{
		"UN Comtrade trade rows":                     dbSummary.TradeFlows,
		"GDELT event-risk rows":                      dbSummary.EventRiskSignals,
		"World Bank Macro rows":                      dbSummary.MacroScores,
		"World Bank Pink Sheet commodity price rows": dbSummary.CommodityPrices,
		"Dependency graph edges":                     dbSummary.DependencyEdges,
		"Custom client data rows":                    customRows,
	}
	for i := range summary.SourcesProcessed {
		if count, ok := loaded[summary.SourcesProcessed[i].Name]; ok {
			summary.SourcesProcessed[i].RowsLoaded = int(count)
		}
	}
}

func outputTablesFromDB(dbSummary analyticsdb.Summary, customRows int64) []string {
	tables := []string{}
	add := func(name string, count int64) {
		if count > 0 {
			tables = append(tables, name)
		}
	}
	add("trade_flows", dbSummary.TradeFlows)
	add("event_risk_signals", dbSummary.EventRiskSignals)
	add("macro_scores", dbSummary.MacroScores)
	add("commodity_prices", dbSummary.CommodityPrices)
	add("dependency_edges", dbSummary.DependencyEdges)
	add("custom_trade_flows", customRows)
	if dbSummary.ScenarioRuns > 0 {
		tables = append(tables, "scenario_runs")
	}
	if dbSummary.DataQualityChecks > 0 {
		tables = append(tables, "data_quality_checks")
	}
	return tables
}

func fileBackedOutputTables(summary PipelineRunSummary) []string {
	tables := []string{}
	for _, src := range summary.SourcesProcessed {
		switch src.Name {
		case "UN Comtrade trade rows":
			tables = append(tables, "trade_flows")
		case "GDELT event-risk rows":
			tables = append(tables, "event_risk_signals")
		case "World Bank Macro rows":
			tables = append(tables, "macro_scores")
		case "World Bank Pink Sheet commodity price rows":
			tables = append(tables, "commodity_prices")
		case "Dependency graph edges":
			tables = append(tables, "dependency_edges")
		case "Custom client data rows":
			tables = append(tables, "custom_trade_flows")
		}
	}
	return tables
}

func deriveStatus(summary PipelineRunSummary, dbEnabled bool) string {
	if summary.TotalRowsProcessed == 0 && len(summary.ValidationChecks) == 0 {
		return "idle"
	}

	if summary.ValidationChecksFailed > 0 {
		return "failed"
	}
	if coreSourceMissing(summary) {
		return "failed"
	}
	if dbEnabled && summary.TotalRowsProcessed > 0 && summary.TotalRowsLoaded == 0 {
		return "failed"
	}
	if summary.ValidationChecksWarnings > 0 || summary.InvalidRows > 0 {
		return "warning"
	}
	if len(summary.SourcesProcessed) > 0 || summary.TotalRowsProcessed > 0 {
		return "completed"
	}
	return "idle"
}

func coreSourceMissing(summary PipelineRunSummary) bool {
	hasTrade := false
	hasGraph := false
	for _, src := range summary.SourcesProcessed {
		switch src.Name {
		case "UN Comtrade trade rows":
			hasTrade = src.RowsProcessed > 0
		case "Dependency graph edges":
			hasGraph = src.RowsProcessed > 0
		}
	}
	if len(summary.SourcesProcessed) > 0 && (!hasTrade || !hasGraph) {
		return true
	}
	return false
}
