package tradegraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Standard generated dataset file names (mirroring internal/data's loader).
const (
	EntitiesFileName     = "entities.json"
	DependenciesFileName = "dependencies.json"
	ScenariosFileName    = "scenarios.json"
)

// Write persists a generated Result to dir as the three dataset files the graph
// loader expects, creating dir if needed.
func Write(dir string, r Result) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("output directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating output directory %q: %w", dir, err)
	}
	files := []struct {
		name string
		v    any
	}{
		{EntitiesFileName, r.Entities},
		{DependenciesFileName, r.Dependencies},
		{ScenariosFileName, r.Scenarios},
	}
	for _, f := range files {
		if err := writeJSONFile(filepath.Join(dir, f.name), f.v); err != nil {
			return err
		}
	}
	return nil
}

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	return nil
}
