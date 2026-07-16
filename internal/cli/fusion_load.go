package cli

import (
	"net/http"

	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graphfusion"
	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/scoring/fragility"
)

// fusionConfig holds optional processed data paths for graph fusion.
type fusionConfig struct {
	GraphData          string
	TradeData          string
	MacroData          string
	ProcessedMacroData string
	ProcessedEventData string
	EventData          string
	CommodityData      string
}

func (c fusionConfig) fromServer(cfg serverConfig) fusionConfig {
	return fusionConfig{
		GraphData:          cfg.GraphData,
		TradeData:          cfg.TradeData,
		MacroData:          cfg.MacroData,
		ProcessedMacroData: cfg.ProcessedMacroData,
		ProcessedEventData: cfg.ProcessedEventData,
		EventData:          cfg.EventData,
		CommodityData:      cfg.CommodityData,
	}
}

func (c serverConfig) fusionConfig() fusionConfig {
	return fusionConfig{}.fromServer(c)
}

func (s *apiServer) loadFused(w http.ResponseWriter) (graphfusion.Output, bool) {
	out, err := loadFusedDataset(s.cfg.fusionConfig())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"build a graph with `atlas graph build-trade` or pass an existing --data dir")
		return graphfusion.Output{}, false
	}
	return out, true
}

func loadFusedDataset(cfg fusionConfig) (graphfusion.Output, error) {
	base, err := loadDataset(cfg.GraphData)
	if err != nil {
		return graphfusion.Output{}, err
	}
	in := graphfusion.Input{Base: base}

	if cfg.TradeData != "" {
		if resolved, err := trade.ResolveTrade(cfg.TradeData); err == nil && resolved.DependencyFile != nil {
			in.Trade = resolved.DependencyFile
			in.TradeReal = resolved.RealTradeData
		}
	}
	if cfg.ProcessedEventData != "" {
		if file, ok := eventrisk.TryLoadProcessed(cfg.ProcessedEventData); ok && eventrisk.IsRealProcessedEventRisk(file) {
			in.EventRisk = &file
		}
	}
	if in.EventRisk == nil && cfg.EventData != "" {
		// Legacy GDELT demo path does not feed vulnerability multipliers in v1.
	}
	if cfg.CommodityData != "" {
		if f, err := commodityprices.Load(cfg.CommodityData); err == nil && len(f.Records) > 0 {
			in.CommodityPrices = &f
		}
	}

	out := graphfusion.Fuse(in)
	graphfusion.FinalizeMeta(&out.Meta, out.SimCtx)
	return out, nil
}

func loadFusedFragilitySources(cfg fusionConfig) (fragility.Sources, graphfusion.Meta) {
	fused, err := loadFusedDataset(cfg)
	if err != nil {
		return loadFragilitySources(cfg.GraphData, cfg.TradeData, cfg.MacroData, cfg.ProcessedMacroData, cfg.ProcessedEventData, cfg.EventData, cfg.CommodityData), graphfusion.Meta{}
	}
	src := loadFragilitySources(cfg.GraphData, cfg.TradeData, cfg.MacroData, cfg.ProcessedMacroData, cfg.ProcessedEventData, cfg.EventData, cfg.CommodityData)
	if fused.Dataset != nil {
		src.Graph = fused.Dataset.Graph
		src.Scenarios = fused.Dataset.Scenarios
	}
	if fused.SimCtx.RealEventRiskUsed || fused.SimCtx.RealPriceStressUsed || fused.SimCtx.RealTradeFusionActive {
		ctx := fused.SimCtx
		src.SimContext = &ctx
	}
	return src, fused.Meta
}

func datasetFromFused(out graphfusion.Output) *data.Dataset {
	if out.Dataset != nil {
		return out.Dataset
	}
	return nil
}
