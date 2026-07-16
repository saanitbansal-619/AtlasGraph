package trade

import (
	"strings"
	"time"
)

// DependenciesOutputFileName is the canonical processed trade dependency file.
const DependenciesOutputFileName = "trade_dependencies.json"

// TradeDependency is one importer-exporter-commodity dependency row with share.
type TradeDependency struct {
	Importer      string  `json:"importer"`
	Exporter      string  `json:"exporter"`
	Commodity     string  `json:"commodity"`
	HSCode        string  `json:"hs_code"`
	Year          int     `json:"year"`
	TradeValueUSD float64 `json:"trade_value_usd"`
	NetWeightKg   float64 `json:"net_weight_kg,omitempty"`
	Quantity      float64 `json:"quantity,omitempty"`
	QuantityUnit  string  `json:"quantity_unit,omitempty"`
	Share         float64 `json:"share"`
	Source        string  `json:"source"`
	// Flow is "import" or "export" from the Comtrade reporter perspective.
	// Import rows: reporter is the importer. Export rows: reporter is the exporter
	// (partner appears as importer). Country trade concentration uses import only.
	Flow string `json:"flow,omitempty"`
}

const (
	FlowImport = "import"
	FlowExport = "export"
)

// IsImportFlow reports whether a dependency row is reporter-side import data.
// Legacy rows with an empty Flow are treated as import when the file has no
// flow tags; callers that know the file is tagged should filter empty Flow out.
func IsImportFlow(d TradeDependency) bool {
	f := strings.ToLower(strings.TrimSpace(d.Flow))
	if f == "" {
		return true
	}
	return f == FlowImport || f == "m" || strings.Contains(f, "import")
}

// IsExportFlow reports whether a dependency row came from an export reporter flow.
func IsExportFlow(d TradeDependency) bool {
	f := strings.ToLower(strings.TrimSpace(d.Flow))
	return f == FlowExport || f == "x" || strings.Contains(f, "export")
}

// DependencyFileHasFlowTags is true when any row records an explicit flow direction.
func DependencyFileHasFlowTags(df DependencyFile) bool {
	for _, d := range df.Dependencies {
		if strings.TrimSpace(d.Flow) != "" {
			return true
		}
	}
	return false
}

// UseForImporterConcentration is true for rows that should drive country
// trade_concentration_score (reporter-side imports only).
func UseForImporterConcentration(d TradeDependency, fileHasFlowTags bool) bool {
	if fileHasFlowTags {
		f := strings.ToLower(strings.TrimSpace(d.Flow))
		return f == FlowImport || f == "m" || strings.Contains(f, "import")
	}
	return IsImportFlow(d)
}

// DependencyFile is the processed trade dependency panel written to disk.
type DependencyFile struct {
	Source       string            `json:"source"`
	Year         int               `json:"year"`
	GeneratedAt  time.Time         `json:"generated_at"`
	Dependencies []TradeDependency `json:"dependencies"`
}

// ComtradeV2LoadResult captures ingest diagnostics for UN Comtrade CSV files.
type ComtradeV2LoadResult struct {
	FilesProcessed       int
	FileNames            []string
	RawRows              int
	ValidRows            int
	SkippedAggregateRows int
	SkippedUnmapped      int
	CommoditiesMapped    map[string]struct{}
	Importers            map[string]struct{}
	Exporters            map[string]struct{}
	YearMin              int
	YearMax              int
	Skipped              []SkippedRow
}

// aggregatedFlow is an intermediate annual row before share calculation.
type aggregatedFlow struct {
	importer      string
	exporter      string
	commodity     string
	hsCode        string
	year          int
	tradeValueUSD float64
	netWeightKg   float64
	quantity      float64
	quantityUnit  string
	flow          string // FlowImport or FlowExport
}
