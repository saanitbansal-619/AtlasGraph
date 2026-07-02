package cli

import (
	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/scoring/fragility"
)

// loadFragilitySources assembles optional upstream datasets for unified
// fragility scoring. Missing paths are skipped so callers can produce partial
// scores with missing_components instead of failing outright.
func loadFragilitySources(graphData, tradeData, macroData, processedEventData, legacyEventData, commodityData string) fragility.Sources {
	src := fragility.Sources{Config: config.Default()}

	if ds, err := loadDataset(graphData); err == nil {
		src.Graph = ds.Graph
		src.Scenarios = ds.Scenarios
	}
	if tradeData != "" {
		if f, err := trade.Load(tradeData); err == nil {
			src.Trade = &f
		}
	}
	if macroData != "" {
		if f, err := worldbank.Load(macroData); err == nil {
			src.Macro = &f
		}
	}
	if processedEventData != "" {
		if f, err := eventrisk.Load(processedEventData); err == nil && len(f.Countries) > 0 {
			src.ProcessedEventRisk = &f
		}
	}
	if src.ProcessedEventRisk == nil && legacyEventData != "" {
		if f, err := gdelt.Load(legacyEventData); err == nil {
			src.Events = &f
		}
	}
	if commodityData != "" {
		if f, err := commodityprices.Load(commodityData); err == nil {
			src.Commodities = &f
		}
	}
	return src
}
