package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/atlasgraph/atlas/internal/config"
)

func runGraph(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas graph <summary|paths|dump> [flags]")
		return 2
	}
	switch args[0] {
	case "summary":
		return graphSummary(args[1:], out, errOut)
	case "paths":
		return graphPaths(args[1:], out, errOut)
	case "dump":
		return graphDump(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown graph subcommand %q (want summary, paths or dump)\n", args[0])
		return 2
	}
}

func graphSummary(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("graph summary", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dataDir := fs.String("data", "", "dataset directory (default: embedded sample)")
	top := fs.Int("top", config.Default().TopN, "how many highest-degree nodes to show")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ds, err := loadDataset(*dataDir)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	renderGraphSummary(out, ds.Graph, *top)
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
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *from == "" || *to == "" {
		fmt.Fprintln(errOut, "error: --from and --to are required")
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

	paths := ds.Graph.PathsBetween(fromNode.ID, toNode.ID, *depth)
	renderGraphPaths(out, ds.Graph, fromNode, toNode, *depth, paths)
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
