package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	analyticsdb "github.com/atlasgraph/atlas/internal/db"
)

// runServe starts the AtlasGraph HTTP API. Data sources are loaded lazily per
// request, so the server starts even when some paths are missing — only the
// affected endpoint then returns a helpful JSON error.
func runServe(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(errOut)
	graphData := fs.String("data", "data/generated/trade_graph", "graph dataset dir (entities/dependencies/scenarios); empty uses the embedded sample")
	tradeData := fs.String("trade-data", "data/processed/trade", "ingested trade data directory")
	macroData := fs.String("macro-data", "data/raw/worldbank", "ingested World Bank macro indicators directory (fallback)")
	processedMacroData := fs.String("processed-macro-data", "data/processed/macro", "processed macro scores directory (macro_scores.json)")
	eventData := fs.String("event-data", "data/raw/gdelt", "legacy ingested GDELT event data directory (demo fallback)")
	processedEventData := fs.String("processed-event-data", "data/processed/events", "processed event-risk panel directory (event_risk.json)")
	commodityData := fs.String("commodity-data", "data/processed/commodity_prices", "ingested commodity price data directory")
	port := fs.Int("port", 8080, "TCP port to listen on")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas serve [--data dir] [--trade-data dir] [--processed-macro-data dir] [--macro-data dir] [--event-data dir] [--processed-event-data dir] [--commodity-data dir] [--port 8080]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *port < 1 || *port > 65535 {
		fmt.Fprintf(errOut, "error: --port must be within 1..65535, got %d\n", *port)
		return 2
	}

	cfg := serverConfig{
		GraphData:          *graphData,
		TradeData:          *tradeData,
		MacroData:          *macroData,
		ProcessedMacroData: *processedMacroData,
		EventData:          *eventData,
		ProcessedEventData: *processedEventData,
		CommodityData:      *commodityData,
	}

	var store *analyticsdb.DB
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		fmt.Fprintln(out, "Postgres disabled; using file-backed analytics only")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var err error
		store, err = analyticsdb.Connect(databaseURL)
		if err != nil {
			fmt.Fprintf(errOut, "error: PostgreSQL startup: %v\n", err)
			return 1
		}
		if err := store.Ping(ctx); err != nil {
			store.Close()
			fmt.Fprintf(errOut, "error: PostgreSQL startup: %v\n", err)
			return 1
		}
		defer store.Close()
		cfg.Database = store
		fmt.Fprintln(out, "Postgres enabled; analytics endpoints and scenario persistence active")
	}
	handler := newAPIServer(cfg)
	addr := fmt.Sprintf(":%d", *port)

	renderServeBanner(out, *port, cfg)

	srv := &http.Server{Addr: addr, Handler: handler}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	return 0
}

// renderServeBanner prints the startup summary: the port, every data path, and
// the available endpoints.
func renderServeBanner(out io.Writer, port int, cfg serverConfig) {
	section(out, "ATLASGRAPH API SERVER")
	fmt.Fprintf(out, "  Port        : %d\n", port)
	fmt.Fprintf(out, "  Graph data  : %s\n", pathOrEmbedded(cfg.GraphData))
	fmt.Fprintf(out, "  Trade data  : %s\n", cfg.TradeData)
	fmt.Fprintf(out, "  Macro data  : %s\n", cfg.MacroData)
	fmt.Fprintf(out, "  Processed macro data: %s\n", cfg.ProcessedMacroData)
	fmt.Fprintf(out, "  Event data  : %s\n", cfg.EventData)
	fmt.Fprintf(out, "  Processed event data: %s\n", cfg.ProcessedEventData)
	fmt.Fprintf(out, "  Commodity data: %s\n", cfg.CommodityData)

	fmt.Fprintln(out, "\n  Endpoints:")
	for _, e := range []string{
		"GET  /health",
		"GET  /api/graph/summary",
		"GET  /api/graph/entities",
		"GET  /api/scenarios",
		"GET  /api/shock/options",
		"GET  /api/shock/valid-options",
		"POST /api/shock",
		"POST /api/scenarios/compare",
		"GET  /api/trade/summary",
		"GET  /api/trade/options",
		"GET  /api/trade/dependency?importer=USA&commodity=semiconductors",
		"GET  /api/trade/concentration?importer=USA&commodity=semiconductors",
		"GET  /api/macro/scores",
		"GET  /api/events/risk",
		"GET  /api/commodities/stress",
		"GET  /api/commodities/history",
		"GET  /api/commodities/history?commodity=crude%20oil",
		"GET  /api/fragility/countries",
		"GET  /api/fragility/commodities",
		"GET  /api/fragility/summary",
		"POST /api/reports/scenario",
		"GET  /api/db/health",
		"GET  /api/db/summary",
		"GET  /api/db/trade/top-suppliers?importer=USA&commodity=semiconductors",
		"GET  /api/db/scenarios/recent",
	} {
		fmt.Fprintf(out, "    %s\n", e)
	}

	fmt.Fprintf(out, "\n  Listening on http://localhost:%d\n", port)
}

func pathOrEmbedded(p string) string {
	if p == "" {
		return "(embedded sample)"
	}
	return p
}
