package pipeline

import (
	"strings"

	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

// buildFileBackedValidationChecks mirrors db load validation using processed files.
func buildFileBackedValidationChecks(cfg Config) ([]ValidationCheck, error) {
	resolvedTrade, tradeErr := trade.ResolveTrade(cfg.TradeData)
	macroFile, macroErr := macro.LoadProcessed(cfg.ProcessedMacroData)
	if macroErr != nil {
		if processed, ok := macro.TryLoadProcessed(cfg.ProcessedMacroData); ok {
			macroFile = processed
			macroErr = nil
		}
	}
	eventDir := cfg.ProcessedEventData
	if eventDir == "" {
		eventDir = cfg.EventData
	}
	eventFile, eventErr := eventrisk.Load(eventDir)
	priceFile, priceErr := commodityprices.Load(cfg.CommodityData)

	dataset, graphErr := loadGraphDataset(cfg.GraphData)

	tradeRows := 0
	missingImporter, missingExporter, missingCommodity := 0, 0, 0
	if tradeErr == nil {
		appendTrade := func(r trade.TradeFlowRecord) {
			tradeRows++
			if strings.TrimSpace(r.ImporterCode) == "" {
				missingImporter++
			}
			if strings.TrimSpace(r.ExporterCode) == "" {
				missingExporter++
			}
			if strings.TrimSpace(r.CommodityName) == "" {
				missingCommodity++
			}
		}
		if resolvedTrade.DependencyFile != nil {
			for _, dependency := range resolvedTrade.DependencyFile.Dependencies {
				appendTrade(trade.TradeDependencyToRecord(dependency))
			}
		} else {
			for _, record := range resolvedTrade.File.Records {
				appendTrade(record)
			}
		}
	}

	missingMacro := 0
	macroRows := 0
	if macroErr == nil {
		macroRows = len(macroFile.Scores)
		for _, score := range macroFile.Scores {
			hasData := false
			for _, component := range score.Components {
				hasData = hasData || component.Available
			}
			if !hasData {
				missingMacro++
			}
		}
	} else if cfg.MacroData != "" {
		if file, err := worldbank.Load(cfg.MacroData); err == nil {
			macroRows = len(macro.ScoreCountries(file, 0, macro.DefaultWeights()))
		}
	}

	eventRows := 0
	if eventErr == nil {
		eventRows = len(eventFile.Countries)
	}

	missingPriceSeries := 0
	if graphErr == nil {
		priceCommodities := map[string]bool{}
		if priceErr == nil {
			for _, record := range priceFile.Records {
				priceCommodities[strings.ToLower(strings.TrimSpace(record.CommodityName))] = true
			}
		}
		for _, node := range dataset.Graph.Nodes() {
			if string(node.Type) == "commodity" && !priceCommodities[strings.ToLower(strings.TrimSpace(node.Name))] {
				missingPriceSeries++
			}
		}
	}

	graphEdges := graphEdgeCount(dataset)

	checks := []ValidationCheck{
		makeValidationCheck("total trade rows loaded", tradeRows, false, "UN Comtrade"),
		makeValidationCheck("missing importer codes", missingImporter, true, "UN Comtrade"),
		makeValidationCheck("missing exporter codes", missingExporter, true, "UN Comtrade"),
		makeValidationCheck("missing commodity names", missingCommodity, true, "UN Comtrade"),
		makeValidationCheck("missing macro scores", missingMacro, true, "World Bank Macro"),
		makeValidationCheck("missing commodity price series", missingPriceSeries, true, "World Bank Pink Sheet"),
		makeValidationCheck("event risk rows loaded", eventRows, false, "GDELT"),
		makeValidationCheck("dependency graph edges loaded", graphEdges, false, "Baseline dependency graph"),
	}

	if tradeErr != nil && cfg.TradeData != "" {
		checks = append(checks, normalizeValidationCheck(
			"trade source available", "failed", 0, "UN Comtrade",
			mustJSON(map[string]any{"error": tradeErr.Error()}),
		))
	}
	if eventErr != nil && eventDir != "" {
		checks = append(checks, normalizeValidationCheck(
			"event source available", "warning", 0, "GDELT",
			mustJSON(map[string]any{"error": eventErr.Error()}),
		))
	}
	if macroErr != nil && macroRows == 0 && (cfg.ProcessedMacroData != "" || cfg.MacroData != "") {
		checks = append(checks, normalizeValidationCheck(
			"macro source available", "warning", 0, "World Bank Macro",
			mustJSON(map[string]any{"error": macroErr.Error()}),
		))
	}
	if priceErr != nil && cfg.CommodityData != "" {
		checks = append(checks, normalizeValidationCheck(
			"commodity price source available", "warning", 0, "World Bank Pink Sheet",
			mustJSON(map[string]any{"error": priceErr.Error()}),
		))
	}
	if graphErr != nil && strings.TrimSpace(cfg.GraphData) != "" {
		checks = append(checks, normalizeValidationCheck(
			"dependency graph available", "failed", 0, "Baseline dependency graph",
			mustJSON(map[string]any{"error": graphErr.Error()}),
		))
	}

	return checks, nil
}
