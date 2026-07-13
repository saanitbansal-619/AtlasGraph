package trade

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// ComtradeSourceName identifies the provenance recorded on records normalised
// from a downloaded UN Comtrade-style CSV export.
const ComtradeSourceName = "UN Comtrade-style CSV"

// ComtradeRequiredColumns is the set of headers a UN Comtrade-style CSV export
// must provide. Matching is order-independent and case-insensitive. These are
// the columns Comtrade's bulk/preview downloads expose; we normalise their
// reporter/partner + flow direction into AtlasGraph's exporter/importer model.
var ComtradeRequiredColumns = []string{
	"refYear",
	"flowDesc",
	"reporterISO",
	"reporterDesc",
	"partnerISO",
	"partnerDesc",
	"cmdCode",
	"cmdDesc",
	"primaryValue",
	"qty",
	"qtyUnitAbbr",
}

// ComtradeLoadResult is the outcome of parsing a Comtrade-style CSV: the
// normalised records plus counts that let the ingest command report exactly
// what happened, including how many import vs export flows were kept.
type ComtradeLoadResult struct {
	Records       []TradeFlowRecord
	TotalRows     int          // data rows seen (excludes the header)
	Skipped       []SkippedRow // rows dropped (with reasons): missing fields, unsupported flow
	FlowsImported int          // valid rows whose flowDesc was an import
	FlowsExported int          // valid rows whose flowDesc was an export
}

// ValidRows is the number of rows that normalised successfully.
func (r ComtradeLoadResult) ValidRows() int { return len(r.Records) }

// LoadComtradeFile opens a Comtrade-style CSV path and parses it.
func LoadComtradeFile(path string) (ComtradeLoadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return ComtradeLoadResult{}, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()
	return ParseComtradeCSV(f, time.Now().UTC())
}

// ParseComtradeCSV reads a UN Comtrade-style CSV from r and normalises it into
// the same TradeFlowRecord model the rest of the pipeline uses. It validates
// that every required column is present, then parses each data row, collecting
// malformed or unsupported rows (with reasons) rather than aborting the whole
// file. An error is returned only for problems that make the file unusable
// (unreadable, empty, or missing required columns).
func ParseComtradeCSV(r io.Reader, ingestedAt time.Time) (ComtradeLoadResult, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // tolerate ragged rows; we validate per row
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err == io.EOF {
		return ComtradeLoadResult{}, fmt.Errorf("comtrade CSV is empty")
	}
	if err != nil {
		return ComtradeLoadResult{}, fmt.Errorf("reading header: %w", err)
	}

	index, err := mapComtradeColumns(header)
	if err != nil {
		return ComtradeLoadResult{}, err
	}

	var res ComtradeLoadResult
	line := 1 // header consumed on line 1; data rows start at line 2
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			return ComtradeLoadResult{}, fmt.Errorf("reading line %d: %w", line, err)
		}
		if isBlankRow(row) {
			continue
		}
		res.TotalRows++

		rec, isImport, reason := normalizeComtradeRow(row, index, ingestedAt)
		if reason != "" {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: reason})
			continue
		}
		res.Records = append(res.Records, rec)
		if isImport {
			res.FlowsImported++
		} else {
			res.FlowsExported++
		}
	}
	return res, nil
}

// mapComtradeColumns builds a header-name -> column-index map and verifies that
// every ComtradeRequiredColumns entry is present. Header names are matched
// case-insensitively (Comtrade headers are camelCase).
func mapComtradeColumns(header []string) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for i, h := range header {
		index[strings.ToLower(strings.TrimSpace(h))] = i
	}
	var missing []string
	for _, col := range ComtradeRequiredColumns {
		if _, ok := index[strings.ToLower(col)]; !ok {
			missing = append(missing, col)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("comtrade CSV is missing required column(s): %s", strings.Join(missing, ", "))
	}
	return index, nil
}

