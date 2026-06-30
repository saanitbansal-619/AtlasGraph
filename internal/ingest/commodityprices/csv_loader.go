package commodityprices

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

// LoadResult is the outcome of parsing a commodity price CSV: the normalised
// records plus counts that let the ingest command report exactly what happened.
type LoadResult struct {
	Records   []PriceRecord
	TotalRows int          // data rows seen (excludes the header)
	Skipped   []SkippedRow // malformed rows that were dropped, with reasons
}

// ValidRows is the number of rows that normalised successfully.
func (r LoadResult) ValidRows() int { return len(r.Records) }

// dateLayouts are the accepted input date formats; all are normalised to
// "YYYY-MM" on output.
var dateLayouts = []string{"2006-01", "2006-01-02", "2006/01", "2006/01/02"}

// LoadFile opens a CSV path and parses it.
func LoadFile(path string) (LoadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return LoadResult{}, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()
	return ParseCSV(f)
}

// ParseCSV reads a commodity price CSV from r and normalises it. It validates
// that every required column is present, then parses each data row, collecting
// malformed rows (with reasons) rather than aborting the whole file. An error is
// returned only for problems that make the file unusable (unreadable, empty, or
// missing required columns).
func ParseCSV(r io.Reader) (LoadResult, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // tolerate ragged rows; we validate per row
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err == io.EOF {
		return LoadResult{}, fmt.Errorf("commodity price CSV is empty")
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

		rec, reason := normalizeRow(row, index)
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
		return nil, fmt.Errorf("commodity price CSV is missing required column(s): %s", strings.Join(missing, ", "))
	}
	return index, nil
}

// normalizeRow turns one raw CSV row into a PriceRecord. It returns a non-empty
// reason string when the row is malformed and should be skipped.
func normalizeRow(row []string, index map[string]int) (PriceRecord, string) {
	get := func(col string) string {
		i := index[col]
		if i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	date, ok := normalizeDate(get("date"))
	if !ok {
		return PriceRecord{}, fmt.Sprintf("invalid date %q (want YYYY-MM)", get("date"))
	}

	code := normalizeCode(get("commodity_code"))
	name := get("commodity_name")
	if code == "" {
		return PriceRecord{}, "missing commodity_code"
	}
	if name == "" {
		return PriceRecord{}, "missing commodity_name"
	}

	price, reason := parsePrice(get("price_usd"))
	if reason != "" {
		return PriceRecord{}, reason
	}

	source := get("source")
	if source == "" {
		source = SourceName
	}

	return PriceRecord{
		Date:          date,
		CommodityCode: code,
		CommodityName: name,
		PriceUSD:      price,
		Unit:          get("unit"),
		Source:        source,
	}, ""
}

// normalizeDate accepts a few common layouts and reduces them to "YYYY-MM".
func normalizeDate(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01"), true
		}
	}
	return "", false
}

// normalizeCode lower-cases a commodity code and collapses spaces and hyphens to
// underscores so codes are stable across files (e.g. "Crude Oil" -> "crude_oil").
func normalizeCode(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

// parsePrice parses a strictly-positive price, tolerating thousands commas and a
// leading currency marker. It returns a reason string when invalid.
func parsePrice(raw string) (float64, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, "missing price_usd"
	}
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Sprintf("invalid price_usd %q", raw)
	}
	if v <= 0 {
		return 0, fmt.Sprintf("non-positive price_usd %q", raw)
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
