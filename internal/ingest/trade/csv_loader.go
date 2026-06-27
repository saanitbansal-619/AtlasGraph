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

// SkippedRow records a data row that could not be normalised, with a 1-based
// line number (matching the source file) and a human-readable reason.
type SkippedRow struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// LoadResult is the outcome of parsing a trade CSV: the normalised records plus
// counts that let the ingest command report exactly what happened.
type LoadResult struct {
	Records  []TradeFlowRecord
	TotalRows int          // data rows seen (excludes the header)
	Skipped  []SkippedRow // malformed rows that were dropped, with reasons
}

// ValidRows is the number of rows that normalised successfully.
func (r LoadResult) ValidRows() int { return len(r.Records) }

// LoadFile opens a CSV path and parses it. sourceFile is recorded as provenance.
func LoadFile(path string) (LoadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return LoadResult{}, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()
	return ParseCSV(f, time.Now().UTC())
}

// ParseCSV reads a trade CSV from r and normalises it. It validates that every
// required column is present, then parses each data row, collecting malformed
// rows (with reasons) rather than aborting the whole file. An error is returned
// only for problems that make the file unusable (unreadable, empty, or missing
// required columns).
func ParseCSV(r io.Reader, ingestedAt time.Time) (LoadResult, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // tolerate ragged rows; we validate per row
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err == io.EOF {
		return LoadResult{}, fmt.Errorf("trade CSV is empty")
	}
	if err != nil {
		return LoadResult{}, fmt.Errorf("reading header: %w", err)
	}

	index, err := mapColumns(header)
	if err != nil {
		return LoadResult{}, err
	}

	var res LoadResult
	line := 1 // header consumed on line 1; data rows start at line 2
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			return LoadResult{}, fmt.Errorf("reading line %d: %w", line, err)
		}
		if isBlankRow(row) {
			continue
		}
		res.TotalRows++

		rec, reason := normalizeRow(row, index, ingestedAt)
		if reason != "" {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: reason})
			continue
		}
		res.Records = append(res.Records, rec)
	}
	return res, nil
}

// mapColumns builds a header-name -> column-index map and verifies that every
// RequiredColumns entry is present. Header names are matched case-insensitively.
func mapColumns(header []string) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for i, h := range header {
		index[strings.ToLower(strings.TrimSpace(h))] = i
	}
	var missing []string
	for _, col := range RequiredColumns {
		if _, ok := index[col]; !ok {
			missing = append(missing, col)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("trade CSV is missing required column(s): %s", strings.Join(missing, ", "))
	}
	return index, nil
}

// normalizeRow turns one raw CSV row into a TradeFlowRecord. It returns a
// non-empty reason string when the row is malformed and should be skipped.
func normalizeRow(row []string, index map[string]int, ingestedAt time.Time) (TradeFlowRecord, string) {
	get := func(col string) string {
		i := index[col]
		if i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	yearStr := get("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return TradeFlowRecord{}, fmt.Sprintf("invalid year %q", yearStr)
	}
	if year <= 0 {
		return TradeFlowRecord{}, fmt.Sprintf("non-positive year %d", year)
	}

	exporterCode := strings.ToUpper(get("exporter_code"))
	importerCode := strings.ToUpper(get("importer_code"))
	commodityName := get("commodity_name")
	if exporterCode == "" || importerCode == "" {
		return TradeFlowRecord{}, "missing exporter_code or importer_code"
	}
	if commodityName == "" {
		return TradeFlowRecord{}, "missing commodity_name"
	}

	value, reason := parseAmount(get("trade_value_usd"), "trade_value_usd")
	if reason != "" {
		return TradeFlowRecord{}, reason
	}
	quantity, reason := parseAmount(get("quantity"), "quantity")
	if reason != "" {
		return TradeFlowRecord{}, reason
	}

	return TradeFlowRecord{
		Year:          year,
		ExporterCode:  exporterCode,
		ExporterName:  get("exporter_name"),
		ImporterCode:  importerCode,
		ImporterName:  get("importer_name"),
		CommodityCode: get("commodity_code"),
		CommodityName: commodityName,
		TradeValueUSD: value,
		Quantity:      quantity,
		Unit:          get("unit"),
		Source:        SourceName,
		IngestedAt:    ingestedAt,
	}, ""
}

// parseAmount parses a non-negative numeric field, tolerating thousands commas
// and a leading currency marker. It returns a reason string when invalid.
func parseAmount(raw, field string) (float64, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		s = "0"
	}
	s = strings.ReplaceAll(s, ",", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Sprintf("invalid %s %q", field, raw)
	}
	if v < 0 {
		return 0, fmt.Sprintf("negative %s %q", field, raw)
	}
	return v, ""
}

func isBlankRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