// normalizeComtradeRow turns one raw Comtrade row into a TradeFlowRecord,
// resolving the reporter/partner pair into exporter/importer according to the
// flow direction:
//
//   - Import: importer = reporter, exporter = partner
//   - Export: exporter = reporter, importer = partner
//
// It returns isImport for accounting and a non-empty reason string when the row
// is unsupported (a re-export/other flow) or malformed and should be skipped.
func normalizeComtradeRow(row []string, index map[string]int, ingestedAt time.Time) (rec TradeFlowRecord, isImport bool, reason string) {
	get := func(col string) string {
		i, ok := index[strings.ToLower(col)]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	flow := strings.ToLower(get("flowDesc"))
	switch flow {
	case "import", "imports", "m":
		isImport = true
	case "export", "exports", "x":
		isImport = false
	default:
		return TradeFlowRecord{}, false, fmt.Sprintf("unsupported flow %q (want Import or Export)", get("flowDesc"))
	}

	reporterISO := strings.ToUpper(get("reporterISO"))
	partnerISO := strings.ToUpper(get("partnerISO"))
	cmdCode := get("cmdCode")
	primaryRaw := get("primaryValue")

	if reporterISO == "" || partnerISO == "" {
		return TradeFlowRecord{}, isImport, "missing reporterISO or partnerISO"
	}
	if cmdCode == "" {
		return TradeFlowRecord{}, isImport, "missing cmdCode"
	}
	if primaryRaw == "" {
		return TradeFlowRecord{}, isImport, "missing primaryValue"
	}

	yearStr := get("refYear")
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return TradeFlowRecord{}, isImport, fmt.Sprintf("invalid refYear %q", yearStr)
	}
	if year <= 0 {
		return TradeFlowRecord{}, isImport, fmt.Sprintf("non-positive refYear %d", year)
	}

	value, vreason := parseAmount(primaryRaw, "primaryValue")
	if vreason != "" {
		return TradeFlowRecord{}, isImport, vreason
	}
	quantity, qreason := parseAmount(get("qty"), "qty")
	if qreason != "" {
		return TradeFlowRecord{}, isImport, qreason
	}

	reporterName := NormalizeCountryName(get("reporterDesc"))
	if reporterName == "" {
		reporterName = get("reporterDesc")
	}
	partnerName := NormalizeCountryName(get("partnerDesc"))
	if partnerName == "" {
		partnerName = get("partnerDesc")
	}

	var exporterCode, exporterName, importerCode, importerName string
	if isImport {
		importerCode, importerName = reporterISO, reporterName
		exporterCode, exporterName = partnerISO, partnerName
	} else {
		exporterCode, exporterName = reporterISO, reporterName
		importerCode, importerName = partnerISO, partnerName
	}
	exporterCode = ResolveCountryCode(exporterCode, exporterName)
	importerCode = ResolveCountryCode(importerCode, importerName)

	commodityName := normalizeComtradeCommodity(get("cmdDesc"), cmdCode)

	return TradeFlowRecord{
		Year:          year,
		ExporterCode:  exporterCode,
		ExporterName:  exporterName,
		ImporterCode:  importerCode,
		ImporterName:  importerName,
		CommodityCode: cmdCode,
		CommodityName: commodityName,
		TradeValueUSD: value,
		Quantity:      quantity,
		Unit:          get("qtyUnitAbbr"),
		Source:        ComtradeSourceName,
		IngestedAt:    ingestedAt,
	}, isImport, ""
}

// normalizeComtradeCommodity maps a Comtrade commodity description (and HS code)
// onto the canonical commodity names AtlasGraph's graph and scenarios use, so
// downstream supplier-dependency and shock analysis line up with the curated
// sample data. Unmapped commodities fall back to a cleaned, lower-cased
// description so nothing is silently dropped.
func normalizeComtradeCommodity(cmdDesc, cmdCode string) string {
	desc := strings.ToLower(strings.TrimSpace(cmdDesc))
	code := strings.TrimSpace(cmdCode)

	switch {
	case strings.Contains(desc, "electronic integrated circuits") || code == "8542":
		return "semiconductors"
	case strings.Contains(desc, "lithium") || strings.Contains(desc, "batteries"):
		return "lithium batteries"
	case strings.Contains(desc, "cobalt"):
		return "cobalt ores"
	case strings.Contains(desc, "petroleum oils") || strings.Contains(desc, "crude"):
		return "crude oil"
	case strings.Contains(desc, "rare earth"):
		return "rare earths"
	default:
		return cleanCommodityDesc(cmdDesc)
	}
}

// cleanCommodityDesc lower-cases a description and collapses internal whitespace
// so unmapped commodities read consistently.
func cleanCommodityDesc(desc string) string {
	return strings.ToLower(strings.Join(strings.Fields(desc), " "))
}
