package cli

import (
	"flag"
	"fmt"
	"io"
	"sort"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/scoring"
)

// rankedEntity is a node paired with its baseline fragility, for leaderboards.
type rankedEntity struct {
	Node      models.Node
	Fragility float64
}

func runRisk(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: atlas risk <leaderboard> [flags]")
		return 2
	}
	switch args[0] {
	case "leaderboard":
		return riskLeaderboard(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown risk subcommand %q (want leaderboard)\n", args[0])
		return 2
	}
}

func riskLeaderboard(args []string, out, errOut io.Writer) int {
	cfg := config.Default()
	fs := flag.NewFlagSet("risk leaderboard", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dataDir := fs.String("data", "", "dataset directory (default: embedded sample)")
	top := fs.Int("top", cfg.TopN, "how many entities to show per category")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ds, err := loadDataset(*dataDir)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	board := func(t models.NodeType) []rankedEntity {
		return baselineLeaderboard(ds.Graph, cfg, t, *top)
	}
	renderRiskLeaderboard(out, board(models.Country), board(models.Commodity), board(models.Sector))
	return 0
}

// baselineLeaderboard ranks all nodes of a type by their baseline (no-shock)
// fragility and returns the top n.
func baselineLeaderboard(g *graph.Graph, cfg config.Config, t models.NodeType, n int) []rankedEntity {
	var ranked []rankedEntity
	for _, node := range g.Nodes() {
		if node.Type != t {
			continue
		}
		ranked = append(ranked, rankedEntity{
			Node:      node,
			Fragility: scoring.NodeFragility(g, node.ID, 0, cfg),
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Fragility != ranked[j].Fragility {
			return ranked[i].Fragility > ranked[j].Fragility
		}
		return ranked[i].Node.Name < ranked[j].Node.Name
	})
	if n > 0 && len(ranked) > n {
		ranked = ranked[:n]
	}
	return ranked
}
