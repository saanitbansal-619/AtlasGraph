package trade

import "time"

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
}
