package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/atlasgraph/atlas/internal/data"
	analyticsdb "github.com/atlasgraph/atlas/internal/db"
	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
)

func runDB(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas db migrate [--migrations migrations] | atlas db load [flags]")
		return 2
	}
	switch args[0] {
	case "migrate":
		return runDBMigrate(args[1:], out, errOut)
	case "load":
		return runDBLoad(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown db command %q\n", args[0])
		return 2
	}
}

func openAnalyticsDB(ctx context.Context) (*analyticsdb.DB, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	store, err := analyticsdb.Connect(databaseURL)
	if err != nil {
		return nil, err
	}
	if err := store.Ping(ctx); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}

func runDBMigrate(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("db migrate", flag.ContinueOnError)
	fs.SetOutput(errOut)
	migrations := fs.String("migrations", "migrations", "directory containing PostgreSQL migration SQL")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	store, err := openAnalyticsDB(ctx)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	defer store.Close()
	if err := store.Migrate(ctx, *migrations); err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, "PostgreSQL migrations applied successfully.")
	return 0
}

func runDBLoad(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("db load", flag.ContinueOnError)
	fs.SetOutput(errOut)
	tradeData := fs.String("trade-data", "data/processed/trade", "processed trade data directory")
	macroData := fs.String("macro-data", "data/processed/macro", "processed macro data directory")
	eventData := fs.String("event-data", "data/processed/events", "processed event-risk data directory")
	commodityData := fs.String("commodity-data", "data/processed/commodity_prices", "processed commodity price data directory")
	graphData := fs.String("graph-data", "data/strategic_global", "graph dataset directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	batch, err := buildDBLoadBatch(*tradeData, *macroData, *eventData, *commodityData, *graphData)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	store, err := openAnalyticsDB(ctx)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	defer store.Close()
	counts, err := store.ReplaceAnalytics(ctx, batch)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	fmt.Fprintln(out, "PostgreSQL analytics load completed.")
	fmt.Fprintf(out, "  trade_flows        : %d\n", counts.TradeFlows)
	fmt.Fprintf(out, "  event_risk_signals : %d\n", counts.EventRiskSignals)
	fmt.Fprintf(out, "  macro_scores       : %d\n", counts.MacroScores)
	fmt.Fprintf(out, "  commodity_prices   : %d\n", counts.CommodityPrices)
	fmt.Fprintf(out, "  dependency_edges   : %d\n", counts.DependencyEdges)
	fmt.Fprintf(out, "  data_quality_checks: %d\n", counts.QualityChecks)
	return 0
}

func buildDBLoadBatch(tradeDir, macroDir, eventDir, commodityDir, graphDir string) (analyticsdb.LoadBatch, error) {
	resolvedTrade, err := trade.ResolveTrade(tradeDir)
	if err != nil {
		return analyticsdb.LoadBatch{}, fmt.Errorf("load trade data: %w", err)
	}
	macroFile, err := macro.LoadProcessed(macroDir)
	if err != nil {
		return analyticsdb.LoadBatch{}, fmt.Errorf("load macro data: %w", err)
	}
	eventFile, err := eventrisk.Load(eventDir)
	if err != nil {
		return analyticsdb.LoadBatch{}, fmt.Errorf("load event data: %w", err)
	}
	priceFile, err := commodityprices.Load(commodityDir)
	if err != nil {
		return analyticsdb.LoadBatch{}, fmt.Errorf("load commodity data: %w", err)
	}
	dataset, err := data.Load(graphDir)
	if err != nil {
		return analyticsdb.LoadBatch{}, fmt.Errorf("load graph data: %w", err)
	}

	batch := analyticsdb.LoadBatch{}
	missingImporter, missingExporter, missingCommodity := 0, 0, 0
	appendTrade := func(r trade.TradeFlowRecord, flow string) {
		source := r.Source
		if source == "" {
			source = resolvedTrade.Source
		}
		flow = strings.TrimSpace(flow)
		if flow == "" {
			flow = "import"
		}
		batch.TradeFlows = append(batch.TradeFlows, analyticsdb.TradeFlow{
			Year: r.Year, ImporterName: r.ImporterName, ImporterCode: r.ImporterCode,
			ExporterName: r.ExporterName, ExporterCode: r.ExporterCode,
			Commodity: r.CommodityName, HSCode: r.CommodityCode,
			TradeValueUSD: r.TradeValueUSD, TradeFlow: flow, Source: source,
		})
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
			appendTrade(trade.TradeDependencyToRecord(dependency), dependency.Flow)
		}
	} else {
		for _, record := range resolvedTrade.File.Records {
			appendTrade(record, "import")
		}
	}

	for _, r := range eventFile.Countries {
		eventType := ""
		if len(r.TopEventTypes) > 0 {
			eventType = r.TopEventTypes[0]
		}
		source := r.Source
		if source == "" {
			source = eventFile.Source
		}
		batch.EventRiskSignals = append(batch.EventRiskSignals, analyticsdb.EventRiskSignal{
			CountryName: r.Country, CountryCode: r.CountryCode, EventType: eventType,
			EventCount: r.EventCount, RiskScore: r.EventRiskScore,
			RiskLevel: r.RiskLevel, Source: source,
		})
	}

	missingMacro := 0
	for _, r := range macroFile.Scores {
		source := r.Source
		if source == "" {
			source = macroFile.Source
		}
		batch.MacroScores = append(batch.MacroScores, analyticsdb.MacroScore{
			CountryName: r.CountryName, CountryCode: r.CountryCode,
			Score: r.MacroExposureScore, RiskLevel: macro.RiskLevel(r.MacroExposureScore),
			Source: source,
		})
		hasData := false
		for _, component := range r.Components {
			hasData = hasData || component.Available
		}
		if !hasData {
			missingMacro++
		}
	}

	priceCommodities := map[string]bool{}
	for _, r := range priceFile.Records {
		source := r.Source
		if source == "" {
			source = priceFile.Source
		}
		batch.CommodityPrices = append(batch.CommodityPrices, analyticsdb.CommodityPrice{
			Commodity: r.CommodityName, DateMonth: r.Date, Price: r.PriceUSD,
			Unit: r.Unit, Source: source,
		})
		priceCommodities[strings.ToLower(strings.TrimSpace(r.CommodityName))] = true
	}

	missingPriceSeries := 0
	for _, node := range dataset.Graph.Nodes() {
		if string(node.Type) == "commodity" && !priceCommodities[strings.ToLower(strings.TrimSpace(node.Name))] {
			missingPriceSeries++
		}
		for _, edge := range dataset.Graph.OutEdges(node.ID) {
			target, ok := dataset.Graph.Node(edge.To)
			if !ok {
				continue
			}
			provenance := edge.DataSource
			if provenance == "" {
				provenance = "Baseline dependency graph"
			}
			batch.DependencyEdges = append(batch.DependencyEdges, analyticsdb.DependencyEdge{
				SourceNode: node.Name, TargetNode: target.Name,
				SourceType: string(node.Type), TargetType: string(target.Type),
				RelationshipType: string(edge.Type), Weight: edge.Weight,
				DataProvenance: provenance,
			})
		}
	}

	batch.QualityChecks = []analyticsdb.DataQualityCheck{
		qualityCheck("total trade rows loaded", len(batch.TradeFlows), false, "UN Comtrade"),
		qualityCheck("missing importer codes", missingImporter, true, "UN Comtrade"),
		qualityCheck("missing exporter codes", missingExporter, true, "UN Comtrade"),
		qualityCheck("missing commodity names", missingCommodity, true, "UN Comtrade"),
		qualityCheck("missing macro scores", missingMacro, true, "World Bank Macro"),
		qualityCheck("missing commodity price series", missingPriceSeries, true, "World Bank Pink Sheet"),
		qualityCheck("event risk rows loaded", len(batch.EventRiskSignals), false, "GDELT"),
		qualityCheck("dependency graph edges loaded", len(batch.DependencyEdges), false, "Baseline dependency graph"),
	}
	return batch, nil
}

func qualityCheck(name string, value int, zeroIsGood bool, source string) analyticsdb.DataQualityCheck {
	status := "ok"
	if (zeroIsGood && value > 0) || (!zeroIsGood && value == 0) {
		status = "warning"
	}
	return analyticsdb.DataQualityCheck{
		CheckName: name, Status: status, MetricValue: float64(value),
		Details: map[string]any{"count": value}, Source: source,
	}
}
