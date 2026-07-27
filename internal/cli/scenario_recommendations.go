package cli

import (
	"github.com/atlasgraph/atlas/internal/recommendations"
	"github.com/atlasgraph/atlas/internal/simulation"
)

func buildRecommendationInput(
	res simulation.Result,
	ctx scenarioReportContext,
	tradeEv []reportTradeEvidence,
	eventCtx []reportContextItem,
	macroCtx []reportContextItem,
) recommendations.Input {
	in := recommendations.Input{
		Source:        res.SourceNode.Name,
		Commodity:     res.CommodityNode.Name,
		ShockType:     res.Profile.Type,
		DropPercent:   res.Request.DropPct,
		Depth:         res.Request.Depth,
		AffectedPaths: len(res.Paths),
		AffectedNodes: len(res.Direct) + len(res.SecondOrder),
	}

	if te, ok := strongestTradeEvidenceForCommodity(tradeEv, in.Commodity); ok {
		hhi := te.HHI
		share := te.TopSupplierShare
		in.HHI = &hhi
		in.SupplierShare = &share
		in.TradeConcentration = te.ConcentrationRisk
	} else if te, ok := strongestTradeEvidence(tradeEv); ok {
		hhi := te.HHI
		share := te.TopSupplierShare
		in.HHI = &hhi
		in.SupplierShare = &share
		in.TradeConcentration = te.ConcentrationRisk
	}

	if ev, ok := findEventScore(ctx.EventScores, in.Source); ok {
		in.EventRisk = ev.Score
	} else if ev, ok := firstAvailableContext(eventCtx); ok {
		in.EventRisk = ev.Score
	} else if ctx.HasEventRisk {
		in.EventRisk = 52
	} else {
		in.EventRisk = 28
	}

	if mf, ok := findMacroScore(ctx.MacroScores, in.Source); ok {
		in.MacroExposure = mf.Score
	} else if mf, ok := firstAvailableContext(macroCtx); ok {
		in.MacroExposure = mf.Score
	} else if ctx.HasMacro {
		in.MacroExposure = 48
	} else {
		in.MacroExposure = 26
	}

	if len(res.TopCountries) > 0 {
		in.FragilityScore = res.TopCountries[0].Delta
	} else if len(res.Direct) > 0 {
		in.FragilityScore = res.Direct[0].Delta
	}

	if res.OperationalAssumptions != nil {
		days := res.OperationalAssumptions.InventoryBufferDays
		in.InventoryBufferDays = &days
	}

	return in
}
