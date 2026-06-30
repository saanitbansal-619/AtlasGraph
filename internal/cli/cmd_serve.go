package cli

import (
	"flag"
	"fmt"
	"io"
	"net/http"
)

// runServe starts the AtlasGraph HTTP API. Data sources are loaded lazily per
// request, so the server starts even when some paths are missing — only the
// affected endpoint then returns a helpful JSON error.
func runServe(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(errOut)
	graphData := fs.String("data", "data/generated/trade_graph", "graph dataset dir (entities/dependencies/scenarios); empty uses the embedded sample")
	tradeData := fs.String("trade-data", "data/processed/trade", "ingested trade data directory")
	macroData := fs.String("macro-data", "data/raw/worldbank", "ingested World Bank macro data directory")
	eventData := fs.String("event-data", "data/raw/gdelt", "ingested GDELT event data directory")
	port := fs.Int("port", 8080, "TCP port to listen on")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas serve [--data dir] [--trade-data dir] [--macro-data dir] [--event-data dir] [--port 8080]")
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
		GraphData: *graphData,
		TradeData: *tradeData,
		MacroData: *macroData,
		EventData: *eventData,
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
	fmt.Fprintf(out, "  Event data  : %s\n", cfg.EventData)

	fmt.Fprintln(out, "\n  Endpoints:")
	for _, e := range []string{
		"GET  /health",
		"GET  /api/graph/summary",
		"GET  /api/scenarios",
		"POST /api/shock",
		"GET  /api/trade/summary",
		"GET  /api/trade/dependency?importer=USA&commodity=semiconductors",
		"GET  /api/trade/concentration?importer=USA&commodity=semiconductors",
		"GET  /api/macro/scores",
		"GET  /api/events/risk",
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
