// Package graphfusion augments the strategic demo graph with real processed
// trade, event-risk and commodity-price signals for simulation and scoring.
package graphfusion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/scoring/commodities"
	"github.com/atlasgraph/atlas/internal/simulation"
)

const (
	SourceStrategic  = "Strategic demo graph"
	SourceUNComtrade = "UN Comtrade"
	SourceWorldBank  = "World Bank Pink Sheet"
	SourceGDELT      = "GDELT"
)

var realShockTypes = []string{
	string(models.ShockExportCollapse),
	string(models.ShockSupplyCut),
	string(models.ShockPriceSpike),
}

// Input is the base graph plus optional processed real-data panels.
type Input struct {
	Base            *data.Dataset
	Trade           *trade.DependencyFile
	TradeReal       bool
	EventRisk       *eventrisk.RiskFile
	CommodityPrices *commodityprices.PriceFile
}

// Meta describes what fusion contributed for API transparency.
type Meta struct {
	FusionEnabled       bool     `json:"fusion_enabled"`
	BaseEntities        int      `json:"base_entities"`
	BaseDependencies    int      `json:"base_dependencies"`
	FusedEntities       int      `json:"fused_entities"`
	FusedDependencies   int      `json:"fused_dependencies"`
	RealTradeEdges      int      `json:"real_trade_edges"`
	RealTradeEdgesUsed  bool     `json:"real_trade_edges_used"`
	RealEventRiskUsed   bool     `json:"real_event_risk_used"`
	RealPriceStressUsed bool     `json:"real_price_stress_used"`
	DataSources         []string `json:"data_sources"`
}

// Output is a fused dataset ready for simulation/scoring plus metadata.
type Output struct {
	Dataset *data.Dataset
	Meta    Meta
	SimCtx  simulation.Context
}

// Fuse clones the base graph and overlays real trade edges when available.
// When no real panels exist, behaviour matches the base dataset unchanged.
func Fuse(in Input) Output {
	out := Output{
		Meta: Meta{DataSources: []string{SourceStrategic}},
	}
	if in.Base == nil || in.Base.Graph == nil {
		return out
	}

	base := in.Base.Graph
	out.Meta.BaseEntities = base.NodeCount()
	out.Meta.BaseDependencies = base.EdgeCount()
	out.Meta.FusedEntities = out.Meta.BaseEntities
	out.Meta.FusedDependencies = out.Meta.BaseDependencies
	out.SimCtx = buildSimContext(in)

	if in.Trade == nil || len(in.Trade.Dependencies) == 0 {
		out.Dataset = in.Base
		return out
	}

	g := base.Clone()
	added := fuseTradeEdges(g, in.Trade)
	out.Meta.FusionEnabled = added > 0
	out.Meta.RealTradeEdges = added
	out.Meta.RealTradeEdgesUsed = in.TradeReal && added > 0
	out.Meta.FusedEntities = g.NodeCount()
	out.Meta.FusedDependencies = g.EdgeCount()
	if out.Meta.RealTradeEdgesUsed {
		out.Meta.DataSources = append(out.Meta.DataSources, SourceUNComtrade)
	}
	out.SimCtx.RealTradeFusionActive = out.Meta.RealTradeEdgesUsed
	out.Dataset = &data.Dataset{Graph: g, Scenarios: in.Base.Scenarios}
	return out
}

func buildSimContext(in Input) simulation.Context {
	ctx := simulation.Context{}
	if in.EventRisk != nil && len(in.EventRisk.Countries) > 0 && eventrisk.IsRealProcessedEventRisk(*in.EventRisk) {
		ctx.EventRiskByCountry = eventrisk.IndexCountryScores(*in.EventRisk)
		if len(ctx.EventRiskByCountry) > 0 {
			ctx.RealEventRiskUsed = true
		}
	}
	if in.CommodityPrices != nil && len(in.CommodityPrices.Records) > 0 {
		scores := commodities.ScoreCommodities(*in.CommodityPrices, commodities.DefaultWeights())
		ctx.CommodityStressByName = map[string]float64{}
		for _, s := range scores {
			name := strings.TrimSpace(s.CommodityName)
			if name == "" {
				continue
			}
			ctx.CommodityStressByName[strings.ToLower(name)] = s.Score
		}
		if len(ctx.CommodityStressByName) > 0 {
			ctx.RealPriceStressUsed = true
		}
	}
	return ctx
}

func finalizeMetaSources(meta *Meta, simCtx simulation.Context) {
	if simCtx.RealEventRiskUsed {
		meta.RealEventRiskUsed = true
		meta.DataSources = appendUnique(meta.DataSources, SourceGDELT)
	}
	if simCtx.RealPriceStressUsed {
		meta.RealPriceStressUsed = true
		meta.DataSources = appendUnique(meta.DataSources, SourceWorldBank)
	}
}

