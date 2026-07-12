package trade

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SaveDependencies writes a DependencyFile to dir/DependenciesOutputFileName.
func SaveDependencies(dir string, file DependencyFile) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("output directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory %q: %w", dir, err)
	}
	path := filepath.Join(dir, DependenciesOutputFileName)
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding dependencies: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("writing %q: %w", path, err)
	}
	return path, nil
}

// LoadDependencies reads dir/DependenciesOutputFileName.
func LoadDependencies(dir string) (DependencyFile, error) {
	path := filepath.Join(dir, DependenciesOutputFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return DependencyFile{}, fmt.Errorf("reading %q: %w", path, err)
	}
	var file DependencyFile
	if err := json.Unmarshal(b, &file); err != nil {
		return DependencyFile{}, fmt.Errorf("parsing %q: %w", path, err)
	}
	return file, nil
}

// ResolvedTrade is the unified trade payload used by API and fragility scoring.
type ResolvedTrade struct {
	Source         string
	RealTradeData  bool
	File           TradeFile
	DependencyFile *DependencyFile
}

// ResolveTrade prefers processed trade_dependencies.json, falling back to trade_flows.json.
func ResolveTrade(dir string) (ResolvedTrade, error) {
	if dir != "" {
		if deps, err := LoadDependencies(dir); err == nil && len(deps.Dependencies) > 0 {
			return ResolvedTrade{
				Source:         deps.Source,
				RealTradeData:  strings.EqualFold(deps.Source, ComtradeRealSourceName),
				File:           DependenciesToTradeFile(deps),
				DependencyFile: &deps,
			}, nil
		}
	}
	file, err := Load(dir)
	if err != nil {
		return ResolvedTrade{}, err
	}
	source := demoTradeSourceLabel(file.Source)
	return ResolvedTrade{
		Source:        source,
		RealTradeData: false,
		File:          file,
	}, nil
}

func demoTradeSourceLabel(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if strings.Contains(lower, "comtrade") {
		return ComtradeRealSourceName
	}
	if strings.Contains(lower, "demo") || strings.Contains(lower, "sample") || strings.Contains(lower, "local") {
		return "demo"
	}
	return "demo"
}

// DependenciesToTradeFile converts dependency rows into TradeFlowRecords for analysis.
func DependenciesToTradeFile(df DependencyFile) TradeFile {
	records := make([]TradeFlowRecord, 0, len(df.Dependencies))
	for _, d := range df.Dependencies {
		records = append(records, TradeDependencyToRecord(d))
	}
	SortRecords(records)
	return TradeFile{
		Source:     df.Source,
		IngestedAt: df.GeneratedAt,
		Records:    records,
	}
}

func TradeDependencyToRecord(d TradeDependency) TradeFlowRecord {
	importerName := strings.TrimSpace(d.Importer)
	exporterName := strings.TrimSpace(d.Exporter)
	return TradeFlowRecord{
		Year:          d.Year,
		ExporterCode:  CountryCodeForName(exporterName),
		ExporterName:  exporterName,
		ImporterCode:  CountryCodeForName(importerName),
		ImporterName:  importerName,
		CommodityCode: d.HSCode,
		CommodityName: d.Commodity,
		TradeValueUSD: d.TradeValueUSD,
		Quantity:      d.Quantity,
		Unit:          d.QuantityUnit,
		Source:        d.Source,
	}
}
