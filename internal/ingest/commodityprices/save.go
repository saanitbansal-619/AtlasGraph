package commodityprices

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Save writes a PriceFile to dir/OutputFileName, creating dir if needed, and
// returns the full path written.
func Save(dir string, file PriceFile) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("output directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory %q: %w", dir, err)
	}
	path := filepath.Join(dir, OutputFileName)
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding records: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("writing %q: %w", path, err)
	}
	return path, nil
}

// Load reads the PriceFile from dir/OutputFileName.
func Load(dir string) (PriceFile, error) {
	path := filepath.Join(dir, OutputFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return PriceFile{}, fmt.Errorf("reading %q: %w", path, err)
	}
	var file PriceFile
	if err := json.Unmarshal(b, &file); err != nil {
		return PriceFile{}, fmt.Errorf("parsing %q: %w", path, err)
	}
	return file, nil
}

// SortRecords orders records deterministically for stable output files and
// diffs: by commodity code, then ascending date.
func SortRecords(records []PriceRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if a.CommodityCode != b.CommodityCode {
			return a.CommodityCode < b.CommodityCode
		}
		return a.Date < b.Date
	})
}