// FinalizeMeta augments fusion metadata with event/price sources after SimCtx is built.
func FinalizeMeta(meta *Meta, simCtx simulation.Context) {
	finalizeMetaSources(meta, simCtx)
}

func appendUnique(items []string, v string) []string {
	for _, s := range items {
		if s == v {
			return items
		}
	}
	return append(items, v)
}

type pendingEdge struct {
	edge  models.Edge
	share float64
}

func fuseTradeEdges(g *graph.Graph, deps *trade.DependencyFile) int {
	seen := map[string]pendingEdge{}
	sourceLabel := strings.TrimSpace(deps.Source)
	if sourceLabel == "" {
		sourceLabel = SourceUNComtrade
	}

	for _, d := range deps.Dependencies {
		exporter := trade.NormalizeCountryName(strings.TrimSpace(d.Exporter))
		if exporter == "" {
			exporter = strings.TrimSpace(d.Exporter)
		}
		importer := trade.NormalizeCountryName(strings.TrimSpace(d.Importer))
		if importer == "" {
			importer = strings.TrimSpace(d.Importer)
		}
		commodity := strings.TrimSpace(d.Commodity)
		if exporter == "" || importer == "" || commodity == "" {
			continue
		}
		share := clampShare(d.Share)
		if share <= 0 {
			continue
		}

		exporterNode := ensureCountry(g, exporter, sourceLabel)
		importerNode := ensureCountry(g, importer, sourceLabel)
		commodityNode := ensureCommodity(g, commodity, sourceLabel)

		exportKey := dedupKey(exporterNode.ID, commodityNode.ID, models.RelRealExports, commodity, importer, sourceLabel)
		exportEdge := models.Edge{
			From: exporterNode.ID, To: commodityNode.ID,
			Type: models.RelRealExports, Weight: share, Concentration: share,
			Commodity: commodity, PropagationEnabled: true,
			AllowedShockTypes: realShockTypes,
			RealData: true, DataSource: sourceLabel,
			TradeValueUSD: d.TradeValueUSD, Year: d.Year, HSCode: d.HSCode, Importer: importer,
		}
		upsertPending(seen, exportKey, exportEdge, share)

		importKey := dedupKey(commodityNode.ID, importerNode.ID, models.RelRealImportDependency, commodity, importer, sourceLabel)
		importEdge := models.Edge{
			From: commodityNode.ID, To: importerNode.ID,
			Type: models.RelRealImportDependency, Weight: share, Concentration: share,
			Commodity: commodity, PropagationEnabled: true,
			AllowedShockTypes: realShockTypes,
			RealData: true, DataSource: sourceLabel,
			TradeValueUSD: d.TradeValueUSD, Year: d.Year, HSCode: d.HSCode, Importer: importer,
		}
		upsertPending(seen, importKey, importEdge, share)
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		g.AddEdge(seen[k].edge)
	}
	return len(keys)
}

func upsertPending(seen map[string]pendingEdge, key string, edge models.Edge, share float64) {
	if prev, ok := seen[key]; ok {
		if share <= prev.share {
			return
		}
	}
	seen[key] = pendingEdge{edge: edge, share: share}
}

func dedupKey(from, to models.NodeID, rel models.EdgeType, commodity, importer, source string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		from, to, rel,
		strings.ToLower(strings.TrimSpace(commodity)),
		strings.ToLower(strings.TrimSpace(importer)),
		strings.ToLower(strings.TrimSpace(source)),
	)
}

func ensureCountry(g *graph.Graph, name, source string) models.Node {
	canon := trade.NormalizeCountryName(name)
	if canon == "" {
		canon = strings.TrimSpace(name)
	}
	if n, ok := g.NodeByName(models.Country, canon); ok {
		return n
	}
	if n, ok := g.FindByName(canon); ok && n.Type == models.Country {
		return n
	}
	// Fall back to raw name for already-present graph nodes with alternate labels.
	if canon != name {
		if n, ok := g.NodeByName(models.Country, name); ok {
			return n
		}
		if n, ok := g.FindByName(name); ok && n.Type == models.Country {
			return n
		}
	}
	n := models.NewNode(models.Country, canon)
	n.Source = source
	n.GeneratedFromRealData = true
	g.AddNode(n)
	return n
}

func ensureCommodity(g *graph.Graph, name, source string) models.Node {
	if n, ok := g.NodeByName(models.Commodity, name); ok {
		return n
	}
	n := models.NewNode(models.Commodity, name)
	n.Source = source
	n.GeneratedFromRealData = true
	g.AddNode(n)
	return n
}

func clampShare(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
