// Package commodityprices ingests commodity price time series from local CSV
// files and normalises them into a flat, typed record set that can be saved to
// disk and scored for recent price stress and volatility.
//
// It is modelled on World Bank "Pink Sheet" style monthly commodity prices:
// one observation per commodity per month. Like the trade importer it depends
// only on the Go standard library, keeps CSV parsing, normalisation, persistence
// and summarisation in small separately-testable pieces, and calls no external
// APIs. The bundled sample file is synthetic demo data, not real prices.
package commodityprices

import "time"

// SourceName identifies the provenance recorded on the saved file when the CSV
// rows do not carry their own source. Individual records keep whatever source
// value their CSV row provided.
const SourceName = "local commodity price CSV"

// PinkSheetSourceName is recorded on PriceFile.Source when ingesting World Bank
// Commodity Markets / Pink Sheet monthly historical XLSX data.
const PinkSheetSourceName = "World Bank Pink Sheet"

// OutputFileName is the canonical file the ingest command writes within its
// output directory and the scoring command reads back.
const OutputFileName = "commodity_prices.json"

// RequiredColumns is the exact set of headers a commodity price CSV must
// provide. The loader validates these are present (order-independent) before
// parsing rows.
var RequiredColumns = []string{
	"date",
	"commodity_code",
	"commodity_name",
	"price_usd",
	"unit",
	"source",
}

// PriceRecord is a single normalised price observation: one commodity in one
// month. Date is normalised to "YYYY-MM".
type PriceRecord struct {
	Date          string  `json:"date"`
	CommodityCode string  `json:"commodity_code"`
	CommodityName string  `json:"commodity_name"`
	PriceUSD      float64 `json:"price_usd"`
	Unit          string  `json:"unit"`
	Source        string  `json:"source"`
}

// PriceFile is the on-disk shape written by the ingest command: a little
// provenance metadata plus the flat record set.
type PriceFile struct {
	Source     string        `json:"source"`
	IngestedAt time.Time     `json:"ingested_at"`
	SourceFile string        `json:"source_file"`
	Records    []PriceRecord `json:"records"`
}
