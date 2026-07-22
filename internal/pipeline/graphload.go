package pipeline

import (
	"strings"

	"github.com/atlasgraph/atlas/internal/data"
)

func loadGraphDataset(graphDir string) (*data.Dataset, error) {
	if strings.TrimSpace(graphDir) == "" {
		return data.Default()
	}
	return data.Load(graphDir)
}

func graphEdgeCount(dataset *data.Dataset) int {
	if dataset == nil {
		return 0
	}
	count := 0
	for _, node := range dataset.Graph.Nodes() {
		count += len(dataset.Graph.OutEdges(node.ID))
	}
	return count
}
