package trade

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Save writes a TradeFile to dir/OutputFileName, creating dir if needed, and
// returns the full path written.
func Save(dir string, file TradeFile) (string, error) {
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

// Load reads the TradeFile from dir/OutputFileName.
func Load(dir string) (TradeFile, error) {
	path := filepath.Join(dir, OutputFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return TradeFile{}, fmt.Errorf("reading %q: %w", path, err)
	}
	var file TradeFile
	if err := json.Unmarshal(b, &file); err != nil {
		return TradeFile{}, fmt.Errorf("parsing %q: %w", path, err)
	}
	return file, nil
}

// SortRecords orders records deterministically for stable output files and
// diffs: by year, then commodity, then importer, then descending trade value
// (largest supplier first), then exporter for a fully stable tie-break.
func SortRecords(records []TradeFlowRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if a.Year != b.Year {
			return a.Year < b.Year
		}
		if a.CommodityName != b.CommodityName {
			return a.CommodityName < b.CommodityName
		}
		if a.ImporterCode != b.ImporterCode {
			return a.ImporterCode < b.ImporterCode
		}
		if a.TradeValueUSD != b.TradeValueUSD {
			return a.TradeValueUSD > b.TradeValueUSD
		}
		return a.ExporterCode < b.ExporterCode
	})
}
