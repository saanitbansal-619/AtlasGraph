package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/simulation"
	"github.com/atlasgraph/atlas/internal/tradegraph"
)

func runGraph(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas graph <summary|paths|dump|build-trade> [flags]")
		return 2
	}
	switch args[0] {
	case "summary":
		return graphSummary(args[1:], out, errOut)
	case "paths":
		return graphPaths(args[1:], out, errOut)
	case "dump":
		return graphDump(args[1:], out, errOut)
	case "build-trade":
		return graphBuildTrade(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown graph subcommand %q (want summary, paths, dump or build-trade)\n", args[0])
		return 2
	}
}

// graphBuildTrade converts an ingested trade panel into an AtlasGraph dataset
// (entities/dependencies/scenarios) usable by the standard graph/shock commands.
func graphBuildTrade(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("graph build-trade", flag.ContinueOnError)
	fs.SetOutput(errOut)
	tradeData := fs.String("trade-data", "data/processed/trade", "directory holding ingested trade data")
	outDir := fs.String("out", "data/generated/trade_graph", "directory to write the generated graph to")
	fs.Usage = func() {
		fmt.Fprintln(errOut, "Usage: atlas graph build-trade [--trade-data dir] [--out dir]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	file, err := trade.Load(*tradeData)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		fmt.Fprintln(errOut, "hint: run `atlas ingest trade --file <csv>` first")
		return 1
	}
	if len(file.Records) == 0 {
		fmt.Fprintf(errOut, "error: no trade records found in %s\n", *tradeData)
		return 1
	}

	result := tradegraph.Build(file)
	if err := tradegraph.Write(*outDir, result); err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	renderTradeGraphBuild(out, *tradeData, *outDir, result)
	return 0
}

func graphSummary(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("graph summary", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dataDir := fs.String("data", "", "dataset directory (default: embedded sample)")
	tradeData := fs.String("trade-data", "", "processed trade directory for graph fusion (optional)")
	eventData := fs.String("event-data", "", "processed event-risk directory for graph fusion (optional)")
	commodityData := fs.String("commodity-data", "", "processed commodity price directory for graph fusion (optional)")
	top := fs.Int("top", config.Default().TopN, "how many highest-degree nodes to show")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	fused, err := loadFusedDataset(fusionConfig{
		GraphData:          *dataDir,
		TradeData:          *tradeData,
		ProcessedEventData: *eventData,
		CommodityData:      *commodityData,
	})
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	renderGraphSummary(out, fused.Dataset.Graph, *top)
	if fused.Meta.FusionEnabled || fused.Meta.RealEventRiskUsed || fused.Meta.RealPriceStressUsed {
		renderGraphFusionSummary(out, fused.Meta)
	}
	return 0
}

func graphPaths(args []string, out, errOut io.Writer) int {
	cfg := config.Default()
	fs := flag.NewFlagSet("graph paths", flag.ContinueOnError)
	fs.SetOutput(errOut)
	from := fs.String("from", "", "source entity name")
	to := fs.String("to", "", "target entity name")
	depth := fs.Int("depth", cfg.MaxPathDepth, "maximum path length in edges")
	dataDir := fs.String("data", "", "dataset directory (default: embedded sample)")
	commodity := fs.String("commodity", "", "restrict paths to a commodity's branches (requires --shock-type)")
	shockType := fs.String("shock-type", "", "apply a shock type's propagation rules: export_collapse, supply_cut, price_spike or route_disruption (requires --commodity)")
	explain := fs.Bool("explain", false, "explain the shock/commodity path filtering (requires --commodity and --shock-type)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *from == "" || *to == "" {
		fmt.Fprintln(errOut, "error: --from and --to are required")
		return 2
	}

	// Shock-aware filtering is opt-in and needs both the commodity and the
	// shock type so the rule engine has a complete picture; --explain only
	// makes sense once filtering is active.
	filtered := *commodity != "" || *shockType != ""
	if filtered && (*commodity == "" || *shockType == "") {
		fmt.Fprintln(errOut, "error: --commodity and --shock-type must be used together")
		return 2
	}
	if *explain && !filtered {
		fmt.Fprintln(errOut, "error: --explain requires --commodity and --shock-type")
		return 2
	}

	ds, err := loadDataset(*dataDir)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	fromNode, ok := ds.Graph.FindByName(*from)
	if !ok {
		fmt.Fprintf(errOut, "error: unknown entity %q\n", *from)
		return 1
	}
	toNode, ok := ds.Graph.FindByName(*to)
	if !ok {
		fmt.Fprintf(errOut, "error: unknown entity %q\n", *to)
		return 1
	}

	if !filtered {
		paths := ds.Graph.PathsBetween(fromNode.ID, toNode.ID, *depth)
		renderGraphPaths(out, ds.Graph, fromNode, toNode, *depth, paths)
		return 0
	}

	profile, ok := simulation.ProfileFor(*shockType)
	if !ok {
		fmt.Fprintf(errOut, "error: unknown shock type %q (valid: %v)\n", *shockType, simulation.ProfileTypes())
		return 1
	}

	// Reuse the exact rule logic the shock engine applies, so a commodity- and
	// shock-aware path query and a shock simulation agree on which branches are
	// economically meaningful: only allowed relationships are followed, and
	// cross-commodity branches are pruned unless the profile permits them.
	var blocked []simulation.BlockedEdge
	blockedSeen := map[string]bool{}
	allow := func(e models.Edge) bool {
		return simulation.Evaluate(profile, e, *commodity).Allowed
	}
	onBlock := func(e models.Edge) {
		key := string(e.From) + "|" + string(e.To) + "|" + string(e.Type)
		if blockedSeen[key] {
			return
		}
		blockedSeen[key] = true
		dec := simulation.Evaluate(profile, e, *commodity)
		ef, _ := ds.Graph.Node(e.From)
		et, _ := ds.Graph.Node(e.To)
		blocked = append(blocked, simulation.BlockedEdge{
			From: ef, To: et, Relationship: string(e.Type),
			Commodity: e.Commodity, Reason: dec.Reason, CrossCommodity: dec.CrossCommodity,
		})
	}

	paths := ds.Graph.PathsBetweenFunc(fromNode.ID, toNode.ID, *depth, allow, onBlock)

	// How many structurally-possible paths the rules removed, for --explain.
	blockedPaths := len(ds.Graph.PathsBetweenFunc(fromNode.ID, toNode.ID, *depth, nil, nil)) - len(paths)
	if blockedPaths < 0 {
		blockedPaths = 0
	}

	renderGraphPathsFiltered(out, fromNode, toNode, *depth, profile, *commodity, paths, blocked, blockedPaths, *explain)
	return 0
}

func graphDump(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("graph dump", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dataDir := fs.String("data", "", "dataset directory (default: embedded sample)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ds, err := loadDataset(*dataDir)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	g := ds.Graph
	fmt.Fprintf(out, "AtlasGraph dependency graph: %d nodes, %d edges\n\n", g.NodeCount(), g.EdgeCount())
	tw := newTable(out)
	fmt.Fprintln(tw, "FROM\tRELATION\tTO\tWEIGHT\tCONCENTRATION")
	for _, n := range g.Nodes() {
		for _, e := range g.OutEdges(n.ID) {
			toN, _ := g.Node(e.To)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%.2f\t%.2f\n", n.Name, e.Type, toN.Name, e.Weight, e.Concentration)
		}
	}
	flush(tw)
	return 0
}
