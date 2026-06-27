// Package trade ingests country-to-country commodity trade flows from local
// CSV files and normalises them into a flat, typed record set that can be saved
// to disk and analysed for supplier dependency and concentration.
//
// This is the foundation for feeding real trade data (e.g. UN Comtrade exports)
// into the AtlasGraph shock-propagation engine. It deliberately depends only on
// the Go standard library and keeps CSV parsing, normalisation, persistence and
// analysis in small, separately testable pieces. No external APIs are called.
package trade

import "time"

// SourceName identifies the provenance recorded on every normalised record.
const SourceName = "local trade CSV"

// OutputFileName is the canonical file the ingest command writes within its
// output directory and the trade commands read back.
const OutputFileName = "trade_flows.json"

// RequiredColumns is the exact set of headers a trade CSV must provide. The
// loader validates these are present (order-independent) before parsing rows.
var RequiredColumns = []string{
	"year",
	"exporter_code",
	"exporter_name",
	"importer_code",
	"importer_name",
	"commodity_code",
	"commodity_name",
	"trade_value_usd",
	"quantity",
	"unit",
}

// TradeFlowRecord is a single normalised trade observation: one exporter selling
// one commodity to one importer in one year.
type TradeFlowRecord struct {
	Year          int       `json:"year"`
	ExporterCode  string    `json:"exporter_code"`
	ExporterName  string    `json:"exporter_name"`
	ImporterCode  string    `json:"importer_code"`
	ImporterName  string    `json:"importer_name"`
	CommodityCode string    `json:"commodity_code"`
	CommodityName string    `json:"commodity_name"`
	TradeValueUSD float64   `json:"trade_value_usd"`
	Quantity      float64   `json:"quantity"`
	Unit          string    `json:"unit"`
	Source        string    `json:"source"`
	IngestedAt    time.Time `json:"ingested_at"`
}

// TradeFile is the on-disk shape written by the ingest command: a little
// provenance metadata plus the flat record set.
type TradeFile struct {
	Source     string            `json:"source"`
	IngestedAt time.Time         `json:"ingested_at"`
	SourceFile string            `json:"source_file"`
	Records    []TradeFlowRecord `json:"records"`
}
