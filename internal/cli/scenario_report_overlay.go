package cli

import (
	"github.com/atlasgraph/atlas/internal/clientoverlay"
	"github.com/atlasgraph/atlas/internal/customdata"
	"github.com/atlasgraph/atlas/internal/recommendations"
	"github.com/atlasgraph/atlas/internal/simulation"
)

// clientAnalysisPayload carries optional uploaded client supplier data for overlay.
type clientAnalysisPayload struct {
	ConcentrationResults []customdata.ConcentrationResult `json:"concentration_results"`
	NormalizedRows       []customdata.Row                 `json:"normalized_rows"`
}

func payloadToAnalysis(payload *clientAnalysisPayload) customdata.Analysis {
	if payload == nil {
		return customdata.Analysis{}
	}
	return customdata.Analysis{
		ConcentrationResults: payload.ConcentrationResults,
		ValidRows:            payload.NormalizedRows,
	}
}

func buildClientOverlay(payload *clientAnalysisPayload, res simulation.Result) *clientoverlay.OverlayResult {
	if payload == nil || len(payload.NormalizedRows) == 0 {
		return nil
	}
	overlay := clientoverlay.Compute(payloadToAnalysis(payload), clientoverlay.Request{
		Source:      res.SourceNode.Name,
		Commodity:   res.CommodityNode.Name,
		DropPercent: res.Request.DropPct,
	})
	if overlay.MatchedImporters == 0 {
		return nil
	}
	return &overlay
}

func applyClientOverlayToRecommendations(
	in recommendations.Input,
	overlay *clientoverlay.OverlayResult,
) recommendations.Input {
	if overlay == nil || overlay.MatchedImporters == 0 {
		return in
	}
	hhi := overlay.HighestHHI
	share := overlay.HighestSupplierShare
	in.HHI = &hhi
	in.SupplierShare = &share
	if overlay.AverageConcentrationRisk != "" {
		in.TradeConcentration = overlay.AverageConcentrationRisk
	}
	return in
}
